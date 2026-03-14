package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Create structure:
	// dir/
	//   adir/
	//     nested.txt
	//   bdir/
	//   alpha.go
	//   beta.txt
	//   .hidden
	os.MkdirAll(filepath.Join(dir, "adir"), 0755)
	os.MkdirAll(filepath.Join(dir, "bdir"), 0755)
	os.WriteFile(filepath.Join(dir, "adir", "nested.txt"), []byte("nested"), 0644)
	os.WriteFile(filepath.Join(dir, "alpha.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "beta.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0644)
	return dir
}

func TestFileTree_LoadDirectory(t *testing.T) {
	dir := setupTestDir(t)
	ft, err := NewFileTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ft.Root == nil {
		t.Fatal("expected root node")
	}
	if len(ft.Root.Children) == 0 {
		t.Fatal("expected children")
	}
}

func TestFileTree_ExpandDir(t *testing.T) {
	dir := setupTestDir(t)
	ft, err := NewFileTree(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Find adir
	var adir *FileNode
	for _, child := range ft.Root.Children {
		if child.Name == "adir" {
			adir = child
			break
		}
	}
	if adir == nil {
		t.Fatal("expected adir")
	}

	// Expand
	ft.ToggleExpand(adir)
	if !adir.Expanded {
		t.Fatal("expected adir to be expanded")
	}
	if len(adir.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(adir.Children))
	}
}

func TestFileTree_CollapseDir(t *testing.T) {
	dir := setupTestDir(t)
	ft, err := NewFileTree(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Root starts expanded
	ft.ToggleExpand(ft.Root)
	if ft.Root.Expanded {
		t.Fatal("expected root to be collapsed")
	}
}

func TestFileTree_SortOrder(t *testing.T) {
	dir := setupTestDir(t)
	ft, err := NewFileTree(dir)
	if err != nil {
		t.Fatal(err)
	}

	children := ft.Root.Children
	// Directories first
	dirsSeen := false
	filesSeen := false
	for _, c := range children {
		if c.IsDir {
			if filesSeen {
				t.Fatal("directories should come before files")
			}
			dirsSeen = true
		} else {
			filesSeen = true
		}
	}
	if !dirsSeen || !filesSeen {
		t.Fatal("expected both dirs and files")
	}
}

func TestFileTree_HiddenFiles_Filtered(t *testing.T) {
	dir := setupTestDir(t)
	ft, err := NewFileTree(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, child := range ft.Root.Children {
		if child.Name == ".hidden" {
			t.Fatal("hidden files should be filtered")
		}
	}
}
