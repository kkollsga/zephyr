package main

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/vim"
)

func TestReplaceAllMatches_OneUndoRestoresEveryMatch(t *testing.T) {
	const src = "foo bar foo baz foo"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.findBar.Query = "foo"
	st.findBar.Replacement = "qux"

	st.replaceAllMatches()
	if got := ed.Buffer.Text(); got != "qux bar qux baz qux" {
		t.Fatalf("replace all = %q", got)
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("one undo after replace-all = %q, want %q", ed.Buffer.Text(), src)
	}
	if ed.History.CanUndo() {
		t.Fatal("replace-all left more than one undo step")
	}
	if !ed.Redo() || ed.Buffer.Text() != "qux bar qux baz qux" {
		t.Fatalf("redo of replace-all = %q", ed.Buffer.Text())
	}
}

func TestReplaceCurrentMatch_EmptyReplacementDeletesTheMatch(t *testing.T) {
	const src = "foo bar foo"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.findBar.Query = "foo"
	st.findBar.Replacement = ""
	st.updateSearchResults()
	st.findBar.CurrentMatch = 1

	st.replaceCurrentMatch()
	if got := ed.Buffer.Text(); got != " bar foo" {
		t.Fatalf("replace with empty = %q, want %q", got, " bar foo")
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("undo of an empty replacement = %q", ed.Buffer.Text())
	}
}

func TestVimVisualPut_OneUndoRestoresTheSelection(t *testing.T) {
	const src = "hello world"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	st.vimState.Registers.Named['a'] = "XYZ"

	ed.Cursor.SetPosition(ed.Buffer, 0, 0)
	ed.Selection.Start(ed.Cursor)
	ed.Cursor.SetPosition(ed.Buffer, 0, 5)
	ed.Selection.Update(ed.Cursor)

	st.vimPut(ed, vim.Action{Kind: vim.ActionPut, Register: 'a', Text: "visual"}, false)
	if got := ed.Buffer.Text(); got != "XYZ world" {
		t.Fatalf("visual put = %q", got)
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("one undo after visual put = %q, want %q", ed.Buffer.Text(), src)
	}
}

func TestDeleteAutoPair_OneUndoRestoresBothBrackets(t *testing.T) {
	const src = "f()"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	ed.Cursor.SetPosition(ed.Buffer, 0, 2)

	if !st.deleteAutoPair() {
		t.Fatal("expected the auto-pair to be deleted")
	}
	if got := ed.Buffer.Text(); got != "f" {
		t.Fatalf("auto-pair delete = %q", got)
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("one undo after an auto-pair delete = %q", ed.Buffer.Text())
	}
}
