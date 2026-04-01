package main

import (
	"path/filepath"
	"strings"

	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/git"
	"github.com/kristianweb/zephyr/internal/navigator"
	"github.com/kristianweb/zephyr/internal/vim"
)

// executeMdReadAction handles vim actions in markdown read mode.
// Only allows scrolling, search, and mode toggle — blocks all editing.
func (st *appState) executeMdReadAction(action vim.Action, ts *tabState) {
	lineH := 0
	if st.textRend != nil {
		lineH = st.textRend.LineHeightPx
	}
	if lineH <= 0 {
		lineH = 20
	}
	editorH := st.lastMaxY - st.tabBarHeight

	switch action.Kind {
	// Scroll: j/k
	case vim.ActionMoveDown:
		count := action.EffectiveCount()
		st.mdScroll(ts, float64(count*lineH), editorH)
	case vim.ActionMoveUp:
		count := action.EffectiveCount()
		st.mdScroll(ts, float64(-count*lineH), editorH)

	// Half-page: Ctrl+d / Ctrl+u
	case vim.ActionMoveHalfPageDown:
		st.mdScroll(ts, float64(editorH/2), editorH)
	case vim.ActionMoveHalfPageUp:
		st.mdScroll(ts, float64(-editorH/2), editorH)

	// Full page: Ctrl+f / Ctrl+b
	case vim.ActionMovePageDown:
		st.mdScroll(ts, float64(editorH), editorH)
	case vim.ActionMovePageUp:
		st.mdScroll(ts, float64(-editorH), editorH)

	// Top/bottom: gg / G
	case vim.ActionMoveFileStart:
		ts.mdScrollY = 0
		st.window.Invalidate()
	case vim.ActionMoveFileEnd:
		maxScroll := float64(ts.mdTotalH - editorH)
		if maxScroll < 0 {
			maxScroll = 0
		}
		ts.mdScrollY = maxScroll
		st.window.Invalidate()

	// Search
	case vim.ActionEnterSearch:
		st.openFindBar(false)
	case vim.ActionEnterSearchBack:
		st.openFindBar(false)
	case vim.ActionSearchNext, vim.ActionSearchPrev, vim.ActionSearchWordUnder:
		// Let the normal vim action handle search navigation
		st.executeVimAction(action)

	// Enter command mode (for :q etc.)
	case vim.ActionEnterCommand:
		// Allow — the command line handler will process it

	// Execute command (Enter in command mode)
	case vim.ActionExecCommand:
		st.executeVimAction(action)

	// Toggle read/edit mode
	case vim.ActionNavToggleReadMode:
		st.toggleMarkdownPreview()

	// Close
	case vim.ActionNavCloseSpecial:
		st.closeTabAt(st.tabBar.ActiveIdx)

	// Everything else (insert, delete, visual, etc.) — silently ignore
	default:
		// Check if it's a navigator action that should still work
		if action.Kind == vim.ActionNavOpenRoot ||
			action.Kind == vim.ActionNavOpenStatus ||
			action.Kind == vim.ActionNavOpenParent {
			st.executeNavAction(action)
		}
	}
}

// mdScroll scrolls the markdown read view by the given pixel delta, clamping to bounds.
func (st *appState) mdScroll(ts *tabState, delta float64, editorH int) {
	ts.mdScrollY += delta
	if ts.mdScrollY < 0 {
		ts.mdScrollY = 0
	}
	maxScroll := float64(ts.mdTotalH - editorH)
	if maxScroll < 0 {
		maxScroll = 0
	}
	if ts.mdScrollY > maxScroll {
		ts.mdScrollY = maxScroll
	}
	st.window.Invalidate()
}

