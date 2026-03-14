package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kristianweb/zephyr/internal/editor"
)

func TestTabBar_OpenFile_CreatesTab(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	tb := NewTabBar()
	_, err := tb.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if tb.TabCount() != 1 {
		t.Fatalf("expected 1 tab, got %d", tb.TabCount())
	}
}

func TestTabBar_OpenSameFile_SwitchesToExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	tb := NewTabBar()
	tb.OpenFile(path)
	tb.OpenFile(path)
	if tb.TabCount() != 1 {
		t.Fatalf("expected 1 tab (reuse), got %d", tb.TabCount())
	}
}

func TestTabBar_CloseTab_RemovesTab(t *testing.T) {
	tb := NewTabBar()
	ed := editor.NewEmptyEditor()
	tb.OpenEditor(ed, "untitled")
	if !tb.CloseTab(0) {
		t.Fatal("expected close to succeed")
	}
	if tb.TabCount() != 0 {
		t.Fatalf("expected 0 tabs, got %d", tb.TabCount())
	}
}

func TestTabBar_CloseTab_ModifiedFile_PromptsSave(t *testing.T) {
	tb := NewTabBar()
	ed := editor.NewEmptyEditor()
	ed.Modified = true
	tb.OpenEditor(ed, "modified")
	if tb.CloseTab(0) {
		t.Fatal("expected close to fail for modified file")
	}
	if tb.TabCount() != 1 {
		t.Fatalf("tab should still be open")
	}
}

func TestTabBar_CloseLastTab(t *testing.T) {
	tb := NewTabBar()
	ed := editor.NewEmptyEditor()
	tb.OpenEditor(ed, "only")
	tb.CloseTab(0)
	if tb.ActiveEditor() != nil {
		t.Fatal("expected nil active editor after closing last tab")
	}
}

func TestTabBar_SwitchTabs(t *testing.T) {
	tb := NewTabBar()
	ed1 := editor.NewEmptyEditor()
	ed2 := editor.NewEmptyEditor()
	tb.OpenEditor(ed1, "tab1")
	tb.OpenEditor(ed2, "tab2")

	tb.SwitchToTab(0)
	if tb.ActiveEditor() != ed1 {
		t.Fatal("expected ed1")
	}
	tb.SwitchToTab(1)
	if tb.ActiveEditor() != ed2 {
		t.Fatal("expected ed2")
	}
}
