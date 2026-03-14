package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/fileio"
)

func TestEditor_TypeAndUndo(t *testing.T) {
	ed := NewEditor(buffer.NewFromString(""), "")
	ed.InsertText("hello")
	if ed.Buffer.Text() != "hello" {
		t.Fatalf("after insert: got %q", ed.Buffer.Text())
	}
	ed.Undo()
	if ed.Buffer.Text() != "" {
		t.Fatalf("after undo: got %q", ed.Buffer.Text())
	}
}

func TestEditor_TypeAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	ed := NewEditor(buffer.NewFromString(""), path)
	ed.InsertText("hello world")
	if err := ed.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Fatalf("got %q", string(data))
	}
	if ed.Modified {
		t.Fatal("expected Modified=false after save")
	}
}

func TestEditor_OpenEditSaveReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.txt")
	os.WriteFile(path, []byte("original"), 0644)

	// Open
	ed, err := NewEditorFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Edit: move to end and append
	ed.Cursor.MoveToFileEnd(ed.Buffer)
	ed.InsertText(" modified")

	// Save
	if err := ed.Save(); err != nil {
		t.Fatal(err)
	}

	// Reopen
	pt, err := fileio.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "original modified" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestEditor_CopyPasteInternal(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello world"), "")
	// Select "world"
	ed.Selection.Start(Cursor{Line: 0, Col: 6})
	ed.Selection.Update(Cursor{Line: 0, Col: 11})

	copied := ed.SelectedText()
	if copied != "world" {
		t.Fatalf("got %q", copied)
	}

	// Move to beginning and paste
	ed.Cursor = Cursor{Line: 0, Col: 0}
	ed.Selection.Clear()
	ed.InsertText(copied)
	if ed.Buffer.Text() != "worldhello world" {
		t.Fatalf("got %q", ed.Buffer.Text())
	}
}

func TestEditor_CutPaste(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello world"), "")
	// Select "hello "
	ed.Selection.Start(Cursor{Line: 0, Col: 0})
	ed.Selection.Update(Cursor{Line: 0, Col: 6})
	cut := ed.SelectedText()
	ed.DeleteSelection()

	if ed.Buffer.Text() != "world" {
		t.Fatalf("after cut: got %q", ed.Buffer.Text())
	}

	// Paste at end
	ed.Cursor.MoveToFileEnd(ed.Buffer)
	ed.InsertText(cut)
	if ed.Buffer.Text() != "worldhello " {
		t.Fatalf("after paste: got %q", ed.Buffer.Text())
	}
}

func TestEditor_InsertNewline(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello"), "")
	ed.Cursor.Col = 5
	ed.InsertText("\nworld")
	if ed.Buffer.Text() != "hello\nworld" {
		t.Fatalf("got %q", ed.Buffer.Text())
	}
	if ed.Cursor.Line != 1 || ed.Cursor.Col != 5 {
		t.Fatalf("cursor at %d:%d, want 1:5", ed.Cursor.Line, ed.Cursor.Col)
	}
}

func TestEditor_BackspaceAtLineStart(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello\nworld"), "")
	ed.Cursor.Line = 1
	ed.Cursor.Col = 0
	ed.DeleteBackward()
	if ed.Buffer.Text() != "helloworld" {
		t.Fatalf("got %q", ed.Buffer.Text())
	}
	if ed.Cursor.Line != 0 || ed.Cursor.Col != 5 {
		t.Fatalf("cursor at %d:%d, want 0:5", ed.Cursor.Line, ed.Cursor.Col)
	}
}

func TestEditor_DeleteForward(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("hello"), "")
	ed.Cursor.Col = 0
	ed.DeleteForward()
	if ed.Buffer.Text() != "ello" {
		t.Fatalf("got %q", ed.Buffer.Text())
	}
}

func TestEditor_UndoRedo(t *testing.T) {
	ed := NewEditor(buffer.NewFromString(""), "")
	ed.InsertText("a")
	ed.InsertText("b")
	ed.InsertText("c")
	// All coalesced, so one undo should remove "abc"
	ed.Undo()
	if ed.Buffer.Text() != "" {
		t.Fatalf("after undo: got %q", ed.Buffer.Text())
	}
	ed.Redo()
	if ed.Buffer.Text() != "abc" {
		t.Fatalf("after redo: got %q", ed.Buffer.Text())
	}
}
