package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/highlight"
	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/ui"
)

// --- Opening the save menu ---

// startQuitFlow begins the quit sequence by showing a save prompt for the
// first unsaved tab. If no tabs are unsaved, exits immediately.
func (st *appState) startQuitFlow() {
	st.quitInProgress = true
	for i, tab := range st.tabBar.Tabs {
		if tab.Editor.Modified {
			st.showSaveMenu(i, true, true)
			return
		}
	}
	st.gracefulExit()
}

// showSaveMenu opens the save menu for a tab. For file-backed tabs the menu
// starts collapsed (Save + toggle). For untitled tabs the Save As rows are
// always visible.
func (st *appState) showSaveMenu(idx int, closeAfter, forQuit bool) {
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return
	}
	tab := st.tabBar.Tabs[idx]

	st.saveMenu.visible = true
	st.saveMenu.tabIdx = idx
	st.saveMenu.closeAfterSave = closeAfter
	st.saveMenu.forQuit = forQuit
	st.saveMenu.saveAsExpanded = false
	st.saveMenu.tags = [7]bool{}

	// Pre-populate Save As fields so they're ready when expanded/shown
	st.populateSaveAsFields(idx)
	st.switchTab(idx)

	// For untitled tabs the Save As rows are always visible (no toggle needed)
	if tab.Editor.FilePath == "" {
		st.saveMenu.saveAsExpanded = false // not used; saveMenuShowSaveAs checks FilePath
	}
}

// showSaveAsMenu opens the save menu with Save As rows already expanded.
func (st *appState) showSaveAsMenu(idx int, closeAfter, forQuit bool) {
	st.showSaveMenu(idx, closeAfter, forQuit)
	st.saveMenu.saveAsExpanded = true
}

// populateSaveAsFields sets the filename, cursor, and directory for Save As.
func (st *appState) populateSaveAsFields(idx int) {
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return
	}
	tab := st.tabBar.Tabs[idx]

	defaultName := tab.Title
	if defaultName == "" || tab.IsUntitled {
		ts := st.tabStates[tab.Editor]
		if ts != nil && ts.langLabel != "" && ts.langLabel != "Plain Text" {
			defaultName = "Untitled" + langToExtension(ts.langLabel)
		} else {
			defaultName = "Untitled.txt"
		}
	}

	dir := ""
	if tab.Editor.FilePath != "" {
		dir = filepath.Dir(tab.Editor.FilePath)
	} else {
		dir, _ = os.UserHomeDir()
	}

	runes := []rune(defaultName)
	st.saveMenu.filename = runes
	st.saveMenu.cursor = len(runes)
	st.saveMenu.selectAll = true
	st.saveMenu.dir = dir
}

// --- Handling clicks ---

