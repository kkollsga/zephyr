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

// mdSelBlock records a block's layout position and its range in the flat
// selection text, used for mapping mouse coordinates to character offsets.
type mdSelBlock struct {
	y, h     int   // screen y and height (before scroll adjustment)
	x        int   // left edge of text
	textOff  int   // character offset into mdSelText
	textLen  int   // number of characters
	lineH    int   // line height in pixels
	charW    int   // average character width for hit testing
	lineLens []int // characters per wrapped line
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

	mk := func(size float32, lh float32, ls float32, face string, weight font.Weight, style font.Style) *render.TextRenderer {
		r := render.NewTextRenderer(st.shaper, render.TextStyle{
			FontSize:      unit.Sp(size),
			LineHeight:    lh,
			LetterSpacing: ls,
			Foreground:    fg,
			Typeface:      face,
			Weight:        weight,
			FontStyle:     style,
		})
		r.ComputeMetrics(gtx)
		return r
	}

	var bodyLS float32 = 0.3 // subtle letter spacing for body text

	st.mdRend = &mdRenderers{
		h1:        mk(28, 1.2, 0, heading, font.Bold, font.Regular),
		h2:        mk(24, 1.2, 0, heading, font.Bold, font.Regular),
		h3:        mk(20, 1.2, 0, heading, font.Bold, font.Regular),
		h4:        mk(17, 1.2, 0, heading, font.Bold, font.Regular),
		h5:        mk(15, 1.2, 0, heading, font.Normal, font.Regular),
		h6:        mk(14, 1.2, 0, heading, font.Normal, font.Regular),
		body:      mk(15, 1.55, bodyLS, body, font.Normal, font.Regular),
		bodySmall: mk(14, 1.5, bodyLS, body, font.Normal, font.Regular),
		code:      mk(13, 1.3, 0, mono, font.Normal, font.Regular),
		bold:      mk(15, 1.55, bodyLS, body, font.Bold, font.Regular),
		ital:      mk(15, 1.55, bodyLS, body, font.Normal, font.Italic),
		boldItal:  mk(15, 1.55, bodyLS, body, font.Bold, font.Italic),
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
	ts.mdCopyBtns = ts.mdCopyBtns[:0]       // reset copy button hit areas
	ts.mdCheckboxes = ts.mdCheckboxes[:0]   // reset checkbox hit areas
	ts.mdSelBlocks = ts.mdSelBlocks[:0]     // reset selection blocks
	var selBuf strings.Builder              // build flat text for selection

	charW := st.textRend.CharWidth
	if charW == 0 {
		charW = 8
	}
	minMargin := charW * 3 // minimum side margin
	windowW := gtx.Constraints.Max.X

	// Apply max body width (pixels) and center
	mdMaxPx := st.themeBundle.MdMaxWidth
	if mdMaxPx <= 0 {
		mdMaxPx = 1230
	}
	maxW := windowW - minMargin*2
	if maxW > mdMaxPx {
		maxW = mdMaxPx
	}
	if maxW < charW {
		maxW = charW
	}
	editorX := (windowW - maxW) / 2 // center the body
	editorH := gtx.Constraints.Max.Y // already clipped to editor area

	scrollY := int(ts.mdScrollY)
	y := editorTopPad - scrollY

	prevKind := render.BlockKind(-1)
	for _, block := range ts.mdDoc.Blocks {
		var blockH int

		// Top spacing — tuned to match the SVG mockup's tighter layout.
		bodyH := mr.body.LineHeightPx
		blanks := block.BlankLinesBefore
		switch {
		case prevKind < 0:
			// first block — generous top margin
			y += bodyH * 2
		case block.Kind == render.BlockHeading:
			// Large gap above headings to clearly separate sections
			y += bodyH * 2
		case block.Kind == render.BlockListItem && prevKind == render.BlockListItem && blanks == 0:
			// Tight list items
			y += bodyH / 4
		case block.Kind == render.BlockCodeBlock:
			y += bodyH * 3 / 4
		case prevKind == render.BlockCodeBlock:
			y += bodyH * 3 / 4
		case blanks >= 2:
			y += bodyH + (blanks-1)*bodyH/3
		case blanks == 1:
			y += bodyH * 2 / 3
		default:
			y += bodyH / 3
		}

		switch block.Kind {
		case render.BlockHeading:
			hr := mr.heading(block.Level)
			text := spansToPlain(block.Spans)
			lines := wrapTextMeasured(text, maxW, gtx, hr)
			blockH = len(lines) * hr.LineHeightPx

			// Add separator line below H1/H2 — tight below, space comes from next block's margin
			sepH := 0
			if block.Level <= 2 {
				sepH = bodyH / 3
				blockH += sepH
			}

			if y+blockH > 0 && y < editorH {
				for i, line := range lines {
					hr.RenderGlyphs(gtx.Ops, gtx, line, editorX, y+i*hr.LineHeightPx, theme.MdHeading)
				}
				// Horizontal rule under H1/H2
				if block.Level <= 2 {
					lineY := y + blockH - sepH/3
					lineRect := clip.Rect{
						Min: image.Pt(editorX, lineY),
						Max: image.Pt(editorX+maxW, lineY+1),
					}.Push(gtx.Ops)
					paint.ColorOp{Color: theme.GutterSep}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					lineRect.Pop()
				}
			}

		case render.BlockParagraph:
			// Estimate height for scroll
			lines := wrapTextMeasured(spansToPlain(block.Spans), maxW, gtx, mr.body)
			blockH = len(lines) * mr.body.LineHeightPx

			if y+blockH > 0 && y < editorH {
				// Render with pixel-precise positioning; update blockH with actual height
				blockH = st.renderStyledParagraph(gtx, mr, block.Spans, maxW, editorX, y, theme)
			}

		case render.BlockCodeBlock:
			padding := 10
			codeCharW := mr.code.CharWidth
			if codeCharW == 0 {
				codeCharW = 8
			}

			// Header row height for language label + copy button
			headerH := mr.code.LineHeightPx + padding
			hasHeader := block.CodeLang != ""

			// Code blocks span the full content width
			rawLines := strings.Split(block.CodeText, "\n")
			codeBoxW := maxW

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

			codeAreaH := len(codeLines)*mr.code.LineHeightPx + padding*2
			if hasHeader {
				blockH = headerH + codeAreaH
			} else {
				blockH = codeAreaH
			}

			if y+blockH > 0 && y < editorH {
				bgColor := shiftColor(theme.Background, 15)
				headerBg := shiftColor(theme.Background, 25)

				// Full block background with rounded corners
				bgOff := op.Offset(image.Pt(editorX, y)).Push(gtx.Ops)
				bgRect := clip.UniformRRect(image.Rectangle{
					Max: image.Pt(codeBoxW, blockH),
				}, 6).Push(gtx.Ops)
				paint.ColorOp{Color: bgColor}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				bgRect.Pop()
				bgOff.Pop()

				if hasHeader {
					// Header row background (top portion, clipped to rounded top)
					hdrOff := op.Offset(image.Pt(editorX, y)).Push(gtx.Ops)
					hdrClip := clip.UniformRRect(image.Rectangle{
						Max: image.Pt(codeBoxW, headerH),
					}, 6).Push(gtx.Ops)
					paint.ColorOp{Color: headerBg}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					hdrClip.Pop()
					hdrOff.Pop()
					// Fill the bottom of the header (below the rounded corners)
					hdrBotOff := op.Offset(image.Pt(editorX, y+6)).Push(gtx.Ops)
					hdrBotRect := clip.Rect{Max: image.Pt(codeBoxW, headerH-6)}.Push(gtx.Ops)
					paint.ColorOp{Color: headerBg}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					hdrBotRect.Pop()
					hdrBotOff.Pop()

					// Language label in header
					langY := y + (headerH-mr.code.LineHeightPx)/2
					mr.code.RenderGlyphs(gtx.Ops, gtx, block.CodeLang, editorX+padding, langY, theme.StatusFg)
				}

				// Subtle "Copy" text — in header row if present, else top-right of code area
				{
					copyText := "Copy"
					copyTextW := len(copyText) * mr.code.CharWidth
					copyPadX := 8
					copyPadY := 3
					copyBtnW := copyTextW + copyPadX*2
					copyBtnH := mr.code.LineHeightPx + copyPadY*2
					copyBtnX := editorX + codeBoxW - copyBtnW - padding
					copyBtnY := y + padding/2
					if hasHeader {
						copyBtnY = y + (headerH-copyBtnH)/2
					}
					copyBg := bgColor
					if hasHeader {
						copyBg = headerBg
					}

					copyHovered := st.hoverX >= copyBtnX && st.hoverX < copyBtnX+copyBtnW &&
						st.hoverY-st.tabBarHeight >= copyBtnY && st.hoverY-st.tabBarHeight < copyBtnY+copyBtnH

					pillFg := theme.TabDimFg
					if copyHovered {
						pillBg := shiftColor(copyBg, 20)
						pillOff := op.Offset(image.Pt(copyBtnX, copyBtnY)).Push(gtx.Ops)
						pillRect := clip.UniformRRect(image.Rectangle{Max: image.Pt(copyBtnW, copyBtnH)}, 3).Push(gtx.Ops)
						paint.ColorOp{Color: pillBg}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						pillRect.Pop()
						pillOff.Pop()
						pillFg = theme.Foreground
					}

					mr.code.RenderGlyphs(gtx.Ops, gtx, copyText, copyBtnX+copyPadX, copyBtnY+copyPadY, pillFg)

					ts.mdCopyBtns = append(ts.mdCopyBtns, codeCopyBtn{
						x: copyBtnX, y: copyBtnY + st.tabBarHeight, w: copyBtnW, h: copyBtnH,
						code: block.CodeText,
					})
				}

				// Code text — offset below header if present, with basic syntax coloring
				codeStartY := y
				if hasHeader {
					codeStartY = y + headerH
				}
				for i, line := range codeLines {
					lineColor := codeLineColor(line, block.CodeLang, theme)
					mr.code.RenderGlyphs(gtx.Ops, gtx, line, editorX+padding, codeStartY+padding+i*mr.code.LineHeightPx, lineColor)
				}
			}

		case render.BlockBlockquote:
			// Render children as italic text with a colored left bar
			childText := blocksToPlain(block.Children)
			bqRend := mr.ital
			lines := wrapTextMeasured(childText, maxW-24, gtx, bqRend)
			blockH = len(lines) * bqRend.LineHeightPx

			if y+blockH > 0 && y < editorH {
				// Left bar (4px wide, accent color)
				barRect := clip.Rect{
					Min: image.Pt(editorX, y),
					Max: image.Pt(editorX+4, y+blockH),
				}.Push(gtx.Ops)
				paint.ColorOp{Color: theme.MdHeading}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				barRect.Pop()

				// Italic text in subdued color
				for i, line := range lines {
					bqRend.RenderGlyphs(gtx.Ops, gtx, line, editorX+20, y+i*bqRend.LineHeightPx, theme.StatusFg)
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
			lines := wrapTextMeasured(text, maxW-indent-24, gtx, listRend)
			if len(lines) == 0 {
				lines = []string{""}
			}
			blockH = len(lines) * listRend.LineHeightPx

			if y+blockH > 0 && y < editorH {
				textX := editorX + indent

				if hasCheckbox {
					lh := listRend.LineHeightPx
					// Checkbox box size — slightly larger for visual weight
					boxSize := lh * 3 / 5
					if boxSize < 10 {
						boxSize = 10
					}
					hitSize := lh            // larger hit target
					cbX := textX + (hitSize-boxSize)/2  // center box in hit area
					cbY := y + (lh-boxSize)/2

					hoverX := st.hoverX
					hoverY := st.hoverY - st.tabBarHeight
					cbHovered := hoverX >= textX && hoverX < textX+hitSize &&
						hoverY >= y && hoverY < y+lh

					if checkboxChecked {
						// Filled teal box with white checkmark
						fillColor := theme.Type // teal (#4ec9b0 dark / #267f99 light)
						if cbHovered {
							fillColor = theme.MdAccent
						}
						// Filled rounded rect
						off := op.Offset(image.Pt(cbX, cbY)).Push(gtx.Ops)
						r := clip.UniformRRect(image.Rectangle{Max: image.Pt(boxSize, boxSize)}, 3).Push(gtx.Ops)
						paint.ColorOp{Color: fillColor}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						r.Pop()
						off.Pop()
						// White checkmark — draw two lines forming a centered check
						checkColor := theme.Background
						sw := max(boxSize/6, 2) // stroke width
						// Checkmark centered in box:
						// Bottom of check at ~65% height, top-left at ~45%, top-right at ~25%
						// Horizontal: left start ~20%, bottom ~40%, right end ~80%
						cx := cbX
						cy := cbY
						x1, y1 := cx+boxSize*20/100, cy+boxSize*50/100  // left start
						x2, y2 := cx+boxSize*40/100, cy+boxSize*72/100  // bottom vertex
						x3, y3 := cx+boxSize*80/100, cy+boxSize*28/100  // right end
						drawThickLine(gtx.Ops, x1, y1, x2, y2, sw, checkColor)
						drawThickLine(gtx.Ops, x2, y2, x3, y3, sw, checkColor)
					} else {
						// Empty bordered box
						borderColor := theme.TabDimFg
						if cbHovered {
							borderColor = theme.MdAccent
						}
						bw := max(boxSize/8, 1) // border width
						// Outer rect
						off := op.Offset(image.Pt(cbX, cbY)).Push(gtx.Ops)
						r := clip.UniformRRect(image.Rectangle{Max: image.Pt(boxSize, boxSize)}, 3).Push(gtx.Ops)
						paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						r.Pop()
						off.Pop()
						// Inner rect (cut out to make border)
						inOff := op.Offset(image.Pt(cbX+bw, cbY+bw)).Push(gtx.Ops)
						inR := clip.UniformRRect(image.Rectangle{Max: image.Pt(boxSize-bw*2, boxSize-bw*2)}, 2).Push(gtx.Ops)
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

		// Record block for text selection with per-line character counts
		blockText := blockPlainText(&block)
		rend := mr.body
		if block.Kind == render.BlockHeading {
			rend = mr.heading(block.Level)
		} else if block.Kind == render.BlockCodeBlock {
			rend = mr.code
		}
		wrappedLines := wrapTextMeasured(blockText, maxW, gtx, rend)
		lineLens := make([]int, len(wrappedLines))
		for i, l := range wrappedLines {
			lineLens[i] = len(l)
		}
		ts.mdSelBlocks = append(ts.mdSelBlocks, mdSelBlock{
			y:        y + scrollY, // absolute y (not scroll-adjusted)
			h:        blockH,
			x:        editorX,
			textOff:  selBuf.Len(),
			textLen:  len(blockText),
			lineH:    rend.LineHeightPx,
			charW:    max(rend.CharWidth, 1),
			lineLens: lineLens,
		})
		selBuf.WriteString(blockText)
		selBuf.WriteByte('\n')

		y += blockH
		prevKind = block.Kind
	}
	ts.mdSelText = selBuf.String()

	// Draw selection highlight (character-level)
	if ts.mdSelAnchor != ts.mdSelCursor {
		selLo, selHi := ts.mdSelAnchor, ts.mdSelCursor
		if selLo > selHi {
			selLo, selHi = selHi, selLo
		}
		selColor := theme.Selection
		for _, sb := range ts.mdSelBlocks {
			blockEnd := sb.textOff + sb.textLen
			if selHi <= sb.textOff || selLo >= blockEnd+1 {
				continue
			}
			by := sb.y - scrollY
			if by+sb.h <= 0 || by >= editorH {
				continue
			}
			// Walk wrapped lines and draw highlight per line
			charOff := sb.textOff
			lineY := by
			for _, ll := range sb.lineLens {
				lineEnd := charOff + ll
				// Does this line overlap selection?
				if selHi > charOff && selLo < lineEnd {
					startCol := 0
					if selLo > charOff {
						startCol = selLo - charOff
					}
					endCol := ll
					if selHi < lineEnd {
						endCol = selHi - charOff
					}
					x1 := sb.x + startCol*sb.charW
					x2 := sb.x + endCol*sb.charW
					hlOff := op.Offset(image.Pt(x1, lineY)).Push(gtx.Ops)
					hlRect := clip.Rect{Max: image.Pt(x2-x1, sb.lineH)}.Push(gtx.Ops)
					paint.ColorOp{Color: selColor}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					hlRect.Pop()
					hlOff.Pop()
				}
				charOff = lineEnd + 1 // +1 for word-wrap space
				lineY += sb.lineH
			}
		}
	}

	// Store total height for scroll clamping.
	// Add bottom padding so the last line isn't hidden behind the status bar.
	statusH := mr.body.LineHeightPx + 6
	ts.mdTotalH = y + scrollY + statusH + mr.body.LineHeightPx*2
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

// blockPlainText returns the plain text of a single block.
func blockPlainText(b *render.Block) string {
	switch b.Kind {
	case render.BlockCodeBlock:
		return b.CodeText
	case render.BlockBlockquote:
		return blocksToPlain(b.Children)
	case render.BlockThematicBreak:
		return ""
	case render.BlockTable:
		var buf strings.Builder
		for _, row := range b.TableCells {
			for c, cell := range row {
				if c > 0 {
					buf.WriteByte('\t')
				}
				buf.WriteString(cell)
			}
			buf.WriteByte('\n')
		}
		return strings.TrimRight(buf.String(), "\n")
	default:
		return spansToPlain(b.Spans)
	}
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

// mdCharOffset maps a screen position (px, py) to a character offset in mdSelText.
// py should be in absolute coordinates (with scroll added back).
func mdCharOffset(blocks []mdSelBlock, px, py int) int {
	// Find the block containing py
	for _, b := range blocks {
		if py < b.y || py >= b.y+b.h {
			continue
		}
		if b.textLen == 0 || len(b.lineLens) == 0 {
			return b.textOff
		}
		// Which wrapped line within the block
		lineIdx := (py - b.y) / b.lineH
		if lineIdx < 0 {
			lineIdx = 0
		}
		if lineIdx >= len(b.lineLens) {
			lineIdx = len(b.lineLens) - 1
		}
		// Character offset to the start of this line
		lineOff := 0
		for i := 0; i < lineIdx; i++ {
			lineOff += b.lineLens[i]
			lineOff++ // account for space consumed by word wrap
		}
		// Column within the line
		col := (px - b.x) / b.charW
		if col < 0 {
			col = 0
		}
		if col > b.lineLens[lineIdx] {
			col = b.lineLens[lineIdx]
		}
		off := b.textOff + lineOff + col
		if off > b.textOff+b.textLen {
			off = b.textOff + b.textLen
		}
		return off
	}
	// Before first block
	if len(blocks) > 0 && py < blocks[0].y {
		return 0
	}
	// After last block
	if len(blocks) > 0 {
		last := blocks[len(blocks)-1]
		if py >= last.y+last.h {
			return last.textOff + last.textLen
		}
	}
	return 0
}

// mdSelectedText returns the selected text from the flat document text.
func mdSelectedText(selText string, anchor, cursor int) string {
	lo, hi := anchor, cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo < 0 {
		lo = 0
	}
	if hi > len(selText) {
		hi = len(selText)
	}
	return selText[lo:hi]
}

// wrapTextMeasured wraps text using actual glyph measurement for proportional fonts.
func wrapTextMeasured(text string, maxW int, gtx layout.Context, tr *render.TextRenderer) []string {
	segments := strings.Split(text, "\n")
	var lines []string
	for _, seg := range segments {
		seg = strings.TrimRight(seg, " ")
		if seg == "" {
			lines = append(lines, "")
			continue
		}
		if tr.MeasureString(gtx, seg) <= maxW {
			lines = append(lines, seg)
			continue
		}
		// Word-wrap by measuring
		words := strings.Fields(seg)
		cur := ""
		for _, w := range words {
			candidate := cur
			if candidate != "" {
				candidate += " "
			}
			candidate += w
			if tr.MeasureString(gtx, candidate) > maxW && cur != "" {
				lines = append(lines, cur)
				cur = w
			} else {
				cur = candidate
			}
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// renderStyledParagraph renders paragraph spans with pixel-based positioning
// and word-level wrapping. Returns the total height in pixels.
func (st *appState) renderStyledParagraph(gtx layout.Context, mr *mdRenderers, spans []render.InlineSpan, maxPixelW, x, y int, theme config.Theme) int {
	if mr.body.CharWidth == 0 {
		return mr.body.LineHeightPx
	}

	pixelX := 0 // current X offset from x
	lineY := y
	lineH := mr.body.LineHeightPx

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
			// Handle leading newline
			if text[0] == '\n' {
				pixelX = 0
				lineY += lineH
				text = text[1:]
				continue
			}

			// Find next newline to scope this segment
			nlIdx := strings.IndexByte(text, '\n')
			segment := text
			if nlIdx >= 0 {
				segment = text[:nlIdx]
			}

			// Word-wrap within this segment
			for len(segment) > 0 {
				// Find next word (including trailing space)
				spIdx := strings.IndexByte(segment, ' ')
				word := segment
				if spIdx >= 0 {
					word = segment[:spIdx+1]
				}

				wordW := r.MeasureString(gtx, word)

				// Wrap to next line if this word overflows
				if pixelX > 0 && pixelX+wordW > maxPixelW {
					pixelX = 0
					lineY += lineH
				}

				r.RenderGlyphs(gtx.Ops, gtx, word, x+pixelX, lineY, fg)
				pixelX += wordW

				segment = segment[len(word):]
			}

			if nlIdx >= 0 {
				text = text[nlIdx:] // let the loop handle the '\n'
			} else {
				text = ""
			}
		}
	}

	return lineY - y + lineH
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

// drawThickLine draws a thick line between two points using a filled rectangle.
// For diagonal lines, it uses a simple approximation with horizontal segments.
func drawThickLine(ops *op.Ops, x1, y1, x2, y2, thickness int, c color.NRGBA) {
	if y1 == y2 {
		// Horizontal line
		minX, maxX := x1, x2
		if x1 > x2 {
			minX, maxX = x2, x1
		}
		r := clip.Rect{Min: image.Pt(minX, y1-thickness/2), Max: image.Pt(maxX, y1-thickness/2+thickness)}.Push(ops)
		paint.ColorOp{Color: c}.Add(ops)
		paint.PaintOp{}.Add(ops)
		r.Pop()
		return
	}
	// Diagonal: step through y values and draw horizontal segments
	dy := y2 - y1
	dx := x2 - x1
	steps := dy
	if steps < 0 {
		steps = -steps
	}
	if steps == 0 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		cx := x1 + int(t*float64(dx))
		cy := y1 + int(t*float64(dy))
		r := clip.Rect{
			Min: image.Pt(cx-thickness/2, cy),
			Max: image.Pt(cx-thickness/2+thickness, cy+1),
		}.Push(ops)
		paint.ColorOp{Color: c}.Add(ops)
		paint.PaintOp{}.Add(ops)
		r.Pop()
	}
}

// codeLineColor returns the syntax color for a line of code in a fenced code block.
// Provides basic highlighting for comments and strings.
func codeLineColor(line, lang string, theme config.Theme) color.NRGBA {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return theme.Foreground
	}
	// Comment patterns by language family
	switch {
	case strings.HasPrefix(trimmed, "//"):
		return theme.Comment
	case strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "#!"):
		// Shell, Python, Ruby, YAML, etc.
		return theme.Comment
	case strings.HasPrefix(trimmed, "--"):
		// SQL, Lua, Haskell
		return theme.Comment
	case strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, " *") || strings.HasPrefix(trimmed, "*/"):
		return theme.Comment
	case strings.HasPrefix(trimmed, ";"):
		// Assembly, INI comments
		if lang == "asm" || lang == "ini" || lang == "toml" {
			return theme.Comment
		}
	}
	return theme.Foreground
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
