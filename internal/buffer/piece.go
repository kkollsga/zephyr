package buffer

// Source indicates whether a piece refers to the original text or the add buffer.
type Source int

const (
	Original Source = iota
	Add
)

// Piece describes a contiguous span of text in either the original or add buffer.
type Piece struct {
	Source Source
	Offset int
	Length int
}

// EditInfo captures byte offsets and row/column points for a single edit,
// suitable for constructing a tree-sitter EditInput.
// Columns are byte offsets within the line (not rune counts).
type EditInfo struct {
	StartByte  int
	OldEndByte int
	NewEndByte int
	StartRow   int
	StartCol   int
	OldEndRow  int
	OldEndCol  int
	NewEndRow  int
	NewEndCol  int
}
