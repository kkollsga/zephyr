package render

import "testing"

func TestViewport_VisibleRange_TopOfFile(t *testing.T) {
	v := &Viewport{FirstLine: 0, VisibleLines: 40, TotalLines: 100, ScrollMargin: 3}
	first, last := v.VisibleRange()
	if first != 0 || last != 39 {
		t.Fatalf("got [%d, %d], want [0, 39]", first, last)
	}
}

func TestViewport_VisibleRange_Scrolled(t *testing.T) {
	v := &Viewport{FirstLine: 20, VisibleLines: 40, TotalLines: 100, ScrollMargin: 3}
	first, last := v.VisibleRange()
	if first != 20 || last != 59 {
		t.Fatalf("got [%d, %d], want [20, 59]", first, last)
	}
}

func TestViewport_VisibleRange_NearEndOfFile(t *testing.T) {
	v := &Viewport{FirstLine: 90, VisibleLines: 40, TotalLines: 100, ScrollMargin: 3}
	first, last := v.VisibleRange()
	if first != 90 || last != 99 {
		t.Fatalf("got [%d, %d], want [90, 99]", first, last)
	}
}

func TestViewport_ScrollToRevealCursor_CursorBelowViewport(t *testing.T) {
	v := &Viewport{FirstLine: 0, VisibleLines: 40, TotalLines: 100, ScrollMargin: 3}
	v.ScrollToRevealCursor(50)
	// Cursor at 50, visible should be ~14-53
	if v.FirstLine > 50 || v.FirstLine+v.VisibleLines-1 < 50 {
		t.Fatalf("cursor 50 not visible, firstLine=%d", v.FirstLine)
	}
}

func TestViewport_ScrollToRevealCursor_CursorAboveViewport(t *testing.T) {
	v := &Viewport{FirstLine: 50, VisibleLines: 40, TotalLines: 100, ScrollMargin: 3}
	v.ScrollToRevealCursor(10)
	if v.FirstLine > 10 || v.FirstLine+v.VisibleLines-1 < 10 {
		t.Fatalf("cursor 10 not visible, firstLine=%d", v.FirstLine)
	}
}

func TestViewport_ScrollToRevealCursor_CursorVisible_NoScroll(t *testing.T) {
	v := &Viewport{FirstLine: 10, VisibleLines: 40, TotalLines: 100, ScrollMargin: 3}
	v.ScrollToRevealCursor(25)
	if v.FirstLine != 10 {
		t.Fatalf("firstLine changed to %d, should stay at 10", v.FirstLine)
	}
}

func TestViewport_ScrollBy(t *testing.T) {
	v := &Viewport{FirstLine: 0, VisibleLines: 40, TotalLines: 100, ScrollMargin: 3}
	v.ScrollBy(10)
	if v.FirstLine != 10 {
		t.Fatalf("got firstLine %d, want 10", v.FirstLine)
	}
	v.ScrollBy(-20)
	if v.FirstLine != 0 {
		t.Fatalf("got firstLine %d, want 0 (clamped)", v.FirstLine)
	}
	v.ScrollBy(200)
	if v.FirstLine != 60 { // 100-40=60
		t.Fatalf("got firstLine %d, want 60 (clamped)", v.FirstLine)
	}
}

func TestViewport_PixelScrollingAndBounds(t *testing.T) {
	v := &Viewport{VisibleLines: 5, TotalLines: 20}
	v.ScrollByPixels(25, 20)
	if v.FirstLine != 1 || v.PixelOffset != 5 || v.LastLine() != 6 {
		t.Fatalf("down scroll = line %d offset %d last %d", v.FirstLine, v.PixelOffset, v.LastLine())
	}
	v.ScrollByPixels(-10, 20)
	if v.FirstLine != 0 || v.PixelOffset != 15 {
		t.Fatalf("up scroll = line %d offset %d", v.FirstLine, v.PixelOffset)
	}
	v.ScrollByPixels(-100, 20)
	if v.FirstLine != 0 || v.PixelOffset != 0 {
		t.Fatalf("top clamp = line %d offset %d", v.FirstLine, v.PixelOffset)
	}
	v.ScrollByPixels(10000, 20)
	if v.FirstLine != 15 || v.PixelOffset != 0 {
		t.Fatalf("bottom clamp = line %d offset %d", v.FirstLine, v.PixelOffset)
	}
	v.ScrollByPixels(1, 0)
}

func TestViewport_ScrollablePixels(t *testing.T) {
	v := &Viewport{FirstLine: 3, PixelOffset: 7, VisibleLines: 5, TotalLines: 20}
	up, down := v.ScrollablePixels(20)
	if up != 67 || down != 233 {
		t.Fatalf("ScrollablePixels = (%d,%d), want (67,233)", up, down)
	}
	if got := NewViewport().ScrollMargin; got != 3 {
		t.Fatalf("NewViewport margin = %d", got)
	}
}
