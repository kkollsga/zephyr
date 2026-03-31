package main

import (
	"image/color"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/vim"
	"github.com/kristianweb/zephyr/pkg/clipboard"
)

// Xbox green — the vim mode accent color.
var vimGreen = color.NRGBA{R: 0x10, G: 0x7C, B: 0x10, A: 255}

// executeVimAction translates a vim Action into editor operations.
func (st *appState) executeVimAction(action vim.Action) {
	// Navigator actions can work even without an active editor
	// (e.g., <Space>e to open root directory when no file is open)
	if st.executeNavAction(action) {
		return
	}

	ed := st.activeEd()
	if ed == nil {
		return
	}

	count := action.EffectiveCount()

	switch action.Kind {
	case vim.ActionNone:
		// Check for special visual mode operations
		if action.Text == "swap_anchor" && st.vimState != nil {
			st.vimSwapVisualAnchorHelper(ed)
		}
		return

	// --- Movement ---
	case vim.ActionMoveLeft:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			ed.Cursor.MoveLeft(ed.Buffer)
		}
	case vim.ActionMoveRight:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			ed.Cursor.MoveRight(ed.Buffer)
		}
	case vim.ActionMoveDown:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			ed.Cursor.MoveDown(ed.Buffer)
			st.skipHiddenLines(ed, 1)
		}
	case vim.ActionMoveUp:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			ed.Cursor.MoveUp(ed.Buffer)
			st.skipHiddenLines(ed, -1)
		}
	case vim.ActionMoveWordForward:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimMoveWordForward(ed)
		}
	case vim.ActionMoveWordBackward:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimMoveWordBackward(ed)
		}
	case vim.ActionMoveWordEnd:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimMoveWordEnd(ed)
		}
	case vim.ActionMoveBigWordFwd:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimMoveBigWordForward(ed)
		}
	case vim.ActionMoveBigWordBack:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimMoveBigWordBackward(ed)
		}
	case vim.ActionMoveBigWordEnd:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimMoveBigWordEnd(ed)
		}
	case vim.ActionMoveLineStart:
		ed.Selection.Clear()
		ed.Cursor.MoveToLineStart()
	case vim.ActionMoveLineEnd:
		ed.Selection.Clear()
		ed.Cursor.MoveToLineEnd(ed.Buffer)
	case vim.ActionMoveFirstNonBlank:
		ed.Selection.Clear()
		vimMoveFirstNonBlank(ed)
	case vim.ActionMoveFileStart:
		ed.Selection.Clear()
		ed.Cursor.MoveToFileStart()
	case vim.ActionMoveFileEnd:
		ed.Selection.Clear()
		ed.Cursor.MoveToFileEnd(ed.Buffer)
	case vim.ActionMoveToLine:
		ed.Selection.Clear()
		line := action.Line - 1
		if line < 0 {
			line = 0
		}
		if line >= ed.Buffer.LineCount() {
			line = ed.Buffer.LineCount() - 1
		}
		ed.Cursor.SetPosition(ed.Buffer, line, 0)
		vimMoveFirstNonBlank(ed)
	case vim.ActionMoveHalfPageDown:
		ed.Selection.Clear()
		ts := st.activeTabState()
		if ts != nil {
			half := ts.viewport.VisibleLines / 2
			if half < 1 {
				half = 1
			}
			for i := 0; i < count; i++ {
				ed.Cursor.PageDown(ed.Buffer, half)
			}
		}
	case vim.ActionMoveHalfPageUp:
		ed.Selection.Clear()
		ts := st.activeTabState()
		if ts != nil {
			half := ts.viewport.VisibleLines / 2
			if half < 1 {
				half = 1
			}
			for i := 0; i < count; i++ {
				ed.Cursor.PageUp(ed.Buffer, half)
			}
		}
	case vim.ActionMovePageDown:
		ed.Selection.Clear()
		ts := st.activeTabState()
		if ts != nil {
			for i := 0; i < count; i++ {
				ed.Cursor.PageDown(ed.Buffer, ts.viewport.VisibleLines)
			}
		}
	case vim.ActionMovePageUp:
		ed.Selection.Clear()
		ts := st.activeTabState()
		if ts != nil {
			for i := 0; i < count; i++ {
				ed.Cursor.PageUp(ed.Buffer, ts.viewport.VisibleLines)
			}
		}
	case vim.ActionMoveParagraphDown:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimMoveParagraph(ed, 1)
		}
	case vim.ActionMoveParagraphUp:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimMoveParagraph(ed, -1)
		}
	case vim.ActionMoveBracketMatch:
		ed.Selection.Clear()
		vimMatchBracket(ed)
	case vim.ActionMoveFindChar:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimFindChar(ed, action.Char, true, false)
		}
	case vim.ActionMoveTillChar:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimFindChar(ed, action.Char, true, true)
		}
	case vim.ActionMoveFindCharBack:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimFindChar(ed, action.Char, false, false)
		}
	case vim.ActionMoveTillCharBack:
		ed.Selection.Clear()
		for i := 0; i < count; i++ {
			vimFindChar(ed, action.Char, false, true)
		}

	// --- Scrolling ---
	case vim.ActionScrollCenter:
		ts := st.activeTabState()
		if ts != nil {
			halfVis := ts.viewport.VisibleLines / 2
			ts.viewport.FirstLine = ed.Cursor.Line - halfVis
			if ts.viewport.FirstLine < 0 {
				ts.viewport.FirstLine = 0
			}
			ts.viewport.PixelOffset = 0
		}
	case vim.ActionScrollTop:
		ts := st.activeTabState()
		if ts != nil {
			ts.viewport.FirstLine = ed.Cursor.Line
			ts.viewport.PixelOffset = 0
		}
	case vim.ActionScrollBottom:
		ts := st.activeTabState()
		if ts != nil {
			ts.viewport.FirstLine = ed.Cursor.Line - ts.viewport.VisibleLines + 1
			if ts.viewport.FirstLine < 0 {
				ts.viewport.FirstLine = 0
			}
			ts.viewport.PixelOffset = 0
		}

	// --- Insert mode transitions ---
	case vim.ActionInsertBefore:
		st.vimState.Mode = vim.ModeInsert
	case vim.ActionInsertAfter:
		ed.Cursor.MoveRight(ed.Buffer)
		st.vimState.Mode = vim.ModeInsert
	case vim.ActionInsertLineStart:
		vimMoveFirstNonBlank(ed)
		st.vimState.Mode = vim.ModeInsert
	case vim.ActionInsertLineEnd:
		ed.Cursor.MoveToLineEnd(ed.Buffer)
		st.vimState.Mode = vim.ModeInsert
	case vim.ActionOpenBelow:
		ed.Cursor.MoveToLineEnd(ed.Buffer)
		indent := st.computeAutoIndent()
		ed.InsertText("\n" + indent)
		st.afterEdit()
		st.vimState.Mode = vim.ModeInsert
	case vim.ActionOpenAbove:
		line := ed.Cursor.Line
		if line == 0 {
			ed.Cursor.MoveToLineStart()
			ed.InsertText("\n")
			ed.Cursor.SetPosition(ed.Buffer, 0, 0)
		} else {
			ed.Cursor.SetPosition(ed.Buffer, line-1, 0)
			ed.Cursor.MoveToLineEnd(ed.Buffer)
			indent := st.computeAutoIndent()
			ed.InsertText("\n" + indent)
		}
		st.afterEdit()
		st.vimState.Mode = vim.ModeInsert
	case vim.ActionSubstChar:
		// s = delete char + enter insert
		for i := 0; i < count; i++ {
			ed.DeleteForward()
		}
		st.afterEdit()
		st.vimState.Mode = vim.ModeInsert
	case vim.ActionSubstLine:
		// S = delete line content + enter insert
		vimDeleteLineContent(ed)
		st.afterEdit()
		st.vimState.Mode = vim.ModeInsert
	case vim.ActionEnterNormal:
		// Move cursor left by one (vim convention when leaving insert)
		if ed.Cursor.Col > 0 {
			ed.Cursor.MoveLeft(ed.Buffer)
		}

	// --- Editing ---
	case vim.ActionDelete:
		st.vimExecuteOperator(ed, action, vim.OpDelete)
	case vim.ActionChange:
		st.vimExecuteOperator(ed, action, vim.OpChange)
	case vim.ActionYank:
		st.vimExecuteOperator(ed, action, vim.OpYank)
	case vim.ActionIndent:
		st.vimExecuteIndent(ed, action, true)
	case vim.ActionDedent:
		st.vimExecuteIndent(ed, action, false)

	case vim.ActionPut:
		st.vimPut(ed, action, false)
	case vim.ActionPutBefore:
		st.vimPut(ed, action, true)

	case vim.ActionReplace:
		for i := 0; i < count; i++ {
			offset := ed.Buffer.LineColToOffsetSafe(ed.Cursor.Line, ed.Cursor.Col+i)
			if offset < ed.Buffer.Length() {
				ed.Buffer.Delete(offset, 1)
				ed.Buffer.Insert(offset, string(action.Char))
			}
		}
		ed.Modified = true
		st.afterEdit()

	case vim.ActionJoinLines:
		for i := 0; i < count; i++ {
			vimJoinLines(ed)
		}
		st.afterEdit()

	case vim.ActionUndo:
		for i := 0; i < count; i++ {
			ed.Undo()
		}
		st.afterEdit()
	case vim.ActionRedo:
		for i := 0; i < count; i++ {
			ed.Redo()
		}
		st.afterEdit()

	case vim.ActionRepeatLast:
		if st.vimState.LastAction.Kind != vim.ActionNone {
			last := st.vimState.LastAction
			if action.Count > 0 {
				last.Count = action.Count
			}
			st.executeVimAction(last)
		}
		return // don't record as last action

	// --- Visual mode ---
	case vim.ActionVisualStart:
		if st.vimState.Mode != vim.ModeVisual {
			st.vimState.Mode = vim.ModeVisual
		}
		st.vimState.VisualAnchorLine = ed.Cursor.Line
		st.vimState.VisualAnchorCol = ed.Cursor.Col
		ed.Selection.Start(ed.Cursor)
	case vim.ActionVisualLineStart:
		if st.vimState.Mode != vim.ModeVisualLine {
			st.vimState.Mode = vim.ModeVisualLine
		}
		st.vimState.VisualAnchorLine = ed.Cursor.Line
		st.vimState.VisualAnchorCol = ed.Cursor.Col
		ed.Selection.SelectLine(ed.Buffer, ed.Cursor.Line)
	case vim.ActionVisualBlockStart:
		if st.vimState.Mode != vim.ModeVisualBlock {
			st.vimState.Mode = vim.ModeVisualBlock
		}
		st.vimState.VisualAnchorLine = ed.Cursor.Line
		st.vimState.VisualAnchorCol = ed.Cursor.Col
		ed.Selection.Start(ed.Cursor)
	case vim.ActionVisualEscape:
		ed.Selection.Clear()

	// --- Command/Search ---
	case vim.ActionEnterCommand, vim.ActionEnterSearch, vim.ActionEnterSearchBack:
		// Mode already set by the state machine; just invalidate for render
		st.window.Invalidate()
	case vim.ActionCancelCommand:
		st.window.Invalidate()
	case vim.ActionExecCommand:
		cmdAction := vim.ParseCommand(action.Text)
		if cmdAction.Kind != vim.ActionNone {
			st.executeVimAction(cmdAction)
		}
		return
	case vim.ActionSearchNext:
		st.vimSearchNext(ed, count)
	case vim.ActionSearchPrev:
		st.vimSearchPrev(ed, count)
	case vim.ActionSearchWordUnder:
		st.vimSearchWordUnderCursor(ed)

	// --- File operations ---
	case vim.ActionWrite:
		tab := st.tabBar.ActiveTab()
		if tab != nil {
			if tab.Editor.FilePath == "" {
				st.showSaveAsMenu(st.tabBar.ActiveIdx, false, false)
			} else {
				st.saveTab(tab)
				st.updateWindowTitle()
			}
		}
	case vim.ActionQuit:
		st.closeCurrentTab()
	case vim.ActionWriteQuit:
		tab := st.tabBar.ActiveTab()
		if tab != nil {
			if tab.Editor.FilePath == "" {
				st.showSaveAsMenu(st.tabBar.ActiveIdx, false, false)
			} else {
				st.saveTab(tab)
				st.closeCurrentTab()
			}
		}
	case vim.ActionForceQuit:
		// Force close without saving
		if st.tabBar.ActiveIdx >= 0 {
			ed.Modified = false
			st.closeCurrentTab()
		}
	case vim.ActionTutor:
		st.openVimTutor()
	}

	// Record repeatable actions for dot
	if isRepeatableAction(action.Kind) {
		st.vimState.LastAction = action
	}

	if st.cursorRend != nil {
		st.cursorRend.ResetBlink()
	}
	st.window.Invalidate()
}

