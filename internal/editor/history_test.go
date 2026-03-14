package editor

import (
	"testing"
	"time"
)

func TestHistory_Undo_SingleInsert(t *testing.T) {
	h := NewHistory()
	h.Record(EditAction{Type: ActionInsert, Offset: 0, Text: "hello", Cursor: Cursor{Line: 0, Col: 0}})
	action := h.Undo()
	if action == nil {
		t.Fatal("expected action")
	}
	if action.Type != ActionInsert || action.Text != "hello" {
		t.Fatalf("got %+v", action)
	}
}

func TestHistory_Undo_SingleDelete(t *testing.T) {
	h := NewHistory()
	h.Record(EditAction{Type: ActionDelete, Offset: 5, Text: "world", Cursor: Cursor{Line: 0, Col: 5}})
	action := h.Undo()
	if action == nil {
		t.Fatal("expected action")
	}
	if action.Type != ActionDelete || action.Text != "world" {
		t.Fatalf("got %+v", action)
	}
}

func TestHistory_Redo_AfterUndo(t *testing.T) {
	h := NewHistory()
	h.Record(EditAction{Type: ActionInsert, Offset: 0, Text: "hello", Cursor: Cursor{Line: 0, Col: 0}})
	h.Undo()
	action := h.Redo()
	if action == nil {
		t.Fatal("expected action")
	}
	if action.Text != "hello" {
		t.Fatalf("got %q", action.Text)
	}
}

func TestHistory_Redo_ClearedByNewEdit(t *testing.T) {
	h := NewHistory()
	h.Record(EditAction{Type: ActionInsert, Offset: 0, Text: "hello"})
	h.Undo()
	// New edit should clear redo
	h.Record(EditAction{Type: ActionInsert, Offset: 0, Text: "world"})
	if h.CanRedo() {
		t.Fatal("redo should be cleared after new edit")
	}
}

func TestHistory_UndoCoalescing_RapidTyping(t *testing.T) {
	h := NewHistory()
	// Simulate rapid character-by-character typing
	for i, ch := range "hello" {
		h.Record(EditAction{
			Type:      ActionInsert,
			Offset:    i,
			Text:      string(ch),
			Cursor:    Cursor{Line: 0, Col: i},
			Timestamp: time.Now(),
		})
	}
	// All chars should be coalesced into one action
	action := h.Undo()
	if action == nil {
		t.Fatal("expected action")
	}
	if action.Text != "hello" {
		t.Fatalf("got %q, want %q", action.Text, "hello")
	}
	// Nothing more to undo
	if h.CanUndo() {
		t.Fatal("expected empty undo stack")
	}
}

func TestHistory_Undo_MultipleSteps(t *testing.T) {
	h := NewHistory()
	h.Record(EditAction{Type: ActionInsert, Offset: 0, Text: "first"})
	time.Sleep(400 * time.Millisecond) // exceed coalescing window
	h.Record(EditAction{Type: ActionInsert, Offset: 5, Text: "second"})

	a1 := h.Undo()
	if a1.Text != "second" {
		t.Fatalf("first undo: got %q, want %q", a1.Text, "second")
	}
	a2 := h.Undo()
	if a2.Text != "first" {
		t.Fatalf("second undo: got %q, want %q", a2.Text, "first")
	}
}

func TestHistory_Undo_EmptyStack_NoOp(t *testing.T) {
	h := NewHistory()
	if h.Undo() != nil {
		t.Fatal("expected nil from empty stack")
	}
}

func TestHistory_Undo_RestoresCursorPosition(t *testing.T) {
	h := NewHistory()
	cursor := Cursor{Line: 5, Col: 10}
	h.Record(EditAction{Type: ActionInsert, Offset: 0, Text: "x", Cursor: cursor})
	action := h.Undo()
	if action.Cursor != cursor {
		t.Fatalf("got cursor %+v, want %+v", action.Cursor, cursor)
	}
}
