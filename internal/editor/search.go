package editor

import (
	"regexp"
	"strings"

	"github.com/kristianweb/zephyr/internal/buffer"
)

// SearchResult represents a single find match.
type SearchResult struct {
	Offset int
	Length int
	Line   int
	Col    int
}

// Find searches for all occurrences of a pattern in the buffer.
func Find(pt *buffer.PieceTable, pattern string, useRegex, caseSensitive bool) ([]SearchResult, error) {
	text := pt.Text()

	if useRegex {
		flags := ""
		if !caseSensitive {
			flags = "(?i)"
		}
		re, err := regexp.Compile(flags + pattern)
		if err != nil {
			return nil, err
		}
		return findRegex(pt, text, re), nil
	}

	return findLiteral(pt, text, pattern, caseSensitive), nil
}

func findLiteral(pt *buffer.PieceTable, text, pattern string, caseSensitive bool) []SearchResult {
	if len(pattern) == 0 {
		return nil
	}

	searchText := text
	searchPattern := pattern
	if !caseSensitive {
		searchText = strings.ToLower(text)
		searchPattern = strings.ToLower(pattern)
	}

	var results []SearchResult
	offset := 0
	for {
		idx := strings.Index(searchText[offset:], searchPattern)
		if idx == -1 {
			break
		}
		absOffset := offset + idx
		lc, _ := pt.OffsetToLineCol(absOffset)
		results = append(results, SearchResult{
			Offset: absOffset,
			Length: len(pattern),
			Line:   lc.Line,
			Col:    lc.Col,
		})
		offset = absOffset + 1
	}
	return results
}

func findRegex(pt *buffer.PieceTable, text string, re *regexp.Regexp) []SearchResult {
	matches := re.FindAllStringIndex(text, -1)
	var results []SearchResult
	for _, m := range matches {
		lc, _ := pt.OffsetToLineCol(m[0])
		results = append(results, SearchResult{
			Offset: m[0],
			Length: m[1] - m[0],
			Line:   lc.Line,
			Col:    lc.Col,
		})
	}
	return results
}

// Replace replaces the match at the given offset with replacement text.
func Replace(pt *buffer.PieceTable, offset, length int, replacement string) {
	pt.Delete(offset, length)
	pt.Insert(offset, replacement)
}

// ReplaceAll replaces all matches with the replacement text.
// Returns the number of replacements made.
func ReplaceAll(pt *buffer.PieceTable, pattern, replacement string, useRegex, caseSensitive bool) (int, error) {
	results, err := Find(pt, pattern, useRegex, caseSensitive)
	if err != nil {
		return 0, err
	}

	// Replace from end to start to preserve offsets
	count := 0
	for i := len(results) - 1; i >= 0; i-- {
		r := results[i]
		if useRegex {
			// For regex, compute the actual replacement with group substitution
			text := pt.Text()
			flags := ""
			if !caseSensitive {
				flags = "(?i)"
			}
			re, _ := regexp.Compile(flags + pattern)
			matchText := text[r.Offset : r.Offset+r.Length]
			expanded := re.ReplaceAllString(matchText, replacement)
			Replace(pt, r.Offset, r.Length, expanded)
		} else {
			Replace(pt, r.Offset, r.Length, replacement)
		}
		count++
	}
	return count, nil
}
