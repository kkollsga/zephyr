package main

import (
	"fmt"
	"image"
	"time"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/vim"
)

// statusLeftPad is the status bar's left margin, and statusCursorCols the
// column count the cursor position is reserved, so the badge beside it does not
// jitter as the line number gains a digit.
const (
	statusLeftPad    = 8
	statusCursorCols = len("999:999")
)

// statusSpan is one horizontal slot in the status bar, covering [X, X+W).
type statusSpan struct {
	X int
	W int
}

func (s statusSpan) end() int { return s.X + s.W }

// overlaps reports whether two drawn spans share a pixel. An undrawn span
// (W <= 0) overlaps nothing.
func (s statusSpan) overlaps(o statusSpan) bool {
	if s.W <= 0 || o.W <= 0 {
		return false
	}
	return s.X < o.end() && o.X < s.end()
}

// statusBarInput is everything the status bar's left-to-right layout depends
// on. It is plain data so the layout can be exercised without a Gio context.
type statusBarInput struct {
	MaxX       int // status bar width in pixels
	CharWidth  int // advance of the status renderer's monospaced glyph
	RightEdge  int // leftmost pixel of the right-hand cluster (pill, language)
	Cursor     string
	Badge      string
	Vim        string
	VimCommand string
	Notify     string
}

// statusBarLayout is the placement the bar draws from: a span per element plus
// the text actually drawn in it, which is the input text truncated when the bar
// ran out of room.
type statusBarLayout struct {
	Cursor     statusSpan
	Badge      statusSpan
	Vim        statusSpan
	VimCommand statusSpan
	Notify     statusSpan

	CursorText     string
	BadgeText      string
	VimText        string
	VimCommandText string
	NotifyText     string
}

func statusTextW(s string, charWidth int) int {
	return utf8.RuneCountInString(s) * charWidth
}

// fitStatusText shortens s to at most width pixels, marking the cut with an
// ellipsis. It returns "" when not even one glyph fits.
func fitStatusText(s string, charWidth, width int) (string, int) {
	full := statusTextW(s, charWidth)
	if full <= width {
		return s, full
	}
	cols := width / charWidth
	if cols < 1 {
		return "", 0
	}
	if cols == 1 {
		return "…", charWidth
	}
	return string([]rune(s)[:cols-1]) + "…", cols * charWidth
}

// layoutStatusBar places the bar's left-hand elements strictly left to right —
// cursor position, conflict badge, mode indicator, its command line, then the
// notification — so no two can be drawn over each other. The mode indicator
// keeps the bar's centre while the badge leaves it free, and slides right of
// the badge (at most up to the right-hand cluster) when it does not; the badge
// is truncated only once that slide has nowhere left to go.
func layoutStatusBar(in statusBarInput) statusBarLayout {
	var out statusBarLayout
	cw := in.CharWidth
	if cw <= 0 {
		return out
	}
	right := in.RightEdge
	if right > in.MaxX {
		right = in.MaxX
	}

	out.CursorText = in.Cursor
	out.Cursor = statusSpan{X: statusLeftPad, W: statusTextW(in.Cursor, cw)}

	// The badge starts at a fixed column, so it holds still while the cursor
	// position changes width, but never underneath a wider position than that.
	flow := statusLeftPad + statusCursorCols*cw
	if end := out.Cursor.end(); end > flow {
		flow = end
	}
	flow += cw

	// The indicator is a click target, so it outranks the badge for room: the
	// badge yields to it, not the other way round.
	vimW := statusTextW(in.Vim, cw)
	if in.Badge != "" {
		avail := right - flow
		if vimW > 0 {
			avail -= vimW + cw
		}
		if text, w := fitStatusText(in.Badge, cw, avail); w > 0 {
			out.BadgeText, out.Badge = text, statusSpan{X: flow, W: w}
			flow = out.Badge.end() + cw
		}
	}

	if vimW > 0 {
		x := (in.MaxX - vimW) / 2
		if x+vimW > right {
			x = right - vimW
		}
		if x < flow {
			x = flow
		}
		out.VimText = in.Vim
		out.Vim = statusSpan{X: x, W: vimW}
		flow = out.Vim.end() + cw

		if in.VimCommand != "" {
			if text, w := fitStatusText(in.VimCommand, cw, right-flow); w > 0 {
				out.VimCommandText, out.VimCommand = text, statusSpan{X: flow, W: w}
				flow = out.VimCommand.end() + cw
			}
		}
	}

	if in.Notify != "" {
		x := flow
		// With no indicator beside it the notification takes the centre, but
		// only where that clears the badge and the right-hand cluster both.
		if vimW == 0 {
			if c := (in.MaxX - statusTextW(in.Notify, cw)) / 2; c > x && c+statusTextW(in.Notify, cw) <= right {
				x = c
			}
		}
		if text, w := fitStatusText(in.Notify, cw, right-x); w > 0 {
			out.NotifyText, out.Notify = text, statusSpan{X: x, W: w}
		}
	}
	return out
}

