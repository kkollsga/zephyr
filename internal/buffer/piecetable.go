package buffer

import (
	"fmt"
	"os"
	"strings"
)

// PieceTable implements a piece table data structure for efficient text editing.
// It maintains an immutable original buffer and an append-only add buffer,
// with a sequence of pieces that reference spans in either buffer.
type PieceTable struct {
	original string
	add      strings.Builder
	pieces   []Piece

	// Cached line starts (byte offsets). Invalidated on edit.
	lineStarts []int
	linesDirty bool
}

// NewFromString creates a PieceTable initialized with the given string.
func NewFromString(s string) *PieceTable {
	pt := &PieceTable{
		original:   s,
		linesDirty: true,
	}
	if len(s) > 0 {
		pt.pieces = []Piece{{Source: Original, Offset: 0, Length: len(s)}}
	}
	return pt
}

// NewFromFile creates a PieceTable by reading the entire file into the original buffer.
func NewFromFile(path string) (*PieceTable, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewFromString(string(data)), nil
}

// Length returns the total length of the text in bytes.
func (pt *PieceTable) Length() int {
	n := 0
	for _, p := range pt.pieces {
		n += p.Length
	}
	return n
}

// Text returns the full text content by concatenating all pieces.
func (pt *PieceTable) Text() string {
	var b strings.Builder
	b.Grow(pt.Length())
	addBuf := pt.add.String()
	for _, p := range pt.pieces {
		switch p.Source {
		case Original:
			b.WriteString(pt.original[p.Offset : p.Offset+p.Length])
		case Add:
			b.WriteString(addBuf[p.Offset : p.Offset+p.Length])
		}
	}
	return b.String()
}

// bufferText returns the text for a given piece.
func (pt *PieceTable) bufferText(p Piece) string {
	switch p.Source {
	case Original:
		return pt.original[p.Offset : p.Offset+p.Length]
	case Add:
		addBuf := pt.add.String()
		return addBuf[p.Offset : p.Offset+p.Length]
	}
	return ""
}

// Insert inserts text at the given byte offset.
func (pt *PieceTable) Insert(offset int, text string) error {
	if offset < 0 || offset > pt.Length() {
		return fmt.Errorf("insert offset %d out of range [0, %d]", offset, pt.Length())
	}
	if len(text) == 0 {
		return nil
	}

	addOffset := pt.add.Len()
	pt.add.WriteString(text)
	newPiece := Piece{Source: Add, Offset: addOffset, Length: len(text)}

	if len(pt.pieces) == 0 {
		pt.pieces = []Piece{newPiece}
		pt.linesDirty = true
		return nil
	}

	// Find the piece and position within it
	pos := 0
	for i, p := range pt.pieces {
		if offset == pos {
			// Insert before this piece
			pt.pieces = splice(pt.pieces, i, 0, newPiece)
			pt.linesDirty = true
			return nil
		}
		if offset < pos+p.Length {
			// Split this piece
			left := Piece{Source: p.Source, Offset: p.Offset, Length: offset - pos}
			right := Piece{Source: p.Source, Offset: p.Offset + (offset - pos), Length: p.Length - (offset - pos)}
			pt.pieces = splice(pt.pieces, i, 1, left, newPiece, right)
			pt.linesDirty = true
			return nil
		}
		pos += p.Length
	}

	// Append at end
	pt.pieces = append(pt.pieces, newPiece)
	pt.linesDirty = true
	return nil
}

// Delete removes length bytes starting at the given byte offset.
func (pt *PieceTable) Delete(offset, length int) error {
	if length == 0 {
		return nil
	}
	if offset < 0 || length < 0 || offset+length > pt.Length() {
		return fmt.Errorf("delete range [%d, %d) out of range [0, %d)", offset, offset+length, pt.Length())
	}

	deleteEnd := offset + length
	pos := 0
	var newPieces []Piece

	for _, p := range pt.pieces {
		pieceEnd := pos + p.Length

		if pieceEnd <= offset || pos >= deleteEnd {
			// Entirely outside the delete range — keep
			newPieces = append(newPieces, p)
		} else {
			// Piece overlaps with delete range
			if pos < offset {
				// Keep left part before deletion
				left := Piece{Source: p.Source, Offset: p.Offset, Length: offset - pos}
				newPieces = append(newPieces, left)
			}
			if pieceEnd > deleteEnd {
				// Keep right part after deletion
				trimmed := deleteEnd - pos
				right := Piece{Source: p.Source, Offset: p.Offset + trimmed, Length: p.Length - trimmed}
				newPieces = append(newPieces, right)
			}
		}

		pos += p.Length
	}

	pt.pieces = newPieces
	pt.linesDirty = true
	return nil
}

// Substring returns a substring of the text from offset with the given length.
func (pt *PieceTable) Substring(offset, length int) (string, error) {
	if offset < 0 || length < 0 || offset+length > pt.Length() {
		return "", fmt.Errorf("substring range [%d, %d) out of range [0, %d)", offset, offset+length, pt.Length())
	}
	if length == 0 {
		return "", nil
	}

	var b strings.Builder
	b.Grow(length)
	remaining := length
	pos := 0

	for _, p := range pt.pieces {
		if remaining == 0 {
			break
		}
		pieceEnd := pos + p.Length
		if pieceEnd <= offset {
			pos += p.Length
			continue
		}
		if pos >= offset+length {
			break
		}

		start := 0
		if offset > pos {
			start = offset - pos
		}
		end := p.Length
		if start+remaining < end {
			end = start + remaining
		}

		text := pt.bufferText(Piece{Source: p.Source, Offset: p.Offset + start, Length: end - start})
		b.WriteString(text)
		remaining -= (end - start)
		pos += p.Length
	}

	return b.String(), nil
}

// buildLineStarts computes the byte offsets of each line start.
func (pt *PieceTable) buildLineStarts() {
	if !pt.linesDirty {
		return
	}
	pt.lineStarts = []int{0}
	offset := 0
	for _, p := range pt.pieces {
		text := pt.bufferText(p)
		for i := 0; i < len(text); i++ {
			if text[i] == '\n' {
				pt.lineStarts = append(pt.lineStarts, offset+i+1)
			}
		}
		offset += p.Length
	}
	pt.linesDirty = false
}

// LineCount returns the number of lines. A trailing newline adds an empty final line.
func (pt *PieceTable) LineCount() int {
	pt.buildLineStarts()
	return len(pt.lineStarts)
}

// Line returns the content of the given 0-based line, excluding the trailing newline.
func (pt *PieceTable) Line(n int) (string, error) {
	pt.buildLineStarts()
	if n < 0 || n >= len(pt.lineStarts) {
		return "", fmt.Errorf("line %d out of range [0, %d)", n, len(pt.lineStarts))
	}

	start := pt.lineStarts[n]
	var end int
	if n+1 < len(pt.lineStarts) {
		end = pt.lineStarts[n+1]
	} else {
		end = pt.Length()
	}

	line, err := pt.Substring(start, end-start)
	if err != nil {
		return "", err
	}
	// Strip trailing newline
	line = strings.TrimSuffix(line, "\n")
	return line, nil
}

// splice replaces count elements at index with the new elements.
func splice(pieces []Piece, index, count int, newPieces ...Piece) []Piece {
	result := make([]Piece, 0, len(pieces)-count+len(newPieces))
	result = append(result, pieces[:index]...)
	result = append(result, newPieces...)
	result = append(result, pieces[index+count:]...)
	return result
}
