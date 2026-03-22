package main

// wrapEntry describes how a single buffer line maps to visual lines.
type wrapEntry struct {
	visualStart int   // first visual line index for this buffer line
	breaks      []int // display column offsets where each wrap occurs (nil = no wrap)
}

// wrapMap maps buffer lines to visual lines for word-wrap rendering.
type wrapMap struct {
	entries    []wrapEntry
	totalVisual int // total number of visual lines
}

// buildWrapMap computes the visual line mapping for all buffer lines.
// wrapCols is the number of display columns that fit in the text area.
func buildWrapMap(lines []string, wrapCols, tabSize int) *wrapMap {
	if wrapCols <= 0 {
		wrapCols = 80
	}
	entries := make([]wrapEntry, len(lines))
	visual := 0
	for i, line := range lines {
		expanded := expandTabs(line, tabSize)
		dispLen := len(expanded) // ASCII chars = display columns for monospace
		entries[i].visualStart = visual
		if dispLen <= wrapCols {
			visual++ // single visual line
		} else {
			// Compute wrap breaks
			col := wrapCols
			for col < dispLen {
				entries[i].breaks = append(entries[i].breaks, col)
				col += wrapCols
			}
			visual += 1 + len(entries[i].breaks) // first segment + wrapped segments
		}
	}
	return &wrapMap{entries: entries, totalVisual: visual}
}

// visualLines returns the total number of visual lines.
func (wm *wrapMap) visualLines() int {
	return wm.totalVisual
}

// segmentCount returns how many visual lines buffer line bufLine occupies.
func (wm *wrapMap) segmentCount(bufLine int) int {
	if bufLine < 0 || bufLine >= len(wm.entries) {
		return 1
	}
	return 1 + len(wm.entries[bufLine].breaks)
}

// bufferLineForVisual finds the buffer line and segment index for a given visual line.
func (wm *wrapMap) bufferLineForVisual(visualLine int) (bufLine, segIdx int) {
	if len(wm.entries) == 0 {
		return 0, 0
	}
	// Binary search for the buffer line whose visualStart <= visualLine
	lo, hi := 0, len(wm.entries)
	for lo < hi {
		mid := (lo + hi) / 2
		if wm.entries[mid].visualStart > visualLine {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	bufLine = lo - 1
	if bufLine < 0 {
		bufLine = 0
	}
	segIdx = visualLine - wm.entries[bufLine].visualStart
	if segIdx < 0 {
		segIdx = 0
	}
	return bufLine, segIdx
}

// bufferToVisual converts a buffer position (line, display column) to a visual line and column.
func (wm *wrapMap) bufferToVisual(bufLine, dispCol int) (visualLine, visualCol int) {
	if bufLine < 0 || bufLine >= len(wm.entries) {
		return 0, dispCol
	}
	e := wm.entries[bufLine]
	visualLine = e.visualStart
	visualCol = dispCol

	for _, brk := range e.breaks {
		if dispCol >= brk {
			visualLine++
			visualCol = dispCol - brk
		} else {
			break
		}
	}
	return visualLine, visualCol
}

// segmentRange returns the display column range [start, end) for a segment of a buffer line.
func (wm *wrapMap) segmentRange(bufLine, segIdx int) (start, end int) {
	if bufLine < 0 || bufLine >= len(wm.entries) {
		return 0, 0
	}
	e := wm.entries[bufLine]
	if segIdx == 0 {
		start = 0
	} else if segIdx-1 < len(e.breaks) {
		start = e.breaks[segIdx-1]
	}
	if segIdx < len(e.breaks) {
		end = e.breaks[segIdx]
	} else {
		end = -1 // to end of line
	}
	return start, end
}
