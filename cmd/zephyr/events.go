package main

import (
	"math"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/highlight"
	"github.com/kristianweb/zephyr/pkg/clipboard"
)

func (st *appState) handleEvents(gtx layout.Context, w *app.Window) {
	areaStack := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, st.tag)
	key.InputHintOp{Tag: st.tag, Hint: key.HintAny}.Add(gtx.Ops)
	areaStack.Pop()
	gtx.Source.Execute(key.FocusCmd{Tag: st.tag})

	// Compute dynamic scroll range based on viewport position.
	scrollRange := pointer.ScrollRange{Min: -10000, Max: 10000}
	if ts := st.activeTabState(); ts != nil && st.textRend != nil && st.textRend.LineHeightPx > 0 {
		if ts.mode == viewMarkdownRead {
			up := int(ts.mdScrollY)
			editorH := st.lastMaxY - st.tabBarHeight
			down := ts.mdTotalH - editorH - up
			if down < 0 {
				down = 0
			}
			scrollRange = pointer.ScrollRange{Min: -up, Max: down}
		} else {
			up, down := ts.viewport.ScrollablePixels(st.textRend.LineHeightPx)
			scrollRange = pointer.ScrollRange{Min: -up, Max: down}
		}
	}

	for {
		ev, ok := gtx.Source.Event(
			key.FocusFilter{Target: st.tag},
			key.Filter{Focus: st.tag, Optional: key.ModShortcut | key.ModShift | key.ModAlt},
			key.Filter{Focus: st.tag, Name: key.NameTab},
			key.Filter{Focus: st.tag, Name: key.NameTab, Optional: key.ModShift},
			pointer.Filter{Target: st.tag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Scroll | pointer.Move, ScrollY: scrollRange},
		)
		if !ok {
			break
		}
		switch ke := ev.(type) {
		case key.Event:
			if ke.State == key.Press {
				st.handleKey(ke)
			}
		case key.EditEvent:
			if st.langSel.Visible {
				break
			}
			st.handleTextInput(ke.Text)
		case pointer.Event:
			st.handlePointer(ke)
		}
	}
}

