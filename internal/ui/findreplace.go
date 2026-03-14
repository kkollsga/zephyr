package ui

// FindReplaceBar manages the inline find/replace bar state.
type FindReplaceBar struct {
	Visible     bool
	Query       string
	Replacement string
	UseRegex    bool
	CaseSensitive bool
	MatchCount  int
	CurrentMatch int
}

// NewFindReplaceBar creates a new find/replace bar.
func NewFindReplaceBar() *FindReplaceBar {
	return &FindReplaceBar{}
}

// Open shows the find bar.
func (fr *FindReplaceBar) Open() {
	fr.Visible = true
}

// OpenReplace shows the find/replace bar.
func (fr *FindReplaceBar) OpenReplace() {
	fr.Visible = true
}

// Close hides the find/replace bar.
func (fr *FindReplaceBar) Close() {
	fr.Visible = false
	fr.Query = ""
	fr.Replacement = ""
	fr.MatchCount = 0
	fr.CurrentMatch = 0
}

// ToggleRegex toggles regex mode.
func (fr *FindReplaceBar) ToggleRegex() {
	fr.UseRegex = !fr.UseRegex
}

// ToggleCaseSensitive toggles case sensitivity.
func (fr *FindReplaceBar) ToggleCaseSensitive() {
	fr.CaseSensitive = !fr.CaseSensitive
}