// isRepeatableAction returns true for actions that can be repeated with dot.
func isRepeatableAction(kind vim.ActionKind) bool {
	switch kind {
	case vim.ActionDelete, vim.ActionChange, vim.ActionYank,
		vim.ActionPut, vim.ActionPutBefore, vim.ActionReplace,
		vim.ActionJoinLines, vim.ActionSubstChar, vim.ActionSubstLine,
		vim.ActionIndent, vim.ActionDedent,
		vim.ActionInsertBefore, vim.ActionInsertAfter,
		vim.ActionInsertLineStart, vim.ActionInsertLineEnd,
		vim.ActionOpenBelow, vim.ActionOpenAbove:
		return true
	}
	return false
}

// --- Operator execution ---

func (st *appState) vimExecuteOperator(ed *editor.Editor, action vim.Action, op vim.Operator) {
	count := action.EffectiveCount()

	// Visual mode operation
	if action.Text == "visual" {
		text := ed.SelectedText()
		if text == "" {
			return
		}
		isLinewise := action.MotionType == vim.MotionLineWise || st.vimState.Mode == vim.ModeVisualLine
		switch op {
		case vim.OpDelete:
			st.vimState.Registers.RecordDelete(text, isLinewise, st.vimState.Register)
			ed.DeleteSelection()
			st.afterEdit()
		case vim.OpChange:
			st.vimState.Registers.RecordDelete(text, isLinewise, st.vimState.Register)
			ed.DeleteSelection()
			st.afterEdit()
			st.vimState.Mode = vim.ModeInsert
		case vim.OpYank:
			st.vimState.Registers.RecordYank(text, st.vimState.Register)
			ed.Selection.Clear()
		}
		return
	}

	// Text object operation
	if action.TextObj != 0 {
		st.vimExecuteTextObject(ed, action, op)
		return
	}

	// Line-wise operation (dd, yy, cc)
	if action.MotionType == vim.MotionLineWise && action.Motion == vim.ActionNone {
		st.vimExecuteLineOp(ed, count, op)
		return
	}

	// Operator + motion
	if action.Motion != vim.ActionNone {
		st.vimExecuteMotionOp(ed, action, op)
		return
	}
}

