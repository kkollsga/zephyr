package main

import (
	"fmt"
	"image"
	"image/color"
	"unicode/utf8"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/kristianweb/zephyr/internal/highlight"
	"github.com/kristianweb/zephyr/internal/render"
)

func (st *appState) draw(gtx layout.Context, w *app.Window) {
	ed := st.activeEd()
	ts := st.activeTabState()

	// Background
	paint.ColorOp{Color: st.theme.Background}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	// Tab bar
	st.drawTabBar(gtx)

	if ed == nil || ts == nil {
		st.lastMaxY = gtx.Constraints.Max.Y
		st.lastMaxX = gtx.Constraints.Max.X
		return
	}

	// Update viewport
	statusH := 0
	if st.statusRend != nil {
		statusH = st.statusRend.LineHeightPx + 6
	}
	ts.viewport.TotalLines = ed.Buffer.LineCount()
	if st.textRend.LineHeightPx > 0 {
		ts.viewport.VisibleLines = (gtx.Constraints.Max.Y - statusH - st.tabBarHeight - editorTopPad) / st.textRend.LineHeightPx
	}
	// Only scroll to reveal cursor when it has actually moved (not during
	// trackpad/mouse-wheel scrolling, which should move the viewport freely).
	if ed.Cursor.Line != ts.lastCursorLine || ed.Cursor.Col != ts.lastCursorCol {
		ts.viewport.ScrollToRevealCursor(ed.Cursor.Line)
		ts.lastCursorLine = ed.Cursor.Line
		ts.lastCursorCol = ed.Cursor.Col
		if st.scrollbarRend != nil {
			st.scrollbarRend.NotifyScroll()
		}
	}

	// Offset everything below the tab bar and clip to the editor area.
	editorH := gtx.Constraints.Max.Y - st.tabBarHeight - statusH
	tabOff := op.Offset(image.Pt(0, st.tabBarHeight)).Push(gtx.Ops)
	editorClip := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, editorH)}.Push(gtx.Ops)

	// Gutter
	firstLine, lastLine := ts.viewport.VisibleRange()
	gutterWidth := st.gutterRend.RenderGutter(gtx, gtx.Ops, firstLine, lastLine, ts.viewport.TotalLines, editorTopPad, ts.viewport.PixelOffset)

	// Gutter right separator
	sepRect := clip.Rect{
		Min: image.Pt(gutterWidth-1, 0),
		Max: image.Pt(gutterWidth, gtx.Constraints.Max.Y),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.GutterSep}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	sepRect.Pop()

	// Highlight tokens
	var allTokens []highlight.Token
	if ts.highlighter != nil {
		allTokens = ts.highlighter.Tokens()
	}

	// Find match highlights (drawn before text so text is readable on top)
	if st.findBar.Visible && len(st.findBar.Matches) > 0 {
		textX := gutterWidth + st.textRend.CharWidth
		for i, match := range st.findBar.Matches {
			if match.Line < firstLine || match.Line > lastLine {
				continue
			}
			lineText, err := ed.Buffer.Line(match.Line)
			if err != nil {
				continue
			}
			dispCol := runeColToDisplayCol(lineText, match.Col, 4)
			matchRuneLen := matchDisplayLen(lineText, match.Col, match.Length, 4)

			bgColor := st.theme.FindMatch
			if i == st.findBar.CurrentMatch-1 {
				bgColor = st.theme.FindCurrent
			}

			visY := (match.Line-firstLine)*st.textRend.LineHeightPx + editorTopPad - ts.viewport.PixelOffset
			x1 := textX + st.textRend.ColX(dispCol)
			x2 := textX + st.textRend.ColX(dispCol+matchRuneLen)
			rect := clip.Rect{
				Min: image.Pt(x1, visY),
				Max: image.Pt(x2, visY+st.textRend.LineHeightPx),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: bgColor}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			rect.Pop()
		}
	}

	// Visible text lines — use cached byte offsets when available
	byteOffset := 0
	if firstLine < len(ts.byteOffsets) {
		byteOffset = ts.byteOffsets[firstLine]
	} else {
		for i := 0; i < firstLine && i < ts.viewport.TotalLines; i++ {
			line, _ := ed.Buffer.Line(i)
			byteOffset += len(line) + 1
		}
	}

	for i := firstLine; i <= lastLine && i < ts.viewport.TotalLines; i++ {
		line, err := ed.Buffer.Line(i)
		if err != nil {
			continue
		}
		y := (i-firstLine)*st.textRend.LineHeightPx + editorTopPad - ts.viewport.PixelOffset

		var spans []render.ColorSpan
		if len(allTokens) > 0 {
			lineStart := byteOffset
			lineEnd := byteOffset + len(line)
			lineTokens := tokensForRange(allTokens, lineStart, lineEnd)
			spans = render.TokensToColorSpans(lineTokens, lineStart, lineEnd, line, st.colorMap, st.theme.Foreground, 4)
		}

		// Override text color to dark on the current find match
		if st.findBar.Visible && st.findBar.CurrentMatch > 0 && st.findBar.CurrentMatch <= len(st.findBar.Matches) {
			cm := st.findBar.Matches[st.findBar.CurrentMatch-1]
			if cm.Line == i {
				darkText := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
				dispStart := runeColToDisplayCol(line, cm.Col, 4)
				dispEnd := dispStart + matchDisplayLen(line, cm.Col, cm.Length, 4)
				// Prepend so it takes priority over syntax color spans
				spans = append([]render.ColorSpan{{Start: dispStart, End: dispEnd, Color: darkText}}, spans...)
			}
		}

		expandedLine := expandTabs(line, 4)
		st.textRend.RenderLine(gtx.Ops, gtx, expandedLine, gutterWidth+st.textRend.CharWidth, y, spans)
		byteOffset += len(line) + 1
	}

	// Offset cursor and selection by top padding minus scroll pixel offset
	padOff := op.Offset(image.Pt(0, editorTopPad-ts.viewport.PixelOffset)).Push(gtx.Ops)

	// Selection (skip when find bar is active — FindCurrent highlight replaces it)
	if ed.Selection.Active && !ed.Selection.IsEmpty() && !st.findBar.Visible {
		start, end := ed.Selection.Ordered()
		st.cursorRend.RenderSelection(gtx.Ops, st.theme.Selection,
			start.Line, start.Col, end.Line, end.Col,
			firstLine, gutterWidth+st.textRend.CharWidth, gtx.Constraints.Max.Y,
			func(line int) int {
				l, _ := ed.Buffer.Line(line)
				return utf8.RuneCountInString(l)
			})
	}

	// Cursor
	if st.cursorRend.UpdateBlink() {
		w.Invalidate()
	}
	st.cursorRend.RenderCursor(gtx.Ops, ed.Cursor.Line, ed.Cursor.Col, firstLine, gutterWidth+st.textRend.CharWidth)

	padOff.Pop()

	// Scrollbar (under overlays, fades out when idle)
	if st.scrollbarRend != nil && st.scrollbarRend.Update() {
		st.scrollbarRend.Render(gtx.Ops,
			gtx.Constraints.Max.X, editorH,
			ts.viewport.FirstLine, ts.viewport.PixelOffset,
			ts.viewport.VisibleLines, ts.viewport.TotalLines,
			st.textRend.LineHeightPx,
		)
	}

	// Find bar overlay (top-right of editor area)
	if st.findBar.Visible {
		st.drawFindBar(gtx, editorH)
	}

	// Scrollbar match indicator strip
	if st.findBar.Visible && len(st.findBar.Matches) > 0 && ts.viewport.TotalLines > 0 {
		st.drawMatchIndicator(gtx, editorH, ts.viewport.TotalLines)
	}

	editorClip.Pop()
	tabOff.Pop()

	// Status line
	st.lastMaxY = gtx.Constraints.Max.Y
	st.lastMaxX = gtx.Constraints.Max.X
	st.drawStatusLine(gtx)

	// Language selector dropdown
	if st.langSel.Visible {
		st.drawLangSelector(gtx)
	}

	// Request redraws for cursor blink and scrollbar fade animations
	gtx.Execute(op.InvalidateCmd{})
}

