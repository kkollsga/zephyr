package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"
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
		st.drawStatusLine(gtx)
		return
	}

	// Status bar height
	statusH := 0
	if st.statusRend != nil {
		statusH = st.statusRend.LineHeightPx + 6
	}
	editorH := gtx.Constraints.Max.Y - st.tabBarHeight - statusH

	// Skip viewport update in markdown read mode
	if ts.mode != viewMarkdownRead {
		ts.viewport.TotalLines = ed.Buffer.LineCount()
		if st.textRend.LineHeightPx > 0 {
			ts.viewport.VisibleLines = (gtx.Constraints.Max.Y - statusH - st.tabBarHeight - editorTopPad) / st.textRend.LineHeightPx
		}
		if ed.Cursor.Line != ts.lastCursorLine || ed.Cursor.Col != ts.lastCursorCol {
			ts.viewport.ScrollToRevealCursor(ed.Cursor.Line)
			ts.lastCursorLine = ed.Cursor.Line
			ts.lastCursorCol = ed.Cursor.Col
			if st.scrollbarRend != nil {
				st.scrollbarRend.NotifyScroll()
			}
		}
	}

	// Offset everything below the tab bar and clip to the editor area.
	tabOff := op.Offset(image.Pt(0, st.tabBarHeight)).Push(gtx.Ops)
	editorClip := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, editorH)}.Push(gtx.Ops)

	// Markdown preview mode — skip gutter, cursor, etc.
	if ts.mode == viewMarkdownRead {
		st.drawMarkdownPreview(gtx, ts)
		editorClip.Pop()
		tabOff.Pop()
		st.lastMaxY = gtx.Constraints.Max.Y
		st.lastMaxX = gtx.Constraints.Max.X
		st.drawStatusLine(gtx)
		// Save menu must render on top even in read mode
		if st.saveMenu.visible {
			st.drawSaveMenu(gtx)
		}
		return
	}

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

	// Highlight tokens — query only visible range
	var allTokens []highlight.Token
	if ts.highlighter != nil {
		allTokens = ts.highlighter.TokensInRange(firstLine, lastLine)
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

	// Visible text lines — use PieceTable's cached lineStarts
	byteOffset := ed.Buffer.LineStartOffset(firstLine)

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

	// Unified save menu dropdown
	if st.saveMenu.visible {
		st.drawSaveMenu(gtx)
	}

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

	// Compute overflow before drawing
	st.computeOverflow(gtx.Constraints.Max.X)

	// Tab bar background
	bgRect := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, st.tabBarHeight)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.TabBarBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgRect.Pop()

	m := st.tabMetrics()
	textY := (st.tabBarHeight - tr.LineHeightPx) / 2
	radius := gtx.Dp(6)

	// Hover detection — is the pointer in the tab bar area?
	inTabBar := st.hoverY >= 0 && st.hoverY < st.tabBarHeight

	dragging := st.tabDrag.active && st.tabDrag.started
	dragIdx := st.tabDrag.tabIdx
	var dragW int
	if dragging && dragIdx < len(st.tabBar.Tabs) {
		dragW = st.tabWidth(st.tabBar.Tabs[dragIdx].Title)
	}

	// --- Phase 1: Draw non-dragged bar tabs, leaving a gap at the drop target ---
	tabX := st.trafficLightPx
	gapInserted := false
	dropSlot := st.tabDrag.dropSlot
	slot := 0
	prevDrawn := false

	for _, ti := range st.barTabIdxs {
		if dragging && ti == dragIdx {
			continue
		}

		// Insert gap before this tab if this is the drop slot
		if dragging && st.tabDrag.dropInBar && !gapInserted && slot >= dropSlot {
			tabX += dragW
			gapInserted = true
		}

		// Separator before this tab (if something was drawn before it)
		if prevDrawn {
			vPad := st.tabBarHeight / 4
			sepRect := clip.Rect{
				Min: image.Pt(tabX-1, vPad),
				Max: image.Pt(tabX, st.tabBarHeight-vPad),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			sepRect.Pop()
		}

		st.drawSingleTab(gtx, ti, tabX, textY, radius, inTabBar)
		tabX += st.tabWidth(st.tabBar.Tabs[ti].Title)
		slot++
		prevDrawn = true
	}

	// Gap at the end if not yet inserted
	if dragging && st.tabDrag.dropInBar && !gapInserted {
		tabX += dragW
	}

	// --- Phase 2: Draw the floating dragged tab with shadow ---
	if dragging && dragIdx < len(st.tabBar.Tabs) {
		tab := st.tabBar.Tabs[dragIdx]
		floatX := st.tabDrag.currentX - dragW/2

		// Clamp to tab bar bounds
		if floatX < st.trafficLightPx {
			floatX = st.trafficLightPx
		}
		maxX := gtx.Constraints.Max.X - dragW
		if floatX > maxX {
			floatX = maxX
		}

		// Shadow behind the floating tab
		shadowOff := op.Offset(image.Pt(floatX+3, 3)).Push(gtx.Ops)
		shadowRect := clip.UniformRRect(image.Rectangle{
			Max: image.Pt(dragW, st.tabBarHeight),
		}, radius).Push(gtx.Ops)
		paint.ColorOp{Color: color.NRGBA{A: 50}}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		shadowRect.Pop()
		shadowOff.Pop()

		// Background for the floating tab
		floatBg := clip.UniformRRect(image.Rectangle{
			Min: image.Pt(floatX, 0),
			Max: image.Pt(floatX+dragW, st.tabBarHeight),
		}, radius).Push(gtx.Ops)
		paint.ColorOp{Color: st.theme.TabActiveBg}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		floatBg.Pop()

		// Title
		fg := st.theme.Foreground
		tr.RenderGlyphs(gtx.Ops, gtx, tab.Title, floatX+m.leftPad, textY, fg)

		// Close button / modified dot
		dotR := gtx.Dp(3)
		closeX := floatX + m.leftPad + utf8.RuneCountInString(tab.Title)*tr.CharWidth + m.innerGap
		closeY := st.tabBarHeight / 2
		if tab.Editor.Modified {
			dotCx := closeX + m.closeW/2
			dotEllipse := clip.Ellipse{
				Min: image.Pt(dotCx-dotR, closeY-dotR),
				Max: image.Pt(dotCx+dotR, closeY+dotR),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabModifiedDot}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			dotEllipse.Pop()
		} else {
			xGlyphX := closeX + (m.closeW-tr.CharWidth)/2
			tr.RenderGlyphs(gtx.Ops, gtx, "x", xGlyphX, textY, st.theme.TabCloseHover)
		}
	}

	// --- Phase 3: Overflow, "+" button, title, border ---
	hasOverflow := len(st.dropdownTabIdxs) > 0
	if hasOverflow {
		if dragging {
			// Float ">" and "+" to the right edge during drag so they
			// aren't pushed around by the tab gap animation.
			tabX = gtx.Constraints.Max.X - m.tabGap - m.plusW - m.tabGap - st.overflowBtnW
		} else {
			tabX += m.tabGap
		}
		st.overflowBtnX = tabX
		overflowHovered := inTabBar && st.hoverX >= tabX && st.hoverX < tabX+st.overflowBtnW
		overflowFg := st.theme.TabCloseBtn
		if overflowHovered || st.overflowOpen {
			overflowFg = st.theme.TabPlusHover
		}
		chevron := ">"
		if st.overflowOpen {
			chevron = "v"
		}
		tr.RenderGlyphs(gtx.Ops, gtx, chevron, tabX+(st.overflowBtnW-tr.CharWidth)/2, textY, overflowFg)
		tabX += st.overflowBtnW
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

		subtitleText := st.themeBundle.Subtitle
		if subtitleText == "" {
			subtitleText = "The caffeinated editor"
		}
		subtitleX := titleX + titleW + tr.CharWidth
		subtitleW := len(subtitleText) * tr.CharWidth
		if subtitleX+subtitleW < gtx.Constraints.Max.X-20 {
			tr.RenderGlyphs(gtx.Ops, gtx, subtitleText, subtitleX, textY, st.theme.SubtitleFg)
		}
	}

	// Theme toggle icon (sun/moon) in upper right
	st.drawThemeToggle(gtx, inTabBar)

	// Bottom border
	tabBorderRect := clip.Rect{
		Min: image.Pt(0, st.tabBarHeight-1),
		Max: image.Pt(gtx.Constraints.Max.X, st.tabBarHeight),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	tabBorderRect.Pop()

	// Overflow dropdown (drawn on top of everything; auto-shown during drag)
	showDropdown := st.overflowOpen
	if showDropdown && hasOverflow {
		st.drawOverflowDropdown(gtx)
	}
}

// drawSingleTab draws one tab at the given X position (used by drawTabBar).
func (st *appState) drawSingleTab(gtx layout.Context, i, tabX, textY, radius int, inTabBar bool) {
	tr := st.tabRend
	m := st.tabMetrics()
	tab := st.tabBar.Tabs[i]
	title := tab.Title
	tabW := st.tabWidth(title)
	dotR := gtx.Dp(3)

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

	// Tab title
	fg := st.theme.TabDimFg
	if i == st.tabBar.ActiveIdx {
		fg = st.theme.Foreground
	}
	tr.RenderGlyphs(gtx.Ops, gtx, title, tabX+m.leftPad, textY, fg)

	// Close button / modified indicator
	closeX := tabX + m.leftPad + utf8.RuneCountInString(title)*tr.CharWidth + m.innerGap
	closeY := st.tabBarHeight / 2
	closeHovered := inTabBar && st.hoverX >= closeX && st.hoverX < tabX+tabW

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
		xGlyphX := closeX + (m.closeW-tr.CharWidth)/2
		tr.RenderGlyphs(gtx.Ops, gtx, "x", xGlyphX, textY, xFg)
	}
}