// vimExecuteLineOp handles dd, yy, cc (line-wise operations).
func (st *appState) vimExecuteLineOp(ed *editor.Editor, count int, op vim.Operator) {
	startLine := ed.Cursor.Line
	endLine := startLine + count - 1
	if endLine >= ed.Buffer.LineCount() {
		endLine = ed.Buffer.LineCount() - 1
	}

	// Select the lines
	ed.Selection.Anchor = editor.Cursor{Line: startLine, Col: 0}
	lastLineText, _ := ed.Buffer.Line(endLine)
	ed.Selection.Head = editor.Cursor{Line: endLine, Col: utf8.RuneCountInString(lastLineText)}
	ed.Selection.Active = true

	// Include the newline after the last line if possible
	text := ed.SelectedText()
	if endLine < ed.Buffer.LineCount()-1 {
		text += "\n"
		ed.Selection.Head = editor.Cursor{Line: endLine + 1, Col: 0}
	} else if startLine > 0 {
		// Last lines: include the newline before instead
		offset, _ := ed.Buffer.LineColToOffset(buffer.LineCol{Line: startLine, Col: 0})
		if offset > 0 {
			ed.Selection.Anchor = editor.Cursor{Line: startLine - 1, Col: 0}
			prevLine, _ := ed.Buffer.Line(startLine - 1)
			ed.Selection.Anchor = editor.Cursor{Line: startLine - 1, Col: utf8.RuneCountInString(prevLine)}
			text = "\n" + text
		}
	}

	switch op {
	case vim.OpDelete:
		st.vimState.Registers.RecordDelete(text, true, st.vimState.Register)
		ed.DeleteSelection()
		ed.Cursor.SetPosition(ed.Buffer, startLine, 0)
		vimMoveFirstNonBlank(ed)
		st.afterEdit()
	case vim.OpChange:
		st.vimState.Registers.RecordDelete(text, true, st.vimState.Register)
		ed.DeleteSelection()
		ed.Cursor.SetPosition(ed.Buffer, startLine, 0)
		st.afterEdit()
		st.vimState.Mode = vim.ModeInsert
	case vim.OpYank:
		st.vimState.Registers.RecordYank(text, st.vimState.Register)
		ed.Selection.Clear()
		ed.Cursor.SetPosition(ed.Buffer, startLine, 0)
	}
}