// handleNavRootDropdownClick handles a click when the root dropdown is open.
// Clicking an item selects that root; clicking outside closes the dropdown.
func (st *appState) handleNavRootDropdownClick(x, y int) {
	tr := st.tabRend
	if tr == nil {
		st.navRootDropdown.open = false
		return
	}

	items := st.navRootDropdown.items
	itemCount := len(items) + 1 // +1 for "Open Folder..."
	itemH := tr.LineHeightPx + 8
	if itemH <= 0 {
		st.navRootDropdown.open = false
		return
	}
	dropW := 36 * tr.CharWidth
	if dropW <= 0 {
		st.navRootDropdown.open = false
		return
	}
	maxX := st.lastMaxX
	dropX := (maxX - dropW) / 2
	dropY := st.tabBarHeight
	dropH := itemCount * itemH

	// Click outside → close
	if x < dropX || x >= dropX+dropW || y < dropY || y >= dropY+dropH {
		st.navRootDropdown.open = false
		return
	}

	// Determine which item was clicked
	idx := (y - dropY) / itemH

	if idx < len(items) {
		// Recent root selected
		st.setNavRoot(items[idx])
	} else {
		// "Open Folder..." selected
		st.navRootDropdown.open = false
		st.pickNavRoot()
	}
}

// navNextHunk moves the cursor to the next changed line (hunk start).
func (st *appState) navNextHunk(ed *editor.Editor, count int) {
	ts := st.activeTabState()
	if ts == nil || ts.gitDiff == nil {
		return
	}
	if count <= 0 {
		count = 1
	}
	starts := ts.gitDiff.HunkStartLines()
	if len(starts) == 0 {
		return
	}
	currentLine := ed.Cursor.Line + 1 // convert to 1-based

	for c := 0; c < count; c++ {
		found := false
		for _, start := range starts {
			if start > currentLine {
				currentLine = start
				found = true
				break
			}
		}
		if !found {
			// Wrap to first hunk
			currentLine = starts[0]
		}
	}
	ed.Cursor.SetPosition(ed.Buffer, currentLine-1, 0)
}

// navPrevHunk moves the cursor to the previous changed line (hunk start).
func (st *appState) navPrevHunk(ed *editor.Editor, count int) {
	ts := st.activeTabState()
	if ts == nil || ts.gitDiff == nil {
		return
	}
	if count <= 0 {
		count = 1
	}
	starts := ts.gitDiff.HunkStartLines()
	if len(starts) == 0 {
		return
	}
	currentLine := ed.Cursor.Line + 1

	for c := 0; c < count; c++ {
		found := false
		for i := len(starts) - 1; i >= 0; i-- {
			if starts[i] < currentLine {
				currentLine = starts[i]
				found = true
				break
			}
		}
		if !found {
			// Wrap to last hunk
			currentLine = starts[len(starts)-1]
		}
	}
	ed.Cursor.SetPosition(ed.Buffer, currentLine-1, 0)
}

// navToggleExplorer toggles between the file explorer and the previous file.
// When in a file: opens the file's directory.
// When in a directory buffer: returns to the previous file.
func (st *appState) navToggleExplorer() {
	ts := st.activeTabState()

	if ts != nil && ts.bufType == bufDirectory {
		// In directory buffer → switch back to previous tab
		idx := st.navPrevTabIdx
		if idx >= 0 && idx < len(st.tabBar.Tabs) {
			// Close the directory buffer tab
			dirIdx := st.tabBar.ActiveIdx
			st.tabBar.SwitchToTab(idx)
			// Adjust index if dir tab is before the target
			if dirIdx < idx {
				st.tabBar.ForceCloseTab(dirIdx)
			} else {
				st.tabBar.ForceCloseTab(dirIdx)
			}
		} else {
			st.navCloseSpecial()
		}
		return
	}

	// In a file → open directory buffer for the file's directory
	ed := st.activeEd()
	st.navPrevTabIdx = st.tabBar.ActiveIdx

	var dirPath string
	if ed != nil && ed.FilePath != "" {
		dirPath = filepath.Dir(ed.FilePath)
	} else if st.navRoot != "" {
		dirPath = st.navRoot
	} else {
		return
	}
	st.openDirBuffer(dirPath)
}

// navDirGoUp navigates to the parent directory, clamped at navRoot.
func (st *appState) navDirGoUp() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufDirectory || ts.dirBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil {
		return
	}

	// Don't go above root
	current := ts.dirBuf.DirPath
	if st.navRoot != "" && (current == st.navRoot || len(current) <= len(st.navRoot)) {
		return
	}

	parent := filepath.Dir(current)
	if parent == current {
		return // already at filesystem root
	}

	// Remember cursor position
	if st.navigator != nil {
		st.navigator.RememberCursor(current, ed.Cursor.Line)
	}

	st.openDirBuffer(parent)
}

