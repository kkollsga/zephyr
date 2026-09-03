package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/config"
	"github.com/kristianweb/zephyr/internal/editor"
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
	st.saveMenu.confirmOverwrite = false
	st.saveMenu.confirmClobber = false
	st.saveMenu.tags = [7]bool{}

	// Pre-populate Save As fields so they're ready when expanded/shown
	st.populateSaveAsFields(idx)
	st.switchTab(idx)

	// For untitled tabs the Save As rows are always visible (no toggle needed)
	if tab.Editor.FilePath == "" {
		st.saveMenu.saveAsExpanded = false // not used; saveMenuShowSaveAs checks FilePath
	}
}

// showSaveAsMenu opens the save menu with Save As rows already expanded. A
// HEAD view has nothing to save under any name — the buffer is the file's
// history — so the menu is refused rather than opened onto a write that will
// be refused at the end.
func (st *appState) showSaveAsMenu(idx int, closeAfter, forQuit bool) {
	if idx >= 0 && idx < len(st.tabBar.Tabs) && st.headViewRefusesSave(st.tabBar.Tabs[idx]) {
		st.notify("HEAD view is read-only — press go to return to the file", 5*time.Second)
		return
	}
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

	// A buffer that already has a path keeps its own folder; only an untitled
	// buffer, which has no folder of its own, follows the last save.
	dir := ""
	switch {
	case tab.Editor.FilePath != "":
		dir = filepath.Dir(tab.Editor.FilePath)
	case st.lastSaveDir != "":
		dir = st.lastSaveDir
	default:
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
		st.saveMenu.confirmClobber = false
		st.quitInProgress = false
		return
	}

	idx := st.saveMenu.tabIdx
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		st.saveMenu.visible = false
		return
	}
	if st.saveMenu.confirmClobber {
		st.handleClobberClick(x, y, dy, dw, itemH)
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
				closeAfter, forQuit := st.saveMenu.closeAfterSave, st.saveMenu.forQuit
				st.saveMenu.visible = false
				if st.saveTabWithPrompt(tab, closeAfter, forQuit) {
					st.showSaveNotification(tab.Editor.FilePath)
					if closeAfter {
						st.forceCloseTab(idx)
					}
					if forQuit {
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

	tab := st.tabBar.Tabs[idx]
	if path == tab.Editor.FilePath && st.saveWouldClobber(tab) {
		// Saving As onto the tab's own changed file is the same clobber the
		// plain Save path refuses; "the file exists" is not the real question.
		st.raiseClobberPrompt(idx, st.saveMenu.closeAfterSave, st.saveMenu.forQuit)
		return
	}

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

// saveTab writes a tab to its file. It refuses and raises the clobber prompt
// when the file changed underneath the buffer, so a save can never silently
// destroy someone else's write.
func (st *appState) saveTab(tab *ui.Tab) bool {
	return st.saveTabWithPrompt(tab, false, false)
}

// saveTabWithPrompt is saveTab carrying the close-tab and quit flags of the
// flow that requested the save, so a refusal can hand them to the prompt.
func (st *appState) saveTabWithPrompt(tab *ui.Tab, closeAfter, forQuit bool) bool {
	if st.headViewRefusesSave(tab) {
		st.notify("HEAD view is read-only — press go to return to the file", 5*time.Second)
		return false
	}
	if tab.Editor.FilePath == "" {
		return st.saveTabAs(tab)
	}
	if st.saveWouldClobber(tab) {
		// A save asked for while this tab's compare overlay is up arrives with
		// no flags of its own — Cmd+S goes through saveTab. The prompt has to
		// come back inside the flow that raised it, so the flags Compare
		// stashed are OR-ed in: dropping them leaves quitInProgress standing
		// with nothing on screen to resolve it, after which Cmd+Q and the
		// window's close button do nothing at all.
		if ts := st.tabStates[tab.Editor]; ts != nil && ts.compareDiff != nil {
			closeAfter = closeAfter || ts.compareCloseAfter
			forQuit = forQuit || ts.compareForQuit
		}
		st.raiseClobberPrompt(st.tabIndexOf(tab), closeAfter, forQuit)
		return false
	}
	return st.forceSaveTab(tab)
}

// forceSaveTab writes the buffer to its file without the conflict check. Only
// the guard above and the prompt's explicit Overwrite may call it.
func (st *appState) forceSaveTab(tab *ui.Tab) bool {
	if st.headViewRefusesSave(tab) {
		return false
	}
	if tab.Editor.FilePath == "" {
		return st.saveTabAs(tab)
	}
	// Suppress the watcher events our own atomic save is about to generate.
	if st.watcher != nil && tab.Editor.FilePath != "" {
		st.watcher.MarkOwnWrite(tab.Editor.FilePath)
	}
	if err := tab.Editor.Save(); err != nil {
		if st.watcher != nil {
			st.watcher.CancelOwnWrite(tab.Editor.FilePath)
		}
		fmt.Fprintf(os.Stderr, "save error: %v\n", err)
		return false
	}
	// Ends the suppression window opened above once the queued self-events have
	// settled. Directory watching survives the rename, so nothing reattaches.
	if st.watcher != nil {
		if err := st.watcher.Rewatch(tab.Editor.FilePath); err != nil {
			fmt.Fprintf(os.Stderr, "rewatch error: %v\n", err)
		}
	}
	// The snapshot must be taken after the write, not the suppression window:
	// an external write that lands inside that window is invisible to the
	// watcher and only the next snapshot comparison will surface it.
	st.snapshotEditorFile(tab.Editor)
	st.clearConflict(tab)
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

// refreshGitDiffAllTabs reloads every open tab's diff. Used when the git cache
// appears after the tabs do.
func (st *appState) refreshGitDiffAllTabs() {
	for _, tab := range st.tabBar.Tabs {
		st.refreshGitDiffForEditor(tab.Editor)
	}
}

// saveTabToPath is the funnel every Save As lands in, so the HEAD-view refusal
// sits here rather than at each entry point: a write of committed content is
// data loss whichever menu, key or prompt asked for it.
func (st *appState) saveTabToPath(tab *ui.Tab, path string) bool {
	if st.headViewRefusesSave(tab) {
		st.notify("HEAD view is read-only — press go to return to the file", 5*time.Second)
		return false
	}
	oldPath := tab.Editor.FilePath
	if err := tab.Editor.SaveAs(path); err != nil {
		fmt.Fprintf(os.Stderr, "save error: %v\n", err)
		return false
	}
	if st.watcher != nil && oldPath != "" && oldPath != tab.Editor.FilePath {
		if err := st.watcher.MoveWatch(oldPath, tab.Editor.FilePath); err != nil {
			fmt.Fprintf(os.Stderr, "watch transfer error: %v\n", err)
		}
	} else {
		st.watchEditorFile(tab.Editor)
	}
	tab.Title = filepath.Base(path)
	tab.IsUntitled = false

	ts := st.tabStates[tab.Editor]
	if ts != nil {
		ts.langLabel = detectLanguage(path)
		h := highlight.NewHighlighter(path)
		if ts.highlighter != nil {
			ts.highlighter.Close()
		}
		ts.highlighter = h
		if h != nil {
			source := tab.Editor.Buffer.TextBytes(ts.sourceBuf)
			ts.sourceBuf = source
			h.Parse(source)
		} else {
			ts.sourceBuf = nil
		}
		ts.conflict = conflictNone
		ts.deleteForcedModified = false
	}
	st.snapshotEditorFile(tab.Editor)
	st.refreshGitDiffForEditor(tab.Editor)
	st.rememberSaveDir(filepath.Dir(path))
	return true
}

// rememberSaveDir records where the last Save As landed, for the next untitled
// buffer's Save As folder, and persists it across launches.
func (st *appState) rememberSaveDir(dir string) {
	if dir == "" || dir == st.lastSaveDir {
		return
	}
	st.lastSaveDir = dir
	if st.persistConfig != nil {
		st.persistConfig(func(cfg *config.Config) { cfg.LastSaveDir = dir })
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
