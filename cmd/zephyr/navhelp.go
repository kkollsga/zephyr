package main

import "strings"

// navBindingMode says which state a help row's keys are dispatched from.
type navBindingMode int

const (
	// bindNormal — the sequence works whenever vim mode is on.
	bindNormal navBindingMode = iota
	// bindNavigator — the sequence needs navigator mode, which is what makes
	// <Space> a leader key.
	bindNavigator
)

// navBinding is one row of the `g?` help picker: the keys, what they do, and
// the state they are dispatched from.
//
// navBindings below is the source of truth for the navigator's key help —
// docs/navigator-mode.md documents this table rather than the other way round.
// navhelp_test.go feeds every row through the vim state machine and sweeps the
// machine for sequences no row covers, so a binding that is added, moved or
// dropped fails the tests instead of leaving the help lying.
//
// It covers the sequences the state machine dispatches globally. Keys that only
// mean something inside a status or directory buffer are reinterpretations of
// ordinary vim actions in that buffer's handler, not bindings of their own, and
// they are documented rather than listed here.
type navBinding struct {
	Keys string
	Desc string
	Mode navBindingMode
}

var navBindings = []navBinding{
	{"<Space>c", "Next changed hunk", bindNavigator},
	{"<Space>C", "Previous changed hunk", bindNavigator},
	{"<Space>n", "Next changed file", bindNavigator},
	{"<Space>N", "Previous changed file", bindNavigator},
	{"<Space>g", "Open the git status buffer", bindNavigator},
	{"<Space>e", "Open the project root directory", bindNavigator},
	{"<Space>f", "Find files under the project root", bindNavigator},
	{"<Space>b", "Find changed files only", bindNavigator},
	{"<Space>r", "Toggle the markdown read view", bindNavigator},
	{"]c", "Next changed hunk", bindNormal},
	{"[c", "Previous changed hunk", bindNormal},
	{"]C", "Next changed file", bindNormal},
	{"[C", "Previous changed file", bindNormal},
	{"ga", "Alternate file (test <-> implementation)", bindNormal},
	{"gf", "Go to the file under the cursor", bindNormal},
	{"go", "Toggle the HEAD view (read-only committed content)", bindNormal},
	{"g?", "Show this list", bindNormal},
	{"-", "Open the parent directory as a buffer", bindNormal},
	{"q", "Close a directory or status buffer", bindNormal},
}

// navHelpRows renders the table as picker rows, keys in one column. The query
// matches the whole row, so both the keys and the description are searchable.
func navHelpRows() []string {
	width := 0
	for _, b := range navBindings {
		if n := len([]rune(b.Keys)); n > width {
			width = n
		}
	}
	rows := make([]string, 0, len(navBindings))
	for _, b := range navBindings {
		pad := strings.Repeat(" ", width-len([]rune(b.Keys)))
		rows = append(rows, b.Keys+pad+"   "+b.Desc)
	}
	return rows
}

// openNavHelp shows the navigator's key bindings in the picker. The rows are
// key sequences, not files, so accepting one only dismisses the list — there is
// nothing to open, and running the sequence from here would fire it against
// whatever buffer happens to be underneath.
func (st *appState) openNavHelp() {
	if st.fuzzyFinder == nil || st.overlayVisible() {
		return
	}
	st.fuzzyFinder.OpenItems("keys", navHelpRows(), func(string) {})
	st.invalidate()
}
