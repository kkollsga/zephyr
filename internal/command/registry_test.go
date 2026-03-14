package command

import "testing"

func TestRegistry_RegisterAndExecute(t *testing.T) {
	reg := NewRegistry()
	executed := false
	reg.Register(&Command{
		ID:      "test.cmd",
		Title:   "Test Command",
		Handler: func() error { executed = true; return nil },
	})
	if err := reg.Execute("test.cmd"); err != nil {
		t.Fatal(err)
	}
	if !executed {
		t.Fatal("handler was not called")
	}
}

func TestRegistry_Search_ByTitle(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{ID: "file.save", Title: "File: Save"})
	reg.Register(&Command{ID: "file.open", Title: "File: Open"})
	reg.Register(&Command{ID: "edit.undo", Title: "Edit: Undo"})

	results := reg.Search("file")
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
}

func TestRegistry_Search_Fuzzy(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{ID: "view.commandPalette", Title: "View: Command Palette"})
	reg.Register(&Command{ID: "file.save", Title: "File: Save"})

	results := reg.Search("cmdpal")
	if len(results) == 0 {
		t.Fatal("expected fuzzy match for 'cmdpal'")
	}
}

func TestRegistry_DuplicateID_Error(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{ID: "dup", Title: "First"})
	err := reg.Register(&Command{ID: "dup", Title: "Second"})
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

func TestRegistry_ExecuteNotFound(t *testing.T) {
	reg := NewRegistry()
	err := reg.Execute("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}
