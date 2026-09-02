package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"gioui.org/io/key"
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

// guiEventTrace is the record emitted after a key or text-edit event. It
// carries a buffer checksum so a harness scenario can assert what the document
// holds without reading pixels or the file on disk.
type guiEventTrace struct {
	Kind       string `json:"kind"`
	Key        string `json:"key,omitempty"`
	Modifiers  string `json:"modifiers,omitempty"`
	Text       string `json:"text,omitempty"`
	Path       string `json:"path"`
	CursorLine int    `json:"cursorLine"`
	CursorCol  int    `json:"cursorCol"`
	LineCount  int    `json:"lineCount"`
	Selection  bool   `json:"selection"`
	Modified   bool   `json:"modified"`
	BufferHash string `json:"bufferHash"`
}

// traceKeyEvent records the state a key press left behind.
func (st *appState) traceKeyEvent(ke key.Event) {
	if !guiTraceEnabled {
		return
	}
	st.emitEventTrace(guiEventTrace{
		Kind:      "Key",
		Key:       string(ke.Name),
		Modifiers: ke.Modifiers.String(),
	})
}

// traceEditEvent records the state a committed text input left behind.
func (st *appState) traceEditEvent(text string) {
	if !guiTraceEnabled {
		return
	}
	st.emitEventTrace(guiEventTrace{Kind: "Edit", Text: text})
}

// emitEventTrace fills in the document half of a key/edit record and writes it.
// The checksum is taken over a fresh slice rather than ts.sourceBuf, which the
// highlighter owns.
func (st *appState) emitEventTrace(record guiEventTrace) {
	if ed := st.activeEd(); ed != nil {
		record.Path = ed.FilePath
		record.CursorLine = ed.Cursor.Line
		record.CursorCol = ed.Cursor.Col
		record.LineCount = ed.Buffer.LineCount()
		record.Selection = ed.Selection.Active && !ed.Selection.IsEmpty()
		record.Modified = ed.Modified
		sum := sha256.Sum256(ed.Buffer.TextBytes(nil))
		record.BufferHash = hex.EncodeToString(sum[:8])
	}
	if data, err := json.Marshal(record); err == nil {
		fmt.Fprintf(os.Stderr, "ZEPHYR_GUI_TRACE %s\n", data)
	}
}
