package editor

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
)

func TestMultiCursor_AddCursorBelow(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("line1\nline2\nline3"), "")
	ed.AddCursorBelow()
	if len(ed.Cursors) != 1 {
		t.Fatalf("expected 1 extra cursor, got %d", len(ed.Cursors))
	}
	if ed.Cursors[0].Line != 1 {
		t.Fatalf("expected cursor on line 1, got %d", ed.Cursors[0].Line)
	}
}

func TestMultiCursor_TypeInsertsAtAllCursors(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("aaa\nbbb\nccc"), "")
	ed.Cursor.Col = 0
	ed.AddCursor(1, 0)
	ed.AddCursor(2, 0)
	ed.InsertTextAtAllCursors("X")
	text := ed.Buffer.Text()
	if text != "Xaaa\nXbbb\nXccc" {
		t.Fatalf("got %q", text)
	}
}

func TestMultiCursor_DeleteAtAllCursors(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("Xaaa\nXbbb\nXccc"), "")
	ed.Cursor = Cursor{Line: 0, Col: 1, PreferredCol: -1}
	ed.AddCursor(1, 1)
	ed.AddCursor(2, 1)
	ed.DeleteBackwardAtAllCursors()
	text := ed.Buffer.Text()
	if text != "aaa\nbbb\nccc" {
		t.Fatalf("got %q", text)
	}
}

func TestMultiCursor_MergeOverlapping(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello"), "")
	ed.AddCursor(0, 0) // same as primary
	ed.MergeOverlappingCursors()
	if len(ed.Cursors) != 0 {
		t.Fatalf("expected overlapping cursors to merge, got %d", len(ed.Cursors))
	}
}

func TestMultiCursor_SelectNextOccurrence(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("foo bar foo baz foo"), "")
	// Select first "foo"
	ed.Selection.Start(Cursor{Line: 0, Col: 0})
	ed.Selection.Update(Cursor{Line: 0, Col: 3})
	ed.Cursor = Cursor{Line: 0, Col: 3, PreferredCol: -1}

	ed.SelectNextOccurrence()
	if len(ed.Cursors) != 1 {
		t.Fatalf("expected 1 extra cursor, got %d", len(ed.Cursors))
	}
	// Second "foo" is at offset 8
	if ed.Cursors[0].Line != 0 || ed.Cursors[0].Col != 11 {
		t.Fatalf("expected cursor at 0:11, got %d:%d", ed.Cursors[0].Line, ed.Cursors[0].Col)
	}
}

func TestMultiCursor_SelectAllOccurrences(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("foo bar foo baz foo"), "")
	ed.Selection.Start(Cursor{Line: 0, Col: 0})
	ed.Selection.Update(Cursor{Line: 0, Col: 3})
	ed.Cursor = Cursor{Line: 0, Col: 3, PreferredCol: -1}

	ed.SelectAllOccurrences()
	// Primary + 2 extra = 3 total cursors
	if len(ed.Cursors) != 2 {
		t.Fatalf("expected 2 extra cursors, got %d", len(ed.Cursors))
	}
}

func TestMultiCursor_SplitSelectionIntoLines(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("line1\nline2\nline3\nline4"), "")
	ed.Selection.Start(Cursor{Line: 0, Col: 0})
	ed.Selection.Update(Cursor{Line: 3, Col: 5})

	ed.SplitSelectionIntoLines()
	// Should have cursor on lines 0,1,2,3 = 4 total
	total := 1 + len(ed.Cursors)
	if total != 4 {
		t.Fatalf("expected 4 cursors, got %d", total)
	}
}

func TestMultiCursor_EscapeClearsToSingle(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello"), "")
	ed.AddCursor(0, 3)
	ed.AddCursor(0, 5)
	ed.ClearExtraCursors()
	if ed.HasMultipleCursors() {
		t.Fatal("expected single cursor after clear")
	}
}

func TestMultiCursor_PasteMultipleLines(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("aaa\nbbb\nccc"), "")
	ed.Cursor.Col = 3
	ed.AddCursor(1, 3)
	ed.AddCursor(2, 3)
	ed.InsertTextAtAllCursors("!")
	if ed.Buffer.Text() != "aaa!\nbbb!\nccc!" {
		t.Fatalf("got %q", ed.Buffer.Text())
	}
}
