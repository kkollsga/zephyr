package render

import (
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"golang.org/x/image/math/fixed"
)

// TextStyle holds the styling parameters for text rendering.
type TextStyle struct {
	FontSize   unit.Sp
	LineHeight float32 // multiplier, e.g. 1.5
	Foreground color.NRGBA
	Typeface   string      // font face name, e.g. "Menlo, monospace"
	Weight     font.Weight // e.g. font.Bold
	FontStyle  font.Style  // e.g. font.Italic
}

// TextRenderer renders lines of monospaced text with per-character coloring.
type TextRenderer struct {
	Shaper *text.Shaper
	Style  TextStyle
	// CharWidth and LineHeightPx are computed after the first layout.
	CharWidth    int
	CharAdvance  float64 // exact fractional advance per character
	LineHeightPx int
}

// NewTextRenderer creates a text renderer with the given shaper and style.
func NewTextRenderer(shaper *text.Shaper, style TextStyle) *TextRenderer {
	return &TextRenderer{
		Shaper: shaper,
		Style:  style,
	}
}

func spToFixed(m unit.Metric, sp unit.Sp) fixed.Int26_6 {
	return fixed.I(m.Sp(sp))
}

func fixedToFloat(i fixed.Int26_6) float32 {
	return float32(i) / 64.0
}

func (tr *TextRenderer) textParams(gtx layout.Context) text.Parameters {
	face := font.Typeface("Menlo, monospace")
	if tr.Style.Typeface != "" {
		face = font.Typeface(tr.Style.Typeface)
	}
	return text.Parameters{
		Font: font.Font{
			Typeface: face,
			Weight:   tr.Style.Weight,
			Style:    tr.Style.FontStyle,
		},
		PxPerEm:  spToFixed(gtx.Metric, tr.Style.FontSize),
		MaxWidth: gtx.Constraints.Max.X,
	}
}

// ComputeMetrics calculates character width and line height from the shaper.
func (tr *TextRenderer) ComputeMetrics(gtx layout.Context) {
	// Use natural font metrics (no LineHeightScale) to get the true glyph size,
	// then apply the line height multiplier once ourselves.
	params := tr.textParams(gtx)
	tr.Shaper.LayoutString(params, "M")
	for g, ok := tr.Shaper.NextGlyph(); ok; g, ok = tr.Shaper.NextGlyph() {
		tr.CharAdvance = float64(g.Advance) / 64.0
		tr.CharWidth = g.Advance.Round()
		ascent := g.Ascent.Round()
		descent := g.Descent.Round()
		base := ascent + descent
		tr.LineHeightPx = int(float32(base) * tr.Style.LineHeight)
		break
	}
	if tr.CharWidth == 0 {
		px := gtx.Metric.Sp(tr.Style.FontSize)
		tr.CharWidth = px
		tr.CharAdvance = float64(px)
		tr.LineHeightPx = int(float32(px) * tr.Style.LineHeight)
	}
}

// ColX returns the pixel X offset for a given display column,
// using the exact fractional advance to prevent sub-pixel drift.
func (tr *TextRenderer) ColX(col int) int {
	return int(math.Round(float64(col) * tr.CharAdvance))
}

// ColorSpan defines a color for a range of columns in a line.
type ColorSpan struct {
	Start int
	End   int
	Color color.NRGBA
}

// RenderLine draws a single line of text at the given pixel position.
// spans provides optional per-column coloring; if nil, Style.Foreground is used.
func (tr *TextRenderer) RenderLine(ops *op.Ops, gtx layout.Context, lineText string, x, y int, spans []ColorSpan) {
	if tr.CharWidth == 0 || tr.LineHeightPx == 0 || len(lineText) == 0 {
		return
	}

	if len(spans) == 0 {
		tr.renderText(ops, gtx, lineText, x, y, tr.Style.Foreground)
		return
	}

	// Multi-color: break into contiguous runs by color
	col := 0
	runStart := 0
	runStartByte := 0
	currentColor := tr.colorForCol(0, spans)

	for i, r := range lineText {
		c := tr.colorForCol(col, spans)
		if c != currentColor {
			if runStartByte < i {
				px := x + tr.ColX(runStart)
				tr.renderText(ops, gtx, lineText[runStartByte:i], px, y, currentColor)
			}
			runStart = col
			runStartByte = i
			currentColor = c
		}
		_ = r
		col++
	}
	if runStartByte < len(lineText) {
		px := x + tr.ColX(runStart)
		tr.renderText(ops, gtx, lineText[runStartByte:], px, y, currentColor)
	}
}

// renderText shapes text and paints it using the same approach as Gio's widget.Label:
// use glyph document coordinates for positioning, call Shape for vector outlines.
func (tr *TextRenderer) renderText(ops *op.Ops, gtx layout.Context, s string, x, y int, c color.NRGBA) {
	params := tr.textParams(gtx)

	tr.Shaper.LayoutString(params, s)

	// Collect all glyphs and find the first glyph's position (line origin)
	var glyphs []text.Glyph
	var lineX fixed.Int26_6
	var lineY int32
	first := true
	for g, ok := tr.Shaper.NextGlyph(); ok; g, ok = tr.Shaper.NextGlyph() {
		if first {
			lineX = g.X
			lineY = g.Y
			first = false
		}
		glyphs = append(glyphs, g)
	}
	if len(glyphs) == 0 {
		return
	}

	// Position the line: use the glyph's document coordinates as offset,
	// matching how Gio's label.go does it with op.Affine.
	lineOff := f32.Point{
		X: float32(x) + fixedToFloat(lineX),
		Y: float32(y) + float32(lineY),
	}

	aff := op.Affine(f32.Affine2D{}.Offset(lineOff)).Push(ops)

	// Vector glyph outlines
	pathSpec := tr.Shaper.Shape(glyphs)
	outline := clip.Outline{Path: pathSpec}.Op().Push(ops)
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
	outline.Pop()

	aff.Pop()
}

// RenderGlyphs is a convenience alias for renderText (used by status line).
func (tr *TextRenderer) RenderGlyphs(ops *op.Ops, gtx layout.Context, s string, x, y int, c color.NRGBA) {
	tr.renderText(ops, gtx, s, x, y, c)
}

func (tr *TextRenderer) colorForCol(col int, spans []ColorSpan) color.NRGBA {
	for _, s := range spans {
		if col >= s.Start && col < s.End {
			return s.Color
		}
	}
	return tr.Style.Foreground
}

// Round32 rounds a float32 to the nearest int.
func Round32(v float32) int {
	return int(math.Round(float64(v)))
}
