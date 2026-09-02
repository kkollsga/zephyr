package main

import (
	"path/filepath"
	"strings"
	"time"

	"gioui.org/io/key"

	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/ui"
	"github.com/kristianweb/zephyr/internal/vim"
)

// headTitleSuffix marks a tab showing committed content rather than the file.
const headTitleSuffix = " (HEAD)"

// headStash is the working document a HEAD view displaced, kept whole so the
// round trip restores the exact piece table the undo history was recorded
// against.
type headStash struct {
	buf      *buffer.PieceTable
	cursor   editor.Cursor
	modified bool
	title    string
}

// headViewActive reports whether the active tab is showing HEAD content.
func (st *appState) headViewActive() bool {
	ts := st.activeTabState()
	return ts != nil && ts.bufType == bufOriginal
}

// navToggleOriginal swaps between the working file and its HEAD content in
// place (the `go` key).
func (st *appState) navToggleOriginal() {
	ed, ts := st.activeEd(), st.activeTabState()
	tab := st.tabBar.ActiveTab()
	if ed == nil || ts == nil || tab == nil {
		return
	}
	if ts.bufType == bufOriginal {
		st.leaveHeadView(ed, ts, tab)
		return
	}
	if ts.bufType != bufFile {
		return // a directory or status listing has no HEAD content
	}
	st.enterHeadView(ed, ts, tab)
}

// enterHeadView replaces the buffer with the file's content at HEAD. A file
// git cannot show at HEAD — untracked, outside the repo, or added since the
// last commit — is reported and left alone rather than silently swapped.
func (st *appState) enterHeadView(ed *editor.Editor, ts *tabState, tab *ui.Tab) {
	if st.gitRepo == nil {
		st.detectNavRoot()
	}
	if st.gitRepo == nil || ed.FilePath == "" {
		st.notify("No git repository for this file", 5*time.Second)
		return
	}
	rel, err := filepath.Rel(st.gitRepo.Root, ed.FilePath)
	if err != nil || strings.HasPrefix(rel, "..") {
		st.notify("File is outside "+filepath.Base(st.gitRepo.Root), 5*time.Second)
		return
	}
	content, err := st.gitRepo.Show("HEAD", filepath.ToSlash(rel))
	if err != nil {
		st.notify("Not in HEAD: "+filepath.Base(ed.FilePath), 5*time.Second)
		return
	}

	line, col := ed.Cursor.Line, ed.Cursor.Col
	ts.headStash = &headStash{
		buf: ed.Buffer, cursor: ed.Cursor, modified: ed.Modified, title: tab.Title,
	}
	// Modified is deliberately carried into the HEAD view rather than cleared:
	// the tab still holds unsaved work, and a cleared flag would let the close
	// and quit flows discard it without a prompt.
	ed.Buffer = buffer.NewFromString(string(content))
	ed.Selection.Clear()
	ed.ClearExtraCursors()
	ed.Cursor.SetPosition(ed.Buffer, line, col)
	ts.bufType = bufOriginal
	tab.Title += headTitleSuffix
	st.afterBufferSwap(ed, ts)
	st.updateWindowTitle()
	st.notify("HEAD view — read-only; go returns to the file", 5*time.Second)
}

// leaveHeadView puts the stashed working document back exactly as it was.
func (st *appState) leaveHeadView(ed *editor.Editor, ts *tabState, tab *ui.Tab) {
	stash := ts.headStash
	ts.headStash = nil
	ts.bufType = bufFile
	if stash == nil {
		return
	}
	ed.Buffer = stash.buf
	ed.Selection.Clear()
	ed.ClearExtraCursors()
	ed.Cursor = stash.cursor
	ed.Modified = stash.modified
	tab.Title = stash.title
	st.afterBufferSwap(ed, ts)
	st.updateWindowTitle()
}

// handleOriginalBufferAction swallows every vim action that would edit a HEAD
// view, and reports whether it consumed the action. Undo and redo are included:
// the history belongs to the stashed working buffer, and applying a step here
// would rewrite HEAD content and desynchronise the stacks from the document
// they were recorded against.
func (st *appState) handleOriginalBufferAction(action vim.Action) bool {
	switch action.Kind {
	case vim.ActionInsertBefore, vim.ActionInsertAfter,
		vim.ActionInsertLineStart, vim.ActionInsertLineEnd,
		vim.ActionEnterInsert,
		vim.ActionOpenBelow, vim.ActionOpenAbove,
		vim.ActionSubstChar, vim.ActionSubstLine,
		vim.ActionDelete, vim.ActionChange,
		vim.ActionPut, vim.ActionPutBefore,
		vim.ActionReplace, vim.ActionJoinLines,
		vim.ActionIndent, vim.ActionDedent,
		vim.ActionUndo, vim.ActionRedo, vim.ActionRepeatLast:
		return true
	}
	return false
}

// headViewSwallowsKey reports whether a key press would edit a HEAD view. It
// covers the native path — Cmd+V, Cmd+X, Cmd+Z and the editing keys — which
// does not go through the vim action dispatch.
func (st *appState) headViewSwallowsKey(ke key.Event) bool {
	if !st.headViewActive() {
		return false
	}
	switch {
	case ke.Name == key.NameDeleteBackward, ke.Name == key.NameDeleteForward:
		return true
	case ke.Name == key.NameReturn && ke.Modifiers == 0:
		return true
	case ke.Name == key.NameTab && ke.Modifiers == 0:
		return true
	case ke.Modifiers&key.ModShortcut == 0:
		return false
	case ke.Name == "Z", ke.Name == "X", ke.Name == "V":
		return true
	}
	return false
}

// headViewRefusesSave reports whether tab is showing HEAD content, in which
// case no flow may write it: the bytes on disk are the working file and HEAD
// content saved over them is the file's own history destroying its present.
func (st *appState) headViewRefusesSave(tab *ui.Tab) bool {
	if tab == nil {
		return false
	}
	ts, ok := st.tabStates[tab.Editor]
	return ok && ts.bufType == bufOriginal
}
