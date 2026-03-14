package fileio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
)

func TestOpenFile_ValidFile(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "small.txt")
	pt, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pt.Length() == 0 {
		t.Fatal("expected non-empty file")
	}
}

func TestOpenFile_NonExistent_Error(t *testing.T) {
	_, err := OpenFile("/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestSaveFile_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	pt := buffer.NewFromString("hello world\n")
	if err := SaveFile(pt, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world\n" {
		t.Fatalf("got %q", string(data))
	}
}

func TestSaveFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	os.WriteFile(path, []byte("old content"), 0644)

	pt := buffer.NewFromString("new content")
	if err := SaveFile(pt, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new content" {
		t.Fatalf("got %q", string(data))
	}
}

func TestSaveFile_PreservesNewlineStyle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "newlines.txt")
	content := "line1\nline2\nline3\n"
	pt := buffer.NewFromString(content)
	if err := SaveFile(pt, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("got %q, want %q", string(data), content)
	}
}
