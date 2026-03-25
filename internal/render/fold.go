package render

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
)

// FoldRegion represents a collapsible range of lines defined by matching brackets.
type FoldRegion struct {
	StartLine    int
	EndLine      int
	OpenCol      int    // byte position of the opening bracket on StartLine
	CloseChar    byte   // closing bracket character: '}', ']', or ')'
	TrailingText string // punctuation after closing bracket on EndLine (e.g. ",")
}

// FoldState tracks collapsed fold regions and maps between buffer and display lines.
type FoldState struct {
	Regions       []FoldRegion
	Collapsed     map[int]bool        // collapsed fold start lines (buffer lines)
	hiddenLines   map[int]bool        // precomputed hidden buffer lines
	displayToBuf  []int               // display line → buffer line
	bufToDisplay  map[int]int         // buffer line → display line (visible only)
	foldStartSet  map[int]bool        // set of lines that start a fold region
	regionByStart map[int]*FoldRegion // fold start line → outermost region
}

// NewFoldState creates an empty fold state.
func NewFoldState() *FoldState {
	return &FoldState{
		Collapsed:     make(map[int]bool),
		hiddenLines:   make(map[int]bool),
		bufToDisplay:  make(map[int]int),
		foldStartSet:  make(map[int]bool),
		regionByStart: make(map[int]*FoldRegion),
	}
}

// SetRegions updates the fold regions (e.g. after text changes) and rebuilds display mapping.
func (fs *FoldState) SetRegions(regions []FoldRegion, totalBufLines int) {
	fs.Regions = regions
	fs.foldStartSet = make(map[int]bool)
	fs.regionByStart = make(map[int]*FoldRegion)
	for i := range fs.Regions {
		r := &fs.Regions[i]
		fs.foldStartSet[r.StartLine] = true
		if existing, ok := fs.regionByStart[r.StartLine]; !ok || r.EndLine > existing.EndLine {
			fs.regionByStart[r.StartLine] = r
		}
	}
	// Remove collapsed entries that no longer have regions
	for line := range fs.Collapsed {
		if !fs.foldStartSet[line] {
			delete(fs.Collapsed, line)
		}
	}
	fs.rebuild(totalBufLines)
}

// IsFoldStart returns true if the buffer line starts a fold region.
func (fs *FoldState) IsFoldStart(bufLine int) bool {
	return fs.foldStartSet[bufLine]
}

// IsCollapsed returns true if the fold at bufLine is collapsed.
func (fs *FoldState) IsCollapsed(bufLine int) bool {
	return fs.Collapsed[bufLine]
}

// IsHidden returns true if the buffer line is hidden inside a collapsed fold.
func (fs *FoldState) IsHidden(bufLine int) bool {
	return fs.hiddenLines[bufLine]
}

// HasCollapsed returns true if any fold is currently collapsed.
func (fs *FoldState) HasCollapsed() bool {
	return len(fs.Collapsed) > 0
}

// Toggle collapses or expands the fold at bufLine.
func (fs *FoldState) Toggle(bufLine, totalBufLines int) {
	if !fs.foldStartSet[bufLine] {
		return
	}
	if fs.Collapsed[bufLine] {
		delete(fs.Collapsed, bufLine)
	} else {
		fs.Collapsed[bufLine] = true
	}
	fs.rebuild(totalBufLines)
}

// ToggleRecursive collapses or expands the fold at bufLine and all nested folds.
func (fs *FoldState) ToggleRecursive(bufLine, totalBufLines int) {
	if !fs.foldStartSet[bufLine] {
		return
	}
	r := fs.regionByStart[bufLine]
	if r == nil {
		return
	}
	if fs.Collapsed[bufLine] {
		// Uncollapse this and all nested folds
		delete(fs.Collapsed, bufLine)
		for line := range fs.Collapsed {
			if line > r.StartLine && line <= r.EndLine {
				delete(fs.Collapsed, line)
			}
		}
	} else {
		// Collapse this and all nested folds
		fs.Collapsed[bufLine] = true
		for startLine := range fs.foldStartSet {
			if startLine > r.StartLine && startLine < r.EndLine {
				fs.Collapsed[startLine] = true
			}
		}
	}
	fs.rebuild(totalBufLines)
}

