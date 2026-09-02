package editor

import "time"

// ActionType represents the type of edit action.
type ActionType int

const (
	ActionInsert ActionType = iota
	ActionDelete
	ActionReplace
	// ActionGroup holds several actions applied as one edit: undone in
	// reverse order, redone in the order they were recorded.
	ActionGroup
)

// EditAction represents a single edit operation for undo/redo.
type EditAction struct {
	Type        ActionType
	Offset      int
	Text        string
	Replacement string
	Cursor      Cursor       // cursor position before the action
	AfterCursor Cursor       // cursor position after a replacement or a group
	Group       []EditAction // the members of an ActionGroup, in applied order
	Timestamp   time.Time
}

// History manages undo/redo stacks with operation coalescing.
type History struct {
	undoStack []EditAction
	redoStack []EditAction
	// Coalescing: group rapid sequential inserts/deletes into one action.
	coalesceWindow time.Duration
	// Open transaction: while groupDepth > 0, actions collect in group
	// instead of reaching the undo stack.
	groupDepth int
	group      []EditAction
}

// NewHistory creates a new History with default coalescing window.
func NewHistory() *History {
	return &History{
		coalesceWindow: 1 * time.Second,
	}
}

// Record adds an action to the undo stack. Clears the redo stack.
// Coalesces with the previous action if they are the same type and within
// the coalescing window. Inside an open group (see BeginGroup) the action
// joins the group instead and never coalesces, so a group is neither merged
// into nor merged across: the next keystroke after one starts a fresh entry.
func (h *History) Record(action EditAction) {
	action.Timestamp = time.Now()

	if h.groupDepth > 0 {
		h.group = append(h.group, action)
		return
	}

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

// PeekUndo returns the action Undo would pop, without moving it. Callers that
// can fail to apply the action use this to leave the stacks untouched.
// The pointer is invalidated by the next stack mutation.
func (h *History) PeekUndo() *EditAction {
	if len(h.undoStack) == 0 {
		return nil
	}
	return &h.undoStack[len(h.undoStack)-1]
}

// PeekRedo returns the action Redo would pop, without moving it.
// The pointer is invalidated by the next stack mutation.
func (h *History) PeekRedo() *EditAction {
	if len(h.redoStack) == 0 {
		return nil
	}
	return &h.redoStack[len(h.redoStack)-1]
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

// Clear empties both undo and redo stacks.
func (h *History) Clear() {
	h.undoStack = h.undoStack[:0]
	h.redoStack = h.redoStack[:0]
}

// RecordExternalChange records a full content replacement as a single undo step.
// The old and new snapshots make the replacement exactly undoable and redoable.
func (h *History) RecordExternalChange(oldContent, newContent string, oldCursor, newCursor Cursor) {
	h.undoStack = append(h.undoStack, EditAction{
		Type:        ActionReplace,
		Text:        oldContent,
		Replacement: newContent,
		Cursor:      oldCursor,
		AfterCursor: newCursor,
		Timestamp:   time.Now(),
	})
	h.redoStack = nil
}

// SetCoalesceWindow overrides the coalescing window. Zero makes recording
// deterministic for tests that must not depend on wall-clock timing.
func (h *History) SetCoalesceWindow(d time.Duration) {
	h.coalesceWindow = d
}
