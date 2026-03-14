package render

import (
	"image/color"

	"github.com/kristianweb/zephyr/internal/config"
	"github.com/kristianweb/zephyr/internal/highlight"
)

// TokenColorMap maps highlight token types to theme colors.
func TokenColorMap(theme config.Theme) highlight.TokenColorMap {
	return highlight.TokenColorMap{
		highlight.TokenKeyword:  theme.Keyword,
		highlight.TokenString:   theme.String,
		highlight.TokenComment:  theme.Comment,
		highlight.TokenFunction: theme.Function,
		highlight.TokenType_:    theme.Type,
		highlight.TokenNumber:   theme.Number,
		highlight.TokenOperator: theme.Operator,
		highlight.TokenVariable: theme.Variable,
	}
}

// TokensToColorSpans converts highlight tokens for a specific line into ColorSpans.
// lineStartByte and lineEndByte define the byte range of the line in the source.
// lineText is the line content used for column-to-byte mapping.
// tabSize specifies how many columns a tab character occupies.
func TokensToColorSpans(tokens []highlight.Token, lineStartByte, lineEndByte int,
	lineText string, colorMap highlight.TokenColorMap, defaultColor color.NRGBA, tabSize int) []ColorSpan {

	if len(tokens) == 0 {
		return nil
	}

	if tabSize <= 0 {
		tabSize = 4
	}

	// Build byte-to-column map for the line, expanding tabs
	byteToCol := make([]int, len(lineText)+1)
	col := 0
	for i, r := range lineText {
		byteToCol[i] = col
		if r == '\t' {
			col += tabSize - (col % tabSize)
		} else {
			col++
		}
	}
	byteToCol[len(lineText)] = col

	var spans []ColorSpan
	for _, tok := range tokens {
		// Clamp token range to line range
		start := tok.StartByte - lineStartByte
		end := tok.EndByte - lineStartByte
		if start < 0 {
			start = 0
		}
		if end > len(lineText) {
			end = len(lineText)
		}
		if start >= end {
			continue
		}

		c, ok := colorMap[tok.Type]
		if !ok {
			c = defaultColor
		}

		startCol := byteToCol[start]
		endCol := byteToCol[end]

		spans = append(spans, ColorSpan{
			Start: startCol,
			End:   endCol,
			Color: c,
		})
	}

	return deduplicateSpans(spans)
}

// deduplicateSpans removes overlapping spans, with earlier spans (higher priority) taking precedence.
func deduplicateSpans(spans []ColorSpan) []ColorSpan {
	if len(spans) <= 1 {
		return spans
	}

	// Build a column-level color map; first span to claim a column wins
	maxCol := 0
	for _, s := range spans {
		if s.End > maxCol {
			maxCol = s.End
		}
	}

	type colEntry struct {
		set   bool
		color color.NRGBA
	}
	cols := make([]colEntry, maxCol)
	for _, s := range spans {
		for c := s.Start; c < s.End && c < maxCol; c++ {
			if !cols[c].set {
				cols[c] = colEntry{set: true, color: s.Color}
			}
		}
	}

	// Merge adjacent columns with same color into spans
	var result []ColorSpan
	i := 0
	for i < maxCol {
		if !cols[i].set {
			i++
			continue
		}
		start := i
		c := cols[i].color
		for i < maxCol && cols[i].set && cols[i].color == c {
			i++
		}
		result = append(result, ColorSpan{Start: start, End: i, Color: c})
	}
	return result
}
