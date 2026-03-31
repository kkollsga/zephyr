package navigator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kristianweb/zephyr/internal/git"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var)
	dir, _ = filepath.EvalSymlinks(dir)

	// Create some files and directories
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.MkdirAll(filepath.Join(dir, "adir"), 0755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "util.go"), []byte("package util"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Hello"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0644)
	return dir
}

func TestDirBuffer_GenerateText(t *testing.T) {
	dir := setupTestDir(t)
	db := NewDirBuffer(dir, nil, nil, "")
	text := db.GenerateText()

	// Should have header
	if !strings.HasPrefix(text, dir) {
		t.Errorf("text should start with dir path, got: %s", text[:50])
	}

	// Should have separator
	if !strings.Contains(text, "──") {
		t.Error("text should contain separator")
	}

	// Directories should come first
	lines := strings.Split(text, "\n")
	var entryLines []string
	for _, l := range lines[2:] { // skip header + separator
		if l != "" {
			entryLines = append(entryLines, l)
		}
	}

	if len(entryLines) < 4 {
		t.Fatalf("expected at least 4 entries, got %d", len(entryLines))
	}

	// First entries should be directories (contain /)
	dirCount := 0
	for _, l := range entryLines {
		if strings.Contains(l, "/") {
			dirCount++
		} else {
			break
		}
	}
	if dirCount < 2 {
		t.Errorf("expected at least 2 directories first, got %d", dirCount)
	}
}

func TestDirBuffer_HiddenFiles(t *testing.T) {
	dir := setupTestDir(t)

	// Default: hidden files filtered
	db := NewDirBuffer(dir, nil, nil, "")
	for _, e := range db.Entries {
		if strings.HasPrefix(e.Name, ".") {
			t.Errorf("hidden file %q should be filtered", e.Name)
		}
	}

	// ShowHidden: hidden files included
	db.ShowHidden = true
	db.Entries = nil
	db.loadEntries(nil, nil, "")
	found := false
	for _, e := range db.Entries {
		if e.Name == ".hidden" {
			found = true
		}
	}
	if !found {
		t.Error("expected .hidden when ShowHidden=true")
	}
}

func TestDirBuffer_EntryAtLine(t *testing.T) {
	dir := setupTestDir(t)
	db := NewDirBuffer(dir, nil, nil, "")

	// Line 0 = header, line 1 = separator
	if db.EntryAtLine(0) != nil {
		t.Error("line 0 (header) should return nil")
	}
	if db.EntryAtLine(1) != nil {
		t.Error("line 1 (separator) should return nil")
	}

	// Line 2 = first entry
	e := db.EntryAtLine(2)
	if e == nil {
		t.Fatal("line 2 should return an entry")
	}
	// First entry should be a directory (sorted first)
	if !e.IsDir {
		t.Errorf("first entry %q should be a directory", e.Name)
	}

	// Out of bounds
	if db.EntryAtLine(1000) != nil {
		t.Error("out of bounds should return nil")
	}
	if db.EntryAtLine(-1) != nil {
		t.Error("negative line should return nil")
	}
}

func TestDirBuffer_SortOrder(t *testing.T) {
	dir := setupTestDir(t)
	db := NewDirBuffer(dir, nil, nil, "")

	// Find where directories end and files begin
	lastDir := -1
	for i, e := range db.Entries {
		if e.IsDir {
			lastDir = i
		}
	}

	// All directories should come before any files
	for i, e := range db.Entries {
		if !e.IsDir && i <= lastDir {
			t.Errorf("file %q at index %d appears before directory at index %d", e.Name, i, lastDir)
		}
	}

	// Directories should be alphabetically sorted
	for i := 1; i <= lastDir && lastDir > 0; i++ {
		if strings.ToLower(db.Entries[i].Name) < strings.ToLower(db.Entries[i-1].Name) {
			t.Errorf("directories not sorted: %q before %q", db.Entries[i-1].Name, db.Entries[i].Name)
		}
	}
}

func TestDirBuffer_GitStatus(t *testing.T) {
	dir := setupTestDir(t)
	statuses := []git.FileStatus{
		{Path: "main.go", Index: ' ', Worktree: 'M'},
		{Path: "util.go", Index: 'A', Worktree: ' '},
	}
	diffStat := map[string][2]int{
		"main.go": {10, 3},
	}

	db := NewDirBuffer(dir, statuses, diffStat, dir)

	// Check git status annotations
	for _, e := range db.Entries {
		switch e.Name {
		case "main.go":
			if e.GitStatus != 'M' {
				t.Errorf("main.go: status = %c, want M", e.GitStatus)
			}
			if e.Added != 10 || e.Deleted != 3 {
				t.Errorf("main.go: stats = +%d -%d, want +10 -3", e.Added, e.Deleted)
			}
		case "util.go":
			if e.GitStatus != 'A' {
				t.Errorf("util.go: status = %c, want A", e.GitStatus)
			}
		case "readme.md":
			if e.GitStatus != ' ' {
				t.Errorf("readme.md: status = %c, want ' '", e.GitStatus)
			}
		}
	}

	// Check text output contains stats
	text := db.GenerateText()
	if !strings.Contains(text, "+10 -3") {
		t.Error("generated text should contain diff stats")
	}
}
