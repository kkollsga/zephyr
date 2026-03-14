package editor

import (
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/fileio"
)

// Editor holds the core state of a single editor pane.
type Editor struct {
	Buffer       *buffer.PieceTable
	Cursor       Cursor
	Selection    Selection
	Cursors      []Cursor     // Additional cursors for multi-cursor mode
	Selections   []Selection  // Selections for each additional cursor
	History      *History
	FilePath     string
	Modified     bool
}

// NewEditor creates an editor for the given piece table.
func NewEditor(pt *buffer.PieceTable, filePath string) *Editor {
	return &Editor{
		Buffer:   pt,
		Cursor:   NewCursor(),
		History:  NewHistory(),
		FilePath: filePath,
	}
}

// NewEditorFromFile loads a file and creates an editor for it.
func NewEditorFromFile(path string) (*Editor, error) {
	pt, err := buffer.NewFromFile(path)
	if err != nil {
		return nil, err
	}
	return NewEditor(pt, path), nil
}

// NewEmptyEditor creates an editor with an empty buffer.
func NewEmptyEditor() *Editor {
	return NewEditor(buffer.NewFromString(""), "")
}

// InsertText inserts text at the current cursor position.
// If there is an active selection, it replaces the selected text as a single undoable action.
func (e *Editor) InsertText(text string) {
	if len(text) == 0 {
		return
	}

	// Delete selection first if active (without recording separate history)
	if e.Selection.Active && !e.Selection.IsEmpty() {
		e.deleteSelectionInternal()
	}

	offset := e.cursorOffset()
	e.History.Record(EditAction{
		Type:   ActionInsert,
		Offset: offset,
		Text:   text,
		Cursor: e.Cursor,
	})

	e.Buffer.Insert(offset, text)
	e.Modified = true

	// Advance cursor past the inserted text
	for _, r := range text {
		if r == '\n' {
			e.Cursor.Line++
			e.Cursor.Col = 0
		} else {
			e.Cursor.Col++
		}
	}
	e.Cursor.PreferredCol = -1
}

// DeleteBackward deletes the character before the cursor (Backspace).
func (e *Editor) DeleteBackward() {
	if e.Selection.Active && !e.Selection.IsEmpty() {
		e.DeleteSelection()
		return
	}

	if e.Cursor.Line == 0 && e.Cursor.Col == 0 {
		return
	}

	offset := e.cursorOffset()
	if offset == 0 {
		return
	}

	// Get the rune before cursor (may be multi-byte)
	text := e.Buffer.Text()
	_, runeSize := utf8.DecodeLastRuneInString(text[:offset])
	deleted, _ := e.Buffer.Substring(offset-runeSize, runeSize)

	e.History.Record(EditAction{
		Type:   ActionDelete,
		Offset: offset - runeSize,
		Text:   deleted,
		Cursor: e.Cursor,
	})

	e.Buffer.Delete(offset-runeSize, runeSize)
	e.Modified = true

	// Move cursor back
	if deleted == "\n" {
		// When joining lines, the new col is where the previous line ended
		// before the join, which equals the offset minus the start of that line.
		e.Cursor.Line--
		lc, _ := e.Buffer.OffsetToLineCol(offset - 1)
		e.Cursor.Col = lc.Col
	} else {
		e.Cursor.Col--
	}
	e.Cursor.PreferredCol = -1
}

// DeleteBackwardN deletes n bytes behind the cursor (used for soft-tab backspace).
func (e *Editor) DeleteBackwardN(n int) {
	if e.Selection.Active && !e.Selection.IsEmpty() {
		e.DeleteSelection()
		return
	}

	offset := e.cursorOffset()
	if offset < n {
		n = offset
	}
	if n == 0 {
		return
	}

	deleted, _ := e.Buffer.Substring(offset-n, n)
	e.History.Record(EditAction{
		Type:   ActionDelete,
		Offset: offset - n,
		Text:   deleted,
		Cursor: e.Cursor,
	})

	e.Buffer.Delete(offset-n, n)
	e.Modified = true
	e.Cursor.Col -= n
	e.Cursor.PreferredCol = -1
}