func (st *appState) handleKey(ke key.Event) {
	// Unified save menu intercepts all input
	if st.saveMenu.visible {
		if st.saveMenuShowSaveAs() {
			// Save As rows visible — handle filename editing keys
			switch {
			case ke.Name == key.NameEscape:
				st.saveMenu.visible = false
				st.quitInProgress = false
			case ke.Name == key.NameReturn:
				st.executeSaveAs()
			case ke.Name == key.NameDeleteBackward:
				st.saveAsDeleteBack()
			case ke.Name == key.NameDeleteForward:
				st.saveAsDeleteForward()
			case ke.Name == key.NameLeftArrow && ke.Modifiers == 0:
				st.saveMenu.selectAll = false
				if st.saveMenu.cursor > 0 {
					st.saveMenu.cursor--
				}
			case ke.Name == key.NameRightArrow && ke.Modifiers == 0:
				st.saveMenu.selectAll = false
				if st.saveMenu.cursor < len(st.saveMenu.filename) {
					st.saveMenu.cursor++
				}
			case ke.Name == key.NameLeftArrow && ke.Modifiers == key.ModShortcut:
				st.saveMenu.selectAll = false
				st.saveMenu.cursor = 0
			case ke.Name == key.NameRightArrow && ke.Modifiers == key.ModShortcut:
				st.saveMenu.selectAll = false
				st.saveMenu.cursor = len(st.saveMenu.filename)
			case ke.Name == "A" && ke.Modifiers == key.ModShortcut:
				st.saveMenu.selectAll = true
				st.saveMenu.cursor = len(st.saveMenu.filename)
			}
		} else {
			// Collapsed mode (file-backed, no Save As rows) — only Escape
			if ke.Name == key.NameEscape {
				st.saveMenu.visible = false
				st.quitInProgress = false
			}
		}
		return
	}

	if st.langSel.Visible {
		switch ke.Name {
		case key.NameEscape:
			st.langSel.Close()
		case key.NameUpArrow:
			st.langSel.MoveUp()
		case key.NameDownArrow:
			st.langSel.MoveDown()
		case key.NameReturn:
			lang := st.langSel.SelectedLanguage()
			st.langSel.Close()
			st.setLanguage(lang)
		}
		return
	}

	// Find bar intercept
	if st.findBar.Visible {
		switch {
		case ke.Name == key.NameEscape:
			st.findBar.Close()
		case ke.Name == key.NameReturn && ke.Modifiers == 0:
			st.findNextMatch()
		case ke.Name == key.NameReturn && ke.Modifiers == key.ModShift:
			st.findPrevMatch()
		case ke.Name == key.NameTab && ke.Modifiers == 0:
			st.findBar.SwitchFocus()
		case ke.Name == key.NameTab && ke.Modifiers == key.ModShift:
			st.findBar.SwitchFocus()
		case ke.Name == key.NameDeleteBackward:
			st.findBar.DeleteChar()
			if st.findBar.FocusField == 0 {
				st.updateSearchResults()
			}
		case ke.Name == key.NameDeleteForward:
			st.findBar.DeleteForwardChar()
			if st.findBar.FocusField == 0 {
				st.updateSearchResults()
			}
		case ke.Name == key.NameLeftArrow && ke.Modifiers == 0:
			st.findBar.MoveCursorLeft()
		case ke.Name == key.NameRightArrow && ke.Modifiers == 0:
			st.findBar.MoveCursorRight()
		case ke.Name == key.NameLeftArrow && ke.Modifiers == key.ModShortcut:
			st.findBar.MoveCursorToStart()
		case ke.Name == key.NameRightArrow && ke.Modifiers == key.ModShortcut:
			st.findBar.MoveCursorToEnd()
		case ke.Name == "A" && ke.Modifiers == key.ModShortcut:
			st.findBar.SelectAll()
		case ke.Name == "F" && ke.Modifiers == key.ModShortcut:
			// Re-open / refocus find bar
			st.openFindBar(false)
		case ke.Name == "H" && ke.Modifiers == key.ModShortcut:
			st.openFindBar(true)
		}
		return
	}

	ed := st.activeEd()
	if ed == nil {
		// Only handle new tab if no editor
		if ke.Name == "T" && ke.Modifiers == key.ModShortcut {
			st.newTab()
		}
		return
	}

	// In markdown read mode, handle mode toggle, tab management, and copy
	if ts := st.activeTabState(); ts != nil && ts.mode == viewMarkdownRead {
		switch {
		case ke.Name == "E" && ke.Modifiers == key.ModShortcut:
			st.toggleMarkdownPreview()
		case ke.Name == "T" && ke.Modifiers == key.ModShortcut:
			st.newTab()
		case ke.Name == "W" && ke.Modifiers == key.ModShortcut:
			st.closeCurrentTab()
		case ke.Name == "Q" && ke.Modifiers == key.ModShortcut:
			st.startQuitFlow()
		case ke.Name == "C" && ke.Modifiers == key.ModShortcut:
			// Copy selection or full document in read mode
			if ts.mdSelAnchor != ts.mdSelCursor {
				sel := mdSelectedText(ts.mdSelText, ts.mdSelAnchor, ts.mdSelCursor)
				clipboard.Set(sel)
				st.notification = "Copied to clipboard"
			} else if ed := st.activeEd(); ed != nil {
				clipboard.Set(string(ed.Buffer.TextBytes(nil)))
				st.notification = "Copied to clipboard"
			}
			st.notificationUntil = time.Now().Add(2 * time.Second)
			st.window.Invalidate()
		case ke.Name == "A" && ke.Modifiers == key.ModShortcut:
			// Select all text in read mode
			ts.mdSelAnchor = 0
			ts.mdSelCursor = len(ts.mdSelText)
			st.window.Invalidate()
		case ke.Name == "F" && ke.Modifiers == key.ModShortcut:
			st.openFindBar(false)
		}
		return
	}

	switch {
	// Tab management
	case ke.Name == "T" && ke.Modifiers == key.ModShortcut:
		st.newTab()
	case ke.Name == "W" && ke.Modifiers == key.ModShortcut:
		st.closeCurrentTab()
	case ke.Name == "Z" && ke.Modifiers == key.ModAlt:
		st.toggleWordWrap()

	case ke.Name == key.NameLeftArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveLeft(ed.Buffer)
	case ke.Name == key.NameRightArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveRight(ed.Buffer)
	case ke.Name == key.NameUpArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveUp(ed.Buffer)
		st.skipHiddenLines(ed, -1)
	case ke.Name == key.NameDownArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveDown(ed.Buffer)
		st.skipHiddenLines(ed, 1)
	case ke.Name == key.NameUpArrow && ke.Modifiers == key.ModShortcut:
		ed.Selection.Clear()
		ed.Cursor.MoveToFileStart()
	case ke.Name == key.NameDownArrow && ke.Modifiers == key.ModShortcut:
		ed.Selection.Clear()
		ed.Cursor.MoveToFileEnd(ed.Buffer)
	case ke.Name == key.NameHome:
		ed.Selection.Clear()
		ed.Cursor.MoveToLineStart()
	case ke.Name == key.NameEnd:
		ed.Selection.Clear()
		ed.Cursor.MoveToLineEnd(ed.Buffer)
	case ke.Name == key.NamePageDown:
		ed.Selection.Clear()
		ed.Cursor.PageDown(ed.Buffer, st.activeTabState().viewport.VisibleLines)
	case ke.Name == key.NamePageUp:
		ed.Selection.Clear()
		ed.Cursor.PageUp(ed.Buffer, st.activeTabState().viewport.VisibleLines)
	case ke.Name == key.NameDeleteBackward && ke.Modifiers == 0:
		if st.deleteAutoPair() {
			st.afterEdit()
		} else if st.softTabBackspace() {
			st.afterEdit()
		} else {
			ed.DeleteBackward()
			st.afterEdit()
		}
	case ke.Name == key.NameDeleteForward && ke.Modifiers == 0:
		ed.DeleteForward()
		st.afterEdit()
	case ke.Name == key.NameReturn && ke.Modifiers == 0:
		indent := st.computeAutoIndent()
		ed.InsertText("\n" + indent)
		st.afterEdit()
	case ke.Name == key.NameTab && ke.Modifiers == 0:
		ed.InsertText("    ")
		st.afterEdit()
	case ke.Name == "Z" && ke.Modifiers == key.ModShortcut:
		ed.Undo()
		st.afterEdit()
	case ke.Name == "Z" && ke.Modifiers == key.ModShortcut|key.ModShift:
		ed.Redo()
		st.afterEdit()
	case ke.Name == "S" && ke.Modifiers == key.ModShortcut:
		tab := st.tabBar.ActiveTab()
		if tab != nil {
			if tab.Editor.FilePath == "" {
				st.showSaveAsMenu(st.tabBar.ActiveIdx, false, false)
			} else {
				st.saveTab(tab)
				st.updateWindowTitle()
			}
		}
	case ke.Name == "E" && ke.Modifiers == key.ModShortcut:
		st.toggleMarkdownPreview()
	case ke.Name == "S" && ke.Modifiers == key.ModShortcut|key.ModShift:
		// Cmd+Shift+S = Save As
		if st.tabBar.ActiveIdx >= 0 {
			st.showSaveAsMenu(st.tabBar.ActiveIdx, false, false)
		}
	case ke.Name == "A" && ke.Modifiers == key.ModShortcut:
		ed.Selection.SelectAll(ed.Buffer)
		_, end := ed.Selection.Ordered()
		ed.Cursor = end
		ed.Cursor.PreferredCol = -1
	case ke.Name == "C" && ke.Modifiers == key.ModShortcut:
		if text := ed.SelectedText(); text != "" {
			clipboard.Set(text)
		}
	case ke.Name == "X" && ke.Modifiers == key.ModShortcut:
		if text := ed.SelectedText(); text != "" {
			clipboard.Set(text)
			ed.DeleteSelection()
			st.afterEdit()
		}
	case ke.Name == "V" && ke.Modifiers == key.ModShortcut:
		if text := clipboard.Get(); text != "" {
			ed.InsertText(text)
			st.afterEdit()
		}
	case ke.Name == "Q" && ke.Modifiers == key.ModShortcut:
		if !st.quitInProgress {
			st.startQuitFlow()
		}
	// Find / Replace
	case ke.Name == "F" && ke.Modifiers == key.ModShortcut:
		st.openFindBar(false)
	case ke.Name == "H" && ke.Modifiers == key.ModShortcut:
		st.openFindBar(true)

	// Selection via shift+arrows
	case ke.Name == key.NameLeftArrow && ke.Modifiers == key.ModShift:
		if !ed.Selection.Active {
			ed.Selection.Start(ed.Cursor)
		}
		ed.Cursor.MoveLeft(ed.Buffer)
		ed.Selection.Update(ed.Cursor)
	case ke.Name == key.NameRightArrow && ke.Modifiers == key.ModShift:
		if !ed.Selection.Active {
			ed.Selection.Start(ed.Cursor)
		}
		ed.Cursor.MoveRight(ed.Buffer)
		ed.Selection.Update(ed.Cursor)
	case ke.Name == key.NameUpArrow && ke.Modifiers == key.ModShift:
		if !ed.Selection.Active {
			ed.Selection.Start(ed.Cursor)
		}
		ed.Cursor.MoveUp(ed.Buffer)
		st.skipHiddenLines(ed, -1)
		ed.Selection.Update(ed.Cursor)
	case ke.Name == key.NameDownArrow && ke.Modifiers == key.ModShift:
		if !ed.Selection.Active {
			ed.Selection.Start(ed.Cursor)
		}
		ed.Cursor.MoveDown(ed.Buffer)
		st.skipHiddenLines(ed, 1)
		ed.Selection.Update(ed.Cursor)
	}
	if st.cursorRend != nil {
		st.cursorRend.ResetBlink()
	}
}

