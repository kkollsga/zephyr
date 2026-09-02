package ui

import (
	"os"
	"path/filepath"
	"slices"
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
		t.Fatalf("top result = %q, want src/editor.go", ff.Results[0].Text)
	}
}

// The scan lists slash paths on every platform. filepath.Join builds the
// nested path with the host separator, so on Windows the walk hands the finder
// "src\editor.go"; listing that verbatim breaks a query typed with "/",
// because the matcher is a plain subsequence test over the displayed string.
func TestFuzzyFinder_ListsSlashPathsAndOpensHostPaths(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	ff.Open(dir)
	nested := filepath.Join("src", "editor.go")
	if slices.Contains(ff.Files, nested) && nested != "src/editor.go" {
		t.Fatalf("scan listed the host-separator path %q", nested)
	}
	if !slices.Contains(ff.Files, "src/editor.go") {
		t.Fatalf("scanned files = %v, want src/editor.go listed", ff.Files)
	}

	ff.UpdateQuery("src/ed")
	if len(ff.Results) == 0 || ff.Results[0].Text != "src/editor.go" {
		t.Fatalf("query \"src/ed\" gave %+v, want src/editor.go on top", ff.Results)
	}
	if got, want := ff.SelectedPath(), filepath.Join(dir, "src", "editor.go"); got != want {
		t.Fatalf("SelectedPath = %q, want the host-separator path %q", got, want)
	}
}

// The changed-file list reaches the same matcher, so it is normalised the same
// way and still opens to a host path.
func TestFuzzyFinder_ChangedFilesUseSlashPaths(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	ff.OpenChanged(dir, []string{filepath.Join("src", "editor.go")})
	if len(ff.Results) != 1 || ff.Results[0].Text != "src/editor.go" {
		t.Fatalf("changed-file results = %+v, want src/editor.go", ff.Results)
	}
	if got, want := ff.SelectedPath(), filepath.Join(dir, "src", "editor.go"); got != want {
		t.Fatalf("SelectedPath = %q, want %q", got, want)
	}
}

func TestToFinderPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		sep  rune
		want string
	}{
		{"windows separator becomes a slash", `src\editor.go`, '\\', "src/editor.go"},
		{"slash paths are untouched", "src/editor.go", '/', "src/editor.go"},
		{"a backslash is a filename byte on posix", `we\ird.go`, '/', `we\ird.go`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := toFinderPath(tc.in, tc.sep); got != tc.want {
				t.Fatalf("toFinderPath(%q, %q) = %q, want %q", tc.in, tc.sep, got, tc.want)
			}
		})
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

// TestFuzzyFinder_ChangedThenAllFilesRescans pins the cache interaction: a
// changed-files open replaces Files with the changed list, and the next
// all-files open on the same root must not reuse it.
func TestFuzzyFinder_ChangedThenAllFilesRescans(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	ff.OpenChanged(dir, []string{"main.go"})
	if len(ff.Results) != 1 {
		t.Fatalf("changed-files open listed %d results, want 1", len(ff.Results))
	}
	ff.Close()
	ff.Open(dir)
	if len(ff.Files) < 4 {
		t.Fatalf("all-files open listed %d files, want the whole scan", len(ff.Files))
	}
}