// handleSaveMenuClick processes a click while the save menu is visible.
// Row order: [Name, Tag, Where] (if showSaveAs) → Save row → Discard/Cancel.
func (st *appState) handleSaveMenuClick(x, y int) {
	dx, dy, dw, dropdownH, itemH := st.saveMenuRect()
	if itemH == 0 {
		return
	}

	// Click outside → cancel
	if x < dx || x >= dx+dw || y < dy || y >= dy+dropdownH {
		st.saveMenu.visible = false
		st.quitInProgress = false
		return
	}

	idx := st.saveMenu.tabIdx
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		st.saveMenu.visible = false
		return
	}
	tab := st.tabBar.Tabs[idx]
	fileBacked := tab.Editor.FilePath != ""
	showSaveAs := st.saveMenuShowSaveAs()

	curY := dy
	halfW := dw / 2
	tr := st.tabRend

	// Save As detail rows (Name, Tag, Where)
	if showSaveAs {
		labelW := 6 * tr.CharWidth
		fieldX := dx + 8 + labelW + 4

		// Name input row
		if y >= curY && y < curY+itemH {
			if tr != nil {
				textX := fieldX + 4
				runePos := (x - textX + tr.CharWidth/2) / tr.CharWidth
				if runePos < 0 {
					runePos = 0
				}
				if runePos > len(st.saveMenu.filename) {
					runePos = len(st.saveMenu.filename)
				}
				st.saveMenu.cursor = runePos
				st.saveMenu.selectAll = false
			}
			return
		}
		curY += itemH

		// Tag dots row (macOS Finder tags only)
		if platformHasFinderTags() {
			if y >= curY && y < curY+itemH {
				dotSize := tr.LineHeightPx - 2
				dotGap := 4
				dotX := fieldX
				for ti := 0; ti < 7; ti++ {
					if x >= dotX && x < dotX+dotSize {
						st.saveMenu.tags[ti] = !st.saveMenu.tags[ti]
						return
					}
					dotX += dotSize + dotGap
				}
				return
			}
			curY += itemH
		}

		// Where directory row
		if y >= curY && y < curY+itemH {
			st.pickSaveDir()
			return
		}
		curY += itemH
	}

	// Save As radio toggle row (file-backed only)
	if fileBacked {
		if y >= curY && y < curY+itemH {
			st.saveMenu.saveAsExpanded = !st.saveMenu.saveAsExpanded
			if st.saveMenu.saveAsExpanded {
				st.populateSaveAsFields(idx)
			}
			st.saveMenu.confirmOverwrite = false
			return
		}
		curY += itemH
	}

	// Overwrite confirmation rows
	if st.saveMenu.confirmOverwrite {
		// Warning text row (not clickable)
		curY += itemH

		// Overwrite / Back split row
		if y >= curY && y < curY+itemH {
			if x < dx+halfW {
				st.saveMenu.confirmOverwrite = false
				st.forceExecuteSaveAs()
			} else {
				st.saveMenu.confirmOverwrite = false
			}
			return
		}
		curY += itemH
	}

	// Bottom row: Save | Discard | Cancel (3-way split)
	if y >= curY && y < curY+itemH {
		thirdW := dw / 3
		if x < dx+thirdW {
			// Save
			if !st.saveMenuCanSave() {
				return
			}
			if showSaveAs {
				st.executeSaveAs()
			} else {
				st.saveMenu.visible = false
				if st.saveTab(tab) {
					st.showSaveNotification(tab.Editor.FilePath)
					if st.saveMenu.closeAfterSave {
						st.forceCloseTab(idx)
					}
					if st.saveMenu.forQuit {
						st.continueQuitFlow()
					}
				}
				st.updateWindowTitle()
			}
		} else if x < dx+thirdW*2 {
			// Discard
			st.saveMenu.visible = false
			if st.saveMenu.closeAfterSave {
				st.forceCloseTab(idx)
			}
			if st.saveMenu.forQuit {
				st.continueQuitFlow()
			}
			st.updateWindowTitle()
		} else {
			// Cancel
			st.saveMenu.visible = false
			st.quitInProgress = false
		}
	}
}

// --- Save execution ---

// executeSaveAs checks if the target file exists and, if so, asks for
// overwrite confirmation. Otherwise it saves immediately.
func (st *appState) executeSaveAs() {
	idx := st.saveMenu.tabIdx
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		st.saveMenu.visible = false
		return
	}
	filename := strings.TrimSpace(string(st.saveMenu.filename))
	if filename == "" {
		return
	}

	path := filepath.Join(st.saveMenu.dir, filename)

	// Check if the target file already exists
	if _, err := os.Stat(path); err == nil {
		// File exists — ask for confirmation
		st.saveMenu.confirmOverwrite = true
		return
	}

	st.forceExecuteSaveAs()
}