// drawOverflowDropdown renders the dropdown menu listing overflowed tabs.
// During a drag it skips the dragged item and inserts a gap at the drop target.
func (st *appState) drawOverflowDropdown(gtx layout.Context) {
	tr := st.tabRend
	if tr == nil || len(st.dropdownTabIdxs) == 0 {
		return
	}

	dragging := st.tabDrag.active && st.tabDrag.started
	dragIdx := st.tabDrag.tabIdx

	hasHeader := st.dropdownHeader >= 0
	if dragging && st.dropdownHeader == dragIdx {
		hasHeader = false
	}
	headerItems := 0
	if hasHeader {
		headerItems = 1
	}

	// Count visible items (exclude the dragged tab)
	visCount := 0
	for _, ti := range st.dropdownTabIdxs {
		if dragging && ti == dragIdx {
			continue
		}
		visCount++
	}

	// Add header and gap slots
	displayCount := visCount + headerItems
	if dragging && !st.tabDrag.dropInBar {
		displayCount++
	}
	if displayCount == 0 {
		return
	}

	itemH := tr.LineHeightPx + 8
	dropdownW := st.overflowDropdownWidth()
	dropdownH := displayCount * itemH

	dropdownX := st.overflowBtnX + st.overflowBtnW - dropdownW
	if dropdownX < 0 {
		dropdownX = 0
	}
	dropdownY := st.tabBarHeight

	dotR := gtx.Dp(3)

	// Drop shadow
	shadowOff := op.Offset(image.Pt(dropdownX+2, dropdownY+2)).Push(gtx.Ops)
	shadowRect := clip.Rect{Max: image.Pt(dropdownW, dropdownH)}.Push(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{A: 60}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	shadowRect.Pop()
	shadowOff.Pop()

	// Background
	bgOff := op.Offset(image.Pt(dropdownX, dropdownY)).Push(gtx.Ops)
	bgClip := clip.Rect{Max: image.Pt(dropdownW, dropdownH)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.TabBarBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgClip.Pop()
	bgOff.Pop()

	// Header item (last bar tab shown for continuity)
	drawSlot := 0
	if hasHeader {
		tab := st.tabBar.Tabs[st.dropdownHeader]
		iy := dropdownY

		hovered := st.hoverX >= dropdownX && st.hoverX < dropdownX+dropdownW &&
			st.hoverY >= iy && st.hoverY < iy+itemH
		if hovered || st.dropdownHeader == st.tabBar.ActiveIdx {
			hlOff := op.Offset(image.Pt(dropdownX, iy)).Push(gtx.Ops)
			hlRect := clip.Rect{Max: image.Pt(dropdownW, itemH)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabActiveBg}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			hlRect.Pop()
			hlOff.Pop()
		}

		fg := st.theme.TabDimFg
		if st.dropdownHeader == st.tabBar.ActiveIdx {
			fg = st.theme.Foreground
		}
		textY := iy + (itemH-tr.LineHeightPx)/2
		tr.RenderGlyphs(gtx.Ops, gtx, tab.Title, dropdownX+8, textY, fg)

		if tab.Editor.Modified {
			dotCx := dropdownX + dropdownW - 12
			dotCy := iy + itemH/2
			dotEllipse := clip.Ellipse{
				Min: image.Pt(dotCx-dotR, dotCy-dotR),
				Max: image.Pt(dotCx+dotR, dotCy+dotR),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabModifiedDot}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			dotEllipse.Pop()
		}

		sepOff := op.Offset(image.Pt(dropdownX+4, iy+itemH-1)).Push(gtx.Ops)
		sepRect := clip.Rect{Max: image.Pt(dropdownW-8, 1)}.Push(gtx.Ops)
		paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		sepRect.Pop()
		sepOff.Pop()

		drawSlot = 1
	}

	// Dropdown items with optional gap
	slot := 0
	gapInserted := false
	dropSlot := st.tabDrag.dropSlot

	for _, ti := range st.dropdownTabIdxs {
		if dragging && ti == dragIdx {
			continue
		}

		// Insert gap with insertion indicator before this item if needed
		if dragging && !st.tabDrag.dropInBar && !gapInserted && slot >= dropSlot {
			st.drawDropdownInsertIndicator(gtx, tr, dropdownX, dropdownY+drawSlot*itemH, dropdownW, itemH, dragIdx)
			drawSlot++
			gapInserted = true
		}

		tab := st.tabBar.Tabs[ti]
		iy := dropdownY + drawSlot*itemH

		// Hover highlight
		hovered := st.hoverX >= dropdownX && st.hoverX < dropdownX+dropdownW &&
			st.hoverY >= iy && st.hoverY < iy+itemH
		if hovered || ti == st.tabBar.ActiveIdx {
			hlOff := op.Offset(image.Pt(dropdownX, iy)).Push(gtx.Ops)
			hlRect := clip.Rect{Max: image.Pt(dropdownW, itemH)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabActiveBg}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			hlRect.Pop()
			hlOff.Pop()
		}

		// Title text
		fg := st.theme.TabDimFg
		if ti == st.tabBar.ActiveIdx {
			fg = st.theme.Foreground
		}
		textY := iy + (itemH-tr.LineHeightPx)/2
		tr.RenderGlyphs(gtx.Ops, gtx, tab.Title, dropdownX+8, textY, fg)

		// Modified dot
		if tab.Editor.Modified {
			dotCx := dropdownX + dropdownW - 12
			dotCy := iy + itemH/2
			dotEllipse := clip.Ellipse{
				Min: image.Pt(dotCx-dotR, dotCy-dotR),
				Max: image.Pt(dotCx+dotR, dotCy+dotR),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabModifiedDot}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			dotEllipse.Pop()
		}

		// Separator between items
		if drawSlot < displayCount-1 {
			sepOff := op.Offset(image.Pt(dropdownX+4, iy+itemH-1)).Push(gtx.Ops)
			sepRect := clip.Rect{Max: image.Pt(dropdownW-8, 1)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			sepRect.Pop()
			sepOff.Pop()
		}

		slot++
		drawSlot++
	}

	// Gap at the end if not yet inserted
	if dragging && !st.tabDrag.dropInBar && !gapInserted {
		st.drawDropdownInsertIndicator(gtx, tr, dropdownX, dropdownY+drawSlot*itemH, dropdownW, itemH, dragIdx)
	}

	// Border around dropdown
	borderColor := st.theme.TabBorder
	// Top
	bOff := op.Offset(image.Pt(dropdownX, dropdownY)).Push(gtx.Ops)
	bRect := clip.Rect{Max: image.Pt(dropdownW, 1)}.Push(gtx.Ops)
	paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bRect.Pop()
	bOff.Pop()
	// Bottom
	bOff = op.Offset(image.Pt(dropdownX, dropdownY+dropdownH-1)).Push(gtx.Ops)
	bRect = clip.Rect{Max: image.Pt(dropdownW, 1)}.Push(gtx.Ops)
	paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bRect.Pop()
	bOff.Pop()
	// Left
	bOff = op.Offset(image.Pt(dropdownX, dropdownY)).Push(gtx.Ops)
	bRect = clip.Rect{Max: image.Pt(1, dropdownH)}.Push(gtx.Ops)
	paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bRect.Pop()
	bOff.Pop()
	// Right
	bOff = op.Offset(image.Pt(dropdownX+dropdownW-1, dropdownY)).Push(gtx.Ops)
	bRect = clip.Rect{Max: image.Pt(1, dropdownH)}.Push(gtx.Ops)
	paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bRect.Pop()
	bOff.Pop()
}

// drawDropdownInsertIndicator renders a highlighted preview row at the gap
// position in the overflow dropdown, showing where the dragged tab will land.
func (st *appState) drawDropdownInsertIndicator(gtx layout.Context, tr *render.TextRenderer, dx, dy, dw, itemH, dragIdx int) {
	// Highlighted background
	hlOff := op.Offset(image.Pt(dx, dy)).Push(gtx.Ops)
	hlRect := clip.Rect{Max: image.Pt(dw, itemH)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.TabActiveBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	hlRect.Pop()
	hlOff.Pop()

	// Accent line at the top of the insertion row
	lineOff := op.Offset(image.Pt(dx+4, dy)).Push(gtx.Ops)
	lineRect := clip.Rect{Max: image.Pt(dw-8, 2)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.Cursor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	lineRect.Pop()
	lineOff.Pop()

	// Semi-transparent preview of the dragged tab title
	if dragIdx < len(st.tabBar.Tabs) {
		title := st.tabBar.Tabs[dragIdx].Title
		textY := dy + (itemH-tr.LineHeightPx)/2
		fg := st.theme.TabDimFg
		fg.A = fg.A / 2
		tr.RenderGlyphs(gtx.Ops, gtx, title, dx+8, textY, fg)
	}
}

// macOS Finder tag colors (Red, Orange, Yellow, Green, Blue, Purple, Gray).
var finderTagColors = [7]color.NRGBA{
	{R: 0xFF, G: 0x3B, B: 0x30, A: 0xFF}, // Red
	{R: 0xFF, G: 0x9F, B: 0x0A, A: 0xFF}, // Orange
	{R: 0xFF, G: 0xCC, B: 0x00, A: 0xFF}, // Yellow
	{R: 0x34, G: 0xC7, B: 0x59, A: 0xFF}, // Green
	{R: 0x00, G: 0x7A, B: 0xFF, A: 0xFF}, // Blue
	{R: 0xAF, G: 0x52, B: 0xDE, A: 0xFF}, // Purple
	{R: 0x8E, G: 0x8E, B: 0x93, A: 0xFF}, // Gray
}

// saveMenuShowSaveAs returns true when the Save As rows (Name/Tag/Where/SaveAs)
// should be visible: always for untitled tabs, or when toggled for file-backed tabs.
func (st *appState) saveMenuShowSaveAs() bool {
	idx := st.saveMenu.tabIdx
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return false
	}
	return st.tabBar.Tabs[idx].Editor.FilePath == "" || st.saveMenu.saveAsExpanded
}

// saveMenuRowCount returns the number of visible rows in the save menu.
func (st *appState) saveMenuRowCount() int {
	idx := st.saveMenu.tabIdx
	fileBacked := idx >= 0 && idx < len(st.tabBar.Tabs) && st.tabBar.Tabs[idx].Editor.FilePath != ""

	n := 1 // bottom row: Save/Discard/Cancel (always)
	if st.saveMenuShowSaveAs() {
		n += 3 // Name, Tag, Where
	}
	if fileBacked {
		n++ // Save As radio toggle row
	}
	if st.saveMenu.confirmOverwrite {
		n += 2 // warning text + Overwrite/Back
	}
	return n
}

// saveMenuRect computes the dropdown position and dimensions.
func (st *appState) saveMenuRect() (x, y, w, h, itemH int) {
	tr := st.tabRend
	if tr == nil {
		return 0, 0, 0, 0, 0
	}
	itemH = tr.LineHeightPx + 8
	nRows := st.saveMenuRowCount()
	h = nRows * itemH
	w = 32 * tr.CharWidth // fixed width for the dropdown

	// Center below the prompted tab
	tabX := st.trafficLightPx
	idx := st.saveMenu.tabIdx
	for _, ti := range st.barTabIdxs {
		if ti == idx {
			tabW := st.tabWidth(st.tabBar.Tabs[ti].Title)
			x = tabX + tabW/2 - w/2
			break
		}
		tabX += st.tabWidth(st.tabBar.Tabs[ti].Title)
	}
	maxX := 0
	if st.lastMaxX > 0 {
		maxX = st.lastMaxX
	}
	if x < 0 {
		x = 0
	}
	if maxX > 0 && x+w > maxX {
		x = maxX - w
	}
	y = st.tabBarHeight
	return
}

// saveMenuCanSave returns true when the Save button should be active.
// For file-backed tabs (without Save As expanded) it's always enabled.
// When Save As rows are visible, a non-empty directory is required.
func (st *appState) saveMenuCanSave() bool {
	idx := st.saveMenu.tabIdx
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return false
	}
	fileBacked := st.tabBar.Tabs[idx].Editor.FilePath != ""
	if fileBacked && !st.saveMenu.saveAsExpanded {
		return true // normal save to existing path
	}
	return st.saveMenu.dir != ""
}

// drawSaveMenu renders the save menu dropdown.
//
// Row layout (conditional visibility):
//
//	(Save As rows, if visible):
//	  Name: [filename input]
//	  Tag:  ● ● ● ● ● ● ●
//	  Where: ~/path  ▼
//	Save button row: [Save]  ○ Save As  (file-backed: radio toggle)
//	                 [Save]              (untitled: no toggle)
//	Bottom row:      [Discard] [Cancel]
func (st *appState) drawSaveMenu(gtx layout.Context) {
	tr := st.tabRend
	if tr == nil {
		return
	}
	idx := st.saveMenu.tabIdx
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		st.saveMenu.visible = false
		return
	}
	tab := st.tabBar.Tabs[idx]
	fileBacked := tab.Editor.FilePath != ""
	showSaveAs := st.saveMenuShowSaveAs()
	canSave := st.saveMenuCanSave()

	dx, dy, dw, dropdownH, itemH := st.saveMenuRect()
	if dropdownH == 0 {
		return
	}

	// Drop shadow
	shadowOff := op.Offset(image.Pt(dx+2, dy+2)).Push(gtx.Ops)
	shadowRect := clip.Rect{Max: image.Pt(dw, dropdownH)}.Push(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{A: 60}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	shadowRect.Pop()
	shadowOff.Pop()

	// Background
	bgOff := op.Offset(image.Pt(dx, dy)).Push(gtx.Ops)
	bgClip := clip.Rect{Max: image.Pt(dw, dropdownH)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.DropdownBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgClip.Pop()
	bgOff.Pop()

	curY := dy // tracks the Y position of the current row

	// --- Save As detail rows (Name, Tag, Where) ---
	if showSaveAs {
		labelW := 6 * tr.CharWidth // width for "Name:", "Tag:", "Where:" labels
		fieldX := dx + 8 + labelW + 4

		// Name: [filename input]
		{
			iy := curY
			textY := iy + (itemH-tr.LineHeightPx)/2
			tr.RenderGlyphs(gtx.Ops, gtx, "Name:", dx+8, textY, st.theme.TabDimFg)

			inputX := fieldX
			inputW := dx + dw - 8 - inputX
			inputFieldY := iy + 3
			inputFieldH := itemH - 6
			inputBgOff := op.Offset(image.Pt(inputX, inputFieldY)).Push(gtx.Ops)
			inputBgRect := clip.Rect{Max: image.Pt(inputW, inputFieldH)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.Background}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			inputBgRect.Pop()
			inputBgOff.Pop()

			filenameStr := string(st.saveMenu.filename)
			textX := inputX + 4
			inputTextY := inputFieldY + (inputFieldH-tr.LineHeightPx)/2

			if st.saveMenu.selectAll && len(st.saveMenu.filename) > 0 {
				selW := len(st.saveMenu.filename) * tr.CharWidth
				selOff := op.Offset(image.Pt(textX, inputTextY)).Push(gtx.Ops)
				selRect := clip.Rect{Max: image.Pt(selW, tr.LineHeightPx)}.Push(gtx.Ops)
				paint.ColorOp{Color: st.theme.Selection}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				selRect.Pop()
				selOff.Pop()
			}

			tr.RenderGlyphs(gtx.Ops, gtx, filenameStr, textX, inputTextY, st.theme.Foreground)

			if !st.saveMenu.selectAll {
				cursorX := textX + st.saveMenu.cursor*tr.CharWidth
				curOff := op.Offset(image.Pt(cursorX, inputTextY)).Push(gtx.Ops)
				curRect := clip.Rect{Max: image.Pt(1, tr.LineHeightPx)}.Push(gtx.Ops)
				paint.ColorOp{Color: st.theme.Cursor}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				curRect.Pop()
				curOff.Pop()
			}

			sepOff := op.Offset(image.Pt(dx+4, iy+itemH-1)).Push(gtx.Ops)
			sepRect := clip.Rect{Max: image.Pt(dw-8, 1)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			sepRect.Pop()
			sepOff.Pop()
			curY += itemH
		}

		// Tag: colored dots
		{
			iy := curY
			textY := iy + (itemH-tr.LineHeightPx)/2
			tr.RenderGlyphs(gtx.Ops, gtx, "Tag:", dx+8, textY, st.theme.TabDimFg)

			dotSize := tr.LineHeightPx - 2
			dotGap := 4
			dotY := iy + (itemH-dotSize)/2
			dotX := fieldX
			for ti := 0; ti < 7; ti++ {
				cx := dotX + dotSize/2
				cy := dotY + dotSize/2
				r := dotSize / 2

				if st.saveMenu.tags[ti] {
					ellOff := op.Offset(image.Pt(cx-r, cy-r)).Push(gtx.Ops)
					ell := clip.Ellipse{Max: image.Pt(r*2, r*2)}.Push(gtx.Ops)
					paint.ColorOp{Color: finderTagColors[ti]}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					ell.Pop()
					ellOff.Pop()
				} else {
					ellOff := op.Offset(image.Pt(cx-r, cy-r)).Push(gtx.Ops)
					ell := clip.Ellipse{Max: image.Pt(r*2, r*2)}.Push(gtx.Ops)
					dimColor := finderTagColors[ti]
					dimColor.A = 0x80
					paint.ColorOp{Color: dimColor}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					ell.Pop()
					ellOff.Pop()

					ir := r - 2
					if ir > 0 {
						ellOff2 := op.Offset(image.Pt(cx-ir, cy-ir)).Push(gtx.Ops)
						ell2 := clip.Ellipse{Max: image.Pt(ir*2, ir*2)}.Push(gtx.Ops)
						paint.ColorOp{Color: st.theme.DropdownBg}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						ell2.Pop()
						ellOff2.Pop()
					}
				}
				dotX += dotSize + dotGap
			}

			sepOff := op.Offset(image.Pt(dx+4, iy+itemH-1)).Push(gtx.Ops)
			sepRect := clip.Rect{Max: image.Pt(dw-8, 1)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			sepRect.Pop()
			sepOff.Pop()
			curY += itemH
		}

		// Where: directory path
		{
			iy := curY
			textY := iy + (itemH-tr.LineHeightPx)/2
			tr.RenderGlyphs(gtx.Ops, gtx, "Where:", dx+8, textY, st.theme.TabDimFg)

			dirLabel := shortenDir(st.saveMenu.dir)
			if dirLabel == "" {
				dirLabel = "Choose…"
			}
			maxDirChars := (dx + dw - 8 - fieldX - 2*tr.CharWidth) / tr.CharWidth
			if maxDirChars > 0 && utf8.RuneCountInString(dirLabel) > maxDirChars {
				runes := []rune(dirLabel)
				dirLabel = "…" + string(runes[len(runes)-maxDirChars+1:])
			}

			whereHover := st.hoverX >= fieldX && st.hoverX < dx+dw-8 && st.hoverY >= iy && st.hoverY < iy+itemH
			fg := st.theme.Foreground
			if whereHover {
				fg = st.theme.Cursor
			}
			tr.RenderGlyphs(gtx.Ops, gtx, dirLabel+" ▼", fieldX, textY, fg)

			sepOff := op.Offset(image.Pt(dx+4, iy+itemH-1)).Push(gtx.Ops)
			sepRect := clip.Rect{Max: image.Pt(dw-8, 1)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			sepRect.Pop()
			sepOff.Pop()
			curY += itemH
		}
	}

	// --- Save As radio toggle row (file-backed only) ---
	if fileBacked {
		iy := curY
		textY := iy + (itemH-tr.LineHeightPx)/2

		radioLabel := "Save As"
		radioLabelW := utf8.RuneCountInString(radioLabel) * tr.CharWidth
		radioR := (tr.LineHeightPx - 4) / 2
		if radioR < 3 {
			radioR = 3
		}
		radioDiam := radioR * 2

		// Center the radio + label in the row
		radioTotalW := radioDiam + 4 + radioLabelW
		radioX := dx + (dw-radioTotalW)/2
		radioCY := iy + itemH/2
		radioCX := radioX + radioR

		toggleHover := st.hoverX >= dx && st.hoverX < dx+dw && st.hoverY >= iy && st.hoverY < iy+itemH
		if toggleHover {
			hlOff := op.Offset(image.Pt(dx, iy)).Push(gtx.Ops)
			hlRect := clip.Rect{Max: image.Pt(dw, itemH)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.DropdownSel}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			hlRect.Pop()
			hlOff.Pop()
		}

		outerOff := op.Offset(image.Pt(radioCX-radioR, radioCY-radioR)).Push(gtx.Ops)
		outerEll := clip.Ellipse{Max: image.Pt(radioDiam, radioDiam)}.Push(gtx.Ops)
		paint.ColorOp{Color: st.theme.Foreground}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		outerEll.Pop()
		outerOff.Pop()

		innerR := radioR - 2
		if innerR > 0 {
			innerOff := op.Offset(image.Pt(radioCX-innerR, radioCY-innerR)).Push(gtx.Ops)
			innerEll := clip.Ellipse{Max: image.Pt(innerR*2, innerR*2)}.Push(gtx.Ops)
			innerColor := st.theme.DropdownBg
			if st.saveMenu.saveAsExpanded {
				innerColor = st.theme.Cursor
			}
			paint.ColorOp{Color: innerColor}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			innerEll.Pop()
			innerOff.Pop()
		}

		tr.RenderGlyphs(gtx.Ops, gtx, radioLabel, radioX+radioDiam+4, textY, st.theme.TabDimFg)

		sepOff := op.Offset(image.Pt(dx+4, iy+itemH-1)).Push(gtx.Ops)
		sepRect := clip.Rect{Max: image.Pt(dw-8, 1)}.Push(gtx.Ops)
		paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		sepRect.Pop()
		sepOff.Pop()
		curY += itemH
	}

	// --- Overwrite confirmation rows ---
	if st.saveMenu.confirmOverwrite {
		// Warning text row
		{
			iy := curY
			textY := iy + (itemH-tr.LineHeightPx)/2
			warning := "\"" + string(st.saveMenu.filename) + "\" exists"
			maxChars := (dw - 16) / tr.CharWidth
			if utf8.RuneCountInString(warning) > maxChars && maxChars > 3 {
				runes := []rune(warning)
				warning = string(runes[:maxChars-1]) + "…"
			}
			tr.RenderGlyphs(gtx.Ops, gtx, warning, dx+8, textY, finderTagColors[1])

			sepOff := op.Offset(image.Pt(dx+4, iy+itemH-1)).Push(gtx.Ops)
			sepRect := clip.Rect{Max: image.Pt(dw-8, 1)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			sepRect.Pop()
			sepOff.Pop()
			curY += itemH
		}

		// Overwrite / Back split row
		{
			iy := curY
			textY := iy + (itemH-tr.LineHeightPx)/2
			halfW := dw / 2

			overwriteHover := st.hoverX >= dx && st.hoverX < dx+halfW && st.hoverY >= iy && st.hoverY < iy+itemH
			if overwriteHover {
				hlOff := op.Offset(image.Pt(dx, iy)).Push(gtx.Ops)
				hlRect := clip.Rect{Max: image.Pt(halfW, itemH)}.Push(gtx.Ops)
				paint.ColorOp{Color: st.theme.DropdownSel}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				hlRect.Pop()
				hlOff.Pop()
			}
			tr.RenderGlyphs(gtx.Ops, gtx, "Overwrite", dx+8, textY, finderTagColors[1])

			divOff := op.Offset(image.Pt(dx+halfW, iy+2)).Push(gtx.Ops)
			divRect := clip.Rect{Max: image.Pt(1, itemH-4)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			divRect.Pop()
			divOff.Pop()

			backHover := st.hoverX >= dx+halfW && st.hoverX < dx+dw && st.hoverY >= iy && st.hoverY < iy+itemH
			if backHover {
				hlOff := op.Offset(image.Pt(dx+halfW+1, iy)).Push(gtx.Ops)
				hlRect := clip.Rect{Max: image.Pt(halfW-1, itemH)}.Push(gtx.Ops)
				paint.ColorOp{Color: st.theme.DropdownSel}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				hlRect.Pop()
				hlOff.Pop()
			}
			tr.RenderGlyphs(gtx.Ops, gtx, "Back", dx+halfW+8, textY, st.theme.Foreground)

			sepOff := op.Offset(image.Pt(dx+4, iy+itemH-1)).Push(gtx.Ops)
			sepRect := clip.Rect{Max: image.Pt(dw-8, 1)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			sepRect.Pop()
			sepOff.Pop()
			curY += itemH
		}
	}

	// --- Bottom row: Save | Discard | Cancel (always visible, 3-way split) ---
	{
		iy := curY
		textY := iy + (itemH-tr.LineHeightPx)/2
		thirdW := dw / 3

		// Save button (left third)
		saveFg := st.theme.Foreground
		if !canSave {
			saveFg = st.theme.TabDimFg
		}
		saveHover := canSave && st.hoverX >= dx && st.hoverX < dx+thirdW && st.hoverY >= iy && st.hoverY < iy+itemH
		if saveHover {
			hlOff := op.Offset(image.Pt(dx, iy)).Push(gtx.Ops)
			hlRect := clip.Rect{Max: image.Pt(thirdW, itemH)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.DropdownSel}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			hlRect.Pop()
			hlOff.Pop()
		}
		tr.RenderGlyphs(gtx.Ops, gtx, "Save", dx+8, textY, saveFg)

		// Divider 1
		div1Off := op.Offset(image.Pt(dx+thirdW, iy+2)).Push(gtx.Ops)
		div1Rect := clip.Rect{Max: image.Pt(1, itemH-4)}.Push(gtx.Ops)
		paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		div1Rect.Pop()
		div1Off.Pop()

		// Discard button (middle third)
		discardX := dx + thirdW + 1
		discardW := thirdW - 1
		discardHover := st.hoverX >= discardX && st.hoverX < discardX+discardW && st.hoverY >= iy && st.hoverY < iy+itemH
		if discardHover {
			hlOff := op.Offset(image.Pt(discardX, iy)).Push(gtx.Ops)
			hlRect := clip.Rect{Max: image.Pt(discardW, itemH)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.DropdownSel}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			hlRect.Pop()
			hlOff.Pop()
		}
		tr.RenderGlyphs(gtx.Ops, gtx, "Discard", discardX+8, textY, st.theme.TabDimFg)

		// Divider 2
		div2X := dx + thirdW*2
		div2Off := op.Offset(image.Pt(div2X, iy+2)).Push(gtx.Ops)
		div2Rect := clip.Rect{Max: image.Pt(1, itemH-4)}.Push(gtx.Ops)
		paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		div2Rect.Pop()
		div2Off.Pop()

		// Cancel button (right third)
		cancelX := div2X + 1
		cancelW := dw - thirdW*2 - 1
		cancelHover := st.hoverX >= cancelX && st.hoverX < cancelX+cancelW && st.hoverY >= iy && st.hoverY < iy+itemH
		if cancelHover {
			hlOff := op.Offset(image.Pt(cancelX, iy)).Push(gtx.Ops)
			hlRect := clip.Rect{Max: image.Pt(cancelW, itemH)}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.DropdownSel}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			hlRect.Pop()
			hlOff.Pop()
		}
		tr.RenderGlyphs(gtx.Ops, gtx, "Cancel", cancelX+8, textY, st.theme.Foreground)
	}

	// Border around dropdown
	borderColor := st.theme.TabBorder
	bOff := op.Offset(image.Pt(dx, dy)).Push(gtx.Ops)
	bRect := clip.Rect{Max: image.Pt(dw, 1)}.Push(gtx.Ops)
	paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bRect.Pop()
	bOff.Pop()
	bOff = op.Offset(image.Pt(dx, dy+dropdownH-1)).Push(gtx.Ops)
	bRect = clip.Rect{Max: image.Pt(dw, 1)}.Push(gtx.Ops)
	paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bRect.Pop()
	bOff.Pop()
	bOff = op.Offset(image.Pt(dx, dy)).Push(gtx.Ops)
	bRect = clip.Rect{Max: image.Pt(1, dropdownH)}.Push(gtx.Ops)
	paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bRect.Pop()
	bOff.Pop()
	bOff = op.Offset(image.Pt(dx+dw-1, dy)).Push(gtx.Ops)
	bRect = clip.Rect{Max: image.Pt(1, dropdownH)}.Push(gtx.Ops)
	paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bRect.Pop()
	bOff.Pop()
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

	// Markdown mode toggle (Edit / Read) — left of language label
	if lang == "Markdown" && ts != nil {
		modeLabel := "Edit"
		if ts.mode == viewMarkdownRead {
			modeLabel = "Read"
		}
		modePad := sr.CharWidth
		modeW := len(modeLabel)*sr.CharWidth + modePad*2
		modeX := st.langLabelX - modeW - sr.CharWidth
		modeY := y

		// Subtle pill background
		pillColor := st.theme.TabBorder
		pillOff := op.Offset(image.Pt(modeX, modeY+1)).Push(gtx.Ops)
		pillRect := clip.UniformRRect(image.Rectangle{
			Max: image.Pt(modeW, statusH-2),
		}, 3).Push(gtx.Ops)
		paint.ColorOp{Color: pillColor}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		pillRect.Pop()
		pillOff.Pop()

		// Label
		sr.RenderGlyphs(gtx.Ops, gtx, modeLabel, modeX+modePad, textY, st.theme.Foreground)

		// Store hit area for click detection
		st.mdToggleX = modeX
		st.mdToggleW = modeW
	} else {
		st.mdToggleX = 0
		st.mdToggleW = 0
	}

	// Centered notification (e.g. "Saved to: ~/path")
	if st.notification != "" && time.Now().Before(st.notificationUntil) {
		notifW := utf8.RuneCountInString(st.notification) * sr.CharWidth
		notifX := (gtx.Constraints.Max.X - notifW) / 2
		sr.RenderGlyphs(gtx.Ops, gtx, st.notification, notifX, textY, st.theme.Foreground)
		// Schedule a repaint so the notification disappears on time
		gtx.Execute(op.InvalidateCmd{})
	} else if st.notification != "" {
		st.notification = ""
	}
}

// themeToggleSize returns the icon radius and hit-area width for the theme toggle.
func (st *appState) themeToggleSize() (radius, hitW int) {
	radius = st.tabBarHeight / 5
	if radius < 5 {
		radius = 5
	}
	hitW = st.tabBarHeight // square hit area
	return
}

// themeToggleX returns the left edge X of the theme toggle hit area.
func (st *appState) themeToggleX(maxX int) int {
	_, hitW := st.themeToggleSize()
	return maxX - hitW
}

// drawThemeToggle draws a subtle sun or moon icon in the upper-right corner of the tab bar.
func (st *appState) drawThemeToggle(gtx layout.Context, inTabBar bool) {
	r, hitW := st.themeToggleSize()
	toggleX := st.themeToggleX(gtx.Constraints.Max.X)
	cx := toggleX + hitW/2
	cy := st.tabBarHeight / 2

	// Hover detection
	hovered := inTabBar && st.hoverX >= toggleX && st.hoverX < toggleX+hitW

	// Use a subtle, dim color; brighten on hover
	fg := st.theme.TabDimFg
	if hovered {
		fg = st.theme.Foreground
	}

	if st.darkMode {
		// Draw sun icon: circle + rays
		st.drawSunIcon(gtx, cx, cy, r, fg)
	} else {
		// Draw moon icon: crescent
		st.drawMoonIcon(gtx, cx, cy, r, fg)
	}
}

// drawSunIcon draws a simple sun: a filled circle with small ray lines around it.
func (st *appState) drawSunIcon(gtx layout.Context, cx, cy, r int, fg color.NRGBA) {
	// Center circle (60% of radius)
	cr := r * 6 / 10
	if cr < 3 {
		cr = 3
	}
	sunCircle := clip.Ellipse{
		Min: image.Pt(cx-cr, cy-cr),
		Max: image.Pt(cx+cr, cy+cr),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	sunCircle.Pop()

	// Rays: 8 small rectangles around the circle
	rayLen := r * 4 / 10
	if rayLen < 2 {
		rayLen = 2
	}
	rayW := 1
	if r > 8 {
		rayW = 2
	}
	innerR := cr + 2
	for i := 0; i < 8; i++ {
		angle := float64(i) * math.Pi / 4
		cos := math.Cos(angle)
		sin := math.Sin(angle)
		x1 := cx + int(float64(innerR)*cos)
		y1 := cy + int(float64(innerR)*sin)
		x2 := cx + int(float64(innerR+rayLen)*cos)
		y2 := cy + int(float64(innerR+rayLen)*sin)

		// Draw ray as a small filled rect rotated to the angle
		// Use a simple 1-2px rect between the two points
		minX := x1
		maxX := x2
		if minX > maxX {
			minX, maxX = maxX, minX
		}
		minY := y1
		maxY := y2
		if minY > maxY {
			minY, maxY = maxY, minY
		}
		// Ensure minimum size
		if maxX-minX < rayW {
			maxX = minX + rayW
		}
		if maxY-minY < rayW {
			maxY = minY + rayW
		}
		rayRect := clip.Rect{
			Min: image.Pt(minX, minY),
			Max: image.Pt(maxX, maxY),
		}.Push(gtx.Ops)
		paint.ColorOp{Color: fg}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		rayRect.Pop()
	}
}

// drawMoonIcon draws a crescent moon using two overlapping circles.
func (st *appState) drawMoonIcon(gtx layout.Context, cx, cy, r int, fg color.NRGBA) {
	// We approximate a crescent by drawing the main moon circle,
	// then "erasing" with a background-colored circle offset to the upper-right.

	// Moon circle
	moonCircle := clip.Ellipse{
		Min: image.Pt(cx-r, cy-r),
		Max: image.Pt(cx+r, cy+r),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	moonCircle.Pop()

	// Cutout circle (offset to upper-right, slightly smaller)
	cutR := r * 8 / 10
	offX := r * 5 / 10
	offY := -r * 3 / 10
	cutCircle := clip.Ellipse{
		Min: image.Pt(cx+offX-cutR, cy+offY-cutR),
		Max: image.Pt(cx+offX+cutR, cy+offY+cutR),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.TabBarBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	cutCircle.Pop()
}