// vimExecuteMotionOp handles operator+motion (e.g., dw, c$).
func (st *appState) vimExecuteMotionOp(ed *editor.Editor, action vim.Action, op vim.Operator) {
	count := action.EffectiveCount()

	// Save position before motion
	startLine, startCol := ed.Cursor.Line, ed.Cursor.Col

	// Execute the motion
	for i := 0; i < count; i++ {
		st.executeMotion(ed, action.Motion, action.Char)
	}

	endLine, endCol := ed.Cursor.Line, ed.Cursor.Col

	// Ensure start < end
	if startLine > endLine || (startLine == endLine && startCol > endCol) {
		startLine, startCol, endLine, endCol = endLine, endCol, startLine, startCol
	}

	isLinewise := action.MotionType == vim.MotionLineWise

	// Set up selection
	if isLinewise {
		ed.Selection.Anchor = editor.Cursor{Line: startLine, Col: 0}
		lastLineText, _ := ed.Buffer.Line(endLine)
		ed.Selection.Head = editor.Cursor{Line: endLine, Col: utf8.RuneCountInString(lastLineText)}
	} else {
		ed.Selection.Anchor = editor.Cursor{Line: startLine, Col: startCol}
		ed.Selection.Head = editor.Cursor{Line: endLine, Col: endCol}
	}
	ed.Selection.Active = true

	text := ed.SelectedText()
	if text == "" {
		ed.Selection.Clear()
		ed.Cursor.SetPosition(ed.Buffer, startLine, startCol)
		return
	}

	switch op {
	case vim.OpDelete:
		st.vimState.Registers.RecordDelete(text, isLinewise, st.vimState.Register)
		ed.DeleteSelection()
		st.afterEdit()
	case vim.OpChange:
		st.vimState.Registers.RecordDelete(text, isLinewise, st.vimState.Register)
		ed.DeleteSelection()
		st.afterEdit()
		st.vimState.Mode = vim.ModeInsert
	case vim.OpYank:
		st.vimState.Registers.RecordYank(text, st.vimState.Register)
		ed.Selection.Clear()
		ed.Cursor.SetPosition(ed.Buffer, startLine, startCol)
	}
}

// executeMotion performs a single motion, moving the editor cursor.
func (st *appState) executeMotion(ed *editor.Editor, motion vim.ActionKind, ch rune) {
	switch motion {
	case vim.ActionMoveLeft:
		ed.Cursor.MoveLeft(ed.Buffer)
	case vim.ActionMoveRight:
		ed.Cursor.MoveRight(ed.Buffer)
	case vim.ActionMoveUp:
		ed.Cursor.MoveUp(ed.Buffer)
	case vim.ActionMoveDown:
		ed.Cursor.MoveDown(ed.Buffer)
	case vim.ActionMoveWordForward:
		vimMoveWordForward(ed)
	case vim.ActionMoveWordBackward:
		vimMoveWordBackward(ed)
	case vim.ActionMoveWordEnd:
		vimMoveWordEnd(ed)
	case vim.ActionMoveBigWordFwd:
		vimMoveBigWordForward(ed)
	case vim.ActionMoveBigWordBack:
		vimMoveBigWordBackward(ed)
	case vim.ActionMoveBigWordEnd:
		vimMoveBigWordEnd(ed)
	case vim.ActionMoveLineStart:
		ed.Cursor.MoveToLineStart()
	case vim.ActionMoveLineEnd:
		ed.Cursor.MoveToLineEnd(ed.Buffer)
	case vim.ActionMoveFirstNonBlank:
		vimMoveFirstNonBlank(ed)
	case vim.ActionMoveFileStart:
		ed.Cursor.MoveToFileStart()
	case vim.ActionMoveFileEnd:
		ed.Cursor.MoveToFileEnd(ed.Buffer)
	case vim.ActionMoveParagraphDown:
		vimMoveParagraph(ed, 1)
	case vim.ActionMoveParagraphUp:
		vimMoveParagraph(ed, -1)
	case vim.ActionMoveBracketMatch:
		vimMatchBracket(ed)
	case vim.ActionMoveFindChar:
		vimFindChar(ed, ch, true, false)
	case vim.ActionMoveTillChar:
		vimFindChar(ed, ch, true, true)
	case vim.ActionMoveFindCharBack:
		vimFindChar(ed, ch, false, false)
	case vim.ActionMoveTillCharBack:
		vimFindChar(ed, ch, false, true)
	}
}

// vimExecuteTextObject handles operations with text objects (e.g., ciw, di").
func (st *appState) vimExecuteTextObject(ed *editor.Editor, action vim.Action, op vim.Operator) {
	inner := action.TextObjType == 'i'
	startLine, startCol, endLine, endCol, ok := vimFindTextObject(ed, action.TextObj, inner)
	if !ok {
		return
	}

	ed.Selection.Anchor = editor.Cursor{Line: startLine, Col: startCol}
	ed.Selection.Head = editor.Cursor{Line: endLine, Col: endCol}
	ed.Selection.Active = true

	text := ed.SelectedText()
	if text == "" {
		ed.Selection.Clear()
		return
	}

	switch op {
	case vim.OpDelete:
		st.vimState.Registers.RecordDelete(text, false, st.vimState.Register)
		ed.DeleteSelection()
		st.afterEdit()
	case vim.OpChange:
		st.vimState.Registers.RecordDelete(text, false, st.vimState.Register)
		ed.DeleteSelection()
		st.afterEdit()
		st.vimState.Mode = vim.ModeInsert
	case vim.OpYank:
		st.vimState.Registers.RecordYank(text, st.vimState.Register)
		ed.Selection.Clear()
	}
}

