package editor

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
)

func selectRange(ed *Editor, l1, c1, l2, c2 int) {
	ed.Cursor.SetPosition(ed.Buffer, l1, c1)
	ed.Selection.Start(ed.Cursor)
	ed.Cursor.SetPosition(ed.Buffer, l2, c2)
	ed.Selection.Update(ed.Cursor)
}

func TestInsertText_OverSelection_UndoRestoresOriginal(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello world"), "")
	selectRange(ed, 0, 0, 0, 5)
	ed.InsertText("X")
	if got := ed.Buffer.Text(); got != "X world" {
		t.Fatalf("replace = %q, want %q", got, "X world")
	}
	if !ed.Undo() {
		t.Fatal("undo reported nothing to do")
	}
	if got := ed.Buffer.Text(); got != "hello world" {
		t.Fatalf("one undo after replace = %q, want %q", got, "hello world")
	}
	if !ed.Redo() || ed.Buffer.Text() != "X world" {
		t.Fatalf("redo = %q, want %q", ed.Buffer.Text(), "X world")
	}
}

func TestInsertText_PasteOverSelection_UndoRestoresOriginal(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("alpha\nbeta\ngamma"), "")
	selectRange(ed, 0, 2, 2, 3)
	ed.InsertText("ONE\nTWO")
	if got := ed.Buffer.Text(); got != "alONE\nTWOma" {
		t.Fatalf("paste = %q", got)
	}
	if !ed.Undo() || ed.Buffer.Text() != "alpha\nbeta\ngamma" {
		t.Fatalf("undo of paste-over-selection = %q", ed.Buffer.Text())
	}
}

// Pre-mortem 1: typing right after a replace must not be swallowed into the
// replace's group by coalescing.
func TestInsertText_TypingAfterReplaceIsASeparateUndoStep(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello world"), "")
	ed.History.SetCoalesceWindow(0)
	selectRange(ed, 0, 0, 0, 5)
	ed.InsertText("X")
	ed.InsertText("Y")
	if got := ed.Buffer.Text(); got != "XY world" {
		t.Fatalf("after replace+type = %q", got)
	}
	if !ed.Undo() || ed.Buffer.Text() != "X world" {
		t.Fatalf("first undo = %q, want the replacement kept", ed.Buffer.Text())
	}
	if !ed.Undo() || ed.Buffer.Text() != "hello world" {
		t.Fatalf("second undo = %q", ed.Buffer.Text())
	}
}

func TestInsertText_EmptyOverSelectionDeletesIt(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello world"), "")
	selectRange(ed, 0, 5, 0, 11)
	ed.InsertText("")
	if got := ed.Buffer.Text(); got != "hello" {
		t.Fatalf("empty replacement = %q, want %q", got, "hello")
	}
	if !ed.Undo() || ed.Buffer.Text() != "hello world" {
		t.Fatalf("undo of empty replacement = %q", ed.Buffer.Text())
	}
}

func TestInsertText_EmptyWithoutSelectionRecordsNothing(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello"), "")
	ed.InsertText("")
	if ed.Buffer.Text() != "hello" {
		t.Fatalf("buffer changed: %q", ed.Buffer.Text())
	}
	if ed.History.CanUndo() {
		t.Fatal("an empty insert with no selection recorded a history entry")
	}
	if ed.Modified {
		t.Fatal("an empty insert with no selection marked the buffer modified")
	}
}

func TestMultiCursorInsert_UndoIsOneStep(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("aa\nbb\ncc"), "")
	ed.History.SetCoalesceWindow(0)
	ed.Cursor.SetPosition(ed.Buffer, 0, 0)
	ed.AddCursor(1, 0)
	ed.AddCursor(2, 0)
	ed.InsertTextAtAllCursors(">")
	if got := ed.Buffer.Text(); got != ">aa\n>bb\n>cc" {
		t.Fatalf("multi-cursor insert = %q", got)
	}
	if !ed.Undo() || ed.Buffer.Text() != "aa\nbb\ncc" {
		t.Fatalf("one undo of a multi-cursor insert = %q", ed.Buffer.Text())
	}
}

