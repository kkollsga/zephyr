package editor

import "strings"

// indentUnit is one indent level for IndentLine/DedentLine: four spaces, the
// width vim's >> and << have always used here.
const indentUnit = "    "

// replaceSpan swaps the byte span [off, off+len(old)) for repl, recording the
// delete and the insert as one undo step. Replacing a span with itself is a
// no-op: recording it would leave an undo step that restores nothing, so Cmd+Z
// would appear to do nothing at all.
func (e *Editor) replaceSpan(off int, old, repl string) {
	if old == repl {
		return
	}
	e.Transact(func() {
		if old != "" {
			e.History.Record(EditAction{Type: ActionDelete, Offset: off, Text: old, Cursor: e.Cursor})
			e.Buffer.Delete(off, len(old))
		}
		if repl != "" {
			e.History.Record(EditAction{Type: ActionInsert, Offset: off, Text: repl, Cursor: e.Cursor})
			e.Buffer.Insert(off, repl)
		}
		e.Modified = true
	})
}

// ReplaceRuneAtCursor overwrites the rune at the cursor with r (vim's r).
func (e *Editor) ReplaceRuneAtCursor(r rune) bool {
	return e.ReplaceRunes(1, r)
}

// ReplaceRunes overwrites the n runes at the cursor with n copies of r, as one
// undo step. n is clamped to the runes left on the cursor's line; with none
// left nothing is recorded. Returns true if the buffer changed.
func (e *Editor) ReplaceRunes(n int, r rune) bool {
	if n <= 0 {
		return false
	}
	line, err := e.Buffer.Line(e.Cursor.Line)
	if err != nil {
		return false
	}
	runes := []rune(line)
	col := e.Cursor.Col
	if col < 0 || col >= len(runes) {
		return false
	}
	if col+n > len(runes) {
		n = len(runes) - col
	}
	old := string(runes[col : col+n])
	off := e.Buffer.LineStartOffset(e.Cursor.Line) + len(string(runes[:col]))
	e.replaceSpan(off, old, strings.Repeat(string(r), n))
	return true
}

// leadingWhitespace returns the leading spaces and tabs of a line.
func (e *Editor) leadingWhitespace(lineIdx int) (string, bool) {
	line, err := e.Buffer.Line(lineIdx)
	if err != nil {
		return "", false
	}
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	return line[:n], true
}

// IndentLine adds one indent level to the line, as one undo step.
func (e *Editor) IndentLine(lineIdx int) bool {
	ws, ok := e.leadingWhitespace(lineIdx)
	if !ok {
		return false
	}
	return e.SetLineLeadingWhitespace(lineIdx, indentUnit+ws)
}

// DedentLine removes up to one indent level of leading spaces from the line,
// as one undo step. A line whose indentation starts with a tab is left alone,
// matching what >> inserts.
func (e *Editor) DedentLine(lineIdx int) bool {
	ws, ok := e.leadingWhitespace(lineIdx)
	if !ok {
		return false
	}
	n := 0
	for n < len(ws) && n < len(indentUnit) && ws[n] == ' ' {
		n++
	}
	if n == 0 {
		return false
	}
	return e.SetLineLeadingWhitespace(lineIdx, ws[n:])
}

// JoinLines joins the line below the cursor's line onto it (vim's J): the
// newline and the next line's leading whitespace become a single space.
// Recorded as one undo step. Returns true if the buffer changed.
func (e *Editor) JoinLines() bool {
	if e.Cursor.Line >= e.Buffer.LineCount()-1 {
		return false
	}
	e.Cursor.MoveToLineEnd(e.Buffer)
	nextLine, err := e.Buffer.Line(e.Cursor.Line + 1)
	if err != nil {
		return false
	}
	off := e.cursorOffset()
	// The span is the newline plus the next line, in bytes: a rune count here
	// leaves a fragment of any non-ASCII next line behind.
	deleted, err := e.Buffer.Substring(off, 1+len(nextLine))
	if err != nil {
		return false
	}
	repl := ""
	if trimmed := strings.TrimLeft(nextLine, " \t"); trimmed != "" {
		repl = " " + trimmed
	}
	e.replaceSpan(off, deleted, repl)
	return true
}
