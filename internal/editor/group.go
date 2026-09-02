package editor

import "time"

// BeginGroup opens a recording group, or nests inside the open one. Every
// action recorded until the matching EndGroup becomes part of a single undo
// step; nested groups flatten into the outermost one.
func (h *History) BeginGroup() {
	h.groupDepth++
}

// EndGroup closes the innermost group. At depth zero the collected actions are
// pushed as one ActionGroup entry: before is the cursor to restore on undo,
// after the one to restore on redo. A group that recorded nothing pushes
// nothing and leaves the redo stack alone.
func (h *History) EndGroup(before, after Cursor) {
	if h.groupDepth == 0 {
		return
	}
	h.groupDepth--
	if h.groupDepth > 0 {
		return
	}
	actions := h.group
	h.group = nil
	if len(actions) == 0 {
		return
	}
	h.undoStack = append(h.undoStack, EditAction{
		Type:        ActionGroup,
		Group:       actions,
		Cursor:      before,
		AfterCursor: after,
		Timestamp:   time.Now(),
	})
	h.redoStack = nil
}

// Transact runs fn and records every edit it makes as one undo step. Nested
// calls flatten into the outermost transaction.
func (e *Editor) Transact(fn func()) {
	before := e.Cursor
	e.History.BeginGroup()
	defer func() { e.History.EndGroup(before, e.Cursor) }()
	fn()
}
