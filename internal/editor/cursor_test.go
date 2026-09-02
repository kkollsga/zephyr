package editor

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
)

func testBuffer() *buffer.PieceTable {
	return buffer.NewFromString("hello\nworld\nfoo bar\nbaz")
}

func TestCursor_MoveRight_WithinLine(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.MoveRight(pt)
	if c.Line != 0 || c.Col != 1 {
		t.Fatalf("got %d:%d, want 0:1", c.Line, c.Col)
	}
}

func TestCursor_MoveRight_EndOfLine_WrapsToNextLine(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.Col = 5 // end of "hello"
	c.MoveRight(pt)
	if c.Line != 1 || c.Col != 0 {
		t.Fatalf("got %d:%d, want 1:0", c.Line, c.Col)
	}
}

func TestCursor_MoveRight_EndOfFile_NoOp(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.Line = 3
	c.Col = 3 // end of "baz"
	c.MoveRight(pt)
	if c.Line != 3 || c.Col != 3 {
		t.Fatalf("got %d:%d, want 3:3", c.Line, c.Col)
	}
}

func TestCursor_MoveLeft_WithinLine(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.Col = 3
	c.MoveLeft(pt)
	if c.Line != 0 || c.Col != 2 {
		t.Fatalf("got %d:%d, want 0:2", c.Line, c.Col)
	}
}

func TestCursor_MoveLeft_BeginningOfLine_WrapsToPrevLine(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.Line = 1
	c.Col = 0
	c.MoveLeft(pt)
	if c.Line != 0 || c.Col != 5 {
		t.Fatalf("got %d:%d, want 0:5", c.Line, c.Col)
	}
}

func TestCursor_MoveLeft_BeginningOfFile_NoOp(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.MoveLeft(pt)
	if c.Line != 0 || c.Col != 0 {
		t.Fatalf("got %d:%d, want 0:0", c.Line, c.Col)
	}
}

func TestCursor_MoveDown_NormalCase(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.Col = 3
	c.MoveDown(pt)
	if c.Line != 1 || c.Col != 3 {
		t.Fatalf("got %d:%d, want 1:3", c.Line, c.Col)
	}
}

func TestCursor_MoveDown_ShorterLine_ClampsCol(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.Line = 2
	c.Col = 6 // "foo bar" col 6
	c.MoveDown(pt)
	// "baz" has length 3
	if c.Line != 3 || c.Col != 3 {
		t.Fatalf("got %d:%d, want 3:3", c.Line, c.Col)
	}
}

func TestCursor_MoveDown_LastLine_NoOp(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.Line = 3
	c.MoveDown(pt)
	if c.Line != 3 {
		t.Fatalf("got line %d, want 3", c.Line)
	}
}

func TestCursor_MoveUp_FirstLine_NoOp(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.MoveUp(pt)
	if c.Line != 0 {
		t.Fatalf("got line %d, want 0", c.Line)
	}
}

func TestCursor_MoveToLineStart(t *testing.T) {
	c := NewCursor()
	c.Col = 5
	c.MoveToLineStart()
	if c.Col != 0 {
		t.Fatalf("got col %d, want 0", c.Col)
	}
}

func TestCursor_MoveToLineEnd(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.MoveToLineEnd(pt)
	if c.Col != 5 {
		t.Fatalf("got col %d, want 5", c.Col)
	}
}

func TestCursor_PageDown(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.PageDown(pt, 2)
	if c.Line != 2 {
		t.Fatalf("got line %d, want 2", c.Line)
	}
}

func TestCursor_PageUp(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.Line = 3
	c.PageUp(pt, 2)
	if c.Line != 1 {
		t.Fatalf("got line %d, want 1", c.Line)
	}
}

func TestCursor_ClickToPosition(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.SetPosition(pt, 2, 4)
	if c.Line != 2 || c.Col != 4 {
		t.Fatalf("got %d:%d, want 2:4", c.Line, c.Col)
	}
}

func TestCursor_ClickToPosition_Clamped(t *testing.T) {
	pt := testBuffer()
	c := NewCursor()
	c.SetPosition(pt, 100, 100)
	if c.Line != 3 || c.Col != 3 {
		t.Fatalf("got %d:%d, want 3:3", c.Line, c.Col)
	}
}

func TestCursor_PreferredCol_Preserved(t *testing.T) {
	// "foo bar" (7 chars) then "baz" (3 chars) then back up
	pt := testBuffer() // "hello\nworld\nfoo bar\nbaz"
	c := NewCursor()
	c.Line = 2
	c.Col = 6 // col 6 in "foo bar"
	c.MoveDown(pt)
	// "baz" clamps to col 3, but preferred stays at 6
	if c.Col != 3 {
		t.Fatalf("expected clamped col 3, got %d", c.Col)
	}
	// Move back up should restore to 6
	c.MoveUp(pt)
	if c.Col != 6 {
		t.Fatalf("expected preferred col 6 restored, got %d", c.Col)
	}
}

// Cursor.Col counts runes everywhere else in the editor, so SetPosition must
// clamp against the rune count. Clamping against the byte length admitted a
// column past the end of a multibyte line, which then failed to convert to an
// offset at all.
func TestCursorSetPosition_ClampsInRunes(t *testing.T) {
	pt := buffer.NewFromString("héllo wörld\nplain")
	var c Cursor
	c.SetPosition(pt, 0, 999)
	if c.Col != 11 {
		t.Fatalf("Col = %d, want 11 (rune count of %q)", c.Col, "héllo wörld")
	}
	c.SetPosition(pt, 0, 12)
	if c.Col != 11 {
		t.Fatalf("Col = %d past the line's 11 runes", c.Col)
	}
	c.SetPosition(pt, 9, 0)
	if c.Line != 1 {
		t.Fatalf("Line = %d, want 1 (last line)", c.Line)
	}
}

// An out-of-range column used to convert to offset 0, so the edit landed at the
// top of the file instead of at the cursor's line.
func TestInsertAfterOverflowingColumnLandsAtLineEnd(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("first\nhéllo wörld\nlast"), "")
	ed.Cursor.SetPosition(ed.Buffer, 1, 999)
	ed.InsertText("!")

	if got := ed.Buffer.Text(); got != "first\nhéllo wörld!\nlast" {
		t.Fatalf("insert landed wrong: %q", got)
	}
}

// The cursor moves back over the runes the bytes held, not over the byte count.
func TestDeleteBackwardN_MovesCursorByRunes(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("aöb"), "")
	ed.Cursor.SetPosition(ed.Buffer, 0, 3)
	ed.DeleteBackwardN(3) // "öb": three bytes, two runes

	if got := ed.Buffer.Text(); got != "a" {
		t.Fatalf("text = %q, want %q", got, "a")
	}
	if ed.Cursor.Col != 1 {
		t.Fatalf("Col = %d, want 1", ed.Cursor.Col)
	}
}
