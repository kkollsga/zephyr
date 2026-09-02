package main

import (
	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/render"
)

// undoSteps applies up to count undo steps to the active editor, resyncing the
// derived tab state after each one. It stops early once the history is empty or
// the buffer refuses a step.
func (st *appState) undoSteps(count int) {
	ed, ts := st.activeEd(), st.activeTabState()
	if ed == nil {
		return
	}
	for i := 0; i < count; i++ {
		r := ed.UndoStep()
		if !r.Applied {
			break
		}
		st.afterUndoRedo(ed, ts, r.Swapped)
	}
	st.afterEdit()
}

// redoSteps applies up to count redo steps, with the same bookkeeping.
func (st *appState) redoSteps(count int) {
	ed, ts := st.activeEd(), st.activeTabState()
	if ed == nil {
		return
	}
	for i := 0; i < count; i++ {
		r := ed.RedoStep()
		if !r.Applied {
			break
		}
		st.afterUndoRedo(ed, ts, r.Swapped)
	}
	st.afterEdit()
}

// afterUndoRedo resyncs the tab state one applied undo/redo step invalidated.
// A step that replaced the whole buffer (undo of a reload, a format or another
// external change) needs the full swap treatment; an in-place edit only moves
// byte offsets.
func (st *appState) afterUndoRedo(ed *editor.Editor, ts *tabState, swapped bool) {
	if ed == nil || ts == nil {
		return
	}
	if swapped {
		st.afterBufferSwap(ed, ts)
		return
	}
	st.afterOffsetsMoved(ts)
}

// afterBufferSwap rebuilds the state keyed to the buffer object after ed.Buffer
// was replaced wholesale. The reparse must happen here rather than through the
// deferred one: the tree belongs to the replaced document, so feeding it the
// next incremental edit taken from the new buffer would corrupt it. The edits
// the swap itself left pending are discarded for the same reason.
func (st *appState) afterBufferSwap(ed *editor.Editor, ts *tabState) {
	if ts.highlighter != nil {
		source := ed.Buffer.TextBytes(ts.sourceBuf)
		ts.sourceBuf = source
		ts.highlighter.Parse(source)
	}
	ed.Buffer.DrainEdits()
	if ts.foldState != nil {
		regions := render.ComputeFoldRegions(ed.Buffer.Text())
		ts.foldState.SetRegions(regions, ed.Buffer.LineCount())
	}
	st.afterOffsetsMoved(ts)
}

// afterOffsetsMoved re-derives what is keyed to positions in the buffer: the
// find bar's match offsets, and the cursor the viewport last synced to.
func (st *appState) afterOffsetsMoved(ts *tabState) {
	if st.findBar.Visible {
		st.refreshSearchMatches()
	}
	ts.lastCursorLine = -1
	ts.lastCursorCol = -1
}

// replaceListingBuffer swaps a regenerated navigator listing (directory or git
// status) into the tab and puts the cursor back on line, clamped to the new
// document. Every listing rebuild goes through it so the state derived from the
// buffer object it replaced — highlighter tree, fold regions, pending
// incremental edits, the viewport's cursor cache — is reset with it.
func (st *appState) replaceListingBuffer(ed *editor.Editor, ts *tabState, content string, line int) {
	ed.Buffer = buffer.NewFromString(content)
	if ts != nil {
		st.afterBufferSwap(ed, ts)
	}
	if line >= ed.Buffer.LineCount() {
		line = ed.Buffer.LineCount() - 1
	}
	if line < 0 {
		line = 0
	}
	ed.Cursor.SetPosition(ed.Buffer, line, 0)
}