func (st *appState) handlePointer(pe pointer.Event) {
	st.hoverX = int(pe.Position.X)
	st.hoverY = int(pe.Position.Y)

	switch pe.Kind {
	case pointer.Move:
		// Check for incoming tab transfers when pointer is in the tab bar
		if int(pe.Position.Y) < st.tabBarHeight {
			st.checkIncomingTabTransfer()
		}
		// Invalidate for hover effects in markdown read mode
		if ts := st.activeTabState(); ts != nil && ts.mode == viewMarkdownRead {
			st.window.Invalidate()
		}

	case pointer.Press:
		// Save menu takes priority over everything
		if st.saveMenu.visible {
			st.handleSaveMenuClick(int(pe.Position.X), int(pe.Position.Y))
			return
		}

		// Check tab bar clicks first (or overflow dropdown which extends below)
		if int(pe.Position.Y) < st.tabBarHeight || st.overflowOpen {
			st.handleTabBarPress(int(pe.Position.X), int(pe.Position.Y))
			return
		}

		// Code block copy buttons and checkboxes in markdown read mode
		if ts := st.activeTabState(); ts != nil && ts.mode == viewMarkdownRead {
			px, py := int(pe.Position.X), int(pe.Position.Y)
			for _, btn := range ts.mdCopyBtns {
				if px >= btn.x && px < btn.x+btn.w && py >= btn.y && py < btn.y+btn.h {
					clipboard.Set(btn.code)
					st.notification = "Copied to clipboard"
					st.notificationUntil = time.Now().Add(2 * time.Second)
					st.window.Invalidate()
					return
				}
			}
			for _, cb := range ts.mdCheckboxes {
				if px >= cb.x && px < cb.x+cb.w && py >= cb.y && py < cb.y+cb.h {
					st.toggleCheckbox(cb)
					return
				}
			}
			// Start text selection
			absY := py - st.tabBarHeight + int(ts.mdScrollY)
			off := mdCharOffset(ts.mdSelBlocks, px, absY)
			ts.mdSelAnchor = off
			ts.mdSelCursor = off
			ts.mdSelActive = true
			st.window.Invalidate()
		}

		sr := st.statusRend
		statusH := 0
		if sr != nil {
			statusH = sr.LineHeightPx + 6
		}
		statusY := st.lastMaxY - statusH

		if st.langSel.Visible && sr != nil {
			itemH := sr.LineHeightPx + 4
			dropdownH := len(st.langSel.Languages) * itemH
			dropdownW := st.langDropdownWidth()
			dropdownX := st.lastMaxX - dropdownW - 4
			dropdownY := statusY - dropdownH
			if dropdownX < 0 {
				dropdownX = 0
			}
			px, py := int(pe.Position.X), int(pe.Position.Y)
			if px >= dropdownX && px <= dropdownX+dropdownW && py >= dropdownY && py < statusY {
				idx := st.langSel.LanguageAtY(py-dropdownY, itemH)
				if idx >= 0 {
					st.langSel.Selected = idx
					lang := st.langSel.SelectedLanguage()
					st.langSel.Close()
					st.setLanguage(lang)
				}
				return
			}
			st.langSel.Close()
			return
		}

		// Find bar clicks — consume click if inside the bar
		if st.findBar.Visible && st.tabRend != nil {
			if st.handleFindBarClick(int(pe.Position.X), int(pe.Position.Y)) {
				return
			}
		}

		// Markdown Edit/Read toggle button
		if st.mdToggleW > 0 && int(pe.Position.Y) >= statusY {
			px := int(pe.Position.X)
			if px >= st.mdToggleX && px < st.mdToggleX+st.mdToggleW {
				st.toggleMarkdownPreview()
				return
			}
		}

		if int(pe.Position.Y) >= statusY && int(pe.Position.X) >= st.langLabelX {
			st.langSel.Open(highlight.LanguageNames())
			return
		}

		ed := st.activeEd()
		if ed == nil {
			return
		}

		gutterWidth := st.gutterRend.Width(ed.Buffer.LineCount())
		if int(pe.Position.X) < gutterWidth {
			// Gutter click — toggle code fold
			st.handleGutterClick(pe)
			return
		}
		line, col := st.pointerToLineCol(pe.Position)

		ed.Selection.Clear()
		ed.Cursor.SetPosition(ed.Buffer, line, col)
		ed.Selection.Start(ed.Cursor)
		st.dragging = true
		st.cursorRend.ResetBlink()

	case pointer.Drag:
		// Tab drag takes priority over text selection drag
		if st.tabDrag.active {
			st.handleTabBarDrag(int(pe.Position.X), int(pe.Position.Y))
			return
		}
		// Markdown read mode drag selection
		if ts := st.activeTabState(); ts != nil && ts.mode == viewMarkdownRead && ts.mdSelActive {
			px, py := int(pe.Position.X), int(pe.Position.Y)
			absY := py - st.tabBarHeight + int(ts.mdScrollY)
			ts.mdSelCursor = mdCharOffset(ts.mdSelBlocks, px, absY)
			st.window.Invalidate()
			return
		}
		if !st.dragging {
			return
		}
		ed := st.activeEd()
		if ed == nil {
			return
		}
		line, col := st.pointerToLineCol(pe.Position)
		ed.Cursor.SetPosition(ed.Buffer, line, col)
		ed.Selection.Update(ed.Cursor)
		st.cursorRend.ResetBlink()

	case pointer.Release:
		if st.tabDrag.active {
			st.handleTabBarRelease(int(pe.Position.X), int(pe.Position.Y))
			return
		}
		// End markdown selection
		if ts := st.activeTabState(); ts != nil && ts.mode == viewMarkdownRead {
			ts.mdSelActive = false
		}
		if st.dragging {
			st.dragging = false
			if ed := st.activeEd(); ed != nil && ed.Selection.IsEmpty() {
				ed.Selection.Clear()
			}
		}

	case pointer.Scroll:
		if ts := st.activeTabState(); ts != nil && st.textRend != nil && st.textRend.LineHeightPx > 0 {
			st.scrollAccum += pe.Scroll.Y
			pixels := int(st.scrollAccum)
			if pixels != 0 {
				if ts.mode == viewMarkdownRead {
					ts.mdScrollY += float64(pixels)
					if ts.mdScrollY < 0 {
						ts.mdScrollY = 0
					}
					editorH := st.lastMaxY - st.tabBarHeight
					maxScroll := float64(ts.mdTotalH - editorH)
					if maxScroll < 0 {
						maxScroll = 0
					}
					if ts.mdScrollY > maxScroll {
						ts.mdScrollY = maxScroll
					}
					st.window.Invalidate()
				} else {
					ts.viewport.ScrollByPixels(pixels, st.textRend.LineHeightPx)
				}
				st.scrollAccum -= float32(pixels)
				if st.scrollbarRend != nil {
					st.scrollbarRend.NotifyScroll()
				}
			}
		}
	}
}

