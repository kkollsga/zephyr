package render

// Viewport tracks which lines are visible and the scroll position.
type Viewport struct {
	// FirstLine is the 0-based index of the first visible line.
	FirstLine int
	// PixelOffset is the number of pixels scrolled within FirstLine.
	// Range: [0, lineHeight). When non-zero, FirstLine is partially
	// scrolled off the top of the screen.
	PixelOffset int
	// VisibleLines is the number of lines that fit in the window.
	VisibleLines int
	// TotalLines is the total number of lines in the buffer.
	TotalLines int
	// ScrollMargin is the number of lines to keep visible above/below the cursor.
	ScrollMargin int
}

// NewViewport creates a viewport with default settings.
func NewViewport() *Viewport {
	return &Viewport{
		ScrollMargin: 3,
	}
}

// LastLine returns the 0-based index of the last visible line (inclusive).
func (v *Viewport) LastLine() int {
	last := v.FirstLine + v.VisibleLines - 1
	// If partially scrolled, one more line peeks in at the bottom.
	if v.PixelOffset > 0 {
		last++
	}
	if last >= v.TotalLines {
		last = v.TotalLines - 1
	}
	if last < 0 {
		last = 0
	}
	return last
}

// VisibleRange returns the first and last (inclusive) visible line indices.
func (v *Viewport) VisibleRange() (int, int) {
	return v.FirstLine, v.LastLine()
}

// maxFirstLine returns the maximum value of FirstLine.
func (v *Viewport) maxFirstLine() int {
	m := v.TotalLines - v.VisibleLines
	if m < 0 {
		m = 0
	}
	return m
}

// clamp ensures FirstLine and PixelOffset are within bounds.
func (v *Viewport) clamp() {
	if v.FirstLine < 0 {
		v.FirstLine = 0
		v.PixelOffset = 0
	}
	m := v.maxFirstLine()
	if v.FirstLine > m {
		v.FirstLine = m
		v.PixelOffset = 0
	} else if v.FirstLine == m {
		// At the bottom — no sub-line offset allowed.
		v.PixelOffset = 0
	}
}

// ScrollByPixels scrolls by dy pixels (positive = down).
// lineHeight must be > 0.
func (v *Viewport) ScrollByPixels(dy int, lineHeight int) {
	if lineHeight <= 0 {
		return
	}
	v.PixelOffset += dy

	// Carry whole lines out of PixelOffset.
	for v.PixelOffset >= lineHeight {
		v.FirstLine++
		v.PixelOffset -= lineHeight
	}
	for v.PixelOffset < 0 {
		v.FirstLine--
		v.PixelOffset += lineHeight
	}

	v.clamp()
}

// ScrollToRevealCursor adjusts FirstLine so the cursor line is visible,
// respecting the scroll margin. Resets PixelOffset for a clean snap.
func (v *Viewport) ScrollToRevealCursor(cursorLine int) {
	if v.VisibleLines <= 0 {
		return
	}

	margin := v.ScrollMargin
	if margin > v.VisibleLines/2 {
		margin = v.VisibleLines / 2
	}

	// Cursor is above viewport
	if cursorLine < v.FirstLine+margin {
		v.FirstLine = cursorLine - margin
		v.PixelOffset = 0
	}

	// Cursor is below viewport
	if cursorLine > v.FirstLine+v.VisibleLines-1-margin {
		v.FirstLine = cursorLine - v.VisibleLines + 1 + margin
		v.PixelOffset = 0
	}

	v.clamp()
}

// ScrollBy adjusts FirstLine by delta lines (positive = down).
// Resets PixelOffset.
func (v *Viewport) ScrollBy(delta int) {
	v.FirstLine += delta
	v.PixelOffset = 0
	v.clamp()
}

// ScrollablePixels returns how many pixels can be scrolled up (negative)
// and down (positive) from the current position.
func (v *Viewport) ScrollablePixels(lineHeight int) (up, down int) {
	up = v.FirstLine*lineHeight + v.PixelOffset
	m := v.maxFirstLine()
	down = (m-v.FirstLine)*lineHeight - v.PixelOffset
	if down < 0 {
		down = 0
	}
	return
}
