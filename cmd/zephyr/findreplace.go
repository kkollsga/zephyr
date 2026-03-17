package main

import (
	"strings"

	"github.com/kristianweb/zephyr/internal/editor"
)

func (st *appState) openFindBar(withReplace bool) {
	ed := st.activeEd()
	// Pre-populate with selected text if any
	if ed != nil {
		if sel := ed.SelectedText(); sel != "" && !strings.Contains(sel, "\n") {
			st.findBar.Query = sel
		}
	}
	if withReplace {
		st.findBar.OpenReplace()
	} else {
		st.findBar.Open()
	}
	st.updateSearchResults()
}

func (st *appState) updateSearchResults() {
	ed := st.activeEd()
	if ed == nil {
		st.findBar.Matches = nil
		st.findBar.MatchCount = 0
		st.findBar.CurrentMatch = 0
		return
	}
	if st.findBar.Query == "" {
		st.findBar.Matches = nil
		st.findBar.MatchCount = 0
		st.findBar.CurrentMatch = 0
		return
	}
	results, _ := editor.Find(ed.Buffer, st.findBar.Query, st.findBar.UseRegex, st.findBar.CaseSensitive)
	st.findBar.Matches = results
	st.findBar.MatchCount = len(results)

	if len(results) == 0 {
		st.findBar.CurrentMatch = 0
		return
	}

	// Clamp current match to valid range, always scroll to show it
	if st.findBar.CurrentMatch < 1 || st.findBar.CurrentMatch > len(results) {
		st.findBar.CurrentMatch = 1
	}
	st.jumpToCurrentMatch()
}

func (st *appState) findNextMatch() {
	if st.findBar.MatchCount == 0 {
		return
	}
	st.findBar.CurrentMatch++
	if st.findBar.CurrentMatch > st.findBar.MatchCount {
		st.findBar.CurrentMatch = 1
	}
	st.jumpToCurrentMatch()
}

func (st *appState) findPrevMatch() {
	if st.findBar.MatchCount == 0 {
		return
	}
	st.findBar.CurrentMatch--
	if st.findBar.CurrentMatch < 1 {
		st.findBar.CurrentMatch = st.findBar.MatchCount
	}
	st.jumpToCurrentMatch()
}

func (st *appState) jumpToCurrentMatch() {
	ed := st.activeEd()
	if ed == nil || st.findBar.CurrentMatch == 0 || st.findBar.CurrentMatch > len(st.findBar.Matches) {
		return
	}
	match := st.findBar.Matches[st.findBar.CurrentMatch-1]

	// Select the match text (VS Code behavior)
	ed.Cursor.SetPosition(ed.Buffer, match.Line, match.Col)
	ed.Selection.Start(ed.Cursor)
	// Compute end position from byte length
	endLC, _ := ed.Buffer.OffsetToLineCol(match.Offset + match.Length)
	ed.Cursor.SetPosition(ed.Buffer, endLC.Line, endLC.Col)
	ed.Selection.Update(ed.Cursor)

	// Force viewport scroll to cursor
	if ts := st.activeTabState(); ts != nil {
		ts.viewport.ScrollToRevealCursor(ed.Cursor.Line)
		ts.lastCursorLine = ed.Cursor.Line
		ts.lastCursorCol = ed.Cursor.Col
	}
}

func (st *appState) replaceCurrentMatch() {
	ed := st.activeEd()
	if ed == nil || st.findBar.CurrentMatch == 0 || st.findBar.CurrentMatch > len(st.findBar.Matches) {
		return
	}
	match := st.findBar.Matches[st.findBar.CurrentMatch-1]

	// Select the match, then InsertText replaces it with history recording
	endLC, _ := ed.Buffer.OffsetToLineCol(match.Offset + match.Length)
	ed.Cursor.SetPosition(ed.Buffer, match.Line, match.Col)
	ed.Selection.Start(ed.Cursor)
	ed.Cursor.SetPosition(ed.Buffer, endLC.Line, endLC.Col)
	ed.Selection.Update(ed.Cursor)
	ed.InsertText(st.findBar.Replacement)

	st.reparseHighlight()
	st.jumpToCurrentMatch()
}

func (st *appState) replaceAllMatches() {
	ed := st.activeEd()
	if ed == nil || st.findBar.Query == "" {
		return
	}
	results, _ := editor.Find(ed.Buffer, st.findBar.Query, st.findBar.UseRegex, st.findBar.CaseSensitive)
	if len(results) == 0 {
		return
	}

	// Replace from end to start to preserve earlier offsets
	for i := len(results) - 1; i >= 0; i-- {
		match := results[i]
		endLC, _ := ed.Buffer.OffsetToLineCol(match.Offset + match.Length)
		ed.Cursor.SetPosition(ed.Buffer, match.Line, match.Col)
		ed.Selection.Start(ed.Cursor)
		ed.Cursor.SetPosition(ed.Buffer, endLC.Line, endLC.Col)
		ed.Selection.Update(ed.Cursor)
		ed.InsertText(st.findBar.Replacement)
	}

	st.reparseHighlight()
}