func (st *appState) pointerToLineCol(pos f32.Point) (line, col int) {
	ts := st.activeTabState()
	if ts == nil {
		return 0, 0
	}
	ed := st.activeEd()
	gutterWidth := st.gutterRend.Width(ed.Buffer.LineCount())
	dispCol := int(math.Floor(float64(int(pos.X)-gutterWidth-st.textRend.CharWidth) / st.textRend.CharAdvance))
	if dispCol < 0 {
		dispCol = 0
	}
	adjustedY := int(pos.Y) - st.tabBarHeight - editorTopPad

	if ts.wrapMap != nil {
		visLine := ts.viewport.FirstLine + adjustedY/st.textRend.LineHeightPx
		bufLine, segIdx := ts.wrapMap.bufferLineForVisual(visLine)
		segStart, _ := ts.wrapMap.segmentRange(bufLine, segIdx)
		col = dispCol + segStart
		line = bufLine
		return
	}

	displayLine := ts.viewport.FirstLine + adjustedY/st.textRend.LineHeightPx

	// Convert display line to buffer line when folds are active
	fs := ts.foldState
	if fs != nil && fs.HasCollapsed() {
		line = fs.DisplayToBuf(displayLine)
	} else {
		line = displayLine
	}
	col = dispCol
	return
}

