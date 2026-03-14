package ui

// LanguageSelector manages the language picker dropdown state.
type LanguageSelector struct {
	Visible   bool
	Languages []string // available language names
	Selected  int      // highlighted index
}

// NewLanguageSelector creates a new language selector.
func NewLanguageSelector() *LanguageSelector {
	return &LanguageSelector{}
}

// Open shows the language selector with the given language list.
func (ls *LanguageSelector) Open(languages []string) {
	ls.Visible = true
	ls.Languages = append([]string{"Plain Text"}, languages...)
	ls.Selected = 0
}

// Close hides the language selector.
func (ls *LanguageSelector) Close() {
	ls.Visible = false
	ls.Selected = 0
}

// MoveUp moves selection up.
func (ls *LanguageSelector) MoveUp() {
	if ls.Selected > 0 {
		ls.Selected--
	}
}

// MoveDown moves selection down.
func (ls *LanguageSelector) MoveDown() {
	if ls.Selected < len(ls.Languages)-1 {
		ls.Selected++
	}
}

// SelectedLanguage returns the currently highlighted language name.
func (ls *LanguageSelector) SelectedLanguage() string {
	if ls.Selected >= 0 && ls.Selected < len(ls.Languages) {
		return ls.Languages[ls.Selected]
	}
	return ""
}

// LanguageAtY returns the language index for a given Y position within the dropdown,
// given the item height. Returns -1 if out of range.
func (ls *LanguageSelector) LanguageAtY(y, itemHeight int) int {
	if itemHeight <= 0 {
		return -1
	}
	idx := y / itemHeight
	if idx >= 0 && idx < len(ls.Languages) {
		return idx
	}
	return -1
}
