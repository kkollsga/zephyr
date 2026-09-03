package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kristianweb/zephyr/internal/vim"
)

// gfApp puts a one-line file at <root>/<rel> whose text quotes target, with the
// cursor inside the quotes, and returns the app.
func gfApp(t *testing.T, rel, target string) (*appState, string) {
	t.Helper()
	repo, _ := headViewRepo(t)
	src := filepath.Join(repo.Root, rel)
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	line := "x := \"" + target + "\""
	if err := os.WriteFile(src, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, ed, _ := testAppWithText(line+"\n", "Go")
	st.vimState = vim.NewState()
	st.gitRepo = repo
	ed.FilePath = src
	ed.Cursor.SetPosition(ed.Buffer, 0, 8) // inside the quoted string
	return st, repo.Root
}

// gf resolves relative to the file before giving up, so a name that exists next
// to the open file but not at the repo root still opens. The repo-root
// candidate used to be checked with filepath.Glob, which returns a nil error
// for a literal path that does not exist, so this branch was unreachable
// whenever the file was in a repo.
func TestGoFileFallsBackToTheFileDirectory(t *testing.T) {
	st, root := gfApp(t, "pkg/a.go", "b.go")
	want := filepath.Join(root, "pkg", "b.go")
	if err := os.WriteFile(want, []byte("sibling\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "b.go")); err == nil {
		t.Fatal("the repo-root candidate must not exist for this test to mean anything")
	}

	st.navGoFile()

	if got := st.activeEd().FilePath; got != want {
		t.Fatalf("gf opened %q, want %q", got, want)
	}
}

// The repo-root candidate still wins when it does exist.
func TestGoFileResolvesRepoRootPath(t *testing.T) {
	st, root := gfApp(t, "pkg/a.go", "sample.txt")
	want := filepath.Join(root, "sample.txt")

	st.navGoFile()

	if got := st.activeEd().FilePath; got != want {
		t.Fatalf("gf opened %q, want %q", got, want)
	}
}

// A name that exists nowhere opens nothing rather than an empty tab.
func TestGoFileMissingEverywhereOpensNothing(t *testing.T) {
	st, _ := gfApp(t, "pkg/a.go", "nowhere.go")
	before := st.activeEd()

	st.navGoFile()

	if st.activeEd() != before {
		t.Fatalf("gf on a missing file opened %q", st.activeEd().FilePath)
	}
}