// navOpenParent opens the parent directory as a buffer.
func (st *appState) navOpenParent() {
	ed := st.activeEd()
	if ed == nil {
		return
	}
	ts := st.activeTabState()

	var dirPath string
	if ts != nil && ts.bufType == bufDirectory && ts.dirBuf != nil {
		// Remember cursor position in current directory
		if st.navigator != nil {
			st.navigator.RememberCursor(ts.dirBuf.DirPath, ed.Cursor.Line)
		}
		dirPath = filepath.Dir(ts.dirBuf.DirPath)
	} else if ed.FilePath != "" {
		dirPath = filepath.Dir(ed.FilePath)
	} else {
		return
	}

	st.openDirBuffer(dirPath)
}

// openDirBuffer creates a directory buffer and opens it as a tab.
func (st *appState) openDirBuffer(dirPath string) {
	var repoRoot string
	if st.gitRepo != nil {
		repoRoot = st.gitRepo.Root
	}

	var statuses []git.FileStatus
	var diffStat map[string][2]int
	if st.gitCache != nil {
		statuses, _ = st.gitCache.Status()
		diffStat, _ = st.gitCache.DiffStat()
	}

	db := navigator.NewDirBuffer(dirPath, statuses, diffStat, repoRoot)
	content := db.GenerateText()

	newEd := editor.NewEditor(buffer.NewFromString(content), "")
	title := filepath.Base(dirPath) + "/"
	st.tabBar.OpenEditor(newEd, title)

	// Set up tab state for directory buffer
	ts := st.activeTabState()
	if ts != nil {
		ts.bufType = bufDirectory
		ts.dirBuf = db
	}

	// Recall cursor position
	if st.navigator != nil {
		line := st.navigator.RecallCursor(dirPath)
		if line >= 2 { // skip header lines
			newEd.Cursor.SetPosition(newEd.Buffer, line, 0)
		} else {
			newEd.Cursor.SetPosition(newEd.Buffer, 2, 0) // first entry
		}
	} else {
		newEd.Cursor.SetPosition(newEd.Buffer, 2, 0)
	}
}

// navOpenEntry opens the file or directory at the cursor in a directory buffer.
func (st *appState) navOpenEntry() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufDirectory || ts.dirBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil {
		return
	}

	entry := ts.dirBuf.EntryAtLine(ed.Cursor.Line)
	if entry == nil {
		return
	}

	// Remember cursor position
	if st.navigator != nil {
		st.navigator.RememberCursor(ts.dirBuf.DirPath, ed.Cursor.Line)
	}

	if entry.IsDir {
		st.openDirBuffer(entry.Path)
	} else {
		st.tabBar.OpenFile(entry.Path)
	}
}

// navCloseSpecial closes the current directory or status buffer.
func (st *appState) navCloseSpecial() {
	ts := st.activeTabState()
	if ts == nil {
		return
	}
	if ts.bufType == bufDirectory || ts.bufType == bufStatus {
		idx := st.tabBar.ActiveIdx
		st.tabBar.ForceCloseTab(idx)
	}
}

// navToggleHidden toggles hidden file visibility in directory buffer.
func (st *appState) navToggleHidden() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufDirectory || ts.dirBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil {
		return
	}

	ts.dirBuf.ShowHidden = !ts.dirBuf.ShowHidden
	var repoRoot string
	if st.gitRepo != nil {
		repoRoot = st.gitRepo.Root
	}
	var statuses []git.FileStatus
	var diffStat map[string][2]int
	if st.gitCache != nil {
		statuses, _ = st.gitCache.Status()
		diffStat, _ = st.gitCache.DiffStat()
	}
	ts.dirBuf.Refresh(statuses, diffStat, repoRoot)
	content := ts.dirBuf.GenerateText()
	ed.Buffer = buffer.NewFromString(content)
	ed.Cursor.SetPosition(ed.Buffer, 2, 0)
}