// skipHiddenLines moves the cursor past any hidden (folded) lines.
// dir should be -1 (moving up) or +1 (moving down).
func (st *appState) skipHiddenLines(ed *editor.Editor, dir int) {
	ts := st.activeTabState()
	if ts == nil || ts.foldState == nil || !ts.foldState.HasCollapsed() {
		return
	}
	fs := ts.foldState
	maxLine := ed.Buffer.LineCount() - 1
	for fs.IsHidden(ed.Cursor.Line) {
		ed.Cursor.Line += dir
		if ed.Cursor.Line < 0 {
			ed.Cursor.Line = 0
			break
		}
		if ed.Cursor.Line > maxLine {
			ed.Cursor.Line = maxLine
			break
		}
	}
	ed.Cursor.PreferredCol = -1
}

// handleGutterClick toggles a code fold when a gutter line number is clicked.
func (st *appState) handleGutterClick(pe pointer.Event) {
	ts := st.activeTabState()
	ed := st.activeEd()
	if ts == nil || ed == nil || ts.foldState == nil {
		return
	}

	adjustedY := int(pe.Position.Y) - st.tabBarHeight - editorTopPad
	if st.textRend == nil || st.textRend.LineHeightPx == 0 {
		return
	}
	displayLine := ts.viewport.FirstLine + adjustedY/st.textRend.LineHeightPx

	fs := ts.foldState
	var bufLine int
	if fs.HasCollapsed() {
		bufLine = fs.DisplayToBuf(displayLine)
	} else {
		bufLine = displayLine
	}

	if !fs.IsFoldStart(bufLine) {
		return
	}

	// Ctrl/Cmd+click toggles recursively
	recursive := pe.Modifiers.Contain(key.ModShortcut)
	if recursive {
		fs.ToggleRecursive(bufLine, ed.Buffer.LineCount())
	} else {
		fs.Toggle(bufLine, ed.Buffer.LineCount())
	}
	st.window.Invalidate()
}

