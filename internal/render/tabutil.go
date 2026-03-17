package render

import (
	"strings"
	"unicode/utf8"
)

// ExpandTabs replaces tab characters with spaces to align to tabSize columns.
func ExpandTabs(s string, tabSize int) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabSize - (col % tabSize)
			for j := 0; j < spaces; j++ {
				b.WriteByte(' ')
			}
			col += spaces
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// RuneColToDisplayCol converts a rune-based column to a display column
// accounting for tab expansion.
func RuneColToDisplayCol(lineText string, runeCol, tabSize int) int {
	dispCol := 0
	for _, r := range lineText {
		if runeCol <= 0 {
			break
		}
		if r == '\t' {
			dispCol += tabSize - (dispCol % tabSize)
		} else {
			dispCol++
		}
		runeCol--
	}
	return dispCol
}

// MatchDisplayLen returns the display width (in columns) of a match starting
// at runeCol with the given byte length, accounting for tab expansion.
func MatchDisplayLen(lineText string, runeCol, byteLen, tabSize int) int {
	runes := []rune(lineText)
	startDispCol := RuneColToDisplayCol(lineText, runeCol, tabSize)
	endDispCol := startDispCol
	ri := runeCol
	bytesConsumed := 0
	for ri < len(runes) && bytesConsumed < byteLen {
		r := runes[ri]
		if r == '\t' {
			endDispCol += tabSize - (endDispCol % tabSize)
		} else {
			endDispCol++
		}
		bytesConsumed += utf8.RuneLen(r)
		ri++
	}
	return endDispCol - startDispCol
}