// handleDirBufferAction handles keybindings specific to directory buffers.
// Returns true if the action was consumed.
func (st *appState) handleDirBufferAction(action vim.Action) bool {
	switch action.Kind {
	case vim.ActionEnterKey, vim.ActionMoveRight:
		// Enter or l → open entry
		st.navOpenEntry()
		return true
	case vim.ActionBackspaceKey, vim.ActionNavOpenParent, vim.ActionMoveLeft:
		// Backspace, -, or h → go up one level (clamped at root)
		st.navDirGoUp()
		return true
	case vim.ActionNavCloseSpecial:
		st.navCloseSpecial()
		return true
	case vim.ActionNavToggleHidden:
		st.navToggleHidden()
		return true
	case vim.ActionRepeatLast:
		// In dir buffer, . toggles hidden files (oil convention)
		st.navToggleHidden()
		return true

	// Block editing actions in directory buffers
	case vim.ActionInsertBefore, vim.ActionInsertAfter,
		vim.ActionInsertLineStart, vim.ActionInsertLineEnd,
		vim.ActionOpenBelow, vim.ActionOpenAbove,
		vim.ActionSubstChar, vim.ActionSubstLine,
		vim.ActionDelete, vim.ActionChange,
		vim.ActionPut, vim.ActionPutBefore,
		vim.ActionReplace, vim.ActionJoinLines,
		vim.ActionIndent, vim.ActionDedent:
		return true // swallow — directory buffers are read-only
	}

	// Let all other actions (j, k, G, gg, /, n, etc.) pass through
	return false
}

// navOpenStatus opens the git status buffer.
func (st *appState) navOpenStatus() {
	if st.gitRepo == nil {
		st.detectNavRoot()
		if st.gitRepo == nil {
			return
		}
	}
	if st.gitCache == nil {
		st.gitCache = git.NewCache(st.gitRepo)
	}

	sb, err := navigator.NewStatusBuffer(st.gitRepo, st.gitCache)
	if err != nil {
		return
	}
	content := sb.GenerateText()
	newEd := editor.NewEditor(buffer.NewFromString(content), "")
	st.tabBar.OpenEditor(newEd, "Git Status")

	ts := st.activeTabState()
	if ts != nil {
		ts.bufType = bufStatus
		ts.statusBuf = sb
	}
}

// navRefreshStatus regenerates the status buffer content.
func (st *appState) navRefreshStatus() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufStatus || ts.statusBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil {
		return
	}
	if st.gitRepo == nil || st.gitCache == nil {
		return
	}

	cursorLine := ed.Cursor.Line
	ts.statusBuf.Refresh(st.gitRepo, st.gitCache)
	content := ts.statusBuf.GenerateText()
	ed.Buffer = buffer.NewFromString(content)
	// Clamp cursor
	if cursorLine >= ed.Buffer.LineCount() {
		cursorLine = ed.Buffer.LineCount() - 1
	}
	if cursorLine < 0 {
		cursorLine = 0
	}
	ed.Cursor.SetPosition(ed.Buffer, cursorLine, 0)
}

// navStage stages the file at cursor in the status buffer.
func (st *appState) navStage() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufStatus || ts.statusBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil || st.gitRepo == nil {
		return
	}

	entry, _ := ts.statusBuf.EntryAtLine(ed.Cursor.Line)
	if entry == nil {
		return
	}
	st.gitRepo.Stage(entry.Path)
	st.gitCache.Invalidate()
	st.navRefreshStatus()
}

// navUnstage unstages the file at cursor in the status buffer.
func (st *appState) navUnstage() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufStatus || ts.statusBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil || st.gitRepo == nil {
		return
	}

	entry, _ := ts.statusBuf.EntryAtLine(ed.Cursor.Line)
	if entry == nil {
		return
	}
	st.gitRepo.Unstage(entry.Path)
	st.gitCache.Invalidate()
	st.navRefreshStatus()
}

// navDiscard discards changes for the file at cursor in the status buffer.
func (st *appState) navDiscard() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufStatus || ts.statusBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil || st.gitRepo == nil {
		return
	}

	entry, _ := ts.statusBuf.EntryAtLine(ed.Cursor.Line)
	if entry == nil {
		return
	}
	// Discard is destructive — execute it
	st.gitRepo.Discard(entry.Path)
	st.gitCache.Invalidate()
	st.navRefreshStatus()
}

