package main

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/kristianweb/zephyr/internal/config"
	"github.com/kristianweb/zephyr/internal/render"
)

// codeCopyBtn tracks a copy button's hit area for a code block.
type codeCopyBtn struct {
	x, y, w, h int
	code        string
}

// mdCheckbox tracks a checkbox hit area for task list toggling.
type mdCheckbox struct {
	x, y, w, h  int
	sourceOffset int  // byte offset of the [ ] or [x] in the source
	checked      bool
}

// Markdown preview renderers, cached on appState and rebuilt when theme changes.
type mdRenderers struct {
	h1       *render.TextRenderer
	h2       *render.TextRenderer
	h3       *render.TextRenderer
	h4       *render.TextRenderer
	h5       *render.TextRenderer
	h6       *render.TextRenderer
	body     *render.TextRenderer
	bodySmall *render.TextRenderer // for nested list items
	code     *render.TextRenderer
	bold     *render.TextRenderer
	ital     *render.TextRenderer
	boldItal *render.TextRenderer
}

// toggleCheckbox toggles a task list checkbox in the source buffer and re-parses.
func (st *appState) toggleCheckbox(cb mdCheckbox) {
	ed := st.activeEd()
	ts := st.activeTabState()
	if ed == nil || ts == nil {
		return
	}
	src := ed.Buffer.TextBytes(nil)

	// Find the [ ] or [x] near the source offset
	searchStart := cb.sourceOffset
	if searchStart >= len(src) {
		return
	}
	idx := -1
	limit := searchStart + 40
	if limit > len(src)-2 {
		limit = len(src) - 2
	}
	for i := searchStart; i < limit; i++ {
		if src[i] == '[' && (src[i+1] == ' ' || src[i+1] == 'x' || src[i+1] == 'X') && src[i+2] == ']' {
			idx = i + 1 // position of the char inside brackets
			break
		}
	}
	if idx < 0 {
		return
	}

	// Convert byte offset to line:col
	line, col := 0, 0
	for i := 0; i < idx; i++ {
		if src[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}

	// Save and restore cursor position
	savedLine, savedCol := ed.Cursor.Line, ed.Cursor.Col

	ed.Cursor.Line = line
	ed.Cursor.Col = col
	ed.Selection.Clear()
	ed.DeleteForward()
	if cb.checked {
		ed.InsertText(" ")
	} else {
		ed.InsertText("x")
	}

	// Restore cursor
	ed.Cursor.Line = savedLine
	ed.Cursor.Col = savedCol

	// Re-parse and refresh
	ts.mdDoc = render.ParseMarkdown(ed.Buffer.TextBytes(nil))
	st.window.Invalidate()
}

// toggleMarkdownPreview switches between edit and read mode for markdown files.
func (st *appState) toggleMarkdownPreview() {
	ts := st.activeTabState()
	ed := st.activeEd()
	if ts == nil || ed == nil {
		return
	}
	if ts.langLabel != "Markdown" {
		return
	}
	if ts.mode == viewEdit {
		ts.mode = viewMarkdownRead
		ts.mdDoc = render.ParseMarkdown(ed.Buffer.TextBytes(nil))
		ts.mdScrollY = 0
	} else {
		ts.mode = viewEdit
		// Force viewport to re-sync with cursor on next frame
		ts.lastCursorLine = -1
		ts.lastCursorCol = -1
	}
}

// ensureMdRenderers lazily creates markdown preview renderers.
func (st *appState) ensureMdRenderers(gtx layout.Context) *mdRenderers {
	if st.mdRend != nil {
		return st.mdRend
	}
	if st.shaper == nil {
		return nil // shaper not yet initialized (theme change mid-frame)
	}
	heading := st.fontCfg.Heading
	body := st.fontCfg.Body
	mono := st.fontCfg.Monospace
	fg := st.theme.Foreground

	mk := func(size float32, lh float32, face string, weight font.Weight, style font.Style) *render.TextRenderer {
		r := render.NewTextRenderer(st.shaper, render.TextStyle{
			FontSize:   unit.Sp(size),
			LineHeight: lh,
			Foreground: fg,
			Typeface:   face,
			Weight:     weight,
			FontStyle:  style,
		})
		r.ComputeMetrics(gtx)
		return r
	}

	st.mdRend = &mdRenderers{
		h1:        mk(28, 1.1, heading, font.Bold, font.Regular),
		h2:        mk(24, 1.1, heading, font.Bold, font.Regular),
		h3:        mk(20, 1.1, heading, font.Bold, font.Regular),
		h4:        mk(17, 1.1, heading, font.Bold, font.Regular),
		h5:        mk(15, 1.1, heading, font.Normal, font.Regular),
		h6:        mk(14, 1.1, heading, font.Normal, font.Regular),
		body:      mk(14, 1.2, body, font.Normal, font.Regular),
		bodySmall: mk(12, 1.2, body, font.Normal, font.Regular),
		code:      mk(13, 1.3, mono, font.Normal, font.Regular),
		bold:      mk(14, 1.2, body, font.Bold, font.Regular),
		ital:      mk(14, 1.2, body, font.Normal, font.Italic),
		boldItal:  mk(14, 1.2, body, font.Bold, font.Italic),
	}
	return st.mdRend
}

// headingRenderer returns the renderer for a heading level.
func (mr *mdRenderers) heading(level int) *render.TextRenderer {
	switch level {
	case 1:
		return mr.h1
	case 2:
		return mr.h2
	case 3:
		return mr.h3
	case 4:
		return mr.h4
	case 5:
		return mr.h5
	default:
		return mr.h6
	}
}

// drawMarkdownPreview renders the parsed markdown in read mode.
// Called inside the editor clip area (already offset by tabBarHeight).
func (st *appState) drawMarkdownPreview(gtx layout.Context, ts *tabState) {
	if ts.mdDoc == nil || st.textRend == nil {
		return
	}
	mr := st.ensureMdRenderers(gtx)
	if mr == nil {
		return
	}
	theme := st.theme
	ts.mdCopyBtns = ts.mdCopyBtns[:0]     // reset copy button hit areas
	ts.mdCheckboxes = ts.mdCheckboxes[:0] // reset checkbox hit areas

	charW := st.textRend.CharWidth
	if charW == 0 {
		charW = 8
	}
	editorX := charW * 2 // left margin
	maxW := gtx.Constraints.Max.X - editorX - charW*2
	if maxW < charW {
		maxW = charW
	}
	editorH := gtx.Constraints.Max.Y // already clipped to editor area

	scrollY := int(ts.mdScrollY)
	y := editorTopPad - scrollY

	prevKind := render.BlockKind(-1)
	for _, block := range ts.mdDoc.Blocks {
		var blockH int

		// Top spacing based on blank lines in source and block type
		bodyH := mr.body.LineHeightPx
		blanks := block.BlankLinesBefore
		switch {
		case prevKind < 0:
			// first block
			if block.Kind == render.BlockHeading {
				y += bodyH / 2
			}
		case block.Kind == render.BlockHeading:
			y += bodyH * 3 / 4
		case block.Kind == render.BlockListItem && prevKind == render.BlockListItem && blanks == 0:
			// tight list items: no extra gap
		case blanks >= 2:
			// 2+ blank lines: scale proportionally (1 blank=1x, 2 blanks=1.5x, etc.)
			y += bodyH/2 + (blanks-1)*bodyH/2
		case blanks == 1:
			y += bodyH / 2
		default:
			y += bodyH / 4
		}

		switch block.Kind {
		case render.BlockHeading:
			hr := mr.heading(block.Level)
			text := spansToPlain(block.Spans)
			lines := splitAndWrap(text, maxW, hr.CharWidth)
			blockH = len(lines) * hr.LineHeightPx

			if y+blockH > 0 && y < editorH {
				for i, line := range lines {
					hr.RenderGlyphs(gtx.Ops, gtx, line, editorX, y+i*hr.LineHeightPx, theme.MdHeading)
				}
			}

		case render.BlockParagraph:
			lines := splitAndWrap(spansToPlain(block.Spans), maxW, mr.body.CharWidth)
			blockH = len(lines) * mr.body.LineHeightPx

			if y+blockH > 0 && y < editorH {
				st.renderStyledLines(gtx, mr, block.Spans, lines, editorX, y, theme)
			}

		case render.BlockCodeBlock:
			padding := 10
			codeCharW := mr.code.CharWidth
			if codeCharW == 0 {
				codeCharW = 8
			}

			// Compute code block width: fit content, clamp to [minW, maxW]
			minCodeW := maxW / 3
			maxCodeW := maxW
			codeContentW := padding * 2
			rawLines := strings.Split(block.CodeText, "\n")
			for _, l := range rawLines {
				lw := len(l)*codeCharW + padding*2
				if lw > codeContentW {
					codeContentW = lw
				}
			}
			codeBoxW := codeContentW
			if codeBoxW < minCodeW {
				codeBoxW = minCodeW
			}
			if codeBoxW > maxCodeW {
				codeBoxW = maxCodeW
			}

			// Wrap lines that exceed the box
			wrapCols := (codeBoxW - padding*2) / codeCharW
			if wrapCols < 10 {
				wrapCols = 10
			}
			var codeLines []string
			for _, l := range rawLines {
				if len(l) <= wrapCols {
					codeLines = append(codeLines, l)
				} else {
					for len(l) > wrapCols {
						codeLines = append(codeLines, l[:wrapCols])
						l = l[wrapCols:]
					}
					if len(l) > 0 {
						codeLines = append(codeLines, l)
					}
				}
			}

			blockH = len(codeLines)*mr.code.LineHeightPx + padding*2

			if y+blockH > 0 && y < editorH {
				// Background
				bgColor := shiftColor(theme.Background, 15)
				bgOff := op.Offset(image.Pt(editorX, y)).Push(gtx.Ops)
				bgRect := clip.UniformRRect(image.Rectangle{
					Max: image.Pt(codeBoxW, blockH),
				}, 4).Push(gtx.Ops)
				paint.ColorOp{Color: bgColor}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				bgRect.Pop()
				bgOff.Pop()

				// Code text (use syntax token colors for visual distinction)
				for i, line := range codeLines {
					mr.code.RenderGlyphs(gtx.Ops, gtx, line, editorX+padding, y+padding+i*mr.code.LineHeightPx, theme.Foreground)
				}

				// Language label (top-left, subtle)
				if block.CodeLang != "" {
					mr.code.RenderGlyphs(gtx.Ops, gtx, block.CodeLang, editorX+padding, y+2, theme.TabDimFg)
				}

				// Copy icon (upper-right corner of code block)
				s := mr.code.LineHeightPx * 2 / 5 // small icon
				if s < 6 {
					s = 6
				}
				copyW := s*2 + 4
				copyH := s*2 + 4
				copyX := editorX + codeBoxW - copyW - 6
				copyY := y + 6

				copyHovered := st.hoverX >= copyX && st.hoverX < copyX+copyW &&
					st.hoverY-st.tabBarHeight >= copyY && st.hoverY-st.tabBarHeight < copyY+copyH

				iconColor := theme.TabDimFg
				if copyHovered {
					iconColor = theme.MdAccent
				}

				// Draw clipboard icon: two overlapping small rects
				ix := copyX + (copyW-s)/2
				iy := copyY + (copyH-s)/2
				iconOff := s / 3

				// Back rect (offset up-left)
				drawRoundedBorder(gtx.Ops, ix-iconOff, iy-iconOff, s, s, 1, iconColor)
				// Front rect (filled with code bg to occlude back)
				fillOff := op.Offset(image.Pt(ix, iy)).Push(gtx.Ops)
				fillRect := clip.UniformRRect(image.Rectangle{Max: image.Pt(s, s)}, 1).Push(gtx.Ops)
				paint.ColorOp{Color: bgColor}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				fillRect.Pop()
				fillOff.Pop()
				// Front rect border
				drawRoundedBorder(gtx.Ops, ix, iy, s, s, 1, iconColor)

				// Register hit area
				ts.mdCopyBtns = append(ts.mdCopyBtns, codeCopyBtn{
					x: copyX, y: copyY + st.tabBarHeight, w: copyW, h: copyH,
					code: block.CodeText,
				})
			}

		case render.BlockBlockquote:
			// Render children as paragraph text with a left bar
			childText := blocksToPlain(block.Children)
			lines := splitAndWrap(childText, maxW-24, mr.body.CharWidth)
			blockH = len(lines) * mr.body.LineHeightPx

			if y+blockH > 0 && y < editorH {
				// Left bar
				barRect := clip.Rect{
					Min: image.Pt(editorX, y),
					Max: image.Pt(editorX+3, y+blockH),
				}.Push(gtx.Ops)
				paint.ColorOp{Color: theme.MdHeading}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				barRect.Pop()

				for i, line := range lines {
					mr.body.RenderGlyphs(gtx.Ops, gtx, line, editorX+20, y+i*mr.body.LineHeightPx, theme.MdHeading)
				}
			}

		case render.BlockListItem:
			indent := 0
			if block.Level >= 0 {
				indent = (block.Level + 1) * 20
			}
			listRend := mr.body
			if block.Level > 0 {
				listRend = mr.bodySmall
			}

			// Check for task checkbox
			hasCheckbox := false
			checkboxChecked := false
			textSpans := block.Spans
			for i, s := range block.Spans {
				if s.Checkbox > 0 {
					hasCheckbox = true
					checkboxChecked = s.Checkbox == 2
					textSpans = block.Spans[i+1:] // text after checkbox
					break
				}
			}

			text := spansToPlain(textSpans)
			lines := splitAndWrap(text, maxW-indent-24, listRend.CharWidth)
			if len(lines) == 0 {
				lines = []string{""}
			}
			blockH = len(lines) * listRend.LineHeightPx

			if y+blockH > 0 && y < editorH {
				textX := editorX + indent

				// Render bullet marker first for list-style checkboxes (- [ ])
				if hasCheckbox && block.Marker != "" {
					listRend.RenderGlyphs(gtx.Ops, gtx, block.Marker, textX, y, theme.MdAccent)
					textX += (len(block.Marker) + 1) * listRend.CharWidth
				}

				if hasCheckbox {
					lh := listRend.LineHeightPx
					// Small thin box, vertically centered
					boxSize := lh * 1 / 2
					if boxSize < 8 {
						boxSize = 8
					}
					hitSize := lh            // larger hit target
					cbX := textX + (hitSize-boxSize)/2  // center box in hit area
					cbY := y + (lh-boxSize)/2

					hoverX := st.hoverX
					hoverY := st.hoverY - st.tabBarHeight
					cbHovered := hoverX >= textX && hoverX < textX+hitSize &&
						hoverY >= y && hoverY < y+lh

					if checkboxChecked {
						checkColor := color.NRGBA{R: 60, G: 180, B: 80, A: 255} // green
						borderColor := theme.TabDimFg
						if cbHovered {
							checkColor = theme.MdAccent
							borderColor = theme.MdAccent
						}
						// Thin 1px border in check color
						off := op.Offset(image.Pt(cbX, cbY)).Push(gtx.Ops)
						r := clip.UniformRRect(image.Rectangle{Max: image.Pt(boxSize, boxSize)}, 3).Push(gtx.Ops)
						paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						r.Pop()
						off.Pop()
						inOff := op.Offset(image.Pt(cbX+1, cbY+1)).Push(gtx.Ops)
						inR := clip.UniformRRect(image.Rectangle{Max: image.Pt(boxSize-2, boxSize-2)}, 2).Push(gtx.Ops)
						paint.ColorOp{Color: theme.Background}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						inR.Pop()
						inOff.Pop()
						// Bold checkmark — h4 bold for larger glyph
						cr := mr.h4
						glyphX := cbX + (boxSize-cr.CharWidth)/2 + cr.CharWidth/8
						glyphY := cbY + (boxSize-cr.LineHeightPx)/2 - 1
						cr.RenderGlyphs(gtx.Ops, gtx, "✓", glyphX, glyphY, checkColor)
					} else {
						borderColor := theme.TabDimFg
						if cbHovered {
							borderColor = theme.MdAccent
						}
						// Thin 1px border
						off := op.Offset(image.Pt(cbX, cbY)).Push(gtx.Ops)
						r := clip.UniformRRect(image.Rectangle{Max: image.Pt(boxSize, boxSize)}, 3).Push(gtx.Ops)
						paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						r.Pop()
						off.Pop()
						inOff := op.Offset(image.Pt(cbX+1, cbY+1)).Push(gtx.Ops)
						inR := clip.UniformRRect(image.Rectangle{Max: image.Pt(boxSize-2, boxSize-2)}, 2).Push(gtx.Ops)
						paint.ColorOp{Color: theme.Background}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						inR.Pop()
						inOff.Pop()
					}

					textX += hitSize

					// Register hit area (full line height)
					ts.mdCheckboxes = append(ts.mdCheckboxes, mdCheckbox{
						x: textX - hitSize, y: y + st.tabBarHeight, w: hitSize, h: lh,
						sourceOffset: block.SourceOffset,
						checked:      checkboxChecked,
					})
				} else if block.Marker != "" {
					// Regular marker (skip for standalone checkboxes with Level -1)
					listRend.RenderGlyphs(gtx.Ops, gtx, block.Marker, textX, y, theme.MdAccent)
					textX += (len(block.Marker) + 1) * listRend.CharWidth
				}

				for i, line := range lines {
					listRend.RenderGlyphs(gtx.Ops, gtx, line, textX, y+i*listRend.LineHeightPx, theme.Foreground)
				}
			}

		case render.BlockThematicBreak:
			blockH = mr.body.LineHeightPx
			if y+blockH > 0 && y < editorH {
				lineY := y + blockH/2
				lineRect := clip.Rect{
					Min: image.Pt(editorX, lineY),
					Max: image.Pt(editorX+maxW, lineY+1),
				}.Push(gtx.Ops)
				paint.ColorOp{Color: theme.GutterSep}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				lineRect.Pop()
			}

		case render.BlockTable:
			blockH = st.drawMarkdownTable(gtx, mr, &block, editorX, y, maxW, editorH, theme)
		}

		y += blockH
		prevKind = block.Kind
	}

	// Store total height for scroll clamping
	ts.mdTotalH = y + scrollY
}

// drawMarkdownTable renders a table block sized to its content and returns its height.
func (st *appState) drawMarkdownTable(gtx layout.Context, mr *mdRenderers, block *render.Block, x, y, maxW, editorH int, theme config.Theme) int {
	if len(block.TableCells) == 0 {
		return 0
	}
	tr := mr.code
	cellPadX := 10
	cellPadY := 4
	rowH := tr.LineHeightPx + cellPadY*2

	// Compute column widths from content
	numCols := 0
	for _, row := range block.TableCells {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	colWidths := make([]int, numCols)
	for _, row := range block.TableCells {
		for c, cell := range row {
			w := len(cell)*tr.CharWidth + cellPadX*2
			if w > colWidths[c] {
				colWidths[c] = w
			}
		}
	}

	// Total table width from column widths
	tableW := 1 // left border
	for _, w := range colWidths {
		tableW += w + 1 // column + right border
	}
	totalH := len(block.TableCells)*rowH + 1 // rows + bottom border

	if y+totalH > 0 && y < editorH {
		borderColor := theme.GutterSep

		// Table background
		bgOff := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		bgRect := clip.Rect{Max: image.Pt(tableW, totalH)}.Push(gtx.Ops)
		paint.ColorOp{Color: shiftColor(theme.Background, 6)}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		bgRect.Pop()
		bgOff.Pop()

		for r, row := range block.TableCells {
			ry := y + r*rowH

			// Header row has a stronger background
			if r == 0 {
				hdrOff := op.Offset(image.Pt(x+1, ry)).Push(gtx.Ops)
				hdrRect := clip.Rect{Max: image.Pt(tableW-2, rowH)}.Push(gtx.Ops)
				paint.ColorOp{Color: shiftColor(theme.Background, 15)}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				hdrRect.Pop()
				hdrOff.Pop()
			}

			// Cell text
			cx := x + 1 // after left border
			for c, cell := range row {
				fg := theme.Foreground
				if r == 0 {
					fg = theme.MdHeading
				}
				tr.RenderGlyphs(gtx.Ops, gtx, cell, cx+cellPadX, ry+cellPadY, fg)
				if c < len(colWidths) {
					cx += colWidths[c]

					// Column separator
					colLine := clip.Rect{
						Min: image.Pt(cx, y),
						Max: image.Pt(cx+1, y+totalH),
					}.Push(gtx.Ops)
					paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					colLine.Pop()
					cx++ // skip border pixel
				}
			}

			// Row bottom border
			borderY := ry + rowH
			rowLine := clip.Rect{
				Min: image.Pt(x, borderY),
				Max: image.Pt(x+tableW, borderY+1),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			rowLine.Pop()
		}

		// Outer border: top, left, right
		// Top
		topLine := clip.Rect{Min: image.Pt(x, y), Max: image.Pt(x+tableW, y+1)}.Push(gtx.Ops)
		paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		topLine.Pop()
		// Left
		leftLine := clip.Rect{Min: image.Pt(x, y), Max: image.Pt(x+1, y+totalH)}.Push(gtx.Ops)
		paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		leftLine.Pop()
		// Right
		rightLine := clip.Rect{Min: image.Pt(x+tableW-1, y), Max: image.Pt(x+tableW, y+totalH)}.Push(gtx.Ops)
		paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		rightLine.Pop()
	}

	return totalH
}

// spansToPlain concatenates all InlineSpan text.
func spansToPlain(spans []render.InlineSpan) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

// blocksToPlain extracts plain text from a slice of blocks.
func blocksToPlain(blocks []render.Block) string {
	var b strings.Builder
	for _, block := range blocks {
		b.WriteString(spansToPlain(block.Spans))
	}
	return b.String()
}

// splitAndWrap splits text on newlines first, then wraps each segment.
func splitAndWrap(text string, maxW, charW int) []string {
	segments := strings.Split(text, "\n")
	var lines []string
	for _, seg := range segments {
		seg = strings.TrimRight(seg, " ")
		if seg == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapText(seg, maxW, charW)...)
	}
	// Trim trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// wrapText wraps text to fit within maxW pixels given a character width.
func wrapText(text string, maxW, charW int) []string {
	if charW <= 0 {
		return []string{text}
	}
	maxCols := maxW / charW
	if maxCols <= 0 {
		maxCols = 1
	}
	if len(text) <= maxCols {
		return []string{text}
	}

	var lines []string
	for len(text) > 0 {
		if len(text) <= maxCols {
			lines = append(lines, text)
			break
		}
		// Find last space before maxCols
		cut := maxCols
		for cut > 0 && text[cut] != ' ' {
			cut--
		}
		if cut == 0 {
			cut = maxCols // no space found, hard break
		}
		lines = append(lines, text[:cut])
		text = strings.TrimLeft(text[cut:], " ")
	}
	return lines
}

// renderStyledLines renders paragraph lines using per-span bold/italic renderers.
func (st *appState) renderStyledLines(gtx layout.Context, mr *mdRenderers, spans []render.InlineSpan, lines []string, x, y int, theme config.Theme) {
	if len(lines) == 0 || mr.body.CharWidth == 0 {
		return
	}

	charW := mr.body.CharWidth
	lineIdx := 0
	lineCol := 0
	lineY := y

	for _, span := range spans {
		text := span.Text

		// Pick renderer and color
		r := mr.body
		fg := theme.Foreground
		switch {
		case span.Bold && span.Italic:
			r = mr.boldItal
			fg = theme.MdAccent
		case span.Bold:
			r = mr.bold
			fg = theme.MdAccent
		case span.Italic:
			r = mr.ital
			fg = theme.MdAccent
		case span.Code:
			r = mr.code
			fg = theme.MdAccent
		}

		for len(text) > 0 {
			if lineIdx >= len(lines) {
				return
			}

			// Handle newlines first
			nlIdx := strings.IndexByte(text, '\n')
			if nlIdx == 0 {
				lineCol = 0
				lineIdx++
				lineY += mr.body.LineHeightPx
				text = text[1:]
				continue
			}

			// Determine chunk to render on current line
			line := lines[lineIdx]
			remaining := len(line) - lineCol
			if remaining <= 0 {
				// Advance to next line
				lineCol = 0
				lineIdx++
				lineY += mr.body.LineHeightPx
				continue
			}

			chunk := text
			if nlIdx >= 0 && nlIdx < len(chunk) {
				chunk = chunk[:nlIdx]
			}
			if len(chunk) > remaining {
				chunk = chunk[:remaining]
			}

			if len(chunk) > 0 {
				px := x + lineCol*charW
				r.RenderGlyphs(gtx.Ops, gtx, chunk, px, lineY, fg)
				lineCol += len(chunk)
			}

			text = text[len(chunk):]

			// Advance line if full
			if lineCol >= len(line) && lineIdx < len(lines)-1 {
				lineCol = 0
				lineIdx++
				lineY += mr.body.LineHeightPx
			}
		}
	}
}

// shiftColor brightens or darkens a color by delta.
func shiftColor(c color.NRGBA, delta uint8) color.NRGBA {
	add := func(v, d uint8) uint8 {
		if int(v)+int(d) > 255 {
			return 255
		}
		return v + d
	}
	sub := func(v, d uint8) uint8 {
		if int(v)-int(d) < 0 {
			return 0
		}
		return v - d
	}
	// If dark (avg < 128), brighten; otherwise darken
	avg := (int(c.R) + int(c.G) + int(c.B)) / 3
	if avg < 128 {
		return color.NRGBA{R: add(c.R, delta), G: add(c.G, delta), B: add(c.B, delta), A: c.A}
	}
	return color.NRGBA{R: sub(c.R, delta), G: sub(c.G, delta), B: sub(c.B, delta), A: c.A}
}

// drawRoundedBorder draws a 1px border of a rounded rectangle using 4 edge rects.
func drawRoundedBorder(ops *op.Ops, x, y, w, h, r int, c color.NRGBA) {
	// Top
	tr := clip.Rect{Min: image.Pt(x+r, y), Max: image.Pt(x+w-r, y+1)}.Push(ops)
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
	tr.Pop()
	// Bottom
	br := clip.Rect{Min: image.Pt(x+r, y+h-1), Max: image.Pt(x+w-r, y+h)}.Push(ops)
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
	br.Pop()
	// Left
	lr := clip.Rect{Min: image.Pt(x, y+r), Max: image.Pt(x+1, y+h-r)}.Push(ops)
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
	lr.Pop()
	// Right
	rr := clip.Rect{Min: image.Pt(x+w-1, y+r), Max: image.Pt(x+w, y+h-r)}.Push(ops)
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
	rr.Pop()
}
