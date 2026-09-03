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
	"github.com/kristianweb/zephyr/internal/vim"
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
			pointer.Filter{Target: st.tag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Scroll | pointer.Move | pointer.Enter | pointer.Leave | pointer.Cancel, ScrollY: scrollRange},
		)
		if !ok {
			break
		}
		st.notePerformanceInput(ev)
		switch ke := ev.(type) {
		case key.Event:
			if ke.State == key.Press {
				// Check vim toggle (Cmd+Shift+V) — works in all modes
				if ke.Name == "V" && ke.Modifiers == key.ModShortcut|key.ModShift {
					st.toggleVimMode()
					break
				}
				// Check navigator toggle (Cmd+Shift+N / Ctrl+Shift+N)
				if ke.Name == "N" && ke.Modifiers == key.ModShortcut|key.ModShift {
					st.toggleNavigatorMode()
					break
				}

				st.dispatchKey(ke)
				st.traceKeyEvent(ke)
			}
		case key.EditEvent:
			st.handleEditEvent(ke.Text)
			st.traceEditEvent(ke.Text)
		case pointer.Event:
			st.handlePointer(ke)
		}
	}
}

// handleEditEvent routes committed text input to whichever surface owns the
// keyboard: an open overlay first, then vim insert mode, then the buffer.
func (st *appState) handleEditEvent(text string) {
	if st.langSel.Visible {
		return
	}
	if st.fuzzyFinder != nil && st.fuzzyFinder.Visible {
		st.fuzzyFinderInsert(text)
		return
	}
	if st.handleConflictText(text) {
		return
	}
	if st.vimEnabled && st.vimState != nil &&
		!st.saveMenu.visible && !st.findBarHasKeys() {
		st.handleVimEditEvent(text)
		return
	}
	st.handleTextInput(text)
}

