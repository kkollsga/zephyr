package editor

import (
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/buffer"
)

// Cursor represents a position in the editor as a 0-based line and column.
type Cursor struct {
	Line int
	Col  int
	// PreferredCol tracks the column the user "wants" to be at, for vertical movement
	// past shorter lines. -1 means use current Col.
	PreferredCol int
}

// NewCursor creates a cursor at line 0, col 0.
func NewCursor() Cursor {
	return Cursor{Line: 0, Col: 0, PreferredCol: -1}
}

// MoveRight moves the cursor one column right, wrapping to the next line if needed.
func (c *Cursor) MoveRight(pt *buffer.PieceTable) {
	line, err := pt.Line(c.Line)
	if err != nil {
		return
	}
	if c.Col < utf8.RuneCountInString(line) {
		c.Col++
	} else if c.Line < pt.LineCount()-1 {
		c.Line++
		c.Col = 0
	}
	c.PreferredCol = -1
}

// MoveLeft moves the cursor one column left, wrapping to the previous line if needed.
func (c *Cursor) MoveLeft(pt *buffer.PieceTable) {
	if c.Col > 0 {
		c.Col--
	} else if c.Line > 0 {
		c.Line--
		line, err := pt.Line(c.Line)
		if err != nil {
			return
		}
		c.Col = utf8.RuneCountInString(line)
	}
	c.PreferredCol = -1
}

// MoveDown moves the cursor one line down, clamping column to line length.
func (c *Cursor) MoveDown(pt *buffer.PieceTable) {
	if c.Line >= pt.LineCount()-1 {
		return
	}
	if c.PreferredCol == -1 {
		c.PreferredCol = c.Col
	}
	c.Line++
	line, err := pt.Line(c.Line)
	if err != nil {
		return
	}
	c.Col = min(c.PreferredCol, utf8.RuneCountInString(line))
}

// MoveUp moves the cursor one line up, clamping column to line length.
func (c *Cursor) MoveUp(pt *buffer.PieceTable) {
	if c.Line == 0 {
		return
	}
	if c.PreferredCol == -1 {
		c.PreferredCol = c.Col
	}
	c.Line--
	line, err := pt.Line(c.Line)
	if err != nil {
		return
	}
	c.Col = min(c.PreferredCol, utf8.RuneCountInString(line))
}

// MoveToLineStart moves the cursor to column 0.
func (c *Cursor) MoveToLineStart() {
	c.Col = 0
	c.PreferredCol = -1
}

// MoveToLineEnd moves the cursor to the end of the current line.
func (c *Cursor) MoveToLineEnd(pt *buffer.PieceTable) {
	line, err := pt.Line(c.Line)
	if err != nil {
		return
	}
	c.Col = utf8.RuneCountInString(line)
	c.PreferredCol = -1
}

// MoveToFileStart moves the cursor to the beginning of the file.
func (c *Cursor) MoveToFileStart() {
	c.Line = 0
	c.Col = 0
	c.PreferredCol = -1
}

// MoveToFileEnd moves the cursor to the end of the file.
func (c *Cursor) MoveToFileEnd(pt *buffer.PieceTable) {
	c.Line = pt.LineCount() - 1
	line, err := pt.Line(c.Line)
	if err != nil {
		return
	}
	c.Col = utf8.RuneCountInString(line)
	c.PreferredCol = -1
}

// PageDown moves the cursor down by pageSize lines.
func (c *Cursor) PageDown(pt *buffer.PieceTable, pageSize int) {
	if c.PreferredCol == -1 {
		c.PreferredCol = c.Col
	}
	c.Line = min(c.Line+pageSize, pt.LineCount()-1)
	line, err := pt.Line(c.Line)
	if err != nil {
		return
	}
	c.Col = min(c.PreferredCol, utf8.RuneCountInString(line))
}

// PageUp moves the cursor up by pageSize lines.
func (c *Cursor) PageUp(pt *buffer.PieceTable, pageSize int) {
	if c.PreferredCol == -1 {
		c.PreferredCol = c.Col
	}
	c.Line = max(c.Line-pageSize, 0)
	line, err := pt.Line(c.Line)
	if err != nil {
		return
	}
	c.Col = min(c.PreferredCol, utf8.RuneCountInString(line))
}

// SetPosition sets the cursor to an exact line/col, clamping to valid range.
func (c *Cursor) SetPosition(pt *buffer.PieceTable, line, col int) {
	c.Line = max(0, min(line, pt.LineCount()-1))
	lineText, err := pt.Line(c.Line)
	if err != nil {
		c.Col = 0
	} else {
		c.Col = max(0, min(col, len(lineText)))
	}
	c.PreferredCol = -1
}

// Clamp ensures the cursor is within valid bounds.
func (c *Cursor) Clamp(pt *buffer.PieceTable) {
	c.SetPosition(pt, c.Line, c.Col)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
