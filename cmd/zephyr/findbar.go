package main

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

const (
	findBarPadding  = 6
	findBarInputH   = 26
	findBarRowGap   = 4
	findBarBtnW     = 22
	findBarMinWidth = 320
)

// findBarGeom holds the computed pixel positions for the find bar layout.
// Shared between drawFindBar and handleFindBarClick to avoid duplication.
type findBarGeom struct {
	barX, barY, barW, barH int
	chevronW               int
	inputX, inputW         int
	arrowColW              int
	matchCountW            int
	closeW                 int
	rowY                   int // find row Y (relative to bar)
	rowY2                  int // replace row Y (relative to bar)
	closeBtnX              int
}

func computeFindBarGeom(maxX int, showReplace bool) findBarGeom {
	barW := 300
	if barW > maxX-20 {
		barW = maxX - 20
	}
	if barW < findBarMinWidth {
		barW = findBarMinWidth
	}
	rows := 1
	if showReplace {
		rows = 2
	}
	barH := findBarPadding*2 + rows*findBarInputH + (rows-1)*findBarRowGap
	barX := maxX - barW - 14
	barY := 4

	chevronW := findBarBtnW
	inputX := chevronW + 2
	arrowColW := findBarBtnW + 6
	matchCountW := 60
	closeW := findBarBtnW
	rightW := arrowColW + matchCountW + closeW + findBarPadding
	inputW := barW - inputX - rightW

	return findBarGeom{
		barX:        barX,
		barY:        barY,
		barW:        barW,
		barH:        barH,
		chevronW:    chevronW,
		inputX:      inputX,
		inputW:      inputW,
		arrowColW:   arrowColW,
		matchCountW: matchCountW,
		closeW:      closeW,
		rowY:        findBarPadding,
		rowY2:       findBarPadding + findBarInputH + findBarRowGap,
		closeBtnX:   barW - closeW - findBarPadding,
	}
}