// rebuild recomputes hidden lines and the display ↔ buffer mapping.
func (fs *FoldState) rebuild(totalBufLines int) {
	fs.hiddenLines = make(map[int]bool)
	for startLine := range fs.Collapsed {
		r := fs.regionByStart[startLine]
		if r == nil {
			continue
		}
		for line := r.StartLine + 1; line <= r.EndLine; line++ {
			fs.hiddenLines[line] = true
		}
	}
	// Rebuild display ↔ buffer mapping
	fs.displayToBuf = fs.displayToBuf[:0]
	fs.bufToDisplay = make(map[int]int, totalBufLines-len(fs.hiddenLines))
	for i := 0; i < totalBufLines; i++ {
		if !fs.hiddenLines[i] {
			fs.bufToDisplay[i] = len(fs.displayToBuf)
			fs.displayToBuf = append(fs.displayToBuf, i)
		}
	}
}

// DisplayLineCount returns the number of visible (non-hidden) lines.
func (fs *FoldState) DisplayLineCount() int {
	return len(fs.displayToBuf)
}

// DisplayToBuf converts a display line index to a buffer line.
func (fs *FoldState) DisplayToBuf(displayLine int) int {
	if displayLine < 0 || displayLine >= len(fs.displayToBuf) {
		if displayLine < 0 {
			return 0
		}
		return displayLine
	}
	return fs.displayToBuf[displayLine]
}

// BufToDisplay converts a buffer line to a display line index.
// Returns the buffer line itself if it is not in the mapping (e.g. hidden).
func (fs *FoldState) BufToDisplay(bufLine int) int {
	if d, ok := fs.bufToDisplay[bufLine]; ok {
		return d
	}
	// If hidden, return the display line of the fold start that hides it
	for startLine, r := range fs.regionByStart {
		if fs.Collapsed[startLine] && bufLine > r.StartLine && bufLine <= r.EndLine {
			if d, ok := fs.bufToDisplay[r.StartLine]; ok {
				return d
			}
		}
	}
	return bufLine
}

// RegionAt returns the outermost fold region that starts at the given line, or nil.
func (fs *FoldState) RegionAt(bufLine int) *FoldRegion {
	return fs.regionByStart[bufLine]
}

// ClampCursorLine ensures the cursor line is not hidden. Returns the fold start line.
func (fs *FoldState) ClampCursorLine(line int) int {
	if !fs.hiddenLines[line] {
		return line
	}
	for startLine, r := range fs.regionByStart {
		if fs.Collapsed[startLine] && line > r.StartLine && line <= r.EndLine {
			return r.StartLine
		}
	}
	return line
}

// CollapsedLineCount returns the number of hidden lines in a collapsed fold.
func CollapsedLineCount(region *FoldRegion) int {
	if region == nil {
		return 0
	}
	return region.EndLine - region.StartLine
}

// superscriptDigits maps ASCII digits to Unicode superscript equivalents.
var superscriptDigits = [10]rune{'⁰', '¹', '²', '³', '⁴', '⁵', '⁶', '⁷', '⁸', '⁹'}

// toSuperscript converts a non-negative integer to a string of Unicode superscript digits.
func toSuperscript(n int) string {
	s := fmt.Sprintf("%d", n)
	runes := make([]rune, len(s))
	for i, ch := range s {
		runes[i] = superscriptDigits[ch-'0']
	}
	return string(runes)
}

// FoldCountColor returns a color-coded NRGBA for a collapsed line count.
// Green for small counts, orange for moderate, red for large.
func FoldCountColor(count int) color.NRGBA {
	switch {
	case count <= 5:
		return color.NRGBA{R: 80, G: 200, B: 80, A: 255} // green
	case count <= 25:
		return color.NRGBA{R: 220, G: 160, B: 40, A: 255} // orange
	default:
		return color.NRGBA{R: 220, G: 60, B: 60, A: 255} // red
	}
}