// navToggleDiff toggles inline diff expansion for a file in the status buffer.
func (st *appState) navToggleDiff() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufStatus || ts.statusBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil {
		return
	}

	entry, _ := ts.statusBuf.EntryAtLine(ed.Cursor.Line)
	if entry == nil {
		return
	}
	entry.Expanded = !entry.Expanded
	if entry.Expanded && entry.DiffText == "" && st.gitRepo != nil {
		// Fetch the diff text
		diff, err := st.gitRepo.DiffFile("HEAD", entry.Path)
		if err == nil && diff != nil {
			var lines []string
			for _, h := range diff.Hunks {
				lines = append(lines, h.Header)
				for _, dl := range h.Lines {
					switch dl.Type {
					case git.DiffLineAdd:
						lines = append(lines, "+ "+dl.Content)
					case git.DiffLineDelete:
						lines = append(lines, "- "+dl.Content)
					case git.DiffLineContext:
						lines = append(lines, "  "+dl.Content)
					}
				}
			}
			entry.DiffText = strings.Join(lines, "\n")
		}
	}

	// Regenerate buffer
	cursorLine := ed.Cursor.Line
	content := ts.statusBuf.GenerateText()
	ed.Buffer = buffer.NewFromString(content)
	if cursorLine >= ed.Buffer.LineCount() {
		cursorLine = ed.Buffer.LineCount() - 1
	}
	ed.Cursor.SetPosition(ed.Buffer, cursorLine, 0)
}

// navSectionNext moves to the next section header in the status buffer.
func (st *appState) navSectionNext() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufStatus || ts.statusBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil {
		return
	}
	line := ts.statusBuf.NextSection(ed.Cursor.Line)
	ed.Cursor.SetPosition(ed.Buffer, line, 0)
}

// navSectionPrev moves to the previous section header in the status buffer.
func (st *appState) navSectionPrev() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufStatus || ts.statusBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil {
		return
	}
	line := ts.statusBuf.PrevSection(ed.Cursor.Line)
	ed.Cursor.SetPosition(ed.Buffer, line, 0)
}

// navStatusOpenFile opens the file at cursor in the status buffer.
func (st *appState) navStatusOpenFile() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufStatus || ts.statusBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil {
		return
	}

	entry, _ := ts.statusBuf.EntryAtLine(ed.Cursor.Line)
	if entry == nil {
		return
	}
	fullPath := filepath.Join(st.gitRepo.Root, entry.Path)
	st.tabBar.OpenFile(fullPath)
}

// navToggleCollapse toggles section collapse in the status buffer.
func (st *appState) navToggleCollapse() {
	ts := st.activeTabState()
	if ts == nil || ts.bufType != bufStatus || ts.statusBuf == nil {
		return
	}
	ed := st.activeEd()
	if ed == nil {
		return
	}
	ts.statusBuf.ToggleCollapse(ed.Cursor.Line)
	cursorLine := ed.Cursor.Line
	content := ts.statusBuf.GenerateText()
	ed.Buffer = buffer.NewFromString(content)
	if cursorLine >= ed.Buffer.LineCount() {
		cursorLine = ed.Buffer.LineCount() - 1
	}
	ed.Cursor.SetPosition(ed.Buffer, cursorLine, 0)
}

// navNextChangedFile opens the next changed file and jumps to its first hunk.
func (st *appState) navNextChangedFile(count int) {
	if st.gitCache == nil {
		return
	}
	statuses, err := st.gitCache.Status()
	if err != nil || len(statuses) == 0 {
		return
	}

	var changedPaths []string
	for _, s := range statuses {
		if s.Index != '?' || s.Worktree != '?' {
			changedPaths = append(changedPaths, s.Path)
		}
	}
	if len(changedPaths) == 0 {
		return
	}

	ed := st.activeEd()
	currentRel := ""
	if ed != nil && ed.FilePath != "" && st.gitRepo != nil {
		currentRel, _ = filepath.Rel(st.gitRepo.Root, ed.FilePath)
	}

	for c := 0; c < count; c++ {
		nextPath := findNextInList(changedPaths, currentRel)
		currentRel = nextPath
	}

	if currentRel != "" && st.gitRepo != nil {
		fullPath := filepath.Join(st.gitRepo.Root, currentRel)
		st.tabBar.OpenFile(fullPath)
		// Jump to first hunk in the new file
		ts := st.activeTabState()
		newEd := st.activeEd()
		if ts != nil && ts.gitDiff != nil && newEd != nil {
			starts := ts.gitDiff.HunkStartLines()
			if len(starts) > 0 {
				newEd.Cursor.SetPosition(newEd.Buffer, starts[0]-1, 0)
			}
		}
	}
}

