package main

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/vim"
)

// TestVimChangeLine_LeavesEmptyLine pins cc: vim clears the line's text and
// enters insert on the now-empty line; it does not remove the line.
func TestVimChangeLine_LeavesEmptyLine(t *testing.T) {
	const src = "alpha\nbeta\ngamma"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	ed.Cursor.SetPosition(ed.Buffer, 1, 0)

	st.executeVimAction(vim.Action{Kind: vim.ActionChange, MotionType: vim.MotionLineWise})

	if got := ed.Buffer.Text(); got != "alpha\n\ngamma" {
		t.Fatalf("cc = %q, want %q", got, "alpha\n\ngamma")
	}
	if st.vimState.Mode != vim.ModeInsert {
		t.Fatalf("cc left mode %v, want insert", st.vimState.Mode)
	}
	if ed.Cursor.Line != 1 || ed.Cursor.Col != 0 {
		t.Fatalf("cc cursor = (%d,%d), want (1,0)", ed.Cursor.Line, ed.Cursor.Col)
	}
	if got := st.vimState.Registers.Unnamed; got != "beta\n" {
		t.Fatalf("cc register = %q, want %q", got, "beta\n")
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("undo of cc = %q, want %q (one step)", ed.Buffer.Text(), src)
	}
}

// TestVimChangeLine_LastLine covers the EOF branch, where there is no trailing
// newline to keep and the line must still survive as an empty one.
func TestVimChangeLine_LastLine(t *testing.T) {
	const src = "alpha\nbeta"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	ed.Cursor.SetPosition(ed.Buffer, 1, 0)

	st.executeVimAction(vim.Action{Kind: vim.ActionChange, MotionType: vim.MotionLineWise})

	if got := ed.Buffer.Text(); got != "alpha\n" {
		t.Fatalf("cc on last line = %q, want %q", got, "alpha\n")
	}
	if ed.Buffer.LineCount() != 2 {
		t.Fatalf("cc on last line left %d lines, want 2", ed.Buffer.LineCount())
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("undo of cc at EOF = %q, want %q", ed.Buffer.Text(), src)
	}
}

// TestVimYankLastLine_PutsLinewise pins the register shape at EOF: yy must
// yield "<line>\n", not "\n<line>", or p pastes charwise onto the current line.
func TestVimYankLastLine_PutsLinewise(t *testing.T) {
	const src = "alpha\nbeta"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	st.vimState.Register = 'a'
	ed.Cursor.SetPosition(ed.Buffer, 1, 0)

	st.executeVimAction(vim.Action{Kind: vim.ActionYank, MotionType: vim.MotionLineWise})
	if got := st.vimState.Registers.Get('a'); got != "beta\n" {
		t.Fatalf("yy on last line = %q, want %q", got, "beta\n")
	}
	if got := ed.Buffer.Text(); got != src {
		t.Fatalf("yy modified the buffer: %q", got)
	}

	st.executeVimAction(vim.Action{Kind: vim.ActionPut, Register: 'a'})
	if got := ed.Buffer.Text(); got != "alpha\nbeta\nbeta" {
		t.Fatalf("p after yy at EOF = %q, want %q", got, "alpha\nbeta\nbeta")
	}
}

// TestVimYankLastLine_PutBefore is the P half of the same register contract.
func TestVimYankLastLine_PutBefore(t *testing.T) {
	const src = "alpha\nbeta"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	st.vimState.Register = 'a'
	ed.Cursor.SetPosition(ed.Buffer, 1, 0)

	st.executeVimAction(vim.Action{Kind: vim.ActionYank, MotionType: vim.MotionLineWise})
	st.executeVimAction(vim.Action{Kind: vim.ActionPutBefore, Register: 'a'})
	if got := ed.Buffer.Text(); got != "alpha\nbeta\nbeta" {
		t.Fatalf("P after yy at EOF = %q, want %q", got, "alpha\nbeta\nbeta")
	}
}

// TestVimDeleteLastLine_RegisterAndBuffer keeps dd's behaviour pinned while the
// register shape changes: the preceding newline still has to go.
func TestVimDeleteLastLine_RegisterAndBuffer(t *testing.T) {
	const src = "alpha\nbeta"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	ed.Cursor.SetPosition(ed.Buffer, 1, 0)

	st.executeVimAction(vim.Action{Kind: vim.ActionDelete, MotionType: vim.MotionLineWise})
	if got := ed.Buffer.Text(); got != "alpha" {
		t.Fatalf("dd on last line = %q, want %q", got, "alpha")
	}
	if got := st.vimState.Registers.Unnamed; got != "beta\n" {
		t.Fatalf("dd register = %q, want %q", got, "beta\n")
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("undo of dd = %q, want %q", ed.Buffer.Text(), src)
	}
}

// TestVimDeleteOnlyLine covers the single-line document, which has neither a
// following nor a preceding newline to absorb.
func TestVimDeleteOnlyLine(t *testing.T) {
	const src = "alpha"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	ed.Cursor.SetPosition(ed.Buffer, 0, 0)

	st.executeVimAction(vim.Action{Kind: vim.ActionDelete, MotionType: vim.MotionLineWise})
	if got := ed.Buffer.Text(); got != "" {
		t.Fatalf("dd on the only line = %q, want empty", got)
	}
	if got := st.vimState.Registers.Unnamed; got != "alpha\n" {
		t.Fatalf("dd register = %q, want %q", got, "alpha\n")
	}
}

// TestVimYankTwoLines_Multiline pins the non-EOF register shape too.
func TestVimYankTwoLines_Multiline(t *testing.T) {
	const src = "alpha\nbeta\ngamma"
	st, ed, _ := testAppWithText(src, "Plain Text")
	ed.History.SetCoalesceWindow(0)
	st.vimState = vim.NewState()
	st.vimState.Register = 'a'
	ed.Cursor.SetPosition(ed.Buffer, 0, 0)

	st.executeVimAction(vim.Action{Kind: vim.ActionYank, MotionType: vim.MotionLineWise, Count: 2})
	if got := st.vimState.Registers.Get('a'); got != "alpha\nbeta\n" {
		t.Fatalf("2yy = %q, want %q", got, "alpha\nbeta\n")
	}
}