// vimExecuteIndent handles >> / << and visual mode > / <.
func (st *appState) vimExecuteIndent(ed *editor.Editor, action vim.Action, indent bool) {
	count := action.EffectiveCount()
	if action.Text == "visual" {
		start, end := ed.Selection.Ordered()
		for line := start.Line; line <= end.Line; line++ {
			vimIndentLine(ed, line, indent)
		}
		ed.Selection.Clear()
		st.afterEdit()
		return
	}
	// Line-wise: indent count lines from cursor
	if action.MotionType == vim.MotionLineWise {
		for i := 0; i < count; i++ {
			line := ed.Cursor.Line + i
			if line < ed.Buffer.LineCount() {
				vimIndentLine(ed, line, indent)
			}
		}
		st.afterEdit()
	}
}

func vimIndentLine(ed *editor.Editor, line int, indent bool) {
	lineText, err := ed.Buffer.Line(line)
	if err != nil {
		return
	}
	offset, _ := ed.Buffer.LineColToOffset(buffer.LineCol{Line: line, Col: 0})
	if indent {
		ed.Buffer.Insert(offset, "    ")
		ed.Modified = true
	} else {
		// Remove up to 4 leading spaces
		remove := 0
		for _, r := range lineText {
			if r == ' ' && remove < 4 {
				remove++
			} else {
				break
			}
		}
		if remove > 0 {
			ed.Buffer.Delete(offset, remove)
			ed.Modified = true
		}
	}
}

// --- Put ---

func (st *appState) vimPut(ed *editor.Editor, action vim.Action, before bool) {
	reg := action.Register
	if reg == 0 {
		reg = '"'
	}

	// Check system clipboard for " register
	var text string
	if reg == '"' || reg == '+' || reg == '*' {
		text = clipboard.Get()
		if text == "" {
			text = st.vimState.Registers.Get(reg)
		}
	} else {
		text = st.vimState.Registers.Get(reg)
	}
	if text == "" {
		return
	}

	// Visual mode put: replace selection
	if action.Text == "visual" {
		if ed.Selection.Active && !ed.Selection.IsEmpty() {
			ed.DeleteSelection()
		}
		ed.InsertText(text)
		st.afterEdit()
		return
	}

	isLinewise := strings.HasSuffix(text, "\n")
	count := action.EffectiveCount()

	for i := 0; i < count; i++ {
		if isLinewise {
			if before {
				ed.Cursor.MoveToLineStart()
				ed.InsertText(text)
				// Move cursor to first non-blank of pasted text
				ed.Cursor.SetPosition(ed.Buffer, ed.Cursor.Line-strings.Count(text, "\n"), 0)
				vimMoveFirstNonBlank(ed)
			} else {
				ed.Cursor.MoveToLineEnd(ed.Buffer)
				ed.InsertText("\n" + strings.TrimSuffix(text, "\n"))
			}
		} else {
			if !before {
				ed.Cursor.MoveRight(ed.Buffer)
			}
			ed.InsertText(text)
		}
	}
	st.afterEdit()
}

// --- Search ---

func (st *appState) vimSearchNext(ed *editor.Editor, count int) {
	pattern := st.vimState.SearchPattern
	if pattern == "" {
		return
	}
	results, _ := editor.Find(ed.Buffer, pattern, false, true)
	if len(results) == 0 {
		return
	}
	curOff := ed.Buffer.LineColToOffsetSafe(ed.Cursor.Line, ed.Cursor.Col)

	// Find next match after cursor
	for c := 0; c < count; c++ {
		found := false
		for _, r := range results {
			if r.Offset > curOff {
				ed.Cursor.SetPosition(ed.Buffer, r.Line, r.Col)
				curOff = r.Offset
				found = true
				break
			}
		}
		if !found && len(results) > 0 {
			// Wrap around
			r := results[0]
			ed.Cursor.SetPosition(ed.Buffer, r.Line, r.Col)
			curOff = r.Offset
		}
	}
	ed.Selection.Clear()
}

func (st *appState) vimSearchPrev(ed *editor.Editor, count int) {
	pattern := st.vimState.SearchPattern
	if pattern == "" {
		return
	}
	results, _ := editor.Find(ed.Buffer, pattern, false, true)
	if len(results) == 0 {
		return
	}
	curOff := ed.Buffer.LineColToOffsetSafe(ed.Cursor.Line, ed.Cursor.Col)

	for c := 0; c < count; c++ {
		found := false
		for i := len(results) - 1; i >= 0; i-- {
			if results[i].Offset < curOff {
				ed.Cursor.SetPosition(ed.Buffer, results[i].Line, results[i].Col)
				curOff = results[i].Offset
				found = true
				break
			}
		}
		if !found && len(results) > 0 {
			// Wrap around
			r := results[len(results)-1]
			ed.Cursor.SetPosition(ed.Buffer, r.Line, r.Col)
			curOff = r.Offset
		}
	}
	ed.Selection.Clear()
}

func (st *appState) vimSearchWordUnderCursor(ed *editor.Editor) {
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	runes := []rune(line)
	col := ed.Cursor.Col
	if col >= len(runes) {
		return
	}

	// Find word boundaries
	left := col
	for left > 0 && isVimWordRune(runes[left-1]) {
		left--
	}
	right := col
	for right < len(runes) && isVimWordRune(runes[right]) {
		right++
	}
	if left == right {
		return
	}

	word := string(runes[left:right])
	st.vimState.SearchPattern = word
	st.vimState.Registers.Search = word
	st.vimSearchNext(ed, 1)
}

func isVimWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// --- Motion helpers ---

func vimMoveWordForward(ed *editor.Editor) {
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	runes := []rune(line)
	col := ed.Cursor.Col

	if col >= len(runes) {
		// Move to next line
		if ed.Cursor.Line < ed.Buffer.LineCount()-1 {
			ed.Cursor.Line++
			ed.Cursor.Col = 0
			// Skip to first non-blank
			line, _ = ed.Buffer.Line(ed.Cursor.Line)
			runes = []rune(line)
			for i, r := range runes {
				if !unicode.IsSpace(r) {
					ed.Cursor.Col = i
					return
				}
			}
		}
		return
	}

	// Skip current word
	if isVimWordRune(runes[col]) {
		for col < len(runes) && isVimWordRune(runes[col]) {
			col++
		}
	} else if !unicode.IsSpace(runes[col]) {
		// Skip punctuation
		for col < len(runes) && !isVimWordRune(runes[col]) && !unicode.IsSpace(runes[col]) {
			col++
		}
	}
	// Skip whitespace
	for col < len(runes) && unicode.IsSpace(runes[col]) {
		col++
	}

	if col >= len(runes) {
		// Move to next line start
		if ed.Cursor.Line < ed.Buffer.LineCount()-1 {
			ed.Cursor.Line++
			ed.Cursor.Col = 0
		} else {
			ed.Cursor.Col = len(runes)
		}
	} else {
		ed.Cursor.Col = col
	}
	ed.Cursor.PreferredCol = -1
}

func vimMoveWordBackward(ed *editor.Editor) {
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	runes := []rune(line)
	col := ed.Cursor.Col

	if col <= 0 {
		if ed.Cursor.Line > 0 {
			ed.Cursor.Line--
			prevLine, _ := ed.Buffer.Line(ed.Cursor.Line)
			ed.Cursor.Col = utf8.RuneCountInString(prevLine)
			vimMoveWordBackward(ed) // recurse to find word
		}
		return
	}

	col--
	// Skip whitespace backward
	for col > 0 && unicode.IsSpace(runes[col]) {
		col--
	}
	// Skip word backward
	if col >= 0 && col < len(runes) {
		if isVimWordRune(runes[col]) {
			for col > 0 && isVimWordRune(runes[col-1]) {
				col--
			}
		} else {
			for col > 0 && !isVimWordRune(runes[col-1]) && !unicode.IsSpace(runes[col-1]) {
				col--
			}
		}
	}

	ed.Cursor.Col = col
	ed.Cursor.PreferredCol = -1
}

func vimMoveWordEnd(ed *editor.Editor) {
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	runes := []rune(line)
	col := ed.Cursor.Col + 1

	if col >= len(runes) {
		if ed.Cursor.Line < ed.Buffer.LineCount()-1 {
			ed.Cursor.Line++
			ed.Cursor.Col = 0
			vimMoveWordEnd(ed)
		}
		return
	}

	// Skip whitespace
	for col < len(runes) && unicode.IsSpace(runes[col]) {
		col++
	}
	if col >= len(runes) {
		if ed.Cursor.Line < ed.Buffer.LineCount()-1 {
			ed.Cursor.Line++
			ed.Cursor.Col = 0
			vimMoveWordEnd(ed)
		}
		return
	}
	// Advance to end of word
	if isVimWordRune(runes[col]) {
		for col+1 < len(runes) && isVimWordRune(runes[col+1]) {
			col++
		}
	} else {
		for col+1 < len(runes) && !isVimWordRune(runes[col+1]) && !unicode.IsSpace(runes[col+1]) {
			col++
		}
	}

	ed.Cursor.Col = col
	ed.Cursor.PreferredCol = -1
}

func vimMoveBigWordForward(ed *editor.Editor) {
	line, _ := ed.Buffer.Line(ed.Cursor.Line)
	runes := []rune(line)
	col := ed.Cursor.Col

	// Skip non-space
	for col < len(runes) && !unicode.IsSpace(runes[col]) {
		col++
	}
	// Skip space
	for col < len(runes) && unicode.IsSpace(runes[col]) {
		col++
	}
	if col >= len(runes) && ed.Cursor.Line < ed.Buffer.LineCount()-1 {
		ed.Cursor.Line++
		ed.Cursor.Col = 0
	} else {
		ed.Cursor.Col = col
	}
	ed.Cursor.PreferredCol = -1
}

func vimMoveBigWordBackward(ed *editor.Editor) {
	line, _ := ed.Buffer.Line(ed.Cursor.Line)
	runes := []rune(line)
	col := ed.Cursor.Col

	if col <= 0 {
		if ed.Cursor.Line > 0 {
			ed.Cursor.Line--
			prevLine, _ := ed.Buffer.Line(ed.Cursor.Line)
			ed.Cursor.Col = utf8.RuneCountInString(prevLine)
			vimMoveBigWordBackward(ed)
		}
		return
	}
	col--
	for col > 0 && unicode.IsSpace(runes[col]) {
		col--
	}
	for col > 0 && !unicode.IsSpace(runes[col-1]) {
		col--
	}
	ed.Cursor.Col = col
	ed.Cursor.PreferredCol = -1
}

