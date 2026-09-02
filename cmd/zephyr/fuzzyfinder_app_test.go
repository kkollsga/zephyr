package main

import (
	"os"
	"path/filepath"
	"testing"

	"gioui.org/io/key"

	"github.com/kristianweb/zephyr/internal/git"
	"github.com/kristianweb/zephyr/internal/ui"
	"github.com/kristianweb/zephyr/internal/vim"
)

// finderApp builds an app whose navigator root is a small tree.
func finderApp(t *testing.T) (*appState, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	for path, body := range map[string]string{
		"main.go":           "package main\n",
		"README.md":         "# readme\n",
		"src/editor.go":     "package src\n",
		"src/buffer_ops.go": "package src\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, _, _ := testAppWithText("", "Plain Text")
	st.vimState = vim.NewState()
	st.fuzzyFinder = ui.NewFuzzyFinder()
	st.navRoot = dir
	st.lastMaxX, st.lastMaxY, st.tabBarHeight = 900, 600, 28
	return st, dir
}

func TestFuzzyFinderOpensFiltersAndOpensATab(t *testing.T) {
	st, dir := finderApp(t)

	st.executeVimAction(vim.Action{Kind: vim.ActionNavFindFiles})
	if !st.fuzzyFinder.Visible {
		t.Fatal("<Space>f did not open the finder")
	}
	if len(st.fuzzyFinder.Results) != 4 {
		t.Fatalf("finder listed %d files, want 4", len(st.fuzzyFinder.Results))
	}

	st.handleTextInput("editor")
	if q := st.fuzzyFinder.Query; q != "editor" {
		t.Fatalf("query = %q after typing", q)
	}
	if got := len(st.fuzzyFinder.Results); got == 0 || got == 4 {
		t.Fatalf("query narrowed to %d results, want fewer than all and more than none", got)
	}
	// The scan builds this path with the host separator; the finder lists and
	// matches it in slash form on every platform, so the assertion is the same
	// string everywhere.
	if top := st.fuzzyFinder.Results[0].Text; top != "src/editor.go" {
		t.Fatalf("top result = %q, want src/editor.go", top)
	}
	// A query typed with a slash reaches the nested file on Windows too.
	st.fuzzyFinder.UpdateQuery("src/ed")
	if len(st.fuzzyFinder.Results) == 0 || st.fuzzyFinder.Results[0].Text != "src/editor.go" {
		t.Fatalf("query \"src/ed\" gave %+v", st.fuzzyFinder.Results)
	}

	tabsBefore := len(st.tabBar.Tabs)
	st.handleKey(key.Event{Name: key.NameReturn})
	if st.fuzzyFinder.Visible {
		t.Fatal("Return left the finder open")
	}
	if len(st.tabBar.Tabs) != tabsBefore+1 {
		t.Fatalf("tab count = %d, want %d", len(st.tabBar.Tabs), tabsBefore+1)
	}
	want := filepath.Join(dir, "src", "editor.go")
	if got := st.tabBar.ActiveEditor().FilePath; got != want {
		t.Fatalf("opened %q, want %q", got, want)
	}
}

func TestFuzzyFinderKeyAndClickRouting(t *testing.T) {
	st, _ := finderApp(t)
	st.executeVimAction(vim.Action{Kind: vim.ActionNavFindFiles})

	st.handleKey(key.Event{Name: key.NameDownArrow})
	if st.fuzzyFinder.Selected != 1 {
		t.Fatalf("Down selected %d, want 1", st.fuzzyFinder.Selected)
	}
	st.handleKey(key.Event{Name: "N", Modifiers: key.ModCtrl})
	if st.fuzzyFinder.Selected != 2 {
		t.Fatalf("Ctrl+N selected %d, want 2", st.fuzzyFinder.Selected)
	}
	st.handleKey(key.Event{Name: "P", Modifiers: key.ModCtrl})
	st.handleKey(key.Event{Name: key.NameUpArrow})
	if st.fuzzyFinder.Selected != 0 {
		t.Fatalf("Ctrl+P then Up selected %d, want 0", st.fuzzyFinder.Selected)
	}

	st.handleTextInput("zz")
	st.handleKey(key.Event{Name: key.NameDeleteBackward})
	if st.fuzzyFinder.Query != "z" {
		t.Fatalf("backspace left query %q", st.fuzzyFinder.Query)
	}

	// A click outside the panel closes it; a click on a row opens that row.
	lay := st.fuzzyLayoutNow()
	if !st.handleFuzzyFinderClick(lay.x-5, lay.y-5) {
		t.Fatal("a click outside the finder was not consumed")
	}
	if st.fuzzyFinder.Visible {
		t.Fatal("a click outside the finder did not close it")
	}

	st.executeVimAction(vim.Action{Kind: vim.ActionNavFindFiles})
	lay = st.fuzzyLayoutNow()
	st.handleFuzzyFinderClick(lay.x+10, lay.listY+lay.itemH+1)
	if st.fuzzyFinder.Visible {
		t.Fatal("a click on a row left the finder open")
	}
	if st.tabBar.ActiveEditor().FilePath == "" {
		t.Fatal("a click on a row opened no file")
	}

	st.executeVimAction(vim.Action{Kind: vim.ActionNavFindFiles})
	st.handleKey(key.Event{Name: key.NameEscape})
	if st.fuzzyFinder.Visible {
		t.Fatal("Escape left the finder open")
	}
}

func TestFuzzyFinderYieldsToOtherOverlays(t *testing.T) {
	st, _ := finderApp(t)
	st.openFindBar(false)
	st.executeVimAction(vim.Action{Kind: vim.ActionNavFindFiles})
	if st.fuzzyFinder.Visible {
		t.Fatal("the finder opened over a focused find bar")
	}
	st.findBar.Close()

	st.saveMenu.visible = true
	st.executeVimAction(vim.Action{Kind: vim.ActionNavFindFiles})
	if st.fuzzyFinder.Visible {
		t.Fatal("the finder opened over the save menu")
	}
	st.saveMenu.visible = false

	st.executeVimAction(vim.Action{Kind: vim.ActionNavFindFiles})
	if !st.fuzzyFinder.Visible {
		t.Fatal("the finder did not open with no other overlay up")
	}
	// With the finder up, keys reach it rather than vim.
	st.vimEnabled = true
	st.dispatchKey(key.Event{Name: key.NameDownArrow})
	if st.fuzzyFinder.Selected != 1 {
		t.Fatal("vim swallowed a key belonging to the finder")
	}
}

func TestFuzzyFinderChangedFilesListsOnlyChangedFiles(t *testing.T) {
	repo, _ := headViewRepo(t)
	st, _, _ := testAppWithText(workingText, "Plain Text")
	st.vimState = vim.NewState()
	st.fuzzyFinder = ui.NewFuzzyFinder()
	st.gitRepo = repo
	st.gitCache = git.NewCache(repo)
	st.navRoot = repo.Root
	if err := os.WriteFile(filepath.Join(repo.Root, "untouched.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := git.RunSilent(repo.Root, "add", "untouched.txt"); err != nil {
		t.Fatal(err)
	}
	if err := git.RunSilent(repo.Root, "commit", "-m", "second"); err != nil {
		t.Fatal(err)
	}
	st.executeVimAction(vim.Action{Kind: vim.ActionNavFindChanged})
	if !st.fuzzyFinder.Visible {
		t.Fatal("<Space>b did not open the finder")
	}
	if len(st.fuzzyFinder.Results) != 1 || st.fuzzyFinder.Results[0].Text != "sample.txt" {
		t.Fatalf("changed-file results = %+v, want only sample.txt", st.fuzzyFinder.Results)
	}

	// The all-files finder over the same root still lists everything.
	st.fuzzyFinder.Close()
	st.executeVimAction(vim.Action{Kind: vim.ActionNavFindFiles})
	if len(st.fuzzyFinder.Results) < 2 {
		t.Fatalf("<Space>f after <Space>b listed %d files", len(st.fuzzyFinder.Results))
	}
}
