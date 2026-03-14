package editor

import (
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/buffer"
)

// Selection represents a text selection between an anchor and a head cursor.
// The anchor is where the selection started, and the head is where it currently is.
type Selection struct {
	Anchor Cursor
	Head   Cursor
	Active bool
}

// NewSelection creates an inactive selection.
func NewSelection() Selection {
	return Selection{}
}

// Start begins a selection at the given cursor position.
func (s *Selection) Start(c Cursor) {
	s.Anchor = c
	s.Head = c
	s.Active = true
}

// Update extends the selection to the given cursor position.
func (s *Selection) Update(c Cursor) {
	s.Head = c
}

// Clear deactivates the selection.
func (s *Selection) Clear() {
	s.Active = false
}

// Ordered returns the selection bounds with start before end.
func (s *Selection) Ordered() (start, end Cursor) {
	if s.Anchor.Line < s.Head.Line ||
		(s.Anchor.Line == s.Head.Line && s.Anchor.Col <= s.Head.Col) {
		return s.Anchor, s.Head
	}
	return s.Head, s.Anchor
}

// IsEmpty returns true if the selection has zero length.
func (s *Selection) IsEmpty() bool {
	return s.Anchor.Line == s.Head.Line && s.Anchor.Col == s.Head.Col
}

// Text returns the selected text from the piece table.
func (s *Selection) Text(pt *buffer.PieceTable) string {
	if !s.Active || s.IsEmpty() {
		return ""
	}
	start, end := s.Ordered()
	startOff, err := pt.LineColToOffset(buffer.LineCol{Line: start.Line, Col: start.Col})
	if err != nil {
		return ""
	}
	endOff, err := pt.LineColToOffset(buffer.LineCol{Line: end.Line, Col: end.Col})
	if err != nil {
		return ""
	}
	text, _ := pt.Substring(startOff, endOff-startOff)
	return text
}

// SelectAll selects all text in the buffer.
func (s *Selection) SelectAll(pt *buffer.PieceTable) {
	s.Anchor = Cursor{Line: 0, Col: 0}
	lastLine := pt.LineCount() - 1
	lineText, _ := pt.Line(lastLine)
	s.Head = Cursor{Line: lastLine, Col: utf8.RuneCountInString(lineText)}
	s.Active = true
}

// SelectWord selects the word at the given cursor position.
func (s *Selection) SelectWord(pt *buffer.PieceTable, c Cursor) {
	line, err := pt.Line(c.Line)
	if err != nil || len(line) == 0 {
		return
	}

	runes := []rune(line)
	col := c.Col
	if col >= len(runes) {
		col = len(runes) - 1
	}
	if col < 0 {
		return
	}

	// Expand left
	left := col
	for left > 0 && isWordRune(runes[left-1]) {
		left--
	}
	// Expand right
	right := col
	for right < len(runes) && isWordRune(runes[right]) {
		right++
	}

	s.Anchor = Cursor{Line: c.Line, Col: left}
	s.Head = Cursor{Line: c.Line, Col: right}
	s.Active = true
}

// SelectLine selects the entire line at the given cursor position.
func (s *Selection) SelectLine(pt *buffer.PieceTable, lineNum int) {
	line, err := pt.Line(lineNum)
	if err != nil {
		return
	}
	s.Anchor = Cursor{Line: lineNum, Col: 0}
	s.Head = Cursor{Line: lineNum, Col: utf8.RuneCountInString(line)}
	s.Active = true
}

func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
