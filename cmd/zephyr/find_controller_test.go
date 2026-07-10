package main

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/editor"
)

func TestFindReplaceControllerWorkflow(t *testing.T) {
	st, ed, _ := testAppWithText("foo bar foo\nFoo", "Plain Text")
	st.findBar.Visible = true
	st.findBar.Query = "foo"
	st.updateSearchResults()
	if st.findBar.MatchCount != 3 || st.findBar.CurrentMatch != 1 || ed.SelectedText() != "foo" {
		t.Fatalf("initial search count=%d current=%d selection=%q", st.findBar.MatchCount, st.findBar.CurrentMatch, ed.SelectedText())
	}

	st.findNextMatch()
	if st.findBar.CurrentMatch != 2 || ed.Cursor.Line != 0 || ed.Cursor.Col != 11 {
		t.Fatalf("next match current=%d cursor=%+v", st.findBar.CurrentMatch, ed.Cursor)
	}
	st.findNextMatch()
	st.findNextMatch()
	if st.findBar.CurrentMatch != 1 {
		t.Fatalf("next-match wrap = %d", st.findBar.CurrentMatch)
	}
	st.findPrevMatch()
	if st.findBar.CurrentMatch != 3 {
		t.Fatalf("previous-match wrap = %d", st.findBar.CurrentMatch)
	}

	st.findBar.CurrentMatch = 1
	st.findBar.Replacement = "X"
	st.replaceCurrentMatch()
	if got := ed.Buffer.Text(); got != "X bar foo\nFoo" {
		t.Fatalf("replace current text = %q", got)
	}
	st.replaceAllMatches()
	if got := ed.Buffer.Text(); got != "X bar X\nX" {
		t.Fatalf("replace all text = %q", got)
	}

	st.findBar.Query = "["
	st.findBar.UseRegex = true
	st.updateSearchResults()
	if st.findBar.MatchCount != 0 || st.findBar.CurrentMatch != 0 {
		t.Fatalf("invalid regex retained results: %+v", st.findBar)
	}
	st.findBar.Query = ""
	st.updateSearchResults()
	st.findNextMatch()
	st.findPrevMatch()
}

func TestOpenFindBarUsesSingleLineSelection(t *testing.T) {
	st, ed, _ := testAppWithText("alpha beta\ngamma", "Plain Text")
	ed.Cursor.SetPosition(ed.Buffer, 0, 6)
	ed.Selection.Start(ed.Cursor)
	ed.Cursor.SetPosition(ed.Buffer, 0, 10)
	ed.Selection.Update(ed.Cursor)

	st.openFindBar(true)
	if !st.findBar.Visible || !st.findBar.ShowReplace || st.findBar.Query != "beta" || st.findBar.MatchCount != 1 {
		t.Fatalf("open replace bar = %+v", st.findBar)
	}

	ed.Selection.Start(editor.Cursor{Line: 0, Col: 0})
	ed.Selection.Update(editor.Cursor{Line: 1, Col: 5})
	st.findBar.Query = "kept"
	st.openFindBar(false)
	if st.findBar.Query != "kept" {
		t.Fatalf("multiline selection replaced query with %q", st.findBar.Query)
	}
}

func TestFindBarClickController(t *testing.T) {
	st, _, _ := testAppWithText("one one", "Plain Text")
	st.lastMaxX = 900
	st.tabBarHeight = 28
	st.findBar.Visible = true
	st.findBar.ShowReplace = true
	st.findBar.Query = "one"
	st.findBar.Replacement = "two"
	st.updateSearchResults()
	g := computeFindBarGeom(st.lastMaxX, true)
	windowY := func(relative int) int { return st.tabBarHeight + g.barY + relative }

	if st.handleFindBarClick(0, 0) {
		t.Fatal("outside click was consumed")
	}
	if !st.handleFindBarClick(g.barX+1, windowY(g.rowY+2)) || st.findBar.ShowReplace {
		t.Fatal("chevron click did not collapse replace row")
	}
	st.findBar.ShowReplace = true
	if !st.handleFindBarClick(g.barX+g.inputX+12, windowY(g.rowY+2)) || st.findBar.FocusField != 0 {
		t.Fatal("find input click did not focus query")
	}
	arrowX := g.barX + g.inputX + g.inputW + 3
	st.handleFindBarClick(arrowX, windowY(g.rowY+1))
	if st.findBar.CurrentMatch != 2 {
		t.Fatalf("up arrow current match = %d", st.findBar.CurrentMatch)
	}
	st.handleFindBarClick(arrowX, windowY(g.rowY+findBarInputH-1))
	if st.findBar.CurrentMatch != 1 {
		t.Fatalf("down arrow current match = %d", st.findBar.CurrentMatch)
	}
	if !st.handleFindBarClick(g.barX+g.inputX+10, windowY(g.rowY2+2)) || st.findBar.FocusField != 1 {
		t.Fatal("replace input click did not focus replacement")
	}
	if !st.handleFindBarClick(g.barX+g.closeBtnX+1, windowY(g.rowY+2)) || st.findBar.Visible {
		t.Fatal("close click did not close find bar")
	}
}

func TestLanguageControllerTransitionsAndReparse(t *testing.T) {
	st, ed, ts := testAppWithText("package p\nfunc f() {\n}\n", "Plain Text")
	st.setLanguage("Go")
	if ts.langLabel != "Go" || ts.highlighter == nil || len(ts.sourceBuf) == 0 {
		t.Fatalf("Go language state lang=%q highlighter=%v source=%d", ts.langLabel, ts.highlighter, len(ts.sourceBuf))
	}
	ed.Cursor.SetPosition(ed.Buffer, 2, 0)
	ed.InsertText("// changed\n")
	st.reparsePending = true
	st.reparseHighlight()
	if st.reparsePending || ts.foldState.RegionAt(1) == nil {
		t.Fatalf("reparse state pending=%v folds=%#v", st.reparsePending, ts.foldState.Regions)
	}
	st.setLanguage("Plain Text")
	if ts.langLabel != "Plain Text" || ts.highlighter != nil {
		t.Fatalf("plain language retained highlighter: %+v", ts)
	}
	st.setLanguage("Not A Language")
	if ts.langLabel != "Not A Language" || ts.highlighter != nil {
		t.Fatalf("unknown language state: lang=%q highlighter=%v", ts.langLabel, ts.highlighter)
	}
}
