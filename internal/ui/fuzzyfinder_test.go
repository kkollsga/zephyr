package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func setupFinderDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "editor.go"), []byte("package src"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "buffer.go"), []byte("package src"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0644)
	return dir
}

func TestFuzzyFinder_Open(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	ff.Open(dir)
	if !ff.Visible {
		t.Fatal("expected visible")
	}
	if len(ff.Files) == 0 {
		t.Fatal("expected files to be scanned")
	}
}

func TestFuzzyFinder_Filter(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	ff.Open(dir)
	ff.UpdateQuery("editor")
	if len(ff.Results) == 0 {
		t.Fatal("expected matches for 'editor'")
	}
	if ff.Results[0].Text != "src/editor.go" {
		t.Logf("top result: %s", ff.Results[0].Text)
	}
}

func TestFuzzyFinder_Navigation(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	ff.Open(dir)
	ff.MoveDown()
	if ff.Selected != 1 {
		t.Fatalf("expected selected=1, got %d", ff.Selected)
	}
}

func TestFuzzyFinder_SelectedPath(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	ff.Open(dir)
	path := ff.SelectedPath()
	if path == "" {
		t.Fatal("expected selected path")
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got %s", path)
	}
}

func TestFuzzyFinder_Close(t *testing.T) {
	ff := NewFuzzyFinder()
	ff.Open("/tmp")
	ff.Close()
	if ff.Visible {
		t.Fatal("expected not visible")
	}
}