func (st *appState) handleKey(ke key.Event) {
	// The read-only guard runs ahead of every other route, minus the one
	// exception it needs: while an overlay owns the keyboard its Return and
	// Backspace belong to the overlay, not to this buffer. Stating the
	// exception here rather than placing the call after the overlay blocks
	// keeps a newly inserted block from silently moving the guard.
	if !st.overlayOwnsKeys() && st.headViewSwallowsKey(ke) {
		return
	}

	// Unified save menu intercepts all input. Each confirmation sub-state gets
	// its own handler so a new one adds a case here rather than another branch
	// inside the filename editor.
	if st.saveMenu.visible {
		if st.saveMenu.confirmClobber {
			st.handleClobberKey(ke)
			return
		}
		if st.saveMenu.confirmOverwrite {
			st.handleOverwriteKey(ke)
			return
		}
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

	if st.handleFuzzyFinderKey(ke) {
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

	// Find bar intercept — only while the bar owns the keyboard. An unfocused
	// bar stays on screen and its remaining keys are handled in findfocus.go.
	if st.findBarHasKeys() {
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
		st.insertNewlineAutoIndent()
		st.afterEdit()
		st.detectErrors() // refresh error markers on every Enter
	case ke.Name == key.NameTab && ke.Modifiers == 0:
		ed.InsertText("    ")
		st.afterEdit()
	case ke.Name == "Z" && ke.Modifiers == key.ModShortcut:
		st.undoSteps(1)
	case ke.Name == "Z" && ke.Modifiers == key.ModShortcut|key.ModShift:
		st.redoSteps(1)
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
	defer st.tracePointer(pe)
	st.hoverX = int(pe.Position.X)
	st.hoverY = int(pe.Position.Y)

	// pe.Buttons is the set of buttons held *after* the event, so which button
	// this event pressed or released is only visible against the previous set.
	heldButtons := st.pointerButtons
	if pe.Source == pointer.Mouse {
		st.pointerButtons = pe.Buttons
	}

	switch pe.Kind {
	case pointer.Move, pointer.Enter:
		// Check for incoming tab transfers when pointer is in the tab bar
		if int(pe.Position.Y) < st.tabBarHeight {
			st.checkIncomingTabTransfer()
		}
		// Invalidate for hover effects in markdown read mode
		if ts := st.activeTabState(); ts != nil && ts.mode == viewMarkdownRead {
			st.window.Invalidate()
		}

	case pointer.Press:
		// Secondary and tertiary mouse buttons must not trigger primary actions,
		// including when one goes down on top of a primary drag already running.
		if !isPrimaryPointerPress(pe, heldButtons) {
			return
		}
		st.activePointer = pe.PointerID
		st.pointerActive = true

		// Save menu takes priority over everything
		if st.saveMenu.visible {
			st.handleSaveMenuClick(int(pe.Position.X), int(pe.Position.Y))
			return
		}

		// Fuzzy finder overlay takes priority while it is open
		if st.handleFuzzyFinderClick(int(pe.Position.X), int(pe.Position.Y)) {
			return
		}

		// Navigator root dropdown takes priority when open
		if st.navRootDropdown.open {
			st.handleNavRootDropdownClick(int(pe.Position.X), int(pe.Position.Y))
			return
		}

		// Check tab bar / breadcrumb clicks
		if int(pe.Position.Y) < st.tabBarHeight || st.overflowOpen {
			if st.navigatorActive {
				x := int(pe.Position.X)
				// Check theme toggle icon
				if st.lastMaxX > 0 {
					toggleX := st.themeToggleX(st.lastMaxX)
					_, hitW := st.themeToggleSize()
					if x >= toggleX && x < toggleX+hitW {
						st.toggleTheme()
						return
					}
				}
				// Folder name → toggle root dropdown
				if x >= st.navRootDropdown.x && x < st.navRootDropdown.x+st.navRootDropdown.w {
					if st.navRootDropdown.open {
						st.navRootDropdown.open = false
					} else {
						st.openNavRootDropdown()
					}
					return
				}
				// Click on empty breadcrumb space → window drag
				startWindowDrag()
				return
			}
			st.handleTabBarPress(int(pe.Position.X), int(pe.Position.Y))
			return
		}

		// Code block copy buttons and checkboxes in markdown read mode.
		// General text selection is handled after overlays and status controls.
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
		}

		sr := st.statusRend
		statusH := 0
		if sr != nil {
			statusH = sr.LineHeightPx + 6
		}
		statusY := st.lastMaxY - statusH

		// Click on "Vim" indicator opens tutor
		if st.vimEnabled && st.vimIndicatorW > 0 {
			px, py := int(pe.Position.X), int(pe.Position.Y)
			if py >= statusY && px >= st.vimIndicatorX && px < st.vimIndicatorX+st.vimIndicatorW {
				st.openVimTutor()
				return
			}
		}

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

		// JSON Compact/Expanded toggle button
		if st.fmtToggleW > 0 && int(pe.Position.Y) >= statusY {
			px := int(pe.Position.X)
			if px >= st.fmtToggleX && px < st.fmtToggleX+st.fmtToggleW {
				st.toggleJSONCompact()
				return
			}
		}

		if int(pe.Position.Y) >= statusY && int(pe.Position.X) >= st.langLabelX {
			st.langSel.Open(highlight.LanguageNames())
			return
		}

		// Markdown read-mode selection consumes editor-area presses without
		// moving the hidden edit-mode cursor.
		if ts := st.activeTabState(); ts != nil && ts.mode == viewMarkdownRead {
			st.blurFindBarForEditorPress()
			px, py := int(pe.Position.X), int(pe.Position.Y)
			absY := py - st.tabBarHeight + int(ts.mdScrollY)
			off := mdCharOffset(ts.mdSelBlocks, px, absY)
			ts.mdSelAnchor = off
			ts.mdSelCursor = off
			ts.mdSelActive = true
			st.window.Invalidate()
			return
		}

		ed := st.activeEd()
		if ed == nil {
			return
		}
		st.blurFindBarForEditorPress()

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
		if !st.pointerActive || pe.PointerID != st.activePointer {
			return
		}
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
		if st.pointerActive && pe.PointerID != st.activePointer {
			return
		}
		// A secondary or tertiary button going up mid-gesture must not end the
		// primary drag it was layered on top of.
		if !isPrimaryPointerRelease(pe, heldButtons) {
			return
		}
		st.pointerActive = false
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

	case pointer.Leave:
		st.tooltipTabIdx = -1
		st.tooltipEnter = time.Time{}
		if st.window != nil {
			st.window.Invalidate()
		}

	case pointer.Cancel:
		st.cancelPointerGesture()
	}
}

// isPrimaryPointerPress reports whether pe pressed the primary mouse button,
// given the buttons held before it. Gio reports Buttons as the set held after
// the event (gioui.org/io/pointer Event.Buttons: "the set of pressed mouse
// buttons for this event"; the macOS backend ORs the button in on MOUSE_DOWN
// before dispatching), so the button this event pressed is what it adds to
// held. Non-mouse sources report no buttons and always count as primary.
func isPrimaryPointerPress(pe pointer.Event, held pointer.Buttons) bool {
	if pe.Source != pointer.Mouse {
		return true
	}
	return (pe.Buttons &^ held).Contain(pointer.ButtonPrimary)
}

// isPrimaryPointerRelease reports whether pe released the primary mouse button:
// the backend clears the released button from the set before dispatching, so a
// secondary release during a primary drag arrives with Buttons still holding
// ButtonPrimary and must leave the gesture running.
func isPrimaryPointerRelease(pe pointer.Event, held pointer.Buttons) bool {
	if pe.Source != pointer.Mouse {
		return true
	}
	return (held &^ pe.Buttons).Contain(pointer.ButtonPrimary)
}

func (st *appState) cancelPointerGesture() {
	st.pointerActive = false
	st.dragging = false
	st.tabDrag.active = false
	st.tabDrag.started = false
	st.tabDrag.fromDropdown = false
	if ts := st.activeTabState(); ts != nil {
		ts.mdSelActive = false
	}
	if st.window != nil {
		st.window.Invalidate()
	}
}

func visualLineAtY(firstLine, pixelOffset, adjustedY, lineHeight int) int {
	if lineHeight <= 0 {
		return firstLine
	}
	value := adjustedY + pixelOffset
	lineDelta := value / lineHeight
	if value < 0 && value%lineHeight != 0 {
		lineDelta--
	}
	return firstLine + lineDelta
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
	visLine := visualLineAtY(ts.viewport.FirstLine, ts.viewport.PixelOffset, adjustedY, st.textRend.LineHeightPx)

	if ts.wrapMap != nil {
		bufLine, segIdx := ts.wrapMap.bufferLineForVisual(visLine)
		segStart, _ := ts.wrapMap.segmentRange(bufLine, segIdx)
		lineText, _ := ed.Buffer.Line(bufLine)
		col = displayColToRuneCol(lineText, dispCol+segStart, 4)
		line = bufLine
		return
	}

	displayLine := visLine

	// Convert display line to buffer line when folds are active
	fs := ts.foldState
	if fs != nil && fs.HasCollapsed() {
		line = fs.DisplayToBuf(displayLine)
	} else {
		line = displayLine
	}
	lineText, _ := ed.Buffer.Line(line)
	col = displayColToRuneCol(lineText, dispCol, 4)
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
	displayLine := visualLineAtY(ts.viewport.FirstLine, ts.viewport.PixelOffset, adjustedY, st.textRend.LineHeightPx)

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
	if st.saveMenu.visible && st.saveMenu.confirmClobber {
		return // the clobber prompt has no text field
	}
	if st.saveMenu.visible && st.saveMenuShowSaveAs() {
		st.saveAsInsertText(text)
		return
	}
	if st.saveMenu.visible {
		return // collapsed mode ignores text input
	}
	if st.findBarHasKeys() {
		st.findBar.InsertChar(text)
		if st.findBar.FocusField == 0 {
			st.updateSearchResults()
		}
		return
	}
	if st.fuzzyFinder != nil && st.fuzzyFinder.Visible {
		st.fuzzyFinderInsert(text)
		return
	}
	if st.headViewActive() {
		return // HEAD content is read-only
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

// handleVimKeyEvent routes key.Event through the vim state machine.
func (st *appState) handleVimKeyEvent(ke key.Event) {
	// In Insert mode, only intercept Escape and Ctrl+[ to return to Normal
	if st.vimState.Mode == vim.ModeInsert {
		if ke.Name == key.NameEscape {
			action := st.vimState.HandleKey(vim.KeyInput{Name: vim.NameEscape})
			st.executeVimAction(action)
			return
		}
		// Ctrl+C also exits insert mode
		if ke.Name == "C" && ke.Modifiers&key.ModCtrl != 0 {
			action := st.vimState.HandleKey(vim.KeyInput{Char: 'c', Ctrl: true})
			st.executeVimAction(action)
			return
		}
		// All other keys fall through to normal editing
		st.handleKey(ke)
		return
	}

	// In markdown read mode, handle limited vim keys for reading
	if ts := st.activeTabState(); ts != nil && ts.mode == viewMarkdownRead {
		ev := gioKeyToVimInput(ke)
		// Allow Ctrl keys through vim (Ctrl+d, Ctrl+u, Ctrl+f, Ctrl+b)
		if ev.Ctrl {
			if action := st.vimState.HandleKey(ev); action.Kind != vim.ActionNone {
				st.executeMdReadAction(action, ts)
				return
			}
		}
		// Allow system shortcuts (Cmd+C, Cmd+F, etc.) through normal handler
		if ke.Modifiers&key.ModShortcut != 0 {
			st.handleKey(ke)
			return
		}
		// Skip other key.Events for printable chars (handled via EditEvent)
		return
	}

	// Normal/Visual/Command/Search modes — convert to vim KeyInput
	ev := gioKeyToVimInput(ke)
	// Skip printable characters that will also arrive via key.EditEvent.
	// Processing them here would cause double-handling (key.Event fires
	// after key.EditEvent in Gio for some keys, and both carry the same character).
	// Named keys (Escape, Return, arrows, etc.) only come via key.Event.
	if ev.Name == "" && ev.Char != 0 && !ev.Ctrl && !ev.Alt && !ev.Shortcut {
		return
	}
	// Skip key.Events that produce no usable vim input (e.g., "Space", "Shift")
	// — these are handled via key.EditEvent instead.
	if ev.Name == "" && ev.Char == 0 && !ev.Ctrl && !ev.Alt && !ev.Shortcut {
		return
	}

	action := st.vimState.HandleKey(ev)
	// A key carrying the platform shortcut modifier that vim declines is a host
	// accelerator (Cmd+S, and off macOS every Ctrl combination vim has no
	// binding for), so hand it to the normal key handler.
	if action.Kind == vim.ActionNone && ev.Shortcut {
		st.handleKey(ke)
		return
	}
	st.executeVimAction(action)

	// Update visual selection if in visual mode
	if st.vimState.Mode == vim.ModeVisual || st.vimState.Mode == vim.ModeVisualLine ||
		st.vimState.Mode == vim.ModeVisualBlock {
		st.updateVimVisualSelection()
	}
}

// handleVimEditEvent routes text input (key.EditEvent) through vim.
func (st *appState) handleVimEditEvent(text string) {
	if st.vimState.Mode == vim.ModeInsert {
		// In insert mode, pass through to normal text editing
		st.handleTextInput(text)
		return
	}

	// In markdown read mode, process limited vim keys
	if ts := st.activeTabState(); ts != nil && ts.mode == viewMarkdownRead {
		// Allow command/search mode entry and processing
		if st.vimState.Mode == vim.ModeCommand || st.vimState.Mode == vim.ModeSearch {
			for _, r := range text {
				st.vimState.HandleKey(vim.KeyInput{Char: r})
			}
			st.window.Invalidate()
			return
		}
		// Normal mode — process through vim then filter to read-safe actions
		for _, r := range text {
			action := st.vimState.HandleKey(vim.KeyInput{Char: r})
			st.executeMdReadAction(action, ts)
		}
		return
	}

	// In Command/Search mode, characters are part of the command line
	if st.vimState.Mode == vim.ModeCommand || st.vimState.Mode == vim.ModeSearch {
		for _, r := range text {
			ev := vim.KeyInput{Char: r}
			st.vimState.HandleKey(ev)
		}
		st.window.Invalidate()
		return
	}

	// In Normal/Visual mode, characters are vim commands
	for _, r := range text {
		ev := vim.KeyInput{Char: r}
		action := st.vimState.HandleKey(ev)
		st.executeVimAction(action)
	}

	// Update visual selection
	if st.vimState.Mode == vim.ModeVisual || st.vimState.Mode == vim.ModeVisualLine ||
		st.vimState.Mode == vim.ModeVisualBlock {
		st.updateVimVisualSelection()
	}
}

// gioKeyToVimInput converts a Gio key.Event to a vim KeyInput.
func gioKeyToVimInput(ke key.Event) vim.KeyInput {
	ev := vim.KeyInput{
		Ctrl:     ke.Modifiers&key.ModCtrl != 0,
		Shift:    ke.Modifiers&key.ModShift != 0,
		Alt:      ke.Modifiers&key.ModAlt != 0,
		Shortcut: ke.Modifiers&key.ModShortcut != 0,
	}

	// Map Gio named keys to vim named keys
	switch ke.Name {
	case key.NameEscape:
		ev.Name = vim.NameEscape
	case key.NameReturn:
		ev.Name = vim.NameReturn
	case key.NameDeleteBackward:
		ev.Name = vim.NameDeleteBackward
	case key.NameDeleteForward:
		ev.Name = vim.NameDeleteForward
	case key.NameUpArrow:
		ev.Name = vim.NameUpArrow
	case key.NameDownArrow:
		ev.Name = vim.NameDownArrow
	case key.NameLeftArrow:
		ev.Name = vim.NameLeftArrow
	case key.NameRightArrow:
		ev.Name = vim.NameRightArrow
	case key.NameHome:
		ev.Name = vim.NameHome
	case key.NameEnd:
		ev.Name = vim.NameEnd
	case key.NamePageUp:
		ev.Name = vim.NamePageUp
	case key.NamePageDown:
		ev.Name = vim.NamePageDown
	case key.NameTab:
		ev.Name = vim.NameTab
	default:
		// Letter keys come as uppercase single chars in Gio (e.g., "A", "B")
		name := string(ke.Name)
		if len(name) == 1 {
			ch := rune(name[0])
			if ch >= 'A' && ch <= 'Z' {
				if ev.Shift {
					ev.Char = ch // uppercase
				} else {
					ev.Char = ch + 32 // lowercase
				}
			} else if ch == ' ' {
				ev.Char = ' '
			}
		}
	}

	return ev
}

// updateVimVisualSelection updates the editor's selection to match visual mode state.
func (st *appState) updateVimVisualSelection() {
	ed := st.activeEd()
	if ed == nil || st.vimState == nil {
		return
	}

	anchor := editor.Cursor{
		Line: st.vimState.VisualAnchorLine,
		Col:  st.vimState.VisualAnchorCol,
	}

	switch st.vimState.Mode {
	case vim.ModeVisual:
		ed.Selection.Anchor = anchor
		ed.Selection.Head = ed.Cursor
		ed.Selection.Active = true
	case vim.ModeVisualLine:
		startLine := anchor.Line
		endLine := ed.Cursor.Line
		if startLine > endLine {
			startLine, endLine = endLine, startLine
		}
		ed.Selection.Anchor = editor.Cursor{Line: startLine, Col: 0}
		endLineText, _ := ed.Buffer.Line(endLine)
		ed.Selection.Head = editor.Cursor{Line: endLine, Col: len([]rune(endLineText))}
		ed.Selection.Active = true
	case vim.ModeVisualBlock:
		// Block mode: simple selection for now (true block mode requires multi-cursor)
		ed.Selection.Anchor = anchor
		ed.Selection.Head = ed.Cursor
		ed.Selection.Active = true
	}
}
