package main

import (
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/vim"
)

// vimVisualTextObject makes the text object under the cursor the visual
// selection (viw, vi", vih). The selection is rebuilt from the visual anchor
// and the cursor after every key, so setting ed.Selection here would be
// overwritten: the anchor and the cursor are what carry the span.
//
// vimFindTextObject's end is exclusive and so is the selection head, so the
// span drawn here is exactly the span the operator form deletes — viw then d
// removes what diw removes.
//
// An object the cursor is not inside leaves the selection and the mode alone.
func (st *appState) vimVisualTextObject(ed *editor.Editor, action vim.Action) {
	if st.vimState == nil {
		return
	}
	if action.TextObj == 'h' {
		st.vimVisualHunkObject(ed)
		return
	}
	startLine, startCol, endLine, endCol, ok := vimFindTextObject(ed, action.TextObj, action.TextObjType == 'i')
	if !ok {
		return
	}
	st.vimState.VisualAnchorLine = startLine
	st.vimState.VisualAnchorCol = startCol
	ed.Cursor.SetPosition(ed.Buffer, endLine, endCol)
	st.updateVimVisualSelection()
}

// vimVisualHunkObject selects the changed lines of the hunk under the cursor.
// The hunk object is linewise, so it switches to Visual Line mode rather than
// selecting a character span.
func (st *appState) vimVisualHunkObject(ed *editor.Editor) {
	ts := st.activeTabState()
	if ts == nil {
		return
	}
	start, end, ok := hunkChangedLineRange(ts.activeDiff(), ed.Cursor.Line+1)
	if !ok {
		return
	}
	st.vimState.Mode = vim.ModeVisualLine
	st.vimState.VisualAnchorLine = start - 1
	st.vimState.VisualAnchorCol = 0
	ed.Cursor.SetPosition(ed.Buffer, end-1, 0)
	st.updateVimVisualSelection()
}