// CollapsedLineText returns the collapsed display text for a fold start line.
// e.g. `"key": {` becomes `"key": {...⁴²},`
func CollapsedLineText(lineText string, region *FoldRegion) string {
	if region == nil {
		return lineText
	}
	openCol := region.OpenCol
	if openCol >= len(lineText) {
		return lineText
	}
	prefix := lineText[:openCol+1]
	closeStr := string(region.CloseChar)
	count := CollapsedLineCount(region)
	sup := toSuperscript(count)
	return prefix + "..." + sup + closeStr + region.TrailingText
}

// CollapsedCountSpan returns the display-column range [start, end) and color for the
// superscript count indicator in a collapsed line, after tab expansion.
// Returns start=end=0 if not applicable.
func CollapsedCountSpan(expandedLine string, region *FoldRegion) (start, end int, clr color.NRGBA) {
	if region == nil {
		return 0, 0, color.NRGBA{}
	}
	count := CollapsedLineCount(region)
	sup := toSuperscript(count)
	supRunes := []rune(sup)
	closeStr := string(region.CloseChar)

	// Find "..." + sup + closeStr in the expanded line by searching backwards from end
	// The count appears right after "..." and before the closing bracket
	target := "..." + sup + closeStr
	idx := strings.LastIndex(expandedLine, target)
	if idx < 0 {
		return 0, 0, color.NRGBA{}
	}
	// Convert byte index to rune/column index
	col := 0
	for i := range expandedLine {
		if i == idx+3 { // +3 for "..."
			start = col
		}
		if i == idx+3+len(sup) {
			end = col
			break
		}
		col++
	}
	if end == 0 && start > 0 {
		// sup extends to end of target
		end = start + len(supRunes)
	}
	return start, end, FoldCountColor(count)
}

// ComputeFoldRegions scans source text for multi-line bracket pairs.
func ComputeFoldRegions(text string) []FoldRegion {
	lines := strings.Split(text, "\n")

	type stackEntry struct {
		line    int
		col     int
		bracket byte
	}
	var stack []stackEntry
	var regions []FoldRegion

	inBlockComment := false

	for lineIdx, line := range lines {
		inString := false
		var stringChar byte
		escaped := false

		for i := 0; i < len(line); i++ {
			ch := line[i]

			if escaped {
				escaped = false
				continue
			}

			// Block comment state
			if inBlockComment {
				if ch == '*' && i+1 < len(line) && line[i+1] == '/' {
					inBlockComment = false
					i++
				}
				continue
			}

			if inString {
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == stringChar {
					inString = false
				}
				continue
			}

			// Line comment — skip rest of line
			if ch == '/' && i+1 < len(line) {
				if line[i+1] == '/' {
					break
				}
				if line[i+1] == '*' {
					inBlockComment = true
					i++
					continue
				}
			}

			// String start
			if ch == '"' || ch == '\'' || ch == '`' {
				inString = true
				stringChar = ch
				continue
			}

			// Brackets
			switch ch {
			case '{', '[', '(':
				stack = append(stack, stackEntry{lineIdx, i, ch})
			case '}', ']', ')':
				var expected byte
				switch ch {
				case '}':
					expected = '{'
				case ']':
					expected = '['
				case ')':
					expected = '('
				}
				for j := len(stack) - 1; j >= 0; j-- {
					if stack[j].bracket == expected {
						if lineIdx > stack[j].line {
							trailing := trailingAfterClose(line, i)
							regions = append(regions, FoldRegion{
								StartLine:    stack[j].line,
								EndLine:      lineIdx,
								OpenCol:      stack[j].col,
								CloseChar:    ch,
								TrailingText: trailing,
							})
						}
						stack = append(stack[:j], stack[j+1:]...)
						break
					}
				}
			}
		}
	}

	sort.Slice(regions, func(i, j int) bool {
		if regions[i].StartLine != regions[j].StartLine {
			return regions[i].StartLine < regions[j].StartLine
		}
		return regions[i].EndLine > regions[j].EndLine
	})

	return regions
}

// trailingAfterClose extracts punctuation immediately after a closing bracket.
func trailingAfterClose(line string, closePos int) string {
	for i := closePos + 1; i < len(line); i++ {
		c := line[i]
		if c == ' ' || c == '\t' {
			continue
		}
		if c == ',' || c == ';' {
			return string(c)
		}
		return ""
	}
	return ""
}