func TestMultiCursorBackspace_UndoIsOneStep(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("xaa\nxbb\nxcc"), "")
	ed.History.SetCoalesceWindow(0)
	ed.Cursor.SetPosition(ed.Buffer, 0, 1)
	ed.AddCursor(1, 1)
	ed.AddCursor(2, 1)
	ed.DeleteBackwardAtAllCursors()
	if got := ed.Buffer.Text(); got != "aa\nbb\ncc" {
		t.Fatalf("multi-cursor backspace = %q", got)
	}
	if !ed.Undo() || ed.Buffer.Text() != "xaa\nxbb\nxcc" {
		t.Fatalf("one undo of a multi-cursor backspace = %q", ed.Buffer.Text())
	}
}

func TestSetLineLeadingWhitespace_UndoIsOneStep(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("  foo\nbar"), "")
	ed.History.SetCoalesceWindow(0)
	// A delete+insert pair (two leading spaces become a tab) is one step.
	if !ed.SetLineLeadingWhitespace(0, "\t") {
		t.Fatal("expected the whitespace to change")
	}
	if got := ed.Buffer.Text(); got != "\tfoo\nbar" {
		t.Fatalf("got %q", got)
	}
	if !ed.Undo() || ed.Buffer.Text() != "  foo\nbar" {
		t.Fatalf("one undo = %q", ed.Buffer.Text())
	}
}
func TestTransact_NestedGroupsFlattenToOneStep(t *testing.T) {
	ed := NewEditor(buffer.NewFromString(""), "")
	ed.History.SetCoalesceWindow(0)
	ed.Transact(func() {
		ed.InsertText("a")
		ed.Transact(func() {
			ed.InsertText("b")
			ed.InsertText("c")
		})
		ed.InsertText("d")
	})
	if ed.Buffer.Text() != "abcd" {
		t.Fatalf("got %q", ed.Buffer.Text())
	}
	if !ed.Undo() || ed.Buffer.Text() != "" {
		t.Fatalf("one undo of a nested transact = %q", ed.Buffer.Text())
	}
	if ed.History.CanUndo() {
		t.Fatal("nested transact left more than one undo step")
	}
	if !ed.Redo() || ed.Buffer.Text() != "abcd" {
		t.Fatalf("redo of a group = %q", ed.Buffer.Text())
	}
}

func TestTransact_CursorBeforeFirstAndAfterLast(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello world"), "")
	selectRange(ed, 0, 0, 0, 5)
	before := ed.Cursor
	ed.InsertText("XYZ")
	after := ed.Cursor
	if !ed.Undo() {
		t.Fatal("undo failed")
	}
	if ed.Cursor != before {
		t.Fatalf("undo cursor = %+v, want the cursor before the group %+v", ed.Cursor, before)
	}
	if !ed.Redo() {
		t.Fatal("redo failed")
	}
	if ed.Cursor.Line != after.Line || ed.Cursor.Col != after.Col {
		t.Fatalf("redo cursor = %+v, want after the last action %+v", ed.Cursor, after)
	}
}

// Typing or pasting exactly what is already selected changes nothing, so it
// records no undo step. Surfaced by the history replay fuzzer.
func TestInsertText_OverIdenticalSelectionRecordsNothing(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("one\ntwo"), "")
	selectRange(ed, 0, 1, 0, 3)

	ed.InsertText("ne")

	if got := ed.Buffer.Text(); got != "one\ntwo" {
		t.Fatalf("buffer changed: %q", got)
	}
	if ed.History.CanUndo() {
		t.Fatal("an identical replacement recorded an undo step")
	}
	if ed.Cursor.Line != 0 || ed.Cursor.Col != 3 {
		t.Fatalf("cursor = %d:%d, want 0:3 (end of the replacement)", ed.Cursor.Line, ed.Cursor.Col)
	}
	if ed.Selection.Active {
		t.Fatal("selection left active")
	}
}
