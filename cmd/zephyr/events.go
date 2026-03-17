package main

import (
	"math"
	"os"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"

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
		up, down := ts.viewport.ScrollablePixels(st.textRend.LineHeightPx)
		scrollRange = pointer.ScrollRange{Min: -up, Max: down}
	}

	for {
		ev, ok := gtx.Source.Event(
			key.FocusFilter{Target: st.tag},
			key.Filter{Focus: st.tag, Optional: key.ModShortcut | key.ModShift},
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

	switch {
	// Tab management
	case ke.Name == "T" && ke.Modifiers == key.ModShortcut:
		st.newTab()
	case ke.Name == "W" && ke.Modifiers == key.ModShortcut:
		st.closeCurrentTab()

	case ke.Name == key.NameLeftArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveLeft(ed.Buffer)
	case ke.Name == key.NameRightArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveRight(ed.Buffer)
	case ke.Name == key.NameUpArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveUp(ed.Buffer)
	case ke.Name == key.NameDownArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveDown(ed.Buffer)
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
			st.reparseHighlight()
		} else if st.softTabBackspace() {
			st.reparseHighlight()
		} else {
			ed.DeleteBackward()
			st.reparseHighlight()
		}
	case ke.Name == key.NameDeleteForward && ke.Modifiers == 0:
		ed.DeleteForward()
		st.reparseHighlight()
	case ke.Name == key.NameReturn && ke.Modifiers == 0:
		indent := st.computeAutoIndent()
		ed.InsertText("\n" + indent)
		st.reparseHighlight()
	case ke.Name == key.NameTab && ke.Modifiers == 0:
		ed.InsertText("    ")
		st.reparseHighlight()
	case ke.Name == "Z" && ke.Modifiers == key.ModShortcut:
		ed.Undo()
		st.reparseHighlight()
	case ke.Name == "Z" && ke.Modifiers == key.ModShortcut|key.ModShift:
		ed.Redo()
		st.reparseHighlight()
	case ke.Name == "S" && ke.Modifiers == key.ModShortcut:
		tab := st.tabBar.ActiveTab()
		if tab != nil {
			if tab.Editor.FilePath == "" {
				go func() {
					st.saveTabAs(tab)
					st.updateWindowTitle()
					st.window.Invalidate()
				}()
			} else {
				st.saveTab(tab)
				st.updateWindowTitle()
			}
		}
	case ke.Name == "S" && ke.Modifiers == key.ModShortcut|key.ModShift:
		// Cmd+Shift+S = Save As
		tab := st.tabBar.ActiveTab()
		if tab != nil {
			go func() {
				st.saveTabAs(tab)
				st.updateWindowTitle()
				st.window.Invalidate()
			}()
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
			st.reparseHighlight()
		}
	case ke.Name == "V" && ke.Modifiers == key.ModShortcut:
		if text := clipboard.Get(); text != "" {
			ed.InsertText(text)
			st.reparseHighlight()
		}
	case ke.Name == "Q" && ke.Modifiers == key.ModShortcut:
		if !st.quitInProgress {
			st.quitInProgress = true
			go func() {
				if st.saveAllBeforeQuit() {
					os.Exit(0)
				}
				st.quitInProgress = false
				st.window.Invalidate()
			}()
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
		ed.Selection.Update(ed.Cursor)
	case ke.Name == key.NameDownArrow && ke.Modifiers == key.ModShift:
		if !ed.Selection.Active {
			ed.Selection.Start(ed.Cursor)
		}
		ed.Cursor.MoveDown(ed.Buffer)
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
	case pointer.Press:
		// Check tab bar clicks first
		if int(pe.Position.Y) < st.tabBarHeight {
			st.handleTabBarClick(int(pe.Position.X))
			return
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

		if int(pe.Position.Y) >= statusY && int(pe.Position.X) >= st.langLabelX {
			st.langSel.Open(highlight.LanguageNames())
			return
		}

		ed := st.activeEd()
		if ed == nil {
			return
		}
		ts := st.activeTabState()

		gutterWidth := st.gutterRend.Width(ts.viewport.TotalLines)
		if int(pe.Position.X) < gutterWidth {
			return
		}
		line, col := st.pointerToLineCol(pe.Position)

		ed.Selection.Clear()
		ed.Cursor.SetPosition(ed.Buffer, line, col)
		ed.Selection.Start(ed.Cursor)
		st.dragging = true
		st.cursorRend.ResetBlink()

	case pointer.Drag:
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
				ts.viewport.ScrollByPixels(pixels, st.textRend.LineHeightPx)
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
	gutterWidth := st.gutterRend.Width(ts.viewport.TotalLines)
	col = int(math.Floor(float64(int(pos.X)-gutterWidth-st.textRend.CharWidth) / st.textRend.CharAdvance))
	if col < 0 {
		col = 0
	}
	adjustedY := int(pos.Y) - st.tabBarHeight - editorTopPad
	line = ts.viewport.FirstLine + adjustedY/st.textRend.LineHeightPx
	return
}

func (st *appState) handleTextInput(text string) {
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
			st.reparseHighlight()
			return
		}
	}

	if closer, ok := autoPairs[text]; ok {
		if text == `"` || text == "'" || text == "`" {
			next := ed.RuneAfterCursor()
			if next != 0 && next != ' ' && next != '\t' && next != '\n' &&
				next != ')' && next != ']' && next != '}' && next != ',' && next != ';' {
				ed.InsertText(text)
				st.reparseHighlight()
				return
			}
		}
		ed.InsertText(text + closer)
		ed.Cursor.MoveLeft(ed.Buffer)
		st.reparseHighlight()
		return
	}

	ed.InsertText(text)
	if text == "}" || text == ")" || text == "]" {
		st.autoDedentClosingBracket()
	}
	st.reparseHighlight()
}
