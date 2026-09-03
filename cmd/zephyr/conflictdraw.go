package main

import (
	"image"
	"image/color"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/kristianweb/zephyr/internal/ui"
)

// clobberCellCount is the number of choices on the clobber prompt's action
// row. The click router and the draw split the row on it, so they cannot
// disagree about where a cell begins.
const clobberCellCount = 4

// drawClobberMenu renders the save menu's clobber sub-state: a line naming
// what happened to the file, then the four ways out of it.
func (st *appState) drawClobberMenu(gtx layout.Context, tab *ui.Tab, dx, dy, dw, dropdownH, itemH int) {
	tr := st.tabRend
	if tr == nil {
		return
	}
	drawMenuPanel(gtx, color.NRGBA{A: 60}, dx+2, dy+2, dw, dropdownH)
	drawMenuPanel(gtx, st.theme.DropdownBg, dx, dy, dw, dropdownH)

	textY := dy + (itemH-tr.LineHeightPx)/2
	warning := st.clobberWarning(tab) + ": " + tab.Title
	maxChars := (dw - 16) / tr.CharWidth
	if utf8.RuneCountInString(warning) > maxChars && maxChars > 3 {
		runes := []rune(warning)
		warning = string(runes[:maxChars-1]) + "…"
	}
	tr.RenderGlyphs(gtx.Ops, gtx, warning, dx+8, textY, warningColor())
	drawMenuSeparator(gtx, st.theme.TabBorder, dx, dy+itemH-1, dw)

	rowY := dy + itemH
	cell := dw / clobberCellCount
	labels := [clobberCellCount]string{"Overwrite", "Reload", "Compare", "Cancel"}
	colors := [clobberCellCount]color.NRGBA{warningColor(), st.theme.Foreground, st.theme.Foreground, st.theme.Foreground}
	for i := 0; i < clobberCellCount; i++ {
		cellX := dx + cell*i
		cellW := cell
		if i == clobberCellCount-1 {
			cellW = dw - cell*(clobberCellCount-1)
		}
		if st.hoverX >= cellX && st.hoverX < cellX+cellW && st.hoverY >= rowY && st.hoverY < rowY+itemH {
			drawMenuPanel(gtx, st.theme.DropdownSel, cellX, rowY, cellW, itemH)
		}
		labelX := cellX + (cellW-utf8.RuneCountInString(labels[i])*tr.CharWidth)/2
		tr.RenderGlyphs(gtx.Ops, gtx, labels[i], labelX, rowY+(itemH-tr.LineHeightPx)/2, colors[i])
		if i > 0 {
			drawMenuPanel(gtx, st.theme.TabBorder, cellX, rowY+2, 1, itemH-4)
		}
	}
}

// drawMenuPanel fills a rectangle in the given color.
func drawMenuPanel(gtx layout.Context, c color.NRGBA, x, y, w, h int) {
	off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	rect := clip.Rect{Max: image.Pt(w, h)}.Push(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	rect.Pop()
	off.Pop()
}

// drawMenuSeparator draws the hairline the dropdown rows are divided by.
func drawMenuSeparator(gtx layout.Context, c color.NRGBA, dx, y, dw int) {
	drawMenuPanel(gtx, c, dx+4, y, dw-8, 1)
}

// tabDotColor returns the color of a tab's modified dot. An unresolved
// external-change conflict recolors it to the warning color, so the tab bar
// carries the badge for as long as the conflict stands.
func (st *appState) tabDotColor(tab *ui.Tab, hovered bool) color.NRGBA {
	if st.tabConflict(tab) != conflictNone {
		return warningColor()
	}
	if hovered {
		return st.theme.TabCloseHover
	}
	return st.theme.TabModifiedDot
}
