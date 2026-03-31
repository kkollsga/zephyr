package render

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// CursorRenderer draws the text cursor and selection highlights.
type CursorRenderer struct {
	Color       color.NRGBA
	Width       int // cursor width in pixels (typically 2)
	CharWidth   int
	CharAdvance float64
	LineHeight  int
	BlinkOn     bool
	BlockMode   bool // true for vim Normal mode block cursor
	lastBlink   time.Time
}

// NewCursorRenderer creates a cursor renderer.
func NewCursorRenderer(c color.NRGBA, charWidth int, charAdvance float64, lineHeight int) *CursorRenderer {
	return &CursorRenderer{
		Color:       c,
		Width:       2,
		CharWidth:   charWidth,
		CharAdvance: charAdvance,
		LineHeight:  lineHeight,
		BlinkOn:     true,
		lastBlink:   time.Now(),
	}
}

// UpdateBlink toggles the blink state based on elapsed time.
// Returns true if the state changed (needs redraw).
func (cr *CursorRenderer) UpdateBlink() bool {
	now := time.Now()
	if now.Sub(cr.lastBlink) >= 530*time.Millisecond {
		cr.BlinkOn = !cr.BlinkOn
		cr.lastBlink = now
		return true
	}
	return false
}

// LastBlinkTime returns the time of the last blink toggle.
func (cr *CursorRenderer) LastBlinkTime() time.Time {
	return cr.lastBlink
}

// ResetBlink makes the cursor visible (e.g. after a keystroke).
func (cr *CursorRenderer) ResetBlink() {
	cr.BlinkOn = true
	cr.lastBlink = time.Now()
}

// RenderCursor draws the cursor at the given line/col relative to the viewport.
func (cr *CursorRenderer) RenderCursor(ops *op.Ops, line, col, firstLine, gutterWidth int) {
	if !cr.BlinkOn {
		return
	}

	x := gutterWidth + int(math.Round(float64(col)*cr.CharAdvance))
	y := (line - firstLine) * cr.LineHeight
	// Cursor height is ~70% of line height, top-aligned with text
	cursorH := cr.LineHeight * 70 / 100

	width := cr.Width
	if cr.BlockMode {
		width = int(math.Round(cr.CharAdvance))
		if width < cr.CharWidth {
			width = cr.CharWidth
		}
	}

	rect := clip.Rect{
		Min: image.Pt(x, y),
		Max: image.Pt(x+width, y+cursorH),
	}.Push(ops)
	paint.ColorOp{Color: cr.Color}.Add(ops)
	paint.PaintOp{}.Add(ops)
	rect.Pop()
}

// RenderSelection draws selection highlight for a range of lines.
func (cr *CursorRenderer) RenderSelection(ops *op.Ops, selColor color.NRGBA,
	startLine, startCol, endLine, endCol, firstLine, gutterWidth, maxWidth int,
	lineLength func(int) int) {

	for line := startLine; line <= endLine; line++ {
		visY := (line - firstLine) * cr.LineHeight
		if visY < 0 || visY > maxWidth {
			continue
		}

		sc := 0
		ec := lineLength(line)
		if line == startLine {
			sc = startCol
		}
		if line == endLine {
			ec = endCol
		}

		x1 := gutterWidth + int(math.Round(float64(sc)*cr.CharAdvance))
		x2 := gutterWidth + int(math.Round(float64(ec)*cr.CharAdvance))

		rect := clip.Rect{
			Min: image.Pt(x1, visY),
			Max: image.Pt(x2, visY+cr.LineHeight),
		}.Push(ops)
		paint.ColorOp{Color: selColor}.Add(ops)
		paint.PaintOp{}.Add(ops)
		rect.Pop()
	}
}
