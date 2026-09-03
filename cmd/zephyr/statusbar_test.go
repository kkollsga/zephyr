package main

import (
	"strings"
	"testing"
	"time"

	"github.com/kristianweb/zephyr/internal/vim"
)

// statusSpans names the drawn spans so a failure can say which two collided.
func statusSpans(lay statusBarLayout) []struct {
	name string
	span statusSpan
	text string
} {
	return []struct {
		name string
		span statusSpan
		text string
	}{
		{"cursor", lay.Cursor, lay.CursorText},
		{"badge", lay.Badge, lay.BadgeText},
		{"vim", lay.Vim, lay.VimText},
		{"vimcommand", lay.VimCommand, lay.VimCommandText},
		{"notification", lay.Notify, lay.NotifyText},
	}
}

func assertStatusBarDisjoint(t *testing.T, in statusBarInput, lay statusBarLayout) {
	t.Helper()
	spans := statusSpans(lay)
	for i := range spans {
		a := spans[i]
		if a.span.W <= 0 {
			continue
		}
		if a.span.X < 0 || a.span.end() > in.RightEdge {
			t.Errorf("%s span %v (%q) escapes the bar [0,%d)", a.name, a.span, a.text, in.RightEdge)
		}
		for _, b := range spans[i+1:] {
			if a.span.overlaps(b.span) {
				t.Errorf("%s %v (%q) overlaps %s %v (%q)",
					a.name, a.span, a.text, b.name, b.span, b.text)
			}
		}
	}
}

// The compare overlay's badge is the longest the bar ever shows. With vim on it
// used to run straight under the centred "Vim" indicator and the notification
// offset beside it, leaving all three unreadable.
func TestStatusBarLayoutSeparatesBadgeIndicatorAndNotification(t *testing.T) {
	st, ed, ts, _ := conflictedTab(t, "mine ", "theirs\n")
	st.saveTab(st.tabBar.Tabs[0])
	st.handleEditEvent("c")
	if ts.compareDiff == nil {
		t.Fatal("compare overlay is not up, so there is no badge to lay out")
	}
	st.vimEnabled = true
	st.vimState = vim.NewState()
	st.notification = "Saved contested.go"
	st.notificationUntil = time.Now().Add(time.Minute)

	const charWidth = 8
	for _, maxX := range []int{1400, 900, 640} {
		rightEdge := maxX - len("Go")*charWidth - 12
		in := st.statusBarInputFor(maxX, charWidth, rightEdge, ed)
		if in.Badge == "" || in.Vim == "" || in.Notify == "" {
			t.Fatalf("width %d: input is missing an element: %+v", maxX, in)
		}
		lay := layoutStatusBar(in)
		if lay.Badge.W <= 0 || lay.Vim.W <= 0 {
			t.Errorf("width %d: badge or indicator dropped: %+v", maxX, lay)
		}
		assertStatusBarDisjoint(t, in, lay)
	}
}

// A bar too narrow for the full badge truncates the badge rather than running
// it under the indicator, and says so with an ellipsis.
func TestStatusBarLayoutTruncatesTheBadgeOnlyWhenOutOfRoom(t *testing.T) {
	badge := "Comparing with disk — markers are the buffer's; Esc returns"
	wide := layoutStatusBar(statusBarInput{
		MaxX: 1400, CharWidth: 8, RightEdge: 1372,
		Cursor: "1:1", Badge: badge, Vim: "Vim",
	})
	if wide.BadgeText != badge {
		t.Errorf("a wide bar truncated the badge: %q", wide.BadgeText)
	}

	narrow := statusBarInput{
		MaxX: 420, CharWidth: 8, RightEdge: 392,
		Cursor: "1:1", Badge: badge, Vim: "Vim", Notify: "Saved",
	}
	lay := layoutStatusBar(narrow)
	if lay.BadgeText == badge {
		t.Errorf("a %dpx bar drew the badge in full", narrow.MaxX)
	}
	if lay.BadgeText != "" && !strings.HasSuffix(lay.BadgeText, "…") {
		t.Errorf("truncated badge %q does not end in an ellipsis", lay.BadgeText)
	}
	if lay.Vim.W <= 0 {
		t.Error("the mode indicator was dropped instead of the badge being cut")
	}
	assertStatusBarDisjoint(t, narrow, lay)
}

// Without a badge the indicator keeps the centre it has always had, and the
// notification keeps the centre when vim is off.
func TestStatusBarLayoutKeepsTheCentreWhenNothingContestsIt(t *testing.T) {
	lay := layoutStatusBar(statusBarInput{
		MaxX: 900, CharWidth: 8, RightEdge: 872, Cursor: "1:1", Vim: "Vim",
	})
	if want := (900 - 3*8) / 2; lay.Vim.X != want {
		t.Errorf("uncontested indicator at x=%d, want centred at %d", lay.Vim.X, want)
	}

	lay = layoutStatusBar(statusBarInput{
		MaxX: 900, CharWidth: 8, RightEdge: 872, Cursor: "1:1", Notify: "Saved",
	})
	if want := (900 - 5*8) / 2; lay.Notify.X != want {
		t.Errorf("notification without vim at x=%d, want centred at %d", lay.Notify.X, want)
	}
}

// The click that opens the tutor hit-tests against vimIndicatorX/W, so the draw
// path has to publish the span the layout actually placed — not the centre it
// used to assume.
func TestDrawStatusLinePublishesTheIndicatorHitArea(t *testing.T) {
	gtx := headlessContext()
	st, ed, ts := testAppWithText("hello\n", "Plain Text")
	st.initRenderers(gtx)
	ts.conflict = conflictModified
	st.vimEnabled = true
	st.vimState = vim.NewState()
	st.notification = "External change: reloaded from disk"
	st.notificationUntil = time.Now().Add(time.Minute)

	st.drawStatusLine(gtx, ed, ts)

	lay := layoutStatusBar(st.statusBarInputFor(
		gtx.Constraints.Max.X, st.statusRend.CharWidth, st.langLabelX, ed))
	if lay.Vim.W <= 0 {
		t.Fatal("no indicator was placed")
	}
	if st.vimIndicatorX != lay.Vim.X || st.vimIndicatorW != lay.Vim.W {
		t.Errorf("hit area = {%d %d}, drawn at %v", st.vimIndicatorX, st.vimIndicatorW, lay.Vim)
	}
	if lay.Badge.W > 0 && lay.Badge.end() > st.vimIndicatorX {
		t.Errorf("badge %v (%q) covers the indicator hit area at x=%d",
			lay.Badge, lay.BadgeText, st.vimIndicatorX)
	}
}
