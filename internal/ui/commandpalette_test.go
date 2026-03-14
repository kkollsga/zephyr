package ui

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/command"
)

func testRegistry() *command.Registry {
	reg := command.NewRegistry()
	reg.Register(&command.Command{ID: "file.save", Title: "File: Save"})
	reg.Register(&command.Command{ID: "file.open", Title: "File: Open"})
	reg.Register(&command.Command{ID: "edit.undo", Title: "Edit: Undo"})
	reg.Register(&command.Command{ID: "view.sidebar", Title: "View: Toggle Sidebar"})
	return reg
}

func TestCommandPalette_Open(t *testing.T) {
	cp := NewCommandPalette(testRegistry())
	cp.Open()
	if !cp.Visible {
		t.Fatal("expected visible")
	}
	if len(cp.Results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(cp.Results))
	}
}

func TestCommandPalette_Close(t *testing.T) {
	cp := NewCommandPalette(testRegistry())
	cp.Open()
	cp.Close()
	if cp.Visible {
		t.Fatal("expected not visible")
	}
}

func TestCommandPalette_Filter(t *testing.T) {
	cp := NewCommandPalette(testRegistry())
	cp.Open()
	cp.UpdateQuery("file")
	if len(cp.Results) < 2 {
		t.Fatalf("expected at least 2 file results, got %d", len(cp.Results))
	}
}

func TestCommandPalette_Navigation(t *testing.T) {
	cp := NewCommandPalette(testRegistry())
	cp.Open()
	cp.MoveDown()
	if cp.Selected != 1 {
		t.Fatalf("expected selected=1, got %d", cp.Selected)
	}
	cp.MoveUp()
	if cp.Selected != 0 {
		t.Fatalf("expected selected=0, got %d", cp.Selected)
	}
	cp.MoveUp() // should not go below 0
	if cp.Selected != 0 {
		t.Fatalf("expected selected=0, got %d", cp.Selected)
	}
}

func TestCommandPalette_Execute(t *testing.T) {
	reg := command.NewRegistry()
	executed := false
	reg.Register(&command.Command{
		ID:      "test",
		Title:   "Test",
		Handler: func() error { executed = true; return nil },
	})
	cp := NewCommandPalette(reg)
	cp.Open()
	cp.Execute()
	if !executed {
		t.Fatal("expected handler to be called")
	}
	if cp.Visible {
		t.Fatal("expected palette to close after execute")
	}
}
