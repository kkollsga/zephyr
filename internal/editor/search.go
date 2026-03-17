package editor

import (
	"regexp"
	"strings"
	"unicode/utf8"

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
	if useRegex {
		flags := ""
		if !caseSensitive {
			flags = "(?i)"
		}
		re, err := regexp.Compile(flags + pattern)
		if err != nil {
			return nil, err
		}
		text := pt.Text()
		return findRegex(pt, text, re), nil
	}

	// For literal search, use line-by-line to avoid full buffer allocation.
	if !strings.Contains(pattern, "\n") {
		return findLiteralByLine(pt, pattern, caseSensitive), nil
	}
	text := pt.Text()
	return findLiteral(pt, text, pattern, caseSensitive), nil
}

// findLiteralByLine searches line-by-line to avoid a full-buffer allocation.
// Only works for single-line patterns (no newlines).
func findLiteralByLine(pt *buffer.PieceTable, pattern string, caseSensitive bool) []SearchResult {
	if len(pattern) == 0 {
		return nil
	}
	searchPattern := pattern
	if !caseSensitive {
		searchPattern = strings.ToLower(pattern)
	}

	var results []SearchResult
	nLines := pt.LineCount()
	for lineIdx := 0; lineIdx < nLines; lineIdx++ {
		line, err := pt.Line(lineIdx)
		if err != nil {
			continue
		}
		searchLine := line
		if !caseSensitive {
			searchLine = strings.ToLower(line)
		}
		offset := 0
		for {
			idx := strings.Index(searchLine[offset:], searchPattern)
			if idx == -1 {
				break
			}
			col := utf8.RuneCountInString(line[:offset+idx])
			results = append(results, SearchResult{
				Offset: pt.LineStartOffset(lineIdx) + offset + idx,
				Length: len(pattern),
				Line:   lineIdx,
				Col:    col,
			})
			offset += idx + 1
		}
	}
	return results
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

	// Pre-compile regex once if needed
	var re *regexp.Regexp
	if useRegex {
		flags := ""
		if !caseSensitive {
			flags = "(?i)"
		}
		re, _ = regexp.Compile(flags + pattern)
	}

	// Replace from end to start to preserve offsets
	count := 0
	for i := len(results) - 1; i >= 0; i-- {
		r := results[i]
		if re != nil {
			matchText, _ := pt.Substring(r.Offset, r.Length)
			expanded := re.ReplaceAllString(matchText, replacement)
			Replace(pt, r.Offset, r.Length, expanded)
		} else {
			Replace(pt, r.Offset, r.Length, replacement)
		}
		count++
	}
	return count, nil
}
