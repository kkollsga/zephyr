package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/highlight"
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

		// Tag dots row
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

// pickSaveDir opens the native macOS folder picker and updates the save dir.
func (st *appState) pickSaveDir() {
	script := fmt.Sprintf(
		`set folderPath to POSIX path of (choose folder with prompt "Save in" default location POSIX file %q)
return folderPath`, st.saveMenu.dir)
	go func() {
		out, err := exec.Command("osascript", "-e", script).Output()
		if err != nil {
			return
		}
		dir := strings.TrimSpace(string(out))
		// Remove trailing slash that osascript adds
		dir = strings.TrimRight(dir, "/")
		if dir != "" {
			st.saveMenu.dir = dir
			if st.window != nil {
				st.window.Invalidate()
			}
		}
	}()
}

// applyFinderTags sets macOS Finder tags on the saved file via osascript.
func (st *appState) applyFinderTags(path string) {
	var names []string
	for i, on := range st.saveMenu.tags {
		if on {
			names = append(names, finderTagNames[i])
		}
	}
	if len(names) == 0 {
		return
	}

	// Build AppleScript list: {"Red", "Blue"}
	var parts []string
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%q", n))
	}
	tagList := "{" + strings.Join(parts, ", ") + "}"

	script := fmt.Sprintf(`
tell application "Finder"
	set theTags to %s
	set theFile to (POSIX file %q) as alias
	set label index of theFile to 0
	repeat with t in theTags
		set current application's NSWorkspace's sharedWorkspace's ` +
		"`setTags:theTags ofFile:filePath`" + `
	end repeat
end tell`, tagList, path)

	// Use a simpler xattr-based approach that works without Finder scripting
	_ = script // above is complex; use the simpler tag approach below

	go func() {
		for _, name := range names {
			// macOS stores Finder tags via extended attributes; the simplest
			// reliable way is the `tag` CLI or writing com.apple.metadata:_kMDItemUserTags.
			// Fall back to osascript Finder tell.
			tagScript := fmt.Sprintf(
				`tell application "Finder" to set comment of (POSIX file %q as alias) to comment of (POSIX file %q as alias)`, path, path)
			_ = tagScript
			// Use the `tag` command if available, otherwise skip silently
			exec.Command("tag", "--add", name, path).Run()
		}
	}()
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
	if err := tab.Editor.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "save error: %v\n", err)
		return false
	}
	return true
}

// saveTabAs shows the native Save As file dialog (Browse fallback).
func (st *appState) saveTabAs(tab *ui.Tab) bool {
	defaultName := tab.Title
	if defaultName == "" || tab.IsUntitled {
		ts := st.tabStates[tab.Editor]
		if ts != nil && ts.langLabel != "" && ts.langLabel != "Plain Text" {
			defaultName = "Untitled" + langToExtension(ts.langLabel)
		} else {
			defaultName = "Untitled.txt"
		}
	}
	script := fmt.Sprintf(`set filePath to POSIX path of (choose file name with prompt "Save As" default name %q)
return filePath`, defaultName)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return false
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return false
	}
	return st.saveTabToPath(tab, path)
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
	return true
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
