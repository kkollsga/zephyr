package render

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
)

func testLayoutContext() layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(800, 600)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
}

func TestTextRendererMetricsMeasurementAndOps(t *testing.T) {
	gtx := testLayoutContext()
	tr := NewTextRenderer(text.NewShaper(), TextStyle{FontSize: 13, LineHeight: 1.4, Foreground: color.NRGBA{A: 255}})
	tr.ComputeMetrics(gtx)
	if tr.CharWidth <= 0 || tr.CharAdvance <= 0 || tr.LineHeightPx <= 0 {
		t.Fatalf("invalid metrics: %+v", tr)
	}
	if tr.MeasureString(gtx, "MMMM") <= tr.MeasureString(gtx, "M") {
		t.Fatal("MeasureString did not grow with text")
	}
	if tr.ColX(3) <= tr.ColX(2) || Round32(2.6) != 3 {
		t.Fatal("column positioning helpers are not monotonic")
	}
	tr.RenderLine(gtx.Ops, gtx, "abc", 5, 5, nil)
	tr.RenderLine(gtx.Ops, gtx, "abc", 5, 25, []ColorSpan{{Start: 1, End: 2, Color: color.NRGBA{R: 255, A: 255}}})
	tr.RenderGlyphs(gtx.Ops, gtx, "status", 5, 45, color.NRGBA{A: 255})
}

func TestCursorBlinkAndRenderPaths(t *testing.T) {
	cr := NewCursorRenderer(color.NRGBA{A: 255}, 8, 8.25, 20)
	cr.lastBlink = time.Now().Add(-time.Second)
	if !cr.UpdateBlink() || cr.BlinkOn {
		t.Fatal("expired cursor blink did not toggle off")
	}
	if cr.UpdateBlink() {
		t.Fatal("cursor toggled again before blink interval")
	}
	cr.ResetBlink()
	if !cr.BlinkOn || time.Since(cr.LastBlinkTime()) > time.Second {
		t.Fatal("ResetBlink did not restore a fresh visible cursor")
	}
	var ops op.Ops
	cr.RenderCursor(&ops, 2, 3, 1, 40)
	cr.BlockMode = true
	cr.RenderCursor(&ops, 2, 3, 1, 40)
	cr.RenderSelection(&ops, color.NRGBA{A: 100}, 0, 1, 2, 3, 0, 40, 600, func(line int) int { return 10 + line })
}

func TestScrollbarLifecycleAndRenderGuards(t *testing.T) {
	sr := NewScrollbarRenderer(color.NRGBA{R: 100, A: 200})
	if sr.IsAnimating() || sr.Update() {
		t.Fatal("new scrollbar should be idle")
	}
	sr.NotifyScroll()
	if !sr.IsAnimating() || !sr.Update() {
		t.Fatal("notified scrollbar should animate")
	}
	sr.lastScrollAt = time.Now().Add(-sr.FadeDelay - sr.FadeDuration - time.Millisecond)
	if sr.Update() || sr.IsAnimating() {
		t.Fatal("scrollbar did not finish fading")
	}
	var ops op.Ops
	sr.opacity = 1
	sr.Render(&ops, 800, 600, 50, 10, 30, 300, 20)
	sr.Render(&ops, 800, 600, 0, 0, 300, 300, 20)
}

func TestGutterSizingAndRenderPaths(t *testing.T) {
	gtx := testLayoutContext()
	gr := &GutterRenderer{
		Shaper:     text.NewShaper(),
		FontSize:   11,
		FgColor:    color.NRGBA{R: 180, G: 180, B: 180, A: 255},
		BgColor:    color.NRGBA{R: 20, G: 20, B: 20, A: 255},
		CharWidth:  8,
		LineHeight: 20,
	}
	if got := gr.Width(9); got != 40 || gr.EstimateWidth(10000) != 56 {
		t.Fatalf("gutter widths small=%d large=%d", got, gr.EstimateWidth(10000))
	}
	gr.RenderLineNumber(gtx, gtx.Ops, 1, 100, 0)
	if got := gr.RenderGutter(gtx, gtx.Ops, 2, 5, 100, 10, 3); got != 40 {
		t.Fatalf("RenderGutter width = %d", got)
	}
	added := color.NRGBA{G: 255, A: 255}
	modified := color.NRGBA{B: 255, A: 255}
	deleted := color.NRGBA{R: 255, A: 255}
	gr.RenderDiffSign(gtx.Ops, 10, 20, '+', added, modified, deleted)
	gr.RenderDiffSign(gtx.Ops, 30, 20, '~', added, modified, deleted)
	gr.RenderDiffSign(gtx.Ops, 50, 20, '-', added, modified, deleted)

	fs := NewFoldState()
	fs.SetRegions([]FoldRegion{{StartLine: 1, EndLine: 4}}, 8)
	fs.Toggle(1, 8)
	if got := gr.RenderGutterFolded(gtx, gtx.Ops, 0, 4, 8, fs, 10, 3); got != 40 {
		t.Fatalf("RenderGutterFolded width = %d", got)
	}
}

// TestDiffSignAppearance asserts what RenderDiffSign draws for each sign. The
// ops list it emits into is opaque to a test, so the colour choice and the
// deleted wedge's outline are asserted through the helpers the renderer itself
// draws from — including '-', which the renderer used to drop on the floor.
func TestDiffSignAppearance(t *testing.T) {
	added := color.NRGBA{G: 255, A: 255}
	modified := color.NRGBA{B: 255, A: 255}
	deleted := color.NRGBA{R: 255, A: 255}

	colorCases := []struct {
		sign  rune
		want  color.NRGBA
		drawn bool
	}{
		{'+', added, true},
		{'~', modified, true},
		{'-', deleted, true},
		{' ', color.NRGBA{}, false},
		{'?', color.NRGBA{}, false},
	}
	for _, tc := range colorCases {
		got, drawn := diffSignColor(tc.sign, added, modified, deleted)
		if drawn != tc.drawn || got != tc.want {
			t.Errorf("diffSignColor(%q) = (%v, %v), want (%v, %v)", tc.sign, got, drawn, tc.want, tc.drawn)
		}
	}

	// A right-pointing wedge on the gutter's left edge, vertically centred on
	// the line it marks.
	pts := deletedWedgePoints(50, 20)
	want := [3]f32.Point{{X: 1, Y: 55}, {X: 1, Y: 65}, {X: 6, Y: 60}}
	if pts != want {
		t.Errorf("deletedWedgePoints(50, 20) = %v, want %v", pts, want)
	}
	// A cramped line still gets a wedge with a floor on its size.
	if pts := deletedWedgePoints(0, 4); pts[2].X-pts[0].X < 3 || pts[1].Y-pts[0].Y < 6 {
		t.Errorf("deletedWedgePoints(0, 4) = %v, want a wedge at the minimum size", pts)
	}
}