func (st *appState) drawFindBar(gtx layout.Context, editorH int) {
	tr := st.tabRend
	if tr == nil || tr.CharWidth == 0 {
		return
	}
	g := computeFindBarGeom(gtx.Constraints.Max.X, st.findBar.ShowReplace)

	off := op.Offset(image.Pt(g.barX, g.barY)).Push(gtx.Ops)

	// Drop shadow
	shadowColor := color.NRGBA{R: 0, G: 0, B: 0, A: 60}
	for i := 1; i <= 3; i++ {
		sRect := clip.Rect{
			Min: image.Pt(-i, g.barH),
			Max: image.Pt(g.barW+i, g.barH+i),
		}.Push(gtx.Ops)
		paint.ColorOp{Color: shadowColor}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		sRect.Pop()
	}

	// Background
	bgRect := clip.Rect{Max: image.Pt(g.barW, g.barH)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.FindBarBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgRect.Pop()

	// Bottom border
	bRect := clip.Rect{
		Min: image.Pt(0, g.barH-1),
		Max: image.Pt(g.barW, g.barH),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.FindBarBorder}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bRect.Pop()

	inputBg := st.theme.FindBarInputBg
	focusBorder := st.theme.FindBarFocus
	textColor := st.theme.FindBarText
	dimColor := st.theme.FindBarDim

	// Hover detection: translate window coords to find bar-relative
	hRelX := st.hoverX - g.barX
	hRelY := st.hoverY - st.tabBarHeight - g.barY
	hoverBlue := color.NRGBA{R: 100, G: 160, B: 255, A: 255}
	hoverRed := color.NRGBA{R: 240, G: 80, B: 80, A: 255}

	// --- Find row ---

	// Chevron toggle button
	chevronText := ">"
	if st.findBar.ShowReplace {
		chevronText = "v"
	}
	chevronHovered := hRelX >= 0 && hRelX < g.chevronW && hRelY >= g.rowY && hRelY < g.rowY+findBarInputH
	chevronColor := dimColor
	if chevronHovered {
		chevronColor = hoverBlue
	}
	tr.RenderGlyphs(gtx.Ops, gtx, chevronText, (g.chevronW-tr.CharWidth)/2, g.rowY+(findBarInputH-tr.LineHeightPx)/2, chevronColor)

	// Find input field background
	iRect := clip.Rect{
		Min: image.Pt(g.inputX, g.rowY),
		Max: image.Pt(g.inputX+g.inputW, g.rowY+findBarInputH),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: inputBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	iRect.Pop()

	// Focus border on active field
	if st.findBar.FocusField == 0 {
		st.drawInputBorder(gtx, g.inputX, g.rowY, g.inputW, findBarInputH, focusBorder)
	}

	// Find query text or placeholder
	textY := g.rowY + (findBarInputH-tr.LineHeightPx)/2
	if st.findBar.Query != "" {
		tr.RenderGlyphs(gtx.Ops, gtx, st.findBar.Query, g.inputX+4, textY, textColor)
	} else {
		tr.RenderGlyphs(gtx.Ops, gtx, "Find", g.inputX+4, textY, dimColor)
	}

	// Text cursor in find field
	if st.findBar.FocusField == 0 {
		cx := g.inputX + 4 + st.findBar.CursorPos*tr.CharWidth
		cursorH := tr.LineHeightPx
		cRect := clip.Rect{
			Min: image.Pt(cx, textY),
			Max: image.Pt(cx+2, textY+cursorH),
		}.Push(gtx.Ops)
		paint.ColorOp{Color: st.theme.Cursor}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		cRect.Pop()
	}

	// Stacked up/down chevrons (right of input)
	arrowX := g.inputX + g.inputW + 2
	arrowCenterX := arrowX + (g.arrowColW-tr.CharWidth)/2
	midY := g.rowY + findBarInputH/2

	// Up chevron
	upHovered := hRelX >= arrowX && hRelX < arrowX+g.arrowColW && hRelY >= g.rowY && hRelY < midY
	upColor := dimColor
	if upHovered {
		upColor = hoverBlue
	}
	glyphY := g.rowY + (findBarInputH-tr.LineHeightPx)/2
	upClip := clip.Rect{
		Min: image.Pt(arrowX, g.rowY),
		Max: image.Pt(arrowX+g.arrowColW, midY),
	}.Push(gtx.Ops)
	tr.RenderGlyphs(gtx.Ops, gtx, "^", arrowCenterX, glyphY-3, upColor)
	upClip.Pop()

	// Down chevron — ^ flipped vertically
	dnHovered := hRelX >= arrowX && hRelX < arrowX+g.arrowColW && hRelY >= midY && hRelY < g.rowY+findBarInputH
	dnColor := dimColor
	if dnHovered {
		dnColor = hoverBlue
	}
	dnClip := clip.Rect{
		Min: image.Pt(arrowX, midY),
		Max: image.Pt(arrowX+g.arrowColW, g.rowY+findBarInputH),
	}.Push(gtx.Ops)
	flipAff := f32.Affine2D{}.Scale(f32.Pt(0, float32(midY)), f32.Pt(1, -1))
	flipOp := op.Affine(flipAff).Push(gtx.Ops)
	tr.RenderGlyphs(gtx.Ops, gtx, "^", arrowCenterX, glyphY-3, dnColor)
	flipOp.Pop()
	dnClip.Pop()

	// Match count (right of arrows)
	matchText := ""
	if st.findBar.Query != "" {
		if st.findBar.MatchCount == 0 {
			matchText = "0 results"
		} else {
			matchText = fmt.Sprintf("%d of %d", st.findBar.CurrentMatch, st.findBar.MatchCount)
		}
	}
	matchCountX := arrowX + g.arrowColW + 2
	tr.RenderGlyphs(gtx.Ops, gtx, matchText, matchCountX, textY, dimColor)

	// Close button (rightmost)
	closeHovered := hRelX >= g.closeBtnX && hRelX < g.closeBtnX+g.closeW && hRelY >= g.rowY && hRelY < g.rowY+findBarInputH
	closeColor := dimColor
	if closeHovered {
		closeColor = hoverRed
	}
	tr.RenderGlyphs(gtx.Ops, gtx, "x", g.closeBtnX+(g.closeW-tr.CharWidth)/2, textY, closeColor)

	// --- Replace row ---
	if st.findBar.ShowReplace {
		textY2 := g.rowY2 + (findBarInputH-tr.LineHeightPx)/2

		// Input background
		riRect := clip.Rect{
			Min: image.Pt(g.inputX, g.rowY2),
			Max: image.Pt(g.inputX+g.inputW, g.rowY2+findBarInputH),
		}.Push(gtx.Ops)
		paint.ColorOp{Color: inputBg}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		riRect.Pop()

		// Focus border on replace field
		if st.findBar.FocusField == 1 {
			st.drawInputBorder(gtx, g.inputX, g.rowY2, g.inputW, findBarInputH, focusBorder)
		}

		// Replace field text or placeholder
		if st.findBar.Replacement != "" {
			tr.RenderGlyphs(gtx.Ops, gtx, st.findBar.Replacement, g.inputX+4, textY2, textColor)
		} else {
			tr.RenderGlyphs(gtx.Ops, gtx, "Replace", g.inputX+4, textY2, dimColor)
		}

		// Text cursor in replace field
		if st.findBar.FocusField == 1 {
			cx := g.inputX + 4 + st.findBar.CursorPos*tr.CharWidth
			cRect := clip.Rect{
				Min: image.Pt(cx, textY2),
				Max: image.Pt(cx+2, textY2+tr.LineHeightPx),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.Cursor}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			cRect.Pop()
		}

		// ">" and "All" buttons
		rAreaX := g.inputX + g.inputW + 2
		rBtnW := g.arrowColW
		rBtnHovered := hRelX >= rAreaX && hRelX < rAreaX+rBtnW && hRelY >= g.rowY2 && hRelY < g.rowY2+findBarInputH
		rBtnColor := dimColor
		if rBtnHovered {
			rBtnColor = hoverBlue
		}
		tr.RenderGlyphs(gtx.Ops, gtx, ">", rAreaX+(g.arrowColW-tr.CharWidth)/2, textY2, rBtnColor)

		// "All" button
		allBtnX := rAreaX + g.arrowColW + 2
		allBtnW := tr.CharWidth*3 + 4
		allHovered := hRelX >= allBtnX && hRelX < allBtnX+allBtnW && hRelY >= g.rowY2 && hRelY < g.rowY2+findBarInputH
		allColor := dimColor
		if allHovered {
			allColor = hoverRed
		}
		tr.RenderGlyphs(gtx.Ops, gtx, "All", allBtnX+2, textY2, allColor)
	}

	off.Pop()
}

func (st *appState) drawInputBorder(gtx layout.Context, x, y, w, h int, c color.NRGBA) {
	// Top
	r := clip.Rect{Min: image.Pt(x, y), Max: image.Pt(x+w, y+1)}.Push(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	r.Pop()
	// Bottom
	r = clip.Rect{Min: image.Pt(x, y+h-1), Max: image.Pt(x+w, y+h)}.Push(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	r.Pop()
	// Left
	r = clip.Rect{Min: image.Pt(x, y), Max: image.Pt(x+1, y+h)}.Push(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	r.Pop()
	// Right
	r = clip.Rect{Min: image.Pt(x+w-1, y), Max: image.Pt(x+w, y+h)}.Push(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	r.Pop()
}

func (st *appState) drawMatchIndicator(gtx layout.Context, editorH, totalLines int) {
	stripW := 8
	stripX := gtx.Constraints.Max.X - stripW

	// Semi-transparent track
	trackRect := clip.Rect{
		Min: image.Pt(stripX, 0),
		Max: image.Pt(stripX+stripW, editorH),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{R: 30, G: 30, B: 30, A: 100}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	trackRect.Pop()

	// Tick marks for matches
	tickColor := color.NRGBA{R: 230, G: 200, B: 50, A: 200}
	tickH := editorH / totalLines
	if tickH < 2 {
		tickH = 2
	}
	for i, match := range st.findBar.Matches {
		tickY := match.Line * editorH / totalLines
		tc := tickColor
		if i == st.findBar.CurrentMatch-1 {
			tc = color.NRGBA{R: 255, G: 230, B: 80, A: 255}
		}
		tRect := clip.Rect{
			Min: image.Pt(stripX+1, tickY),
			Max: image.Pt(stripX+stripW-1, tickY+tickH),
		}.Push(gtx.Ops)
		paint.ColorOp{Color: tc}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		tRect.Pop()
	}
}

func (st *appState) handleFindBarClick(px, py int) bool {
	// Translate to editor-relative coords (below tab bar)
	py -= st.tabBarHeight
	g := computeFindBarGeom(st.lastMaxX, st.findBar.ShowReplace)

	// Check if click is inside the find bar
	if px < g.barX || px > g.barX+g.barW || py < g.barY || py > g.barY+g.barH {
		return false
	}

	relX := px - g.barX
	relY := py - g.barY

	tr := st.tabRend
	if tr == nil {
		return true
	}

	// Chevron button
	if relX < g.chevronW && relY >= g.rowY && relY < g.rowY+findBarInputH {
		st.findBar.ToggleReplace()
		return true
	}

	// Find input field click
	if relX >= g.inputX && relX < g.inputX+g.inputW && relY >= g.rowY && relY < g.rowY+findBarInputH {
		st.findBar.FocusField = 0
		clickCol := (relX - g.inputX - 4) / tr.CharWidth
		queryLen := len([]rune(st.findBar.Query))
		if clickCol < 0 {
			clickCol = 0
		}
		if clickCol > queryLen {
			clickCol = queryLen
		}
		st.findBar.CursorPos = clickCol
		return true
	}

	// Stacked arrows column (right of input)
	arrowX := g.inputX + g.inputW + 2
	midY := g.rowY + findBarInputH/2
	if relX >= arrowX && relX < arrowX+g.arrowColW {
		if relY >= g.rowY && relY < midY {
			st.findPrevMatch()
			return true
		}
		if relY >= midY && relY < g.rowY+findBarInputH {
			st.findNextMatch()
			return true
		}
	}

	// Close button (rightmost)
	if relX >= g.closeBtnX && relX < g.closeBtnX+g.closeW && relY >= g.rowY && relY < g.rowY+findBarInputH {
		st.findBar.Close()
		return true
	}

	// Replace row clicks
	if st.findBar.ShowReplace {
		// Replace input field
		if relX >= g.inputX && relX < g.inputX+g.inputW && relY >= g.rowY2 && relY < g.rowY2+findBarInputH {
			st.findBar.FocusField = 1
			clickCol := (relX - g.inputX - 4) / tr.CharWidth
			replLen := len([]rune(st.findBar.Replacement))
			if clickCol < 0 {
				clickCol = 0
			}
			if clickCol > replLen {
				clickCol = replLen
			}
			st.findBar.CursorPos = clickCol
			return true
		}

		// ">" replace one button
		rAreaX := g.inputX + g.inputW + 2
		if relY >= g.rowY2 && relY < g.rowY2+findBarInputH {
			if relX >= rAreaX && relX < rAreaX+g.arrowColW {
				st.replaceCurrentMatch()
				return true
			}
			// "All" replace all button
			allBtnX := rAreaX + g.arrowColW + 2
			allBtnW := tr.CharWidth*3 + 4
			if relX >= allBtnX && relX < allBtnX+allBtnW {
				st.replaceAllMatches()
				return true
			}
		}
	}
	return true
}
