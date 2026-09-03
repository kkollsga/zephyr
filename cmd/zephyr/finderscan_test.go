package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kristianweb/zephyr/internal/git"
)

// finderScanRepo builds a repository whose worktree exercises every case the
// scanner has to tell apart: a tracked file, an untracked one, a file an
// ignore rule hides, and a tracked file removed from the worktree but still in
// the index.
func finderScanRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := git.RunSilent(dir, "init"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	git.RunSilent(dir, "config", "user.email", "test@test.com")
	git.RunSilent(dir, "config", "user.name", "Test")

	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "build/\n")
	write("main.go", "package main\n")
	write("gone.go", "package main\n")
	write("build/out.txt", "artifact\n")
	if err := git.RunSilent(dir, "add", ".gitignore", "main.go", "gone.go"); err != nil {
		t.Fatal(err)
	}
	if err := git.RunSilent(dir, "commit", "-m", "initial"); err != nil {
		t.Fatal(err)
	}
	// Removed from the worktree, still in the index: git lists it, but the
	// finder must not offer a path that cannot be opened.
	if err := os.Remove(filepath.Join(dir, "gone.go")); err != nil {
		t.Fatal(err)
	}
	write("notes.md", "untracked\n")
	return dir
}

// The finder lists what git lists: an ignored file is absent, an untracked one
// is present. The plain walk had no way to know about .gitignore, so build/
// showed up in every result list.
func TestGitFinderScanHonoursGitignore(t *testing.T) {
	dir := finderScanRepo(t)
	files := gitFinderScan(dir, nil)

	if slices.Contains(files, "build/out.txt") {
		t.Errorf("an ignored file is listed: %v", files)
	}
	if !slices.Contains(files, "main.go") {
		t.Errorf("the tracked file is missing: %v", files)
	}
	if !slices.Contains(files, "notes.md") {
		t.Errorf("the untracked file is missing: %v", files)
	}
}

// git ls-files reports the index, which still holds a file deleted from the
// worktree. Opening it would fail, so it is filtered out.
func TestGitFinderScanDropsWorktreeDeletions(t *testing.T) {
	dir := finderScanRepo(t)
	if files := gitFinderScan(dir, nil); slices.Contains(files, "gone.go") {
		t.Errorf("a file deleted from the worktree is listed: %v", files)
	}
}

// A root git knows nothing about still lists its files: the walk is the
// fallback, not a second-class path.
func TestGitFinderScanFallsBackOutsideARepository(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"main.go", "src/editor.go"} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := gitFinderScan(dir, nil)
	if !slices.Contains(files, "main.go") || !slices.Contains(files, "src/editor.go") {
		t.Fatalf("a non-git root listed %v, want both files", files)
	}
}
