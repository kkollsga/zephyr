package editor

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
)

// A rejected buffer edit must leave both stacks where they were: the action has
// to stay undoable, because moving it to the redo stack while the buffer never
// changed puts every later undo at offsets describing a different document.
func TestUndo_RejectedBufferEditLeavesStacksUnchanged(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("abc"), "")
	// An insert recorded past the end of the buffer: undoing it deletes an
	// out-of-range span, which the piece table refuses.
	ed.History.Record(EditAction{Type: ActionInsert, Offset: 99, Text: "xyz"})

	if ed.Undo() {
		t.Fatal("Undo reported success for an edit the buffer refused")
	}
	if ed.Buffer.Text() != "abc" {
		t.Fatalf("buffer changed on a refused undo: %q", ed.Buffer.Text())
	}
	if !ed.History.CanUndo() {
		t.Fatal("refused action was popped off the undo stack")
	}
	if ed.History.CanRedo() {
		t.Fatal("refused action was pushed onto the redo stack")
	}
}

func TestRedo_RejectedBufferEditLeavesStacksUnchanged(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("abcdef"), "")
	ed.Cursor.SetPosition(ed.Buffer, 0, 6)
	ed.InsertText("XY")
	if !ed.Undo() {
		t.Fatal("Undo of a plain insert failed")
	}
	// Shrink the buffer behind history's back so the redo's insert offset is
	// past the end and the piece table refuses it.
	ed.Buffer.Delete(0, 6)

	if ed.Redo() {
		t.Fatal("Redo reported success for an edit the buffer refused")
	}
	if ed.Buffer.Text() != "" {
		t.Fatalf("buffer changed on a refused redo: %q", ed.Buffer.Text())
	}
	if !ed.History.CanRedo() {
		t.Fatal("refused action was popped off the redo stack")
	}
	if ed.History.CanUndo() {
		t.Fatal("refused action was pushed onto the undo stack")
	}
}

func TestUndoRedo_ReportSuccess(t *testing.T) {
	ed := NewEditor(buffer.NewFromString(""), "")
	if ed.Undo() || ed.Redo() {
		t.Fatal("empty history reported a completed undo/redo")
	}
	ed.InsertText("hi")
	if !ed.Undo() || ed.Buffer.Text() != "" {
		t.Fatalf("undo failed: %q", ed.Buffer.Text())
	}
	if !ed.Redo() || ed.Buffer.Text() != "hi" {
		t.Fatalf("redo failed: %q", ed.Buffer.Text())
	}
}

// EditAction carries one cursor, so extra cursors are not restorable and the
// positions they hold may not exist in the document undo restores. Both
// directions therefore drop them rather than editing at stale positions.
func TestUndoRedo_ClearExtraCursors(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("aa\nbb\ncc"), "")
	ed.AddCursorBelow()
	ed.AddCursorBelow()
	ed.InsertTextAtAllCursors("x")
	if len(ed.Cursors) != 2 {
		t.Fatalf("fixture: %d extra cursors, want 2", len(ed.Cursors))
	}

	if !ed.Undo() || len(ed.Cursors) != 0 || len(ed.Selections) != 0 {
		t.Fatalf("undo left %d cursors / %d selections", len(ed.Cursors), len(ed.Selections))
	}
	ed.AddCursorBelow()
	if !ed.Redo() || len(ed.Cursors) != 0 || len(ed.Selections) != 0 {
		t.Fatalf("redo left %d cursors / %d selections", len(ed.Cursors), len(ed.Selections))
	}
}

// A whole-buffer replacement is reported as a swap so the caller can rebuild
// the state it derives from the buffer object; an in-place edit is not.
func TestUndoStep_ReportsBufferSwap(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("one"), "")
	ed.History.RecordExternalChange("one", "two", ed.Cursor, ed.Cursor)
	ed.Buffer = buffer.NewFromString("two")
	ed.InsertText("!")

	if r := ed.UndoStep(); !r.Applied || r.Swapped {
		t.Fatalf("undo of a plain insert = %+v, want applied without a swap", r)
	}
	if r := ed.UndoStep(); !r.Applied || !r.Swapped {
		t.Fatalf("undo of a full replacement = %+v, want a swap", r)
	}
	if ed.Buffer.Text() != "one" {
		t.Fatalf("buffer = %q, want %q", ed.Buffer.Text(), "one")
	}
	if r := ed.RedoStep(); !r.Applied || !r.Swapped {
		t.Fatalf("redo of a full replacement = %+v, want a swap", r)
	}
}