func (st *appState) handleTextInput(text string) {
	if st.saveMenu.visible && st.saveMenuShowSaveAs() {
		st.saveAsInsertText(text)
		return
	}
	if st.saveMenu.visible {
		return // collapsed mode ignores text input
	}
	if st.findBar.Visible {
		st.findBar.InsertChar(text)
		if st.findBar.FocusField == 0 {
			st.updateSearchResults()
		}
		return
	}

	ed := st.activeEd()
	if ed == nil {
		return
	}

	if closerSet[text] {
		next := ed.RuneAfterCursor()
		if string(next) == text {
			ed.Cursor.MoveRight(ed.Buffer)
			st.afterEdit()
			return
		}
	}

	if closer, ok := autoPairs[text]; ok {
		if text == `"` || text == "'" || text == "`" {
			next := ed.RuneAfterCursor()
			if next != 0 && next != ' ' && next != '\t' && next != '\n' &&
				next != ')' && next != ']' && next != '}' && next != ',' && next != ';' {
				ed.InsertText(text)
				st.afterEdit()
				return
			}
		}
		ed.InsertText(text + closer)
		ed.Cursor.MoveLeft(ed.Buffer)
		st.afterEdit()
		return
	}

	ed.InsertText(text)
	if text == "}" || text == ")" || text == "]" {
		st.autoDedentClosingBracket()
	}
	st.afterEdit()
}
