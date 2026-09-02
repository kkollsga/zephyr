package main

import (
	"os"
	"path/filepath"
	"testing"

	"gioui.org/io/key"
)

// overwriteApp opens a one-tab app on src and stages a Save As onto an
// existing target, leaving the menu in whatever sub-state executeSaveAs
// decides on.
func overwriteApp(t *testing.T, bufferText, targetText string) (*appState, string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "source.go")
	if err := os.WriteFile(src, []byte(bufferText), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte(targetText), 0o600); err != nil {
		t.Fatal(err)
	}
	st, _, _ := conflictTestApp(t, src)
	st.saveMenu.visible = true
	st.saveMenu.tabIdx = 0
	st.saveMenu.saveAsExpanded = true
	st.saveMenu.dir = dir
	st.saveMenu.filename = []rune("target.go")
	st.handleKey(key.Event{Name: key.NameReturn})
	if !st.saveMenu.confirmOverwrite {
		t.Fatalf("Save As onto an existing file did not raise the overwrite prompt")
	}
	return st, target
}

// TestOverwriteKey_EscapeGoesBackWithoutWriting pins decision 8's safe half:
// Escape leaves the sub-state and the target's bytes untouched.
func TestOverwriteKey_EscapeGoesBackWithoutWriting(t *testing.T) {
	const targetText = "package target\n"
	st, target := overwriteApp(t, "package source\n", targetText)

	st.handleKey(key.Event{Name: key.NameEscape})

	if st.saveMenu.confirmOverwrite {
		t.Fatal("Escape left the overwrite prompt up")
	}
	if !st.saveMenu.visible {
		t.Fatal("Escape closed the save menu instead of going back to the filename")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != targetText {
		t.Fatalf("Escape wrote the target: %q, want %q", got, targetText)
	}
}

// TestOverwriteKey_ReturnWritesTheBuffer is the other half: a second Return
// confirms, and the target ends up holding the buffer.
func TestOverwriteKey_ReturnWritesTheBuffer(t *testing.T) {
	const bufferText = "package source\n"
	st, target := overwriteApp(t, bufferText, "package target\n")

	st.handleKey(key.Event{Name: key.NameReturn})

	if st.saveMenu.confirmOverwrite {
		t.Fatal("Return left the overwrite prompt up")
	}
	if st.saveMenu.visible {
		t.Fatal("Return confirmed the overwrite but left the save menu open")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != bufferText {
		t.Fatalf("target after confirming = %q, want %q", got, bufferText)
	}
}

// TestOverwriteKey_EscapeThenReturnStillWrites covers the round trip the GUI
// scenario drives: back out once, then confirm.
func TestOverwriteKey_EscapeThenReturnStillWrites(t *testing.T) {
	const bufferText = "package source\n"
	st, target := overwriteApp(t, bufferText, "package target\n")

	st.handleKey(key.Event{Name: key.NameEscape})
	st.handleKey(key.Event{Name: key.NameReturn}) // re-raises the prompt
	if !st.saveMenu.confirmOverwrite {
		t.Fatal("Return after Escape did not raise the overwrite prompt again")
	}
	st.handleKey(key.Event{Name: key.NameReturn})

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != bufferText {
		t.Fatalf("target after Escape then confirm = %q, want %q", got, bufferText)
	}
}
