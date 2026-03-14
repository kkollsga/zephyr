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