// DeleteForward deletes the character at the cursor (Delete key).
func (e *Editor) DeleteForward() {
	if e.Selection.Active && !e.Selection.IsEmpty() {
		e.DeleteSelection()
		return
	}

	offset := e.cursorOffset()
	if offset >= e.Buffer.Length() {
		return
	}

	// Get the rune at cursor (may be multi-byte)
	text := e.Buffer.Text()
	_, runeSize := utf8.DecodeRuneInString(text[offset:])
	deleted, _ := e.Buffer.Substring(offset, runeSize)

	e.History.Record(EditAction{
		Type:   ActionDelete,
		Offset: offset,
		Text:   deleted,
		Cursor: e.Cursor,
	})

	e.Buffer.Delete(offset, runeSize)
	e.Modified = true
	e.Cursor.PreferredCol = -1
}

// deleteSelectionInternal removes the selected text without recording history.
// Used by InsertText to make replace-selection atomic with the insert.
func (e *Editor) deleteSelectionInternal() {
	if !e.Selection.Active || e.Selection.IsEmpty() {
		return
	}
	start, _ := e.Selection.Ordered()
	text := e.Selection.Text(e.Buffer)
	startOff, err := e.Buffer.LineColToOffset(buffer.LineCol{Line: start.Line, Col: start.Col})
	if err != nil {
		return
	}
	e.Buffer.Delete(startOff, len(text))
	e.Cursor = start
	e.Cursor.PreferredCol = -1
	e.Selection.Clear()
	e.Modified = true
}

// DeleteSelection deletes the currently selected text.
func (e *Editor) DeleteSelection() {
	if !e.Selection.Active || e.Selection.IsEmpty() {
		return
	}

	start, _ := e.Selection.Ordered()
	text := e.Selection.Text(e.Buffer)
	startOff, err := e.Buffer.LineColToOffset(buffer.LineCol{Line: start.Line, Col: start.Col})
	if err != nil {
		return
	}

	e.History.Record(EditAction{
		Type:   ActionDelete,
		Offset: startOff,
		Text:   text,
		Cursor: e.Cursor,
	})

	e.Buffer.Delete(startOff, len(text))
	e.Cursor = start
	e.Cursor.PreferredCol = -1
	e.Selection.Clear()
	e.Modified = true
}

// Undo reverses the last edit action.
func (e *Editor) Undo() {
	action := e.History.Undo()
	if action == nil {
		return
	}

	switch action.Type {
	case ActionInsert:
		// Undo insert = delete
		e.Buffer.Delete(action.Offset, len(action.Text))
	case ActionDelete:
		// Undo delete = insert
		e.Buffer.Insert(action.Offset, action.Text)
	}

	e.Cursor = action.Cursor
	e.Selection.Clear()
	e.Modified = true
}

// Redo reapplies the last undone action.
func (e *Editor) Redo() {
	action := e.History.Redo()
	if action == nil {
		return
	}

	switch action.Type {
	case ActionInsert:
		e.Buffer.Insert(action.Offset, action.Text)
		// Move cursor to end of inserted text
		e.setCursorFromOffset(action.Offset + len(action.Text))
	case ActionDelete:
		e.Buffer.Delete(action.Offset, len(action.Text))
		e.setCursorFromOffset(action.Offset)
	}

	e.Selection.Clear()
	e.Modified = true
}

// Save writes the buffer content to the file.
func (e *Editor) Save() error {
	if e.FilePath == "" {
		return nil
	}
	if err := fileio.SaveFile(e.Buffer, e.FilePath); err != nil {
		return err
	}
	e.Modified = false
	return nil
}

