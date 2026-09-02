package main

import (
	"strings"

	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/format"
)

// isReindentLang reports whether the language supports token-aware
// re-indentation (JSON is treated as a subset of the JS re-indenter).
func isReindentLang(lang string) bool {
	switch lang {
	case "JSON", "JavaScript", "TypeScript":
		return true
	}
	return false
}

// reindentUnit returns one indentation level (spaces) for the given language,
// honoring the user-configurable indent width.
func (st *appState) reindentUnit() string {
	w := st.indentWidth
	if w < 1 {
		w = 2
	}
	return strings.Repeat(" ", w)
}

// fmtIsCompact reports whether the editor buffer is currently in compact
// (single-line) form. Kept cheap for per-frame status-bar rendering: a compact
// document is one physical line, optionally with a single trailing empty line.
func fmtIsCompact(ed *editor.Editor) bool {
	switch n := ed.Buffer.LineCount(); {
	case n <= 1:
		return true
	case n == 2:
		last, _ := ed.Buffer.Line(1)
		return strings.TrimSpace(last) == ""
	default:
		return false
	}
}

// toggleJSONCompact converts the active JSON buffer between compact (single
// line) and expanded (indented) form. Invalid JSON is a safe no-op.
func (st *appState) toggleJSONCompact() {
	ts := st.activeTabState()
	ed := st.activeEd()
	if ts == nil || ed == nil || ts.langLabel != "JSON" {
		return
	}
	src := ed.Buffer.TextBytes(nil)
	var out []byte
	var ok bool
	if format.IsSingleLine(string(src)) {
		out, ok = format.JSONIndent(src, st.reindentUnit())
	} else {
		out, ok = format.JSONCompact(src)
	}
	if !ok {
		return // invalid JSON: no-op (no toast mechanism to surface an error)
	}
	st.applyFormattedBuffer(ed, ts, string(out))
}

// applyFormattedBuffer swaps the editor buffer for formatted content as a
// single undoable step, marks the tab dirty, and re-highlights. Mirrors the
// external-change swap used by Editor.Reload, but keeps the tab modified.
func (st *appState) applyFormattedBuffer(ed *editor.Editor, ts *tabState, formatted string) {
	if formatted == ed.Buffer.Text() {
		return
	}
	oldContent := ed.Buffer.Text()
	oldCursor := ed.Cursor
	pt := buffer.NewFromString(formatted)
	ed.Buffer = pt
	ed.Cursor.Clamp(ed.Buffer)
	ed.Selection.Clear()
	ed.History.RecordExternalChange(oldContent, ed.Buffer.Text(), oldCursor, ed.Cursor)
	// RecordExternalChange does not touch the dirty flag (Reload clears it after
	// calling this); a format edits the buffer, so the tab must be marked dirty.
	ed.Modified = true

	st.afterBufferSwap(ed, ts)
	st.invalidate()
}

// insertNewlineAutoIndent handles an Enter keypress. When auto-indent is
// enabled and the language is supported, it inserts a bare newline, fixes the
// indentation of the line being left, then indents the new line (including any
// remainder text after a mid-line split) at its engine-computed depth, placing
// the cursor after that indentation. Lines beginning inside a template literal
// or block comment are left untouched. Otherwise it falls back to the legacy
// behavior of copying the previous line's leading whitespace.
func (st *appState) insertNewlineAutoIndent() {
	ed := st.activeEd()
	ts := st.activeTabState()
	if ed == nil {
		return
	}
	if !st.autoIndent || ts == nil || !isReindentLang(ts.langLabel) {
		indent := st.computeAutoIndent()
		ed.InsertText("\n" + indent)
		return
	}

	ed.InsertText("\n")
	unit := st.reindentUnit()

	// Fix the line the cursor just left.
	leftLine := ed.Cursor.Line - 1
	if leftLine >= 0 {
		if indent, skip, ok := format.LineIndent(ed.Buffer.Text(), unit, leftLine); ok && !skip {
			ed.SetLineLeadingWhitespace(leftLine, indent)
		}
	}

	// Indent the new line for its position (recomputed, since the fix above may
	// have shifted content) and land the cursor after the indentation.
	if indent, skip, ok := format.LineIndent(ed.Buffer.Text(), unit, ed.Cursor.Line); ok && !skip {
		ed.SetLineLeadingWhitespace(ed.Cursor.Line, indent)
		ed.Cursor.Col = len(indent)
		ed.Cursor.PreferredCol = -1
	}
}
