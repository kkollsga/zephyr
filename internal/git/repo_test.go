package git

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repo with an initial commit.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var)
	dir, _ = filepath.EvalSymlinks(dir)

	// git init
	if err := RunSilent(dir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	// Configure user for commits
	RunSilent(dir, "config", "user.email", "test@test.com")
	RunSilent(dir, "config", "user.name", "Test")

	// Create initial file and commit
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RunSilent(dir, "add", "README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := RunSilent(dir, "commit", "-m", "initial commit"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return dir, func() {}
}

func TestDiscover(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if repo == nil {
		t.Fatal("expected repo, got nil")
	}
	if repo.Root != dir {
		t.Errorf("Root = %q, want %q", repo.Root, dir)
	}
}

func TestDiscover_Subdirectory(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	sub := filepath.Join(dir, "sub", "dir")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	repo, err := Discover(sub)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if repo == nil {
		t.Fatal("expected repo, got nil")
	}
	if repo.Root != dir {
		t.Errorf("Root = %q, want %q", repo.Root, dir)
	}
}

func TestDiscover_NotGit(t *testing.T) {
	dir := t.TempDir()
	repo, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if repo != nil {
		t.Errorf("expected nil repo for non-git dir, got %+v", repo)
	}
}

func TestHead(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := Discover(dir)
	branch, hash, err := repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	// Default branch may be main or master depending on git config
	if branch == "" {
		t.Error("expected non-empty branch")
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestStatus_Modified(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Modify the file
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repo, _ := Discover(dir)
	statuses, err := repo.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Path != "README.md" {
		t.Errorf("path = %q, want README.md", statuses[0].Path)
	}
	if statuses[0].Worktree != 'M' {
		t.Errorf("worktree = %c, want M", statuses[0].Worktree)
	}
}

func TestStatus_Untracked(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package new\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repo, _ := Discover(dir)
	statuses, err := repo.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Index != '?' {
		t.Errorf("index = %c, want ?", statuses[0].Index)
	}
}

func TestDiff(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Modify the file
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Changed\nNew line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repo, _ := Discover(dir)
	diffs, err := repo.Diff("HEAD")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Path != "README.md" {
		t.Errorf("path = %q, want README.md", diffs[0].Path)
	}
	if len(diffs[0].Hunks) == 0 {
		t.Error("expected at least 1 hunk")
	}
}

func TestShow(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Modify the file
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Changed\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repo, _ := Discover(dir)
	original, err := repo.Show("HEAD", "README.md")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if string(original) != "# Test\n" {
		t.Errorf("original = %q, want %q", string(original), "# Test\n")
	}
}

func TestStageUnstage(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a new file
	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package new\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repo, _ := Discover(dir)

	// Stage
	if err := repo.Stage("new.go"); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	statuses, _ := repo.Status()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Index != 'A' {
		t.Errorf("after stage: index = %c, want A", statuses[0].Index)
	}

	// Unstage
	if err := repo.Unstage("new.go"); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	statuses, _ = repo.Status()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Index != '?' {
		t.Errorf("after unstage: index = %c, want ?", statuses[0].Index)
	}
}

func TestDiffStat(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Modify the file
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Changed\nNew line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repo, _ := Discover(dir)
	stat, err := repo.DiffStat("HEAD")
	if err != nil {
		t.Fatalf("DiffStat: %v", err)
	}
	counts, ok := stat["README.md"]
	if !ok {
		t.Fatal("expected README.md in stat")
	}
	if counts[0] == 0 && counts[1] == 0 {
		t.Error("expected non-zero stat counts")
	}
}
