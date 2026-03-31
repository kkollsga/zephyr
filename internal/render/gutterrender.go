package render

import (
	"fmt"
	"image"
	"image/color"

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

// GutterRenderer renders line numbers in the gutter area.
type GutterRenderer struct {
	Shaper     *text.Shaper
	FontSize   unit.Sp
	FgColor    color.NRGBA
	BgColor    color.NRGBA
	CharWidth  int
	LineHeight int
}

// foldCollapsedColor is the red color used for collapsed fold line numbers.
var foldCollapsedColor = color.NRGBA{R: 220, G: 60, B: 60, A: 255}

// Width returns the pixel width of the gutter for the given max line number.
func (gr *GutterRenderer) Width(maxLineNum int) int {
	digits := len(fmt.Sprintf("%d", maxLineNum))
	if digits < 3 {
		digits = 3
	}
	return (digits + 2) * gr.CharWidth
}

// EstimateWidth returns the expected gutter width for the given line count.
func (gr *GutterRenderer) EstimateWidth(totalLines int) int {
	return gr.Width(totalLines)
}

// RenderLineNumber draws a single line number at the given Y position.
func (gr *GutterRenderer) RenderLineNumber(gtx layout.Context, ops *op.Ops, lineNum, totalLines, y int) {
	gr.renderLineNumberColored(gtx, ops, lineNum, totalLines, y, gr.FgColor)
}

// renderLineNumberColored draws a line number at the given Y position in the specified color.
func (gr *GutterRenderer) renderLineNumberColored(gtx layout.Context, ops *op.Ops, lineNum, totalLines, y int, fg color.NRGBA) {
	width := gr.Width(totalLines)
	maxDigits := len(fmt.Sprintf("%d", totalLines))
	numStr := fmt.Sprintf("%*d", maxDigits, lineNum)
	xOffset := width - (len(numStr)+1)*gr.CharWidth

	params := text.Parameters{
		Font:     font.Font{Typeface: "Menlo, monospace"},
		PxPerEm:  spToFixed(gtx.Metric, gr.FontSize),
		MaxWidth: 1 << 30,
	}

	gr.Shaper.LayoutString(params, numStr)
	var glyphs []text.Glyph
	var lineX fixed.Int26_6
	var lineY int32
	first := true
	for g, ok := gr.Shaper.NextGlyph(); ok; g, ok = gr.Shaper.NextGlyph() {
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

	lineOff := f32.Point{
		X: float32(xOffset) + fixedToFloat(lineX),
		Y: float32(y) + float32(lineY),
	}

	aff := op.Affine(f32.Affine2D{}.Offset(lineOff)).Push(ops)
	pathSpec := gr.Shaper.Shape(glyphs)
	cl := clip.Outline{Path: pathSpec}.Op().Push(ops)
	paint.ColorOp{Color: fg}.Add(ops)
	paint.PaintOp{}.Add(ops)
	cl.Pop()
	aff.Pop()
}

// renderFoldIcon draws a small "<" icon to the right of the line number for collapsed folds.
func (gr *GutterRenderer) renderFoldIcon(gtx layout.Context, ops *op.Ops, totalLines, y int) {
	width := gr.Width(totalLines)
	// Position the icon in the right padding area of the gutter
	xOffset := width - gr.CharWidth

	params := text.Parameters{
		Font:     font.Font{Typeface: "Menlo, monospace"},
		PxPerEm:  spToFixed(gtx.Metric, gr.FontSize),
		MaxWidth: 1 << 30,
	}

	gr.Shaper.LayoutString(params, "‹")
	var glyphs []text.Glyph
	var lineX fixed.Int26_6
	var lineY int32
	first := true
	for g, ok := gr.Shaper.NextGlyph(); ok; g, ok = gr.Shaper.NextGlyph() {
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

	lineOff := f32.Point{
		X: float32(xOffset) + fixedToFloat(lineX),
		Y: float32(y) + float32(lineY),
	}

	aff := op.Affine(f32.Affine2D{}.Offset(lineOff)).Push(ops)
	pathSpec := gr.Shaper.Shape(glyphs)
	cl := clip.Outline{Path: pathSpec}.Op().Push(ops)
	paint.ColorOp{Color: foldCollapsedColor}.Add(ops)
	paint.PaintOp{}.Add(ops)
	cl.Pop()
	aff.Pop()
}

// RenderGutter draws line numbers for visible lines.
// extraOffsets: [0] = topPad (vertical padding), [1] = pixelOffset (sub-line scroll).
func (gr *GutterRenderer) RenderGutter(gtx layout.Context, ops *op.Ops, firstLine, lastLine, totalLines int, extraOffsets ...int) int {
	topPad := 0
	pixelOff := 0
	if len(extraOffsets) > 0 {
		topPad = extraOffsets[0]
	}
	if len(extraOffsets) > 1 {
		pixelOff = extraOffsets[1]
	}
	width := gr.Width(totalLines)

	// Background
	rect := clip.Rect{Max: image.Pt(width, gtx.Constraints.Max.Y)}.Push(ops)
	paint.ColorOp{Color: gr.BgColor}.Add(ops)
	paint.PaintOp{}.Add(ops)
	rect.Pop()

	params := text.Parameters{
		Font:     font.Font{Typeface: "Menlo, monospace"},
		PxPerEm:  spToFixed(gtx.Metric, gr.FontSize),
		MaxWidth: gtx.Constraints.Max.X,
	}

	maxDigits := len(fmt.Sprintf("%d", totalLines))

	for i := firstLine; i <= lastLine && i < totalLines; i++ {
		lineNum := fmt.Sprintf("%*d", maxDigits, i+1)
		y := (i-firstLine)*gr.LineHeight + topPad - pixelOff

		xOffset := width - (len(lineNum)+1)*gr.CharWidth

		gr.Shaper.LayoutString(params, lineNum)
		var glyphs []text.Glyph
		var lineX fixed.Int26_6
		var lineY int32
		first := true
		for g, ok := gr.Shaper.NextGlyph(); ok; g, ok = gr.Shaper.NextGlyph() {
			if first {
				lineX = g.X
				lineY = g.Y
				first = false
			}
			glyphs = append(glyphs, g)
		}
		if len(glyphs) == 0 {
			continue
		}

		lineOff := f32.Point{
			X: float32(xOffset) + fixedToFloat(lineX),
			Y: float32(y) + float32(lineY),
		}

		aff := op.Affine(f32.Affine2D{}.Offset(lineOff)).Push(ops)
		pathSpec := gr.Shaper.Shape(glyphs)
		cl := clip.Outline{Path: pathSpec}.Op().Push(ops)
		paint.ColorOp{Color: gr.FgColor}.Add(ops)
		paint.PaintOp{}.Add(ops)
		cl.Pop()
		aff.Pop()
	}

	return width
}

// RenderDiffSign draws a 2px colored bar at the left edge of the gutter for a diff sign.
// signType: '+' = added, '~' = modified.
func (gr *GutterRenderer) RenderDiffSign(ops *op.Ops, y, lineHeight int, signType rune, added, modified, deleted color.NRGBA) {
	var c color.NRGBA
	switch signType {
	case '+':
		c = added
	case '~':
		c = modified
	default:
		return
	}
	rect := clip.Rect{
		Min: image.Pt(1, y),
		Max: image.Pt(3, y+lineHeight),
	}.Push(ops)
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
	rect.Pop()
}

// RenderGutterFolded draws line numbers for visible lines with fold indicators.
// firstDisplay/lastDisplay are display line indices. fs maps display lines to buffer lines.
// Buffer line numbers are shown (not display line numbers).
func (gr *GutterRenderer) RenderGutterFolded(gtx layout.Context, ops *op.Ops,
	firstDisplay, lastDisplay, totalBufLines int, fs *FoldState, extraOffsets ...int) int {

	topPad := 0
	pixelOff := 0
	if len(extraOffsets) > 0 {
		topPad = extraOffsets[0]
	}
	if len(extraOffsets) > 1 {
		pixelOff = extraOffsets[1]
	}
	width := gr.Width(totalBufLines)

	// Background
	rect := clip.Rect{Max: image.Pt(width, gtx.Constraints.Max.Y)}.Push(ops)
	paint.ColorOp{Color: gr.BgColor}.Add(ops)
	paint.PaintOp{}.Add(ops)
	rect.Pop()

	displayCount := fs.DisplayLineCount()
	for dispLine := firstDisplay; dispLine <= lastDisplay && dispLine < displayCount; dispLine++ {
		bufLine := fs.DisplayToBuf(dispLine)
		y := (dispLine-firstDisplay)*gr.LineHeight + topPad - pixelOff

		collapsed := fs.IsCollapsed(bufLine)
		fg := gr.FgColor
		if collapsed {
			fg = foldCollapsedColor
		}
		gr.renderLineNumberColored(gtx, ops, bufLine+1, totalBufLines, y, fg)

		if collapsed {
			gr.renderFoldIcon(gtx, ops, totalBufLines, y)
		}
	}

	return width
}