func vimMoveBigWordEnd(ed *editor.Editor) {
	line, _ := ed.Buffer.Line(ed.Cursor.Line)
	runes := []rune(line)
	col := ed.Cursor.Col + 1

	if col >= len(runes) {
		if ed.Cursor.Line < ed.Buffer.LineCount()-1 {
			ed.Cursor.Line++
			ed.Cursor.Col = 0
			vimMoveBigWordEnd(ed)
		}
		return
	}
	for col < len(runes) && unicode.IsSpace(runes[col]) {
		col++
	}
	for col+1 < len(runes) && !unicode.IsSpace(runes[col+1]) {
		col++
	}
	ed.Cursor.Col = col
	ed.Cursor.PreferredCol = -1
}

func vimMoveFirstNonBlank(ed *editor.Editor) {
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	for i, r := range []rune(line) {
		if !unicode.IsSpace(r) {
			ed.Cursor.Col = i
			ed.Cursor.PreferredCol = -1
			return
		}
	}
	ed.Cursor.Col = 0
	ed.Cursor.PreferredCol = -1
}

func vimMoveParagraph(ed *editor.Editor, dir int) {
	line := ed.Cursor.Line
	total := ed.Buffer.LineCount()

	// Skip current empty lines
	for {
		line += dir
		if line < 0 || line >= total {
			break
		}
		text, _ := ed.Buffer.Line(line)
		if strings.TrimSpace(text) != "" {
			break
		}
	}
	// Find next empty line
	for {
		line += dir
		if line < 0 || line >= total {
			break
		}
		text, _ := ed.Buffer.Line(line)
		if strings.TrimSpace(text) == "" {
			break
		}
	}

	if line < 0 {
		line = 0
	}
	if line >= total {
		line = total - 1
	}
	ed.Cursor.SetPosition(ed.Buffer, line, 0)
}

func vimMatchBracket(ed *editor.Editor) {
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	runes := []rune(line)
	col := ed.Cursor.Col

	// Find a bracket at or after cursor on this line
	bracketIdx := -1
	for i := col; i < len(runes); i++ {
		if isBracket(runes[i]) {
			bracketIdx = i
			break
		}
	}
	if bracketIdx == -1 {
		return
	}

	ch := runes[bracketIdx]
	match, dir := bracketPair(ch)
	if match == 0 {
		return
	}

	depth := 1
	curLine := ed.Cursor.Line
	curCol := bracketIdx + dir
	total := ed.Buffer.LineCount()

	for depth > 0 {
		lineText, err := ed.Buffer.Line(curLine)
		if err != nil {
			return
		}
		lineRunes := []rune(lineText)

		for curCol >= 0 && curCol < len(lineRunes) {
			if lineRunes[curCol] == ch {
				depth++
			} else if lineRunes[curCol] == match {
				depth--
				if depth == 0 {
					ed.Cursor.SetPosition(ed.Buffer, curLine, curCol)
					return
				}
			}
			curCol += dir
		}

		curLine += dir
		if curLine < 0 || curLine >= total {
			return
		}
		if dir > 0 {
			curCol = 0
		} else {
			nextLine, _ := ed.Buffer.Line(curLine)
			curCol = utf8.RuneCountInString(nextLine) - 1
		}
	}
}

func isBracket(r rune) bool {
	return r == '(' || r == ')' || r == '{' || r == '}' || r == '[' || r == ']'
}

func bracketPair(r rune) (rune, int) {
	switch r {
	case '(':
		return ')', 1
	case ')':
		return '(', -1
	case '{':
		return '}', 1
	case '}':
		return '{', -1
	case '[':
		return ']', 1
	case ']':
		return '[', -1
	}
	return 0, 0
}

func vimFindChar(ed *editor.Editor, ch rune, forward, till bool) {
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	runes := []rune(line)
	col := ed.Cursor.Col

	if forward {
		for i := col + 1; i < len(runes); i++ {
			if runes[i] == ch {
				if till {
					ed.Cursor.Col = i - 1
				} else {
					ed.Cursor.Col = i
				}
				ed.Cursor.PreferredCol = -1
				return
			}
		}
	} else {
		for i := col - 1; i >= 0; i-- {
			if runes[i] == ch {
				if till {
					ed.Cursor.Col = i + 1
				} else {
					ed.Cursor.Col = i
				}
				ed.Cursor.PreferredCol = -1
				return
			}
		}
	}
}

// vimDeleteLineContent deletes the content of the current line but keeps the line.
func vimDeleteLineContent(ed *editor.Editor) {
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	if len(line) == 0 {
		return
	}
	ed.Selection.Anchor = editor.Cursor{Line: ed.Cursor.Line, Col: 0}
	ed.Selection.Head = editor.Cursor{Line: ed.Cursor.Line, Col: utf8.RuneCountInString(line)}
	ed.Selection.Active = true
	ed.DeleteSelection()
	ed.Cursor.Col = 0
}

func vimJoinLines(ed *editor.Editor) {
	if ed.Cursor.Line >= ed.Buffer.LineCount()-1 {
		return
	}
	// Move to end of current line
	ed.Cursor.MoveToLineEnd(ed.Buffer)
	nextLine, _ := ed.Buffer.Line(ed.Cursor.Line + 1)
	trimmed := strings.TrimLeft(nextLine, " \t")
	// Delete the newline and leading whitespace of next line
	offset := ed.Buffer.LineColToOffsetSafe(ed.Cursor.Line, ed.Cursor.Col)
	nextLineLen := utf8.RuneCountInString(nextLine) + 1 // +1 for newline
	ed.Buffer.Delete(offset, nextLineLen)
	// Insert a space and the trimmed content
	if len(trimmed) > 0 {
		ed.Buffer.Insert(offset, " "+trimmed)
	}
	ed.Modified = true
}

func (st *appState) vimSwapVisualAnchorHelper(ed *editor.Editor) {
	if !ed.Selection.Active {
		return
	}
	ed.Selection.Anchor, ed.Selection.Head = ed.Selection.Head, ed.Selection.Anchor
	ed.Cursor = ed.Selection.Head
}

