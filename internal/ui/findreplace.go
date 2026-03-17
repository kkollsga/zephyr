package ui

import (
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/editor"
)

// FindReplaceBar manages the inline find/replace bar state.
type FindReplaceBar struct {
	Visible       bool
	ShowReplace   bool
	Query         string
	Replacement   string
	UseRegex      bool
	CaseSensitive bool
	MatchCount    int
	CurrentMatch  int // 1-based index; 0 = no current match
	Matches       []editor.SearchResult
	FocusField    int // 0 = find, 1 = replace
	CursorPos     int // rune-based cursor position in active field
}

// NewFindReplaceBar creates a new find/replace bar.
func NewFindReplaceBar() *FindReplaceBar {
	return &FindReplaceBar{}
}

// Open shows the find bar (without replace).
func (fr *FindReplaceBar) Open() {
	fr.Visible = true
	fr.FocusField = 0
	fr.CursorPos = utf8.RuneCountInString(fr.Query)
}

// OpenReplace shows the find/replace bar.
func (fr *FindReplaceBar) OpenReplace() {
	fr.Visible = true
	fr.ShowReplace = true
	fr.FocusField = 0
	fr.CursorPos = utf8.RuneCountInString(fr.Query)
}

// Close hides the find/replace bar.
func (fr *FindReplaceBar) Close() {
	fr.Visible = false
	fr.Matches = nil
	fr.MatchCount = 0
	fr.CurrentMatch = 0
}

// ToggleReplace toggles the replace row visibility.
func (fr *FindReplaceBar) ToggleReplace() {
	fr.ShowReplace = !fr.ShowReplace
	if !fr.ShowReplace && fr.FocusField == 1 {
		fr.FocusField = 0
		fr.CursorPos = utf8.RuneCountInString(fr.Query)
	}
}

// ToggleRegex toggles regex mode.
func (fr *FindReplaceBar) ToggleRegex() {
	fr.UseRegex = !fr.UseRegex
}

// ToggleCaseSensitive toggles case sensitivity.
func (fr *FindReplaceBar) ToggleCaseSensitive() {
	fr.CaseSensitive = !fr.CaseSensitive
}

// ActiveText returns the text of the currently focused field.
func (fr *FindReplaceBar) ActiveText() string {
	if fr.FocusField == 1 {
		return fr.Replacement
	}
	return fr.Query
}

// setActiveText sets the text of the currently focused field.
func (fr *FindReplaceBar) setActiveText(s string) {
	if fr.FocusField == 1 {
		fr.Replacement = s
	} else {
		fr.Query = s
	}
}

// InsertChar inserts text at CursorPos in the active field.
func (fr *FindReplaceBar) InsertChar(ch string) {
	text := fr.ActiveText()
	runes := []rune(text)
	if fr.CursorPos > len(runes) {
		fr.CursorPos = len(runes)
	}
	inserted := []rune(ch)
	newRunes := make([]rune, 0, len(runes)+len(inserted))
	newRunes = append(newRunes, runes[:fr.CursorPos]...)
	newRunes = append(newRunes, inserted...)
	newRunes = append(newRunes, runes[fr.CursorPos:]...)
	fr.setActiveText(string(newRunes))
	fr.CursorPos += len(inserted)
}

// DeleteChar performs backspace at CursorPos.
func (fr *FindReplaceBar) DeleteChar() {
	if fr.CursorPos <= 0 {
		return
	}
	text := fr.ActiveText()
	runes := []rune(text)
	if fr.CursorPos > len(runes) {
		fr.CursorPos = len(runes)
	}
	newRunes := append(runes[:fr.CursorPos-1], runes[fr.CursorPos:]...)
	fr.setActiveText(string(newRunes))
	fr.CursorPos--
}

// DeleteForwardChar performs forward delete at CursorPos.
func (fr *FindReplaceBar) DeleteForwardChar() {
	text := fr.ActiveText()
	runes := []rune(text)
	if fr.CursorPos >= len(runes) {
		return
	}
	newRunes := append(runes[:fr.CursorPos], runes[fr.CursorPos+1:]...)
	fr.setActiveText(string(newRunes))
}

// MoveCursorLeft moves the cursor one rune left in the active field.
func (fr *FindReplaceBar) MoveCursorLeft() {
	if fr.CursorPos > 0 {
		fr.CursorPos--
	}
}

// MoveCursorRight moves the cursor one rune right in the active field.
func (fr *FindReplaceBar) MoveCursorRight() {
	text := fr.ActiveText()
	if fr.CursorPos < utf8.RuneCountInString(text) {
		fr.CursorPos++
	}
}

// MoveCursorToStart moves the cursor to the beginning of the active field.
func (fr *FindReplaceBar) MoveCursorToStart() {
	fr.CursorPos = 0
}

// MoveCursorToEnd moves the cursor to the end of the active field.
func (fr *FindReplaceBar) MoveCursorToEnd() {
	fr.CursorPos = utf8.RuneCountInString(fr.ActiveText())
}

// SwitchFocus toggles focus between find and replace fields.
func (fr *FindReplaceBar) SwitchFocus() {
	if !fr.ShowReplace {
		return
	}
	if fr.FocusField == 0 {
		fr.FocusField = 1
		fr.CursorPos = utf8.RuneCountInString(fr.Replacement)
	} else {
		fr.FocusField = 0
		fr.CursorPos = utf8.RuneCountInString(fr.Query)
	}
}

// SelectAll selects all text in the active field by placing cursor at end.
func (fr *FindReplaceBar) SelectAll() {
	fr.CursorPos = utf8.RuneCountInString(fr.ActiveText())
}