// navPrevChangedFile opens the previous changed file and jumps to its first hunk.
func (st *appState) navPrevChangedFile(count int) {
	if st.gitCache == nil {
		return
	}
	statuses, err := st.gitCache.Status()
	if err != nil || len(statuses) == 0 {
		return
	}

	var changedPaths []string
	for _, s := range statuses {
		if s.Index != '?' || s.Worktree != '?' {
			changedPaths = append(changedPaths, s.Path)
		}
	}
	if len(changedPaths) == 0 {
		return
	}

	ed := st.activeEd()
	currentRel := ""
	if ed != nil && ed.FilePath != "" && st.gitRepo != nil {
		currentRel, _ = filepath.Rel(st.gitRepo.Root, ed.FilePath)
	}

	for c := 0; c < count; c++ {
		prevPath := findPrevInList(changedPaths, currentRel)
		currentRel = prevPath
	}

	if currentRel != "" && st.gitRepo != nil {
		fullPath := filepath.Join(st.gitRepo.Root, currentRel)
		st.tabBar.OpenFile(fullPath)
		ts := st.activeTabState()
		newEd := st.activeEd()
		if ts != nil && ts.gitDiff != nil && newEd != nil {
			starts := ts.gitDiff.HunkStartLines()
			if len(starts) > 0 {
				newEd.Cursor.SetPosition(newEd.Buffer, starts[0]-1, 0)
			}
		}
	}
}

func findNextInList(items []string, current string) string {
	if len(items) == 0 {
		return ""
	}
	for i, item := range items {
		if item == current {
			return items[(i+1)%len(items)]
		}
	}
	return items[0]
}

func findPrevInList(items []string, current string) string {
	if len(items) == 0 {
		return ""
	}
	for i, item := range items {
		if item == current {
			idx := (i - 1 + len(items)) % len(items)
			return items[idx]
		}
	}
	return items[len(items)-1]
}

// navGoAlternate opens the alternate file (test <-> implementation).
func (st *appState) navGoAlternate() {
	ed := st.activeEd()
	if ed == nil || ed.FilePath == "" {
		return
	}
	alt := navigator.AlternateFile(ed.FilePath)
	if alt != "" {
		st.tabBar.OpenFile(alt)
	}
}

// navGoFile opens the file path under the cursor.
func (st *appState) navGoFile() {
	ed := st.activeEd()
	if ed == nil {
		return
	}
	// Get the current line and try to extract a file path or import string
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	// Try to extract a quoted string at cursor
	path := extractQuotedString(line, ed.Cursor.Col)
	if path == "" {
		return
	}

	// Try to resolve as a file path
	if st.gitRepo != nil {
		// Try relative to repo root
		fullPath := filepath.Join(st.gitRepo.Root, path)
		if _, err := filepath.Glob(fullPath); err == nil {
			st.tabBar.OpenFile(fullPath)
			return
		}
	}
	if ed.FilePath != "" {
		// Try relative to current file
		dir := filepath.Dir(ed.FilePath)
		fullPath := filepath.Join(dir, path)
		if _, err := filepath.Glob(fullPath); err == nil {
			st.tabBar.OpenFile(fullPath)
			return
		}
	}
}

// extractQuotedString extracts the string content at the cursor position from a line.
func extractQuotedString(line string, col int) string {
	if col >= len(line) {
		return ""
	}
	// Find the quote boundaries around the cursor
	for _, quote := range []byte{'"', '\'', '`'} {
		start := strings.LastIndexByte(line[:col+1], quote)
		if start < 0 {
			continue
		}
		end := strings.IndexByte(line[start+1:], quote)
		if end < 0 {
			continue
		}
		end += start + 1
		if col >= start && col <= end {
			return line[start+1 : end]
		}
	}
	return ""
}