func (st *appState) drawTabBar(gtx layout.Context) {
	tr := st.tabRend
	if tr == nil {
		return
	}

	// Tab bar background
	bgRect := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, st.tabBarHeight)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.TabBarBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgRect.Pop()

	hoverFg := st.theme.Foreground

	m := st.tabMetrics()

	tabX := st.trafficLightPx
	textY := (st.tabBarHeight - tr.LineHeightPx) / 2

	// Hover detection — is the pointer in the tab bar area?
	inTabBar := st.hoverY >= 0 && st.hoverY < st.tabBarHeight

	radius := gtx.Dp(6)
	dotR := gtx.Dp(3)

	for i, tab := range st.tabBar.Tabs {
		title := tab.Title
		tabW := st.tabWidth(title)

		// Active tab background with rounded top corners.
		if i == st.tabBar.ActiveIdx {
			activeRect := clip.UniformRRect(image.Rectangle{
				Min: image.Pt(tabX, 0),
				Max: image.Pt(tabX+tabW, st.tabBarHeight+radius),
			}, radius).Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabActiveBg}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			activeRect.Pop()
		}

		// Tab title — layout: [leftPad] title [innerGap] closeBtn [rightPad]
		fg := st.theme.TabDimFg
		if i == st.tabBar.ActiveIdx {
			fg = hoverFg
		}
		tr.RenderGlyphs(gtx.Ops, gtx, title, tabX+m.leftPad, textY, fg)

		// Close button / modified indicator — centered in closeW area
		closeX := tabX + m.leftPad + len(title)*tr.CharWidth + m.innerGap
		closeY := st.tabBarHeight / 2
		closeHitLeft := closeX
		closeHitRight := closeX + m.closeW
		closeHovered := inTabBar && st.hoverX >= closeHitLeft && st.hoverX < closeHitRight

		if tab.Editor.Modified {
			dotColor := st.theme.TabModifiedDot
			if closeHovered {
				dotColor = st.theme.TabCloseHover
			}
			dotCx := closeX + m.closeW/2
			dotEllipse := clip.Ellipse{
				Min: image.Pt(dotCx-dotR, closeY-dotR),
				Max: image.Pt(dotCx+dotR, closeY+dotR),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: dotColor}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			dotEllipse.Pop()
		} else {
			xFg := st.theme.TabCloseBtn
			if closeHovered {
				xFg = st.theme.TabCloseHover
			}
			// Center the "x" glyph within closeW
			xGlyphX := closeX + (m.closeW-tr.CharWidth)/2
			tr.RenderGlyphs(gtx.Ops, gtx, "x", xGlyphX, textY, xFg)
		}

		tabX += tabW

		// Separator between tabs — same color as bottom border
		if i < len(st.tabBar.Tabs)-1 {
			vPad := st.tabBarHeight / 4
			sepRect := clip.Rect{
				Min: image.Pt(tabX-1, vPad),
				Max: image.Pt(tabX, st.tabBarHeight-vPad),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			sepRect.Pop()
		}
	}

	// "+" button
	tabX += m.tabGap
	plusHovered := inTabBar && st.hoverX >= tabX && st.hoverX < tabX+m.plusW
	plusFg := st.theme.TabCloseBtn
	if plusHovered {
		plusFg = st.theme.TabPlusHover
	}
	plusY := (st.tabBarHeight - st.plusRend.LineHeightPx) / 2
	st.plusRend.RenderGlyphs(gtx.Ops, gtx, "+", tabX+(m.plusW-st.plusRend.CharWidth)/2, plusY, plusFg)
	plusEndX := tabX + m.plusW

	// App title and subtitle (right of "+" if space allows)
	titleX := plusEndX + m.titleGap
	titleText := "Zephyr"
	titleW := len(titleText) * tr.CharWidth
	if titleX+titleW < gtx.Constraints.Max.X-20 {
		tr.RenderGlyphs(gtx.Ops, gtx, titleText, titleX, textY, st.theme.TitleFg)

		subtitleText := "The caffeinated editor"
		subtitleX := titleX + titleW + tr.CharWidth
		subtitleW := len(subtitleText) * tr.CharWidth
		if subtitleX+subtitleW < gtx.Constraints.Max.X-20 {
			tr.RenderGlyphs(gtx.Ops, gtx, subtitleText, subtitleX, textY, st.theme.SubtitleFg)
		}
	}

	// Bottom border — same color as tab separators
	tabBorderRect := clip.Rect{
		Min: image.Pt(0, st.tabBarHeight-1),
		Max: image.Pt(gtx.Constraints.Max.X, st.tabBarHeight),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	tabBorderRect.Pop()
}

func (st *appState) drawStatusLine(gtx layout.Context) {
	sr := st.statusRend
	if sr == nil || sr.LineHeightPx == 0 {
		return
	}
	ed := st.activeEd()
	ts := st.activeTabState()

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

	// line:col on left
	if ed != nil {
		status := fmt.Sprintf("%d:%d", ed.Cursor.Line+1, ed.Cursor.Col+1)
		sr.RenderGlyphs(gtx.Ops, gtx, status, 8, textY, st.theme.StatusFg)
	}

	// Language on right
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
	langWidth := len(lang) * sr.CharWidth
	st.langLabelX = gtx.Constraints.Max.X - langWidth - 12
	sr.RenderGlyphs(gtx.Ops, gtx, lang, st.langLabelX, textY, st.theme.StatusFg)
}