// SaveAs writes the buffer content to the given path and updates FilePath.
func (e *Editor) SaveAs(path string) error {
	e.FilePath = path
	if err := fileio.SaveFile(e.Buffer, e.FilePath); err != nil {
		return err
	}
	e.Modified = false
	return nil
}

// cursorOffset returns the byte offset for the current cursor position.
// If the cursor is out of bounds, it is clamped to a valid position first.
func (e *Editor) cursorOffset() int {
	offset, err := e.Buffer.LineColToOffset(buffer.LineCol{Line: e.Cursor.Line, Col: e.Cursor.Col})
	if err != nil {
		// Clamp cursor to valid position and retry
		e.Cursor.Clamp(e.Buffer)
		offset, err = e.Buffer.LineColToOffset(buffer.LineCol{Line: e.Cursor.Line, Col: e.Cursor.Col})
		if err != nil {
			return 0
		}
	}
	return offset
}

// setCursorFromOffset sets the cursor position from a byte offset.
func (e *Editor) setCursorFromOffset(offset int) {
	lc, err := e.Buffer.OffsetToLineCol(offset)
	if err != nil {
		return
	}
	e.Cursor.Line = lc.Line
	e.Cursor.Col = lc.Col
	e.Cursor.PreferredCol = -1
}

// RuneAfterCursor returns the rune immediately after the cursor, or 0 if at end.
func (e *Editor) RuneAfterCursor() rune {
	offset := e.cursorOffset()
	text := e.Buffer.Text()
	if offset >= len(text) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(text[offset:])
	return r
}

// SelectedText returns the currently selected text, or empty string.
func (e *Editor) SelectedText() string {
	return e.Selection.Text(e.Buffer)
}

// --- Multi-cursor operations ---

// AddCursor adds a cursor at the given position.
func (e *Editor) AddCursor(line, col int) {
	c := Cursor{PreferredCol: -1}
	c.SetPosition(e.Buffer, line, col)
	e.Cursors = append(e.Cursors, c)
	e.Selections = append(e.Selections, NewSelection())
}

// AddCursorBelow adds a cursor one line below the last cursor.
func (e *Editor) AddCursorBelow() {
	last := e.Cursor
	if len(e.Cursors) > 0 {
		last = e.Cursors[len(e.Cursors)-1]
	}
	if last.Line >= e.Buffer.LineCount()-1 {
		return
	}
	newLine := last.Line + 1
	line, _ := e.Buffer.Line(newLine)
	col := min(last.Col, utf8.RuneCountInString(line))
	e.AddCursor(newLine, col)
}

// ClearExtraCursors removes all extra cursors, keeping only the primary.
func (e *Editor) ClearExtraCursors() {
	e.Cursors = nil
	e.Selections = nil
}

// HasMultipleCursors returns true if there are extra cursors.
func (e *Editor) HasMultipleCursors() bool {
	return len(e.Cursors) > 0
}

// AllCursors returns the primary cursor followed by all extra cursors.
func (e *Editor) AllCursors() []Cursor {
	result := make([]Cursor, 1+len(e.Cursors))
	result[0] = e.Cursor
	copy(result[1:], e.Cursors)
	return result
}