// statusBarInputFor collects the bar's texts. It clears an expired
// notification, which is the only state it touches.
func (st *appState) statusBarInputFor(maxX, charWidth, rightEdge int, ed *editor.Editor) statusBarInput {
	in := statusBarInput{
		MaxX:      maxX,
		CharWidth: charWidth,
		RightEdge: rightEdge,
		Badge:     st.statusConflictBadge(),
	}
	if ed != nil {
		in.Cursor = fmt.Sprintf("%d:%d", ed.Cursor.Line+1, ed.Cursor.Col+1)
	}
	if st.vimEnabled && st.vimState != nil {
		in.Vim = "Vim"
		switch st.vimState.Mode {
		case vim.ModeCommand:
			in.VimCommand = ":" + st.vimState.CommandLine
		case vim.ModeSearch:
			prefix := "/"
			if st.vimState.SearchDir < 0 {
				prefix = "?"
			}
			in.VimCommand = prefix + st.vimState.CommandLine
		}
	}
	if st.notification != "" {
		if time.Now().Before(st.notificationUntil) {
			in.Notify = st.notification
		} else {
			st.notification = ""
		}
	}
	return in
}

func (st *appState) drawStatusLine(gtx layout.Context, ed *editor.Editor, ts *tabState) {
	sr := st.statusRend
	if sr == nil || sr.LineHeightPx == 0 {
		return
	}

	statusH := sr.LineHeightPx + 6
	y := gtx.Constraints.Max.Y - statusH

	// Top border
	borderOff := op.Offset(image.Pt(0, y-1)).Push(gtx.Ops)
	borderRect := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, 1)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.StatusBorder}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	borderRect.Pop()
	borderOff.Pop()

	// Background
	offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
	rect := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, statusH)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.StatusBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	rect.Pop()
	offset.Pop()

	textY := y + 3

	// The right-hand cluster is placed first: its left edge is the ceiling the
	// left-to-right layout has to stay under.
	rightEdge := st.drawStatusRight(gtx, ed, ts, y, statusH, textY)
	lay := layoutStatusBar(st.statusBarInputFor(gtx.Constraints.Max.X, sr.CharWidth, rightEdge, ed))

	if lay.Cursor.W > 0 {
		sr.RenderGlyphs(gtx.Ops, gtx, lay.CursorText, lay.Cursor.X, textY, st.theme.StatusFg)
	}
	// The badge stands for as long as the conflict does, unlike the
	// notification beside it.
	if lay.Badge.W > 0 {
		sr.RenderGlyphs(gtx.Ops, gtx, lay.BadgeText, lay.Badge.X, textY, warningColor())
	}
	// vimIndicatorX/W are the click target that opens the tutor.
	st.vimIndicatorX, st.vimIndicatorW = lay.Vim.X, lay.Vim.W
	if lay.Vim.W > 0 {
		sr.RenderGlyphs(gtx.Ops, gtx, lay.VimText, lay.Vim.X, textY, vimGreen)
	}
	if lay.VimCommand.W > 0 {
		sr.RenderGlyphs(gtx.Ops, gtx, lay.VimCommandText, lay.VimCommand.X, textY, st.theme.StatusFg)
		gtx.Execute(op.InvalidateCmd{})
	}
	if lay.Notify.W > 0 {
		sr.RenderGlyphs(gtx.Ops, gtx, lay.NotifyText, lay.Notify.X, textY, st.theme.Foreground)
	}
	if st.notification != "" {
		gtx.Execute(op.InvalidateCmd{})
	}
}

// drawStatusRight draws the right-hand cluster — the language label and the
// Markdown or JSON pill beside it — and returns the leftmost pixel it occupies.
// The two pills share the slot: JSON and Markdown are mutually exclusive
// languages, so only one of them is ever live.
func (st *appState) drawStatusRight(gtx layout.Context, ed *editor.Editor, ts *tabState, y, statusH, textY int) int {
	sr := st.statusRend

	lang := ""
	if ts != nil {
		lang = ts.langLabel
	}
	if lang == "" && ed != nil {
		lang = detectLanguage(ed.FilePath)
	}
	if lang == "" {
		lang = "Plain Text"
	}
	st.langLabelX = gtx.Constraints.Max.X - len(lang)*sr.CharWidth - 12
	sr.RenderGlyphs(gtx.Ops, gtx, lang, st.langLabelX, textY, st.theme.StatusFg)

	st.mdToggleX, st.mdToggleW = 0, 0
	st.fmtToggleX, st.fmtToggleW = 0, 0

	// The pill label shows the document's CURRENT state, not the state a click
	// would move it to.
	label := ""
	switch {
	case lang == "Markdown" && ts != nil:
		label = "Edit"
		if ts.mode == viewMarkdownRead {
			label = "Read"
		}
	case lang == "JSON" && ts != nil && ed != nil:
		label = "Expanded"
		if fmtIsCompact(ed) {
			label = "Compact"
		}
	}
	if label == "" {
		return st.langLabelX
	}

	pad := sr.CharWidth
	pillW := len(label)*sr.CharWidth + pad*2
	pillX := st.langLabelX - pillW - sr.CharWidth

	pillOff := op.Offset(image.Pt(pillX, y+1)).Push(gtx.Ops)
	pillRect := clip.UniformRRect(image.Rectangle{Max: image.Pt(pillW, statusH-2)}, 3).Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	pillRect.Pop()
	pillOff.Pop()

	sr.RenderGlyphs(gtx.Ops, gtx, label, pillX+pad, textY, st.theme.Foreground)

	if lang == "JSON" {
		st.fmtToggleX, st.fmtToggleW = pillX, pillW
	} else {
		st.mdToggleX, st.mdToggleW = pillX, pillW
	}
	return pillX
}
