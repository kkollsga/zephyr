package main

import (
	"os"
	"path/filepath"
	"testing"

	"gioui.org/io/key"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/git"
	"github.com/kristianweb/zephyr/internal/ui"
	"github.com/kristianweb/zephyr/internal/vim"
)

const (
	headText    = "committed one\ncommitted two\ncommitted three\n"
	workingText = "committed one\nworking two\ncommitted three\n"
)

// headViewRepo commits headText as sample.txt, then leaves workingText on disk.
func headViewRepo(t *testing.T) (*git.Repo, string) {
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
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte(headText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := git.RunSilent(dir, "add", "sample.txt"); err != nil {
		t.Fatal(err)
	}
	if err := git.RunSilent(dir, "commit", "-m", "initial"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(workingText), 0o644); err != nil {
		t.Fatal(err)
	}
	repo, err := git.Discover(dir)
	if err != nil || repo == nil {
		t.Fatalf("Discover: %v", err)
	}
	return repo, path
}

// headViewApp opens the working copy of the committed file in a tab.
func headViewApp(t *testing.T) (*appState, *editor.Editor, *tabState, *ui.Tab, string) {
	t.Helper()
	repo, path := headViewRepo(t)
	st, ed, ts := testAppWithText(workingText, "Plain Text")
	st.vimState = vim.NewState()
	st.gitRepo = repo
	ed.FilePath = path
	tab := st.tabBar.ActiveTab()
	tab.Title = "sample.txt"
	tab.IsUntitled = false
	return st, ed, ts, tab, path
}

func TestHeadViewRoundTripRestoresTheWorkingBuffer(t *testing.T) {
	st, ed, ts, tab, path := headViewApp(t)
	// An edit so the round trip has a history and a modified flag to preserve.
	ed.Cursor.SetPosition(ed.Buffer, 1, 0)
	ed.InsertText("edited ")
	ed.Modified = true
	before := ed.Buffer.Text()
	beforeCursor := ed.Cursor
	beforeUndo := ed.History.CanUndo()
	if !beforeUndo {
		t.Fatal("fixture edit recorded no undo step")
	}

	st.navToggleOriginal()
	if ts.bufType != bufOriginal {
		t.Fatalf("bufType = %v, want bufOriginal", ts.bufType)
	}
	if got := ed.Buffer.Text(); got != headText {
		t.Fatalf("HEAD view text = %q, want the committed content", got)
	}
	if tab.Title != "sample.txt (HEAD)" {
		t.Fatalf("tab title = %q", tab.Title)
	}
	if !ed.Modified {
		t.Fatal("HEAD view cleared Modified, which would let the tab close without a prompt")
	}

	st.navToggleOriginal()
	if ts.bufType != bufFile {
		t.Fatalf("bufType after the round trip = %v, want bufFile", ts.bufType)
	}
	if got := ed.Buffer.Text(); got != before {
		t.Fatalf("restored text = %q, want %q", got, before)
	}
	if ed.Cursor.Line != beforeCursor.Line || ed.Cursor.Col != beforeCursor.Col {
		t.Fatalf("restored cursor = %+v, want line %d col %d", ed.Cursor, beforeCursor.Line, beforeCursor.Col)
	}
	if !ed.Modified {
		t.Fatal("restored buffer lost its Modified flag")
	}
	if ed.History.CanUndo() != beforeUndo {
		t.Fatal("the HEAD round trip touched the undo history")
	}
	if tab.Title != "sample.txt" {
		t.Fatalf("restored tab title = %q", tab.Title)
	}
	// The undo history still describes the restored buffer.
	st.undoSteps(1)
	if got := ed.Buffer.Text(); got != workingText {
		t.Fatalf("undo after the round trip = %q, want %q", got, workingText)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != workingText {
		t.Fatalf("file on disk = %q (err %v), want it untouched", data, err)
	}
}

func TestHeadViewSwallowsEveryEditPath(t *testing.T) {
	st, ed, _, tab, path := headViewApp(t)
	st.navToggleOriginal()
	undoBefore := ed.History.CanUndo()

	edits := map[string]func(){
		"typing":          func() { st.handleTextInput("x") },
		"backspace":       func() { st.handleKey(key.Event{Name: key.NameDeleteBackward}) },
		"forward delete":  func() { st.handleKey(key.Event{Name: key.NameDeleteForward}) },
		"return":          func() { st.handleKey(key.Event{Name: key.NameReturn}) },
		"tab":             func() { st.handleKey(key.Event{Name: key.NameTab}) },
		"native paste":    func() { st.handleKey(key.Event{Name: "V", Modifiers: key.ModShortcut}) },
		"native cut":      func() { st.handleKey(key.Event{Name: "X", Modifiers: key.ModShortcut}) },
		"native undo":     func() { st.handleKey(key.Event{Name: "Z", Modifiers: key.ModShortcut}) },
		"native redo":     func() { st.handleKey(key.Event{Name: "Z", Modifiers: key.ModShortcut | key.ModShift}) },
		"vim dd":          func() { st.executeVimAction(vim.Action{Kind: vim.ActionDelete, MotionType: vim.MotionLineWise}) },
		"vim put":         func() { st.executeVimAction(vim.Action{Kind: vim.ActionPut}) },
		"vim undo":        func() { st.executeVimAction(vim.Action{Kind: vim.ActionUndo}) },
		"vim redo":        func() { st.executeVimAction(vim.Action{Kind: vim.ActionRedo}) },
		"vim insert mode": func() { st.executeVimAction(vim.Action{Kind: vim.ActionInsertBefore}) },
		"vim join":        func() { st.executeVimAction(vim.Action{Kind: vim.ActionJoinLines}) },
	}
	for name, edit := range edits {
		ed.Cursor.SetPosition(ed.Buffer, 1, 3)
		ed.Selection.Clear()
		edit()
		if got := ed.Buffer.Text(); got != headText {
			t.Errorf("%s changed the HEAD view: %q", name, got)
		}
		if ed.History.CanUndo() != undoBefore {
			t.Errorf("%s touched the undo history", name)
		}
	}

	if st.saveTab(tab) {
		t.Fatal("a save in the HEAD view was accepted")
	}
	if st.notification == "" {
		t.Fatal("the refused save said nothing")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != workingText {
		t.Fatalf("file on disk = %q (err %v), want the working copy untouched", data, err)
	}
}

func TestHeadViewRefusesAFileWithNoCommittedContent(t *testing.T) {
	st, ed, ts, tab, _ := headViewApp(t)
	untracked := filepath.Join(st.gitRepo.Root, "untracked.txt")
	if err := os.WriteFile(untracked, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ed.FilePath = untracked

	st.navToggleOriginal()
	if ts.bufType != bufFile {
		t.Fatalf("bufType = %v, want the buffer left alone", ts.bufType)
	}
	if got := ed.Buffer.Text(); got != workingText {
		t.Fatalf("buffer changed for a file with no HEAD content: %q", got)
	}
	if tab.Title != "sample.txt" {
		t.Fatalf("tab title = %q, want it unchanged", tab.Title)
	}
	if st.notification == "" {
		t.Fatal("a file with no HEAD content was refused silently")
	}
}