// InsertTextAtAllCursors inserts text at all cursor positions.
func (e *Editor) InsertTextAtAllCursors(text string) {
	if len(e.Cursors) == 0 {
		e.InsertText(text)
		return
	}

	// Collect all cursors and sort by offset (descending) to preserve positions
	type cursorInfo struct {
		cursor *Cursor
		offset int
		isPrimary bool
	}

	var infos []cursorInfo
	off := e.cursorOffset()
	infos = append(infos, cursorInfo{cursor: &e.Cursor, offset: off, isPrimary: true})
	for i := range e.Cursors {
		o, _ := e.Buffer.LineColToOffset(buffer.LineCol{Line: e.Cursors[i].Line, Col: e.Cursors[i].Col})
		infos = append(infos, cursorInfo{cursor: &e.Cursors[i], offset: o})
	}

	// Sort descending by offset
	for i := 0; i < len(infos)-1; i++ {
		for j := i + 1; j < len(infos); j++ {
			if infos[j].offset > infos[i].offset {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}

	// Insert at each position (from end to start), recording history for each
	for _, info := range infos {
		e.History.Record(EditAction{
			Type:   ActionInsert,
			Offset: info.offset,
			Text:   text,
			Cursor: e.Cursor,
		})
		e.Buffer.Insert(info.offset, text)
		// Update cursor position
		newOffset := info.offset + len(text)
		lc, _ := e.Buffer.OffsetToLineCol(newOffset)
		info.cursor.Line = lc.Line
		info.cursor.Col = lc.Col
		info.cursor.PreferredCol = -1
	}
	e.Modified = true
}

// DeleteBackwardAtAllCursors performs backspace at all cursor positions.
func (e *Editor) DeleteBackwardAtAllCursors() {
	if len(e.Cursors) == 0 {
		e.DeleteBackward()
		return
	}

	type cursorInfo struct {
		cursor *Cursor
		offset int
	}

	var infos []cursorInfo
	off := e.cursorOffset()
	infos = append(infos, cursorInfo{cursor: &e.Cursor, offset: off})
	for i := range e.Cursors {
		o, _ := e.Buffer.LineColToOffset(buffer.LineCol{Line: e.Cursors[i].Line, Col: e.Cursors[i].Col})
		infos = append(infos, cursorInfo{cursor: &e.Cursors[i], offset: o})
	}

	// Sort descending
	for i := 0; i < len(infos)-1; i++ {
		for j := i + 1; j < len(infos); j++ {
			if infos[j].offset > infos[i].offset {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}

	for _, info := range infos {
		if info.offset == 0 {
			continue
		}
		// Get the rune before this cursor
		text := e.Buffer.Text()
		_, runeSize := utf8.DecodeLastRuneInString(text[:info.offset])
		deleted, _ := e.Buffer.Substring(info.offset-runeSize, runeSize)
		e.History.Record(EditAction{
			Type:   ActionDelete,
			Offset: info.offset - runeSize,
			Text:   deleted,
			Cursor: e.Cursor,
		})
		e.Buffer.Delete(info.offset-runeSize, runeSize)
		newOffset := info.offset - runeSize
		lc, _ := e.Buffer.OffsetToLineCol(newOffset)
		info.cursor.Line = lc.Line
		info.cursor.Col = lc.Col
		info.cursor.PreferredCol = -1
	}
	e.Modified = true

	e.MergeOverlappingCursors()
}

// SelectNextOccurrence selects the next occurrence of the currently selected text
// and adds a cursor there (Cmd+D behavior).
func (e *Editor) SelectNextOccurrence() {
	selected := e.SelectedText()
	if selected == "" {
		// Select the word at cursor
		e.Selection.SelectWord(e.Buffer, e.Cursor)
		return
	}

	// Find next occurrence after the last cursor/selection
	results, _ := Find(e.Buffer, selected, false, true)
	if len(results) == 0 {
		return
	}

	// Find the next one after current selection
	_, end := e.Selection.Ordered()
	endOff, _ := e.Buffer.LineColToOffset(buffer.LineCol{Line: end.Line, Col: end.Col})

	for _, r := range results {
		if r.Offset > endOff {
			// Also check extra cursor selections
			alreadySelected := false
			for _, sel := range e.Selections {
				if sel.Active {
					st, _ := sel.Ordered()
					stOff, _ := e.Buffer.LineColToOffset(buffer.LineCol{Line: st.Line, Col: st.Col})
					if stOff == r.Offset {
						alreadySelected = true
						break
					}
				}
			}
			if alreadySelected {
				continue
			}

			// Add cursor and selection at this occurrence
			endLC, _ := e.Buffer.OffsetToLineCol(r.Offset + r.Length)
			newCursor := Cursor{Line: endLC.Line, Col: endLC.Col, PreferredCol: -1}
			newSel := NewSelection()
			newSel.Start(Cursor{Line: r.Line, Col: r.Col})
			newSel.Update(newCursor)

			e.Cursors = append(e.Cursors, newCursor)
			e.Selections = append(e.Selections, newSel)
			return
		}
	}

	// Wrap around: check from beginning
	for _, r := range results {
		if r.Offset >= endOff {
			break
		}
		// Skip if it's the primary selection
		startOff, _ := e.Buffer.LineColToOffset(buffer.LineCol{Line: e.Selection.Anchor.Line, Col: e.Selection.Anchor.Col})
		if r.Offset == startOff {
			continue
		}
		endLC, _ := e.Buffer.OffsetToLineCol(r.Offset + r.Length)
		newCursor := Cursor{Line: endLC.Line, Col: endLC.Col, PreferredCol: -1}
		newSel := NewSelection()
		newSel.Start(Cursor{Line: r.Line, Col: r.Col})
		newSel.Update(newCursor)

		e.Cursors = append(e.Cursors, newCursor)
		e.Selections = append(e.Selections, newSel)
		return
	}
}

// SelectAllOccurrences selects all occurrences of the selected text.
func (e *Editor) SelectAllOccurrences() {
	selected := e.SelectedText()
	if selected == "" {
		return
	}

	results, _ := Find(e.Buffer, selected, false, true)
	if len(results) <= 1 {
		return
	}

	e.Cursors = nil
	e.Selections = nil

	for i, r := range results {
		endLC, _ := e.Buffer.OffsetToLineCol(r.Offset + r.Length)
		cursor := Cursor{Line: endLC.Line, Col: endLC.Col, PreferredCol: -1}
		sel := NewSelection()
		sel.Start(Cursor{Line: r.Line, Col: r.Col})
		sel.Update(cursor)

		if i == 0 {
			e.Cursor = cursor
			e.Selection = sel
		} else {
			e.Cursors = append(e.Cursors, cursor)
			e.Selections = append(e.Selections, sel)
		}
	}
}

// SplitSelectionIntoLines creates one cursor per line of the current selection.
func (e *Editor) SplitSelectionIntoLines() {
	if !e.Selection.Active || e.Selection.IsEmpty() {
		return
	}

	start, end := e.Selection.Ordered()
	if start.Line == end.Line {
		return
	}

	e.Cursors = nil
	e.Selections = nil
	e.Selection.Clear()

	for line := start.Line; line <= end.Line; line++ {
		lineText, _ := e.Buffer.Line(line)
		col := utf8.RuneCountInString(lineText)
		if line == start.Line {
			col = start.Col
		}
		if line == end.Line {
			col = end.Col
		}

		if line == start.Line {
			e.Cursor = Cursor{Line: line, Col: col, PreferredCol: -1}
		} else {
			e.Cursors = append(e.Cursors, Cursor{Line: line, Col: col, PreferredCol: -1})
			e.Selections = append(e.Selections, NewSelection())
		}
	}
}

// MergeOverlappingCursors removes duplicate/overlapping cursors.
func (e *Editor) MergeOverlappingCursors() {
	if len(e.Cursors) == 0 {
		return
	}

	seen := map[[2]int]bool{}
	seen[[2]int{e.Cursor.Line, e.Cursor.Col}] = true

	var newCursors []Cursor
	var newSelections []Selection
	for i, c := range e.Cursors {
		key := [2]int{c.Line, c.Col}
		if seen[key] {
			continue
		}
		seen[key] = true
		newCursors = append(newCursors, c)
		if i < len(e.Selections) {
			newSelections = append(newSelections, e.Selections[i])
		}
	}
	e.Cursors = newCursors
	e.Selections = newSelections
}
