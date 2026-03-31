package buffer

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

// LineCol represents a position in the buffer as a 0-based line and column.
type LineCol struct {
	Line int
	Col  int
}

// OffsetToLineCol converts a byte offset to a line/column position.
// Both line and column are 0-based. Column counts runes, not bytes.
// Uses binary search on lineStarts for O(log n) line lookup,
// then Substring for the column — no full-buffer allocation.
func (pt *PieceTable) OffsetToLineCol(offset int) (LineCol, error) {
	if offset < 0 || offset > pt.Length() {
		return LineCol{}, fmt.Errorf("offset %d out of range [0, %d]", offset, pt.Length())
	}

	pt.buildLineStarts()

	// Binary search: find the last lineStart <= offset
	line := sort.Search(len(pt.lineStarts), func(i int) bool {
		return pt.lineStarts[i] > offset
	}) - 1
	if line < 0 {
		line = 0
	}

	lineStart := pt.lineStarts[line]
	segLen := offset - lineStart
	if segLen == 0 {
		return LineCol{Line: line, Col: 0}, nil
	}

	seg, err := pt.Substring(lineStart, segLen)
	if err != nil {
		return LineCol{}, err
	}
	col := utf8.RuneCountInString(seg)
	return LineCol{Line: line, Col: col}, nil
}

// offsetToPoint returns the row and byte-column for a byte offset.
// Uses binary search on lineStarts — O(log n) with zero allocation.
// Column is a byte offset within the line, not a rune count.
func (pt *PieceTable) offsetToPoint(offset int) (row, col int) {
	pt.buildLineStarts()
	row = sort.Search(len(pt.lineStarts), func(i int) bool {
		return pt.lineStarts[i] > offset
	}) - 1
	if row < 0 {
		row = 0
	}
	col = offset - pt.lineStarts[row]
	return
}

// LineColToOffset converts a 0-based line/column to a byte offset.
// Column is counted in runes, not bytes.
// Uses lineStarts for O(1) line lookup, then Substring for the column —
// no full-buffer allocation.
func (pt *PieceTable) LineColToOffset(lc LineCol) (int, error) {
	if lc.Line < 0 || lc.Col < 0 {
		return 0, fmt.Errorf("invalid line:col %d:%d", lc.Line, lc.Col)
	}

	pt.buildLineStarts()

	if lc.Line >= len(pt.lineStarts) {
		return 0, fmt.Errorf("line %d out of range [0, %d)", lc.Line, len(pt.lineStarts))
	}

	lineStart := pt.lineStarts[lc.Line]

	if lc.Col == 0 {
		return lineStart, nil
	}

	// Compute the byte length of this line (excluding newline)
	var lineEnd int
	if lc.Line+1 < len(pt.lineStarts) {
		lineEnd = pt.lineStarts[lc.Line+1] - 1 // exclude the '\n'
	} else {
		lineEnd = pt.Length()
	}

	lineLen := lineEnd - lineStart
	if lineLen <= 0 {
		return 0, fmt.Errorf("column %d exceeds line length", lc.Col)
	}

	lineText, err := pt.Substring(lineStart, lineLen)
	if err != nil {
		return 0, err
	}

	// Walk runes up to lc.Col
	byteOff := 0
	col := 0
	for col < lc.Col && byteOff < len(lineText) {
		_, size := utf8.DecodeRuneInString(lineText[byteOff:])
		byteOff += size
		col++
	}
	if col < lc.Col {
		return 0, fmt.Errorf("column %d exceeds line length", lc.Col)
	}

	return lineStart + byteOff, nil
}

// LineColToOffsetSafe converts a 0-based line/column to a byte offset.
// Returns 0 if the position is invalid. This is a convenience wrapper
// around LineColToOffset for callers that don't need error handling.
func (pt *PieceTable) LineColToOffsetSafe(line, col int) int {
	off, err := pt.LineColToOffset(LineCol{Line: line, Col: col})
	if err != nil {
		return 0
	}
	return off
}