// forceExecuteSaveAs saves without checking for existing files (used after
// overwrite confirmation or when the target is known to be new).
func (st *appState) forceExecuteSaveAs() {
	idx := st.saveMenu.tabIdx
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		st.saveMenu.visible = false
		return
	}
	filename := strings.TrimSpace(string(st.saveMenu.filename))
	if filename == "" {
		return
	}

	tab := st.tabBar.Tabs[idx]
	path := filepath.Join(st.saveMenu.dir, filename)
	closeAfter := st.saveMenu.closeAfterSave
	forQuit := st.saveMenu.forQuit

	if !st.saveTabToPath(tab, path) {
		return
	}

	// Apply Finder tags after successful save
	st.applyFinderTags(path)

	st.showSaveNotification(path)
	st.saveMenu.visible = false
	if closeAfter {
		st.forceCloseTab(idx)
		if forQuit {
			st.continueQuitFlow()
		}
	}
	st.updateWindowTitle()
}

// showSaveNotification shows a "Saved to: ..." message in the footer for 10s.
func (st *appState) showSaveNotification(path string) {
	st.notification = "Saved to: " + shortenDir(path)
	st.notificationUntil = time.Now().Add(10 * time.Second)
}

// continueQuitFlow checks for more unsaved tabs after one was handled.
func (st *appState) continueQuitFlow() {
	for i, tab := range st.tabBar.Tabs {
		if tab.Editor.Modified {
			st.showSaveMenu(i, true, true)
			return
		}
	}
	st.gracefulExit()
}


// --- Save As text input helpers ---

func (st *appState) saveAsInsertText(text string) {
	if st.saveMenu.selectAll {
		st.saveMenu.filename = []rune(text)
		st.saveMenu.cursor = utf8.RuneCountInString(text)
		st.saveMenu.selectAll = false
		return
	}
	runes := []rune(text)
	fn := st.saveMenu.filename
	c := st.saveMenu.cursor
	newFn := make([]rune, 0, len(fn)+len(runes))
	newFn = append(newFn, fn[:c]...)
	newFn = append(newFn, runes...)
	newFn = append(newFn, fn[c:]...)
	st.saveMenu.filename = newFn
	st.saveMenu.cursor = c + len(runes)
}

func (st *appState) saveAsDeleteBack() {
	if st.saveMenu.selectAll {
		st.saveMenu.filename = nil
		st.saveMenu.cursor = 0
		st.saveMenu.selectAll = false
		return
	}
	if st.saveMenu.cursor > 0 {
		fn := st.saveMenu.filename
		st.saveMenu.filename = append(fn[:st.saveMenu.cursor-1], fn[st.saveMenu.cursor:]...)
		st.saveMenu.cursor--
	}
}

func (st *appState) saveAsDeleteForward() {
	if st.saveMenu.selectAll {
		st.saveMenu.filename = nil
		st.saveMenu.cursor = 0
		st.saveMenu.selectAll = false
		return
	}
	fn := st.saveMenu.filename
	if st.saveMenu.cursor < len(fn) {
		st.saveMenu.filename = append(fn[:st.saveMenu.cursor], fn[st.saveMenu.cursor+1:]...)
	}
}

// --- Shared save helpers ---

func (st *appState) saveTab(tab *ui.Tab) bool {
	if tab.Editor.FilePath == "" {
		return st.saveTabAs(tab)
	}
	// Mark own write so the watcher ignores this save
	if st.watcher != nil && tab.Editor.FilePath != "" {
		st.watcher.MarkOwnWrite(tab.Editor.FilePath)
	}
	if err := tab.Editor.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "save error: %v\n", err)
		return false
	}
	// Re-watch (inode may change from atomic rename)
	st.watchEditorFile(tab.Editor)
	st.refreshGitDiffForEditor(tab.Editor)
	return true
}

// refreshGitDiffForEditor reloads the git diff for a file after save.
func (st *appState) refreshGitDiffForEditor(ed *editor.Editor) {
	if st.gitCache == nil || st.gitRepo == nil || ed == nil || ed.FilePath == "" {
		return
	}
	relPath, err := filepath.Rel(st.gitRepo.Root, ed.FilePath)
	if err != nil {
		return
	}
	st.gitCache.InvalidateFile(relPath)
	diff, _ := st.gitCache.FileDiff(relPath)
	if ts, ok := st.tabStates[ed]; ok {
		ts.gitDiff = diff
	}
}


