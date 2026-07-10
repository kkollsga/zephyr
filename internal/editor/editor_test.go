package editor

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestEditor_SetLineLeadingWhitespace(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("a\n        b\nc"), "")

	// Dedent line 1 from 8 spaces to 4.
	if !ed.SetLineLeadingWhitespace(1, "    ") {
		t.Fatal("expected change")
	}
	if got, _ := ed.Buffer.Line(1); got != "    b" {
		t.Fatalf("after dedent: %q", got)
	}
	if !ed.Modified {
		t.Fatal("expected Modified after indent change")
	}
	// Pure dedent is a single undo step.
	ed.Undo()
	if got, _ := ed.Buffer.Line(1); got != "        b" {
		t.Fatalf("after undo: %q", got)
	}

	// No-op when the whitespace already matches.
	if ed.SetLineLeadingWhitespace(2, "") {
		t.Fatal("expected no change for already-correct line")
	}

	// Indent a line that has none.
	if !ed.SetLineLeadingWhitespace(0, "  ") {
		t.Fatal("expected change")
	}
	if got, _ := ed.Buffer.Line(0); got != "  a" {
		t.Fatalf("after indent: %q", got)
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

func TestEditor_SaveAsFailureDoesNotChangeFilePath(t *testing.T) {
	dir := t.TempDir()
	originalPath := filepath.Join(dir, "original.txt")
	ed := NewEditor(buffer.NewFromString("unsaved"), originalPath)
	ed.Modified = true
	failedPath := filepath.Join(dir, "missing", "failed.txt")

	if err := ed.SaveAs(failedPath); err == nil {
		t.Fatal("expected SaveAs to fail")
	}
	if ed.FilePath != originalPath {
		t.Fatalf("FilePath changed after failed SaveAs: got %q, want %q", ed.FilePath, originalPath)
	}
	if !ed.Modified {
		t.Fatal("failed SaveAs cleared modified state")
	}
}

func TestEditor_SaveAsWritesAndUpdatesState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "saved-as.txt")
	ed := NewEditor(buffer.NewFromString("save-as content"), "")
	ed.Modified = true

	if err := ed.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "save-as content"; got != want {
		t.Fatalf("saved content = %q, want %q", got, want)
	}
	if ed.FilePath != path {
		t.Fatalf("FilePath = %q, want %q", ed.FilePath, path)
	}
	if ed.Modified {
		t.Fatal("successful SaveAs left editor modified")
	}
}

func TestEditor_SaveThroughSymlinkKeepsVisiblePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires elevated privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "visible-link.txt")
	if err := os.WriteFile(target, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	ed := NewEditor(buffer.NewFromString("after"), link)
	ed.Modified = true

	if err := ed.Save(); err != nil {
		t.Fatal(err)
	}
	if ed.FilePath != link {
		t.Fatalf("FilePath = %q, want editor-visible symlink %q", ed.FilePath, link)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("save replaced editor-visible symlink")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "after"; got != want {
		t.Fatalf("target content = %q, want %q", got, want)
	}
}

func TestEditor_ReloadExternalChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reload.txt")
	if err := os.WriteFile(path, []byte("first line\nsecond line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ed, err := NewEditorFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ed.Cursor = Cursor{Line: 1, Col: 6}
	ed.Selection.Start(Cursor{Line: 0, Col: 0})
	ed.Selection.Update(Cursor{Line: 1, Col: 3})
	ed.Modified = true

	if err := os.WriteFile(path, []byte("external\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ed.Reload(); err != nil {
		t.Fatal(err)
	}
	if got, want := ed.Buffer.Text(), "external\n"; got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
	if ed.Modified {
		t.Fatal("reload left editor modified")
	}
	if ed.Selection.Active {
		t.Fatal("reload left a selection active")
	}
	if ed.Cursor.Line >= ed.Buffer.LineCount() {
		t.Fatalf("cursor was not clamped: %+v", ed.Cursor)
	}
}

func TestEditor_UndoReloadRestoresPreviousContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reload-undo.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	ed, err := NewEditorFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ed.Cursor = Cursor{Line: 0, Col: 4}
	if err := os.WriteFile(path, []byte("after"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ed.Reload(); err != nil {
		t.Fatal(err)
	}

	ed.Undo()
	if got, want := ed.Buffer.Text(), "before"; got != want {
		t.Fatalf("undo after reload = %q, want %q", got, want)
	}
	if got, want := ed.Cursor, (Cursor{Line: 0, Col: 4}); got != want {
		t.Fatalf("cursor after undo = %+v, want %+v", got, want)
	}
	if !ed.Modified {
		t.Fatal("undoing reload should mark the editor modified")
	}

	ed.Redo()
	if got, want := ed.Buffer.Text(), "after"; got != want {
		t.Fatalf("redo after reload = %q, want %q", got, want)
	}
	if ed.Modified {
		t.Fatal("redoing reload should restore the unmodified disk state")
	}
}

func TestEditor_FailedReloadDoesNotChangeHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reload-failure.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	ed, err := NewEditorFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeUndo := len(ed.History.undoStack)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := ed.Reload(); err == nil {
		t.Fatal("expected reload of removed file to fail")
	}
	if got := len(ed.History.undoStack); got != beforeUndo {
		t.Fatalf("failed reload changed undo history length: got %d, want %d", got, beforeUndo)
	}
	if got, want := ed.Buffer.Text(), "before"; got != want {
		t.Fatalf("failed reload changed buffer: got %q, want %q", got, want)
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
