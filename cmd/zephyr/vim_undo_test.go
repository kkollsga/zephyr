package main

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/vim"
)

func TestVimJoinLines_NonASCIINextLine(t *testing.T) {
	// J deletes the newline plus the whole next line by byte length. A rune
	// count there deletes too few bytes and leaves a fragment behind.
	const src = "first\n  héllo wörld"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	ed.Cursor.SetPosition(ed.Buffer, 0, 0)

	st.executeVimAction(vim.Action{Kind: vim.ActionJoinLines})
	if got := ed.Buffer.Text(); got != "first héllo wörld" {
		t.Fatalf("J = %q, want %q", got, "first héllo wörld")
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("undo of J = %q, want %q", ed.Buffer.Text(), src)
	}
}

func TestVimReplaceChar_Undoable(t *testing.T) {
	const src = "abcdef"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	ed.Cursor.SetPosition(ed.Buffer, 0, 1)

	st.executeVimAction(vim.Action{Kind: vim.ActionReplace, Char: 'X', Count: 3})
	if got := ed.Buffer.Text(); got != "aXXXef" {
		t.Fatalf("3rX = %q, want %q", got, "aXXXef")
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("undo of r = %q, want %q", ed.Buffer.Text(), src)
	}
}

func TestVimIndentDedent_Undoable(t *testing.T) {
	const src = "alpha\nbeta"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	ed.Cursor.SetPosition(ed.Buffer, 0, 0)

	st.executeVimAction(vim.Action{Kind: vim.ActionIndent, MotionType: vim.MotionLineWise})
	if got := ed.Buffer.Text(); got != "    alpha\nbeta" {
		t.Fatalf(">> = %q", got)
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("undo of >> = %q, want %q", ed.Buffer.Text(), src)
	}

	st.executeVimAction(vim.Action{Kind: vim.ActionIndent, MotionType: vim.MotionLineWise})
	st.executeVimAction(vim.Action{Kind: vim.ActionDedent, MotionType: vim.MotionLineWise})
	if got := ed.Buffer.Text(); got != src {
		t.Fatalf(">> then << = %q, want %q", got, src)
	}
	if !ed.Undo() || ed.Buffer.Text() != "    alpha\nbeta" {
		t.Fatalf("undo of << = %q", ed.Buffer.Text())
	}
}

// The desync probe: an unrecorded indent leaves every older undo entry holding
// offsets computed against a buffer that has since changed length.
func TestVimIndentAfterInsert_UndoRestoresBothSteps(t *testing.T) {
	const src = "alpha\nbeta"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()

	ed.Cursor.SetPosition(ed.Buffer, 1, 4)
	ed.InsertText("ZZZ")
	if got := ed.Buffer.Text(); got != "alpha\nbetaZZZ" {
		t.Fatalf("insert = %q", got)
	}

	ed.Cursor.SetPosition(ed.Buffer, 0, 0)
	st.executeVimAction(vim.Action{Kind: vim.ActionIndent, MotionType: vim.MotionLineWise})
	if got := ed.Buffer.Text(); got != "    alpha\nbetaZZZ" {
		t.Fatalf(">> = %q", got)
	}

	if !ed.Undo() || ed.Buffer.Text() != "alpha\nbetaZZZ" {
		t.Fatalf("undo of >> = %q, want %q", ed.Buffer.Text(), "alpha\nbetaZZZ")
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("undo of the insert = %q, want %q", ed.Buffer.Text(), src)
	}
}