// handleStatusBufferAction handles keybindings specific to status buffers.
// Returns true if the action was consumed.
func (st *appState) handleStatusBufferAction(action vim.Action) bool {
	switch action.Kind {
	// Status-specific actions from leader keys
	case vim.ActionNavStage:
		st.navStage()
		return true
	case vim.ActionNavUnstage:
		st.navUnstage()
		return true
	case vim.ActionNavDiscard:
		st.navDiscard()
		return true
	case vim.ActionNavToggleDiff:
		st.navToggleDiff()
		return true
	case vim.ActionNavSectionNext:
		st.navSectionNext()
		return true
	case vim.ActionNavSectionPrev:
		st.navSectionPrev()
		return true
	case vim.ActionNavRefresh:
		st.navRefreshStatus()
		return true
	case vim.ActionEnterKey:
		st.navStatusOpenFile()
		return true
	case vim.ActionTabKey:
		st.navToggleCollapse()
		return true
	case vim.ActionNavCloseSpecial:
		st.navCloseSpecial()
		return true

	// Override normal vim keys for status buffer semantics:
	// s → stage (normally ActionSubstChar)
	case vim.ActionSubstChar:
		st.navStage()
		return true
	// u → unstage (normally ActionUndo)
	case vim.ActionUndo:
		st.navUnstage()
		return true
	// n → next section (normally ActionSearchNext)
	case vim.ActionSearchNext:
		st.navSectionNext()
		return true
	// p → handled below as ActionPut is not 'p' in our case
	// N → prev section (normally ActionSearchPrev)
	case vim.ActionSearchPrev:
		st.navSectionPrev()
		return true

	// Block all other editing actions
	case vim.ActionInsertBefore, vim.ActionInsertAfter,
		vim.ActionInsertLineStart, vim.ActionInsertLineEnd,
		vim.ActionOpenBelow, vim.ActionOpenAbove,
		vim.ActionSubstLine,
		vim.ActionDelete, vim.ActionChange,
		vim.ActionPut, vim.ActionPutBefore,
		vim.ActionReplace, vim.ActionJoinLines,
		vim.ActionIndent, vim.ActionDedent,
		vim.ActionRedo, vim.ActionRepeatLast:
		return true
	}
	return false
}

// executeNavAction handles navigator-related vim actions.
// Returns true if the action was handled.
func (st *appState) executeNavAction(action vim.Action) bool {
	// Actions that work without an active editor
	switch action.Kind {
	case vim.ActionNavOpenRoot:
		st.navToggleExplorer()
		return true
	case vim.ActionNavOpenStatus:
		st.navOpenStatus()
		return true
	}

	ed := st.activeEd()
	if ed == nil {
		return false
	}

	// Special buffer dispatch
	ts := st.activeTabState()
	if ts != nil && ts.bufType == bufDirectory {
		if st.handleDirBufferAction(action) {
			return true
		}
	}
	if ts != nil && ts.bufType == bufStatus {
		if st.handleStatusBufferAction(action) {
			return true
		}
	}

	count := action.EffectiveCount()

	switch action.Kind {
	case vim.ActionNavNextHunk:
		st.navNextHunk(ed, count)
		return true
	case vim.ActionNavPrevHunk:
		st.navPrevHunk(ed, count)
		return true
	case vim.ActionNavOpenParent:
		st.navOpenParent()
		return true
	case vim.ActionNavCloseSpecial:
		st.navCloseSpecial()
		return true
	case vim.ActionNavToggleHidden:
		st.navToggleHidden()
		return true
	case vim.ActionNavNextFile:
		st.navNextChangedFile(count)
		return true
	case vim.ActionNavPrevFile:
		st.navPrevChangedFile(count)
		return true
	case vim.ActionNavGoAlternate:
		st.navGoAlternate()
		return true
	case vim.ActionNavGoFile:
		st.navGoFile()
		return true
	case vim.ActionNavToggleReadMode:
		st.toggleMarkdownPreview()
		return true

	// Stubs for features implemented in later phases
	case vim.ActionNavToggleOriginal:
		return true
	case vim.ActionNavOpenEntry:
		return true
	case vim.ActionNavStage:
		return true
	case vim.ActionNavUnstage:
		return true
	case vim.ActionNavDiscard:
		return true
	case vim.ActionNavToggleDiff:
		return true
	case vim.ActionNavSectionNext:
		return true
	case vim.ActionNavSectionPrev:
		return true
	case vim.ActionNavRefresh:
		return true
	case vim.ActionNavGoImports:
		return true
	case vim.ActionNavFindFiles:
		return true
	case vim.ActionNavFindChanged:
		return true
	case vim.ActionNavHelp:
		return true
	}
	return false
}
