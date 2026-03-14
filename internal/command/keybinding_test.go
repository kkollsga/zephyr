package command

import (
	"testing"

	"gioui.org/io/key"
)

func TestKeybinding_Parse_CmdS(t *testing.T) {
	name, mods, err := ParseKeybinding("Cmd+S")
	if err != nil {
		t.Fatal(err)
	}
	if name != "S" {
		t.Fatalf("got name %q, want S", name)
	}
	if mods != key.ModShortcut {
		t.Fatalf("got mods %v, want ModShortcut", mods)
	}
}

func TestKeybinding_Parse_CmdShiftP(t *testing.T) {
	name, mods, err := ParseKeybinding("Cmd+Shift+P")
	if err != nil {
		t.Fatal(err)
	}
	if name != "P" {
		t.Fatalf("got name %q, want P", name)
	}
	if mods != key.ModShortcut|key.ModShift {
		t.Fatalf("got mods %v, want ModShortcut|ModShift", mods)
	}
}

func TestKeybinding_Match_GioKeyEvent(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{ID: "file.save", Title: "Save"})
	km := NewKeybindingManager(reg)
	km.Bind(Keybinding{Key: "S", Modifiers: key.ModShortcut, CommandID: "file.save"})

	cmdID := km.Match("S", key.ModShortcut)
	if cmdID != "file.save" {
		t.Fatalf("got %q, want file.save", cmdID)
	}
}

func TestKeybinding_NoMatch(t *testing.T) {
	reg := NewRegistry()
	km := NewKeybindingManager(reg)
	cmdID := km.Match("X", key.ModShortcut)
	if cmdID != "" {
		t.Fatalf("expected no match, got %q", cmdID)
	}
}

func TestKeybinding_Conflict_Detection(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{ID: "cmd1", Title: "Command 1"})
	km := NewKeybindingManager(reg)
	km.Bind(Keybinding{Key: "S", Modifiers: key.ModShortcut, CommandID: "cmd1"})

	conflict, existing := km.HasConflict("S", key.ModShortcut)
	if !conflict {
		t.Fatal("expected conflict")
	}
	if existing != "cmd1" {
		t.Fatalf("got %q", existing)
	}
}

func TestKeybinding_Execute(t *testing.T) {
	reg := NewRegistry()
	executed := false
	reg.Register(&Command{
		ID:      "test",
		Title:   "Test",
		Handler: func() error { executed = true; return nil },
	})
	km := NewKeybindingManager(reg)
	km.Bind(Keybinding{Key: "T", Modifiers: key.ModShortcut, CommandID: "test"})

	found, err := km.Execute("T", key.ModShortcut)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected command to be found")
	}
	if !executed {
		t.Fatal("handler was not called")
	}
}
