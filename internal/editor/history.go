package editor

import "time"

// ActionType represents the type of edit action.
type ActionType int

const (
	ActionInsert ActionType = iota
	ActionDelete
)

// EditAction represents a single edit operation for undo/redo.
type EditAction struct {
	Type      ActionType
	Offset    int
	Text      string
	Cursor    Cursor // cursor position before the action
	Timestamp time.Time
}

// History manages undo/redo stacks with operation coalescing.
type History struct {
	undoStack []EditAction
	redoStack []EditAction
	// Coalescing: group rapid sequential inserts/deletes into one action.
	coalesceWindow time.Duration
}

// NewHistory creates a new History with default coalescing window.
func NewHistory() *History {
	return &History{
		coalesceWindow: 300 * time.Millisecond,
	}
}

// Record adds an action to the undo stack. Clears the redo stack.
// Coalesces with the previous action if they are the same type and within
// the coalescing window.
func (h *History) Record(action EditAction) {
	action.Timestamp = time.Now()

	if len(h.undoStack) > 0 {
		last := &h.undoStack[len(h.undoStack)-1]
		if h.canCoalesce(last, &action) {
			h.coalesce(last, &action)
			h.redoStack = nil
			return
		}
	}

	h.undoStack = append(h.undoStack, action)
	h.redoStack = nil
}

func (h *History) canCoalesce(last, next *EditAction) bool {
	if last.Type != next.Type {
		return false
	}
	if next.Timestamp.Sub(last.Timestamp) > h.coalesceWindow {
		return false
	}
	switch next.Type {
	case ActionInsert:
		// Coalesce consecutive inserts
		return next.Offset == last.Offset+len(last.Text)
	case ActionDelete:
		// Coalesce consecutive backspace deletes
		return next.Offset == last.Offset-len(next.Text) || next.Offset == last.Offset
	}
	return false
}

func (h *History) coalesce(last, next *EditAction) {
	switch next.Type {
	case ActionInsert:
		last.Text += next.Text
		last.Timestamp = next.Timestamp
	case ActionDelete:
		if next.Offset < last.Offset {
			// Backspace: prepend
			last.Text = next.Text + last.Text
			last.Offset = next.Offset
		} else {
			// Forward delete: append
			last.Text += next.Text
		}
		last.Timestamp = next.Timestamp
	}
}

// Undo pops the top action from the undo stack and returns it.
// Returns nil if the stack is empty.
func (h *History) Undo() *EditAction {
	if len(h.undoStack) == 0 {
		return nil
	}
	action := h.undoStack[len(h.undoStack)-1]
	h.undoStack = h.undoStack[:len(h.undoStack)-1]
	h.redoStack = append(h.redoStack, action)
	return &action
}

// Redo pops the top action from the redo stack and returns it.
// Returns nil if the stack is empty.
func (h *History) Redo() *EditAction {
	if len(h.redoStack) == 0 {
		return nil
	}
	action := h.redoStack[len(h.redoStack)-1]
	h.redoStack = h.redoStack[:len(h.redoStack)-1]
	h.undoStack = append(h.undoStack, action)
	return &action
}

// CanUndo returns true if there are actions to undo.
func (h *History) CanUndo() bool {
	return len(h.undoStack) > 0
}

// CanRedo returns true if there are actions to redo.
func (h *History) CanRedo() bool {
	return len(h.redoStack) > 0
}