// --- Text Object finder ---

func vimFindTextObject(ed *editor.Editor, obj rune, inner bool) (startLine, startCol, endLine, endCol int, ok bool) {
	switch obj {
	case 'w', 'W':
		return vimTextObjectWord(ed, obj == 'W', inner)
	case '"', '\'', '`':
		return vimTextObjectQuote(ed, obj, inner)
	case '(', ')', 'b':
		return vimTextObjectPair(ed, '(', ')', inner)
	case '[', ']':
		return vimTextObjectPair(ed, '[', ']', inner)
	case '{', '}', 'B':
		return vimTextObjectPair(ed, '{', '}', inner)
	case '<', '>':
		return vimTextObjectPair(ed, '<', '>', inner)
	}
	return 0, 0, 0, 0, false
}

func vimTextObjectWord(ed *editor.Editor, bigWord, inner bool) (int, int, int, int, bool) {
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return 0, 0, 0, 0, false
	}
	runes := []rune(line)
	col := ed.Cursor.Col
	if col >= len(runes) {
		return 0, 0, 0, 0, false
	}

	isWord := func(r rune) bool {
		if bigWord {
			return !unicode.IsSpace(r)
		}
		return isVimWordRune(r)
	}

	left := col
	right := col

	if isWord(runes[col]) {
		for left > 0 && isWord(runes[left-1]) {
			left--
		}
		for right+1 < len(runes) && isWord(runes[right+1]) {
			right++
		}
		right++ // exclusive end
		if !inner {
			// Include trailing whitespace
			for right < len(runes) && unicode.IsSpace(runes[right]) {
				right++
			}
		}
	} else if unicode.IsSpace(runes[col]) {
		for left > 0 && unicode.IsSpace(runes[left-1]) {
			left--
		}
		for right+1 < len(runes) && unicode.IsSpace(runes[right+1]) {
			right++
		}
		right++
	}

	return ed.Cursor.Line, left, ed.Cursor.Line, right, true
}

func vimTextObjectQuote(ed *editor.Editor, quote rune, inner bool) (int, int, int, int, bool) {
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return 0, 0, 0, 0, false
	}
	runes := []rune(line)
	col := ed.Cursor.Col

	// Find opening quote
	openIdx := -1
	for i := col; i >= 0; i-- {
		if runes[i] == quote && (i == 0 || runes[i-1] != '\\') {
			openIdx = i
			break
		}
	}
	if openIdx == -1 {
		// Try forward
		for i := col; i < len(runes); i++ {
			if runes[i] == quote && (i == 0 || runes[i-1] != '\\') {
				openIdx = i
				break
			}
		}
	}
	if openIdx == -1 {
		return 0, 0, 0, 0, false
	}

	// Find closing quote
	closeIdx := -1
	for i := openIdx + 1; i < len(runes); i++ {
		if runes[i] == quote && runes[i-1] != '\\' {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return 0, 0, 0, 0, false
	}

	if inner {
		return ed.Cursor.Line, openIdx + 1, ed.Cursor.Line, closeIdx, true
	}
	return ed.Cursor.Line, openIdx, ed.Cursor.Line, closeIdx + 1, true
}

func vimTextObjectPair(ed *editor.Editor, open, close rune, inner bool) (int, int, int, int, bool) {
	// Search backward for the opening bracket
	curLine := ed.Cursor.Line
	curCol := ed.Cursor.Col
	depth := 0

	// First check if cursor is on a bracket
	lineText, _ := ed.Buffer.Line(curLine)
	lineRunes := []rune(lineText)
	if curCol < len(lineRunes) && lineRunes[curCol] == open {
		// Cursor is on the opening bracket
	} else {
		// Search backward for opening bracket
		found := false
		scanLine := curLine
		scanCol := curCol
		for scanLine >= 0 {
			text, _ := ed.Buffer.Line(scanLine)
			runes := []rune(text)
			if scanCol >= len(runes) {
				scanCol = len(runes) - 1
			}
			for i := scanCol; i >= 0; i-- {
				if runes[i] == close {
					depth++
				} else if runes[i] == open {
					if depth == 0 {
						curLine = scanLine
						curCol = i
						found = true
						break
					}
					depth--
				}
			}
			if found {
				break
			}
			scanLine--
			if scanLine >= 0 {
				prevText, _ := ed.Buffer.Line(scanLine)
				scanCol = utf8.RuneCountInString(prevText) - 1
			}
		}
		if !found {
			return 0, 0, 0, 0, false
		}
	}

	// Now find the matching closing bracket
	startLine, startCol := curLine, curCol
	depth = 1
	scanLine := curLine
	scanCol := curCol + 1

	for scanLine < ed.Buffer.LineCount() {
		text, _ := ed.Buffer.Line(scanLine)
		runes := []rune(text)
		for i := scanCol; i < len(runes); i++ {
			if runes[i] == open {
				depth++
			} else if runes[i] == close {
				depth--
				if depth == 0 {
					endLine, endCol := scanLine, i
					if inner {
						return startLine, startCol + 1, endLine, endCol, true
					}
					return startLine, startCol, endLine, endCol + 1, true
				}
			}
		}
		scanLine++
		scanCol = 0
	}

	return 0, 0, 0, 0, false
}

// openVimTutor opens the vim tutor as a new tab.
func (st *appState) openVimTutor() {
	pt := buffer.NewFromString(vim.TutorContent)
	ed := editor.NewEditor(pt, "")
	st.tabBar.OpenEditor(ed, "Vim Tutor")
	st.activeTabState()
	st.updateWindowTitle()
	st.window.Invalidate()
}

