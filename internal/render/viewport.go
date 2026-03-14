package render

// Viewport tracks which lines are visible and the scroll position.
type Viewport struct {
	// FirstLine is the 0-based index of the first visible line.
	FirstLine int
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

// ScrollToRevealCursor adjusts FirstLine so the cursor line is visible,
// respecting the scroll margin.
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
	}

	// Cursor is below viewport
	if cursorLine > v.FirstLine+v.VisibleLines-1-margin {
		v.FirstLine = cursorLine - v.VisibleLines + 1 + margin
	}

	// Clamp
	if v.FirstLine < 0 {
		v.FirstLine = 0
	}
	maxFirst := v.TotalLines - v.VisibleLines
	if maxFirst < 0 {
		maxFirst = 0
	}
	if v.FirstLine > maxFirst {
		v.FirstLine = maxFirst
	}
}

// ScrollBy adjusts FirstLine by delta lines (positive = down).
func (v *Viewport) ScrollBy(delta int) {
	v.FirstLine += delta
	if v.FirstLine < 0 {
		v.FirstLine = 0
	}
	maxFirst := v.TotalLines - v.VisibleLines
	if maxFirst < 0 {
		maxFirst = 0
	}
	if v.FirstLine > maxFirst {
		v.FirstLine = maxFirst
	}
}
