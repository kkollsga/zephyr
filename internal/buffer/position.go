package buffer

import (
	"fmt"
	"unicode/utf8"
)

// LineCol represents a position in the buffer as a 0-based line and column.
type LineCol struct {
	Line int
	Col  int
}

// OffsetToLineCol converts a byte offset to a line/column position.
// Both line and column are 0-based. Column counts runes, not bytes.
func (pt *PieceTable) OffsetToLineCol(offset int) (LineCol, error) {
	if offset < 0 || offset > pt.Length() {
		return LineCol{}, fmt.Errorf("offset %d out of range [0, %d]", offset, pt.Length())
	}

	text := pt.Text()
	line := 0
	col := 0
	for i := 0; i < offset; {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
		i += size
	}
	return LineCol{Line: line, Col: col}, nil
}

// LineColToOffset converts a 0-based line/column to a byte offset.
// Column is counted in runes, not bytes.
func (pt *PieceTable) LineColToOffset(lc LineCol) (int, error) {
	if lc.Line < 0 || lc.Col < 0 {
		return 0, fmt.Errorf("invalid line:col %d:%d", lc.Line, lc.Col)
	}

	text := pt.Text()
	line := 0
	i := 0
	// Find the start of the target line
	for i <= len(text) {
		if line == lc.Line {
			// Advance by lc.Col runes
			col := 0
			for col < lc.Col && i < len(text) {
				r, size := utf8.DecodeRuneInString(text[i:])
				if r == '\n' {
					return 0, fmt.Errorf("column %d exceeds line length", lc.Col)
				}
				i += size
				col++
			}
			if col < lc.Col {
				return 0, fmt.Errorf("column %d exceeds line length", lc.Col)
			}
			return i, nil
		}
		if i < len(text) && text[i] == '\n' {
			line++
		}
		if line <= lc.Line {
			i++
		}
	}

	return 0, fmt.Errorf("line %d out of range [0, %d)", lc.Line, pt.LineCount())
}
