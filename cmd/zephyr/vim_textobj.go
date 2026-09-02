package main

import (
	"strings"
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/git"
	"github.com/kristianweb/zephyr/internal/vim"
)

// hunkChangedLineRange returns the 1-based new-file line span of the changed
// lines around line: the maximal run of added or modified lines containing it.
// A hunk carries context lines at its edges and can carry them in its middle,
// and those lines are not the user's change — operating on the hunk's declared
// span would delete text the user never touched. A cursor that is not itself on
// a changed line therefore has no hunk object, and ok is false.
func hunkChangedLineRange(fd *git.FileDiff, line int) (start, end int, ok bool) {
	changed := make(map[int]bool)
	for _, l := range fd.ChangedNewLines() {
		changed[l] = true
	}
	if !changed[line] {
		return 0, 0, false
	}
	start, end = line, line
	for changed[start-1] {
		start--
	}
	for changed[end+1] {
		end++
	}
	return start, end, true
}

// vimHunkObject applies op to the hunk text object under the cursor and reports
// whether it found one. Delivered linewise: whole lines including their
// newlines, and a register that ends in one so a later p pastes on a new line.
func (st *appState) vimHunkObject(ed *editor.Editor, op vim.Operator) bool {
	ts := st.activeTabState()
	if ts == nil || st.vimState == nil {
		return false
	}
	start, end, ok := hunkChangedLineRange(ts.gitDiff, ed.Cursor.Line+1)
	if !ok {
		return false
	}
	st.vimLineRangeOp(ed, start-1, end-1, op)
	return true
}

// vimLineRangeOp deletes, changes or yanks buffer lines [start,end] (0-based,
// inclusive) as one undo step. Delete takes the trailing newline with the lines
// — or the leading one at end of file — so no blank line is stranded; change
// keeps the lines' newlines so insert opens on an empty line where they were.
func (st *appState) vimLineRangeOp(ed *editor.Editor, start, end int, op vim.Operator) {
	last := ed.Buffer.LineCount() - 1
	if start < 0 || end > last || start > end {
		return
	}
	lines := make([]string, 0, end-start+1)
	for i := start; i <= end; i++ {
		text, err := ed.Buffer.Line(i)
		if err != nil {
			return
		}
		lines = append(lines, text)
	}
	regText := strings.Join(lines, "\n") + "\n"

	if op == vim.OpYank {
		st.vimState.Registers.RecordYank(regText, st.vimState.Register)
		ed.Selection.Clear()
		ed.Cursor.SetPosition(ed.Buffer, start, 0)
		return
	}

	ed.Selection.Anchor = editor.Cursor{Line: start, Col: 0}
	ed.Selection.Head = editor.Cursor{Line: end, Col: utf8.RuneCountInString(lines[len(lines)-1])}
	if op == vim.OpDelete {
		switch {
		case end < last:
			ed.Selection.Head = editor.Cursor{Line: end + 1, Col: 0}
		case start > 0:
			prev, _ := ed.Buffer.Line(start - 1)
			ed.Selection.Anchor = editor.Cursor{Line: start - 1, Col: utf8.RuneCountInString(prev)}
		}
	}
	ed.Selection.Active = true

	st.vimState.Registers.RecordDelete(regText, true, st.vimState.Register)
	ed.DeleteSelection()
	ed.Cursor.SetPosition(ed.Buffer, start, 0)
	if op == vim.OpChange {
		st.vimState.Mode = vim.ModeInsert
	} else {
		vimMoveFirstNonBlank(ed)
	}
	st.afterEdit()
}
