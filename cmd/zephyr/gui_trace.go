package main

import (
	"encoding/json"
	"fmt"
	"os"

	"gioui.org/io/pointer"
)

var guiTraceEnabled = os.Getenv("ZEPHYR_GUI_TRACE") != ""

type guiPointerTrace struct {
	Kind           string  `json:"kind"`
	Source         string  `json:"source"`
	Buttons        string  `json:"buttons"`
	X              float32 `json:"x"`
	Y              float32 `json:"y"`
	ScrollX        float32 `json:"scrollX,omitempty"`
	ScrollY        float32 `json:"scrollY,omitempty"`
	CursorLine     int     `json:"cursorLine,omitempty"`
	CursorCol      int     `json:"cursorCol,omitempty"`
	Selection      bool    `json:"selection"`
	Dragging       bool    `json:"dragging"`
	TabDragging    bool    `json:"tabDragging"`
	ViewportLine   int     `json:"viewportLine,omitempty"`
	ViewportOffset int     `json:"viewportOffset,omitempty"`
	MarkdownSelect bool    `json:"markdownSelect"`
}

// tracePointer records state after a pointer event when ZEPHYR_GUI_TRACE is
// enabled. The GUI harness captures stderr, keeping normal launches unchanged.
func (st *appState) tracePointer(pe pointer.Event) {
	if !guiTraceEnabled {
		return
	}
	record := guiPointerTrace{
		Kind:        pe.Kind.String(),
		Source:      pe.Source.String(),
		Buttons:     pe.Buttons.String(),
		X:           pe.Position.X,
		Y:           pe.Position.Y,
		ScrollX:     pe.Scroll.X,
		ScrollY:     pe.Scroll.Y,
		Dragging:    st.dragging,
		TabDragging: st.tabDrag.active,
	}
	if ed := st.activeEd(); ed != nil {
		record.CursorLine = ed.Cursor.Line
		record.CursorCol = ed.Cursor.Col
		record.Selection = ed.Selection.Active && !ed.Selection.IsEmpty()
	}
	if ts := st.activeTabState(); ts != nil {
		record.ViewportLine = ts.viewport.FirstLine
		record.ViewportOffset = ts.viewport.PixelOffset
		record.MarkdownSelect = ts.mdSelActive
	}
	if data, err := json.Marshal(record); err == nil {
		fmt.Fprintf(os.Stderr, "ZEPHYR_GUI_TRACE %s\n", data)
	}
}
