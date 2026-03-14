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
