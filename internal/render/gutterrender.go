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

// Width returns the pixel width of the gutter for the given max line number.
func (gr *GutterRenderer) Width(maxLineNum int) int {
	digits := len(fmt.Sprintf("%d", maxLineNum))
	if digits < 3 {
		digits = 3
	}
	return (digits + 2) * gr.CharWidth
}

// RenderGutter draws line numbers for visible lines.
// yOffset adds vertical padding before the first line number.
func (gr *GutterRenderer) RenderGutter(gtx layout.Context, ops *op.Ops, firstLine, lastLine, totalLines int, yOffset ...int) int {
	topPad := 0
	if len(yOffset) > 0 {
		topPad = yOffset[0]
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
		y := (i-firstLine)*gr.LineHeight + topPad

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