func (st *appState) saveTabToPath(tab *ui.Tab, path string) bool {
	if err := tab.Editor.SaveAs(path); err != nil {
		fmt.Fprintf(os.Stderr, "save error: %v\n", err)
		return false
	}
	tab.Title = filepath.Base(path)
	tab.IsUntitled = false

	ts := st.tabStates[tab.Editor]
	if ts != nil {
		ts.langLabel = detectLanguage(path)
		h := highlight.NewHighlighter(path)
		if h != nil {
			if ts.highlighter != nil {
				ts.highlighter.Close()
			}
			ts.highlighter = h
			source := tab.Editor.Buffer.TextBytes(ts.sourceBuf)
			ts.sourceBuf = source
			h.Parse(source)
		}
	}
	st.refreshGitDiffForEditor(tab.Editor)
	return true
}

// --- File watcher ---

// watchEditorFile adds a file to the watcher.
func (st *appState) watchEditorFile(ed *editor.Editor) {
	if st.watcher != nil && ed != nil && ed.FilePath != "" {
		st.watcher.Watch(ed.FilePath)
	}
}

// unwatchEditorFile removes a file from the watcher.
func (st *appState) unwatchEditorFile(ed *editor.Editor) {
	if st.watcher != nil && ed != nil && ed.FilePath != "" {
		st.watcher.Unwatch(ed.FilePath)
	}
}

// pollFileWatcher drains pending file watcher events (non-blocking).
func (st *appState) pollFileWatcher() {
	if st.watcher == nil {
		return
	}
	handled := false
	for {
		select {
		case evt, ok := <-st.watcher.Events:
			if !ok {
				return
			}
			st.handleExternalFileChange(evt.Path)
			handled = true
		default:
			if handled && st.window != nil {
				st.window.Invalidate()
			}
			return
		}
	}
}

// handleExternalFileChange processes a file change detected by the watcher.
func (st *appState) handleExternalFileChange(path string) {
	for _, tab := range st.tabBar.Tabs {
		if tab.Editor.FilePath != path {
			continue
		}
		if tab.Editor.Modified {
			// Has unsaved edits — warn but don't reload
			st.notification = "File changed externally: " + filepath.Base(path)
			st.notificationUntil = time.Now().Add(10 * time.Second)
		} else {
			// No unsaved edits — reload silently
			st.reloadEditorFromDisk(tab.Editor)
			st.notification = "Reloaded: " + filepath.Base(path)
			st.notificationUntil = time.Now().Add(5 * time.Second)
		}
		st.refreshGitDiffForEditor(tab.Editor)
		break
	}
}

// reloadEditorFromDisk reloads a file from disk, re-parses syntax, and refreshes state.
func (st *appState) reloadEditorFromDisk(ed *editor.Editor) {
	if err := ed.Reload(); err != nil {
		return
	}
	if ts, ok := st.tabStates[ed]; ok {
		// Re-parse highlighting
		if ts.highlighter != nil {
			ts.sourceBuf = ed.Buffer.TextBytes(ts.sourceBuf)
			ts.highlighter.Parse(ts.sourceBuf)
		}
		// Recompute fold regions
		source := ed.Buffer.TextBytes(nil)
		regions := render.ComputeFoldRegions(string(source))
		ts.foldState.SetRegions(regions, ed.Buffer.LineCount())
		// Reset viewport
		ts.lastCursorLine = -1
		ts.lastCursorCol = -1
	}
	// Re-watch the file (inode may have changed from atomic rename)
	if st.watcher != nil {
		st.watcher.Watch(ed.FilePath)
	}
}

func (st *appState) hasUnsavedChanges() bool {
	for _, tab := range st.tabBar.Tabs {
		if tab.Editor.Modified {
			return true
		}
	}
	return false
}

func shortenDir(dir string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(dir, home) {
		return "~" + dir[len(home):]
	}
	return dir
}
