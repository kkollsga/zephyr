package main

import (
	"strings"
	"testing"

	"github.com/kristianweb/zephyr/internal/ui"
	"github.com/kristianweb/zephyr/internal/vim"
)

// navHelpUnlisted holds sequences the vim state machine answers but the
// navigator help deliberately leaves out, each with the reason. The sweep below
// checks the exemptions too: one that no longer produces an action is a stale
// exemption hiding a hole in the gate.
var navHelpUnlisted = map[string]string{
	"gg": "a core vim motion, documented in docs/vim-mode.md, not a navigator binding",
	"gi": "retired: no handler consumes ActionNavGoImports, so the key does nothing",
}

// navSequence feeds keys into a fresh state machine and returns the last action
// it produced. "<Space>" is the only multi-character token; every other
// character is one key press.
func navSequence(keys string, navigator bool) vim.Action {
	s := vim.NewState()
	s.NavigatorEnabled = navigator
	var last vim.Action
	rest := keys
	for rest != "" {
		var ch rune
		if strings.HasPrefix(rest, "<Space>") {
			ch, rest = ' ', rest[len("<Space>"):]
		} else {
			r := []rune(rest)
			ch, rest = r[0], string(r[1:])
		}
		last = s.HandleKey(vim.KeyInput{Char: ch})
	}
	return last
}

// isNavAction reports whether kind is one of the navigator's own actions. The
// constants form one contiguous block in internal/vim/action.go; a non-Nav
// constant moved into it only makes the sweep demand more rows, which fails
// loudly rather than quietly.
func isNavAction(kind vim.ActionKind) bool {
	return kind >= vim.ActionNavNextHunk && kind <= vim.ActionNavToggleReadMode
}

// Every row of the help table is a sequence the state machine actually answers.
// A row for a key that does nothing is the help lying to the reader.
func TestNavHelpRowsAreLiveBindings(t *testing.T) {
	for _, b := range navBindings {
		act := navSequence(b.Keys, b.Mode == bindNavigator)
		if act.Kind == vim.ActionNone {
			t.Errorf("help row %q (%s) produces no action; the binding is gone or the row is wrong", b.Keys, b.Desc)
		}
	}
}

// …and every sequence the state machine answers has a row. Between the two the
// table cannot drift from the code: a binding added without a row, or a row
// left behind after a binding moved, fails here.
func TestNavHelpCoversEveryLiveSequence(t *testing.T) {
	listed := make(map[string]string, len(navBindings))
	for _, b := range navBindings {
		if prev, dup := listed[b.Keys]; dup {
			t.Errorf("help lists %q twice: %q and %q", b.Keys, prev, b.Desc)
		}
		listed[b.Keys] = b.Desc
	}

	// Prefixed sequences: everything behind them belongs to the navigator, so
	// any action at all demands a row.
	for _, prefix := range []string{"g", "]", "[", "<Space>"} {
		for r := rune('!'); r <= '~'; r++ {
			seq := prefix + string(r)
			if navSequence(seq, true).Kind == vim.ActionNone {
				continue
			}
			if _, ok := navHelpUnlisted[seq]; ok {
				continue
			}
			if _, ok := listed[seq]; !ok {
				t.Errorf("%q produces an action but no help row lists it", seq)
			}
		}
	}

	// Bare keys: the state machine answers most of them, so only the ones
	// bound to a navigator action are the navigator's to document.
	for r := rune('!'); r <= '~'; r++ {
		seq := string(r)
		if !isNavAction(navSequence(seq, true).Kind) {
			continue
		}
		if _, ok := navHelpUnlisted[seq]; ok {
			continue
		}
		if _, ok := listed[seq]; !ok {
			t.Errorf("%q is bound to a navigator action but no help row lists it", seq)
		}
	}

	for seq, why := range navHelpUnlisted {
		if navSequence(seq, true).Kind == vim.ActionNone {
			t.Errorf("%q is exempt from the help table (%s) but produces no action; drop the exemption", seq, why)
		}
		if desc, ok := listed[seq]; ok {
			t.Errorf("%q is both exempt and listed as %q", seq, desc)
		}
	}
}

// The rows reach the picker as one column of keys and one of descriptions, and
// typing filters on both.
func TestNavHelpPickerListsAndFilters(t *testing.T) {
	st, _, _ := testAppWithText("", "Plain Text")
	st.vimState = vim.NewState()
	st.vimState.NavigatorEnabled = true
	st.fuzzyFinder = ui.NewFuzzyFinder()

	if !st.executeNavAction(vim.Action{Kind: vim.ActionNavHelp}) {
		t.Fatal("g? was not handled")
	}
	ff := st.fuzzyFinder
	if !ff.Visible {
		t.Fatal("g? did not open the picker")
	}
	if len(ff.Results) != len(navBindings) {
		t.Fatalf("picker shows %d rows, want %d", len(ff.Results), len(navBindings))
	}
	if !strings.HasPrefix(ff.Prompt(), "keys") {
		t.Fatalf("prompt = %q, want it labelled", ff.Prompt())
	}

	st.handleTextInput("hunk")
	if len(ff.Results) == 0 || len(ff.Results) == len(navBindings) {
		t.Fatalf("typing \"hunk\" left %d of %d rows", len(ff.Results), len(navBindings))
	}
	for _, m := range ff.Results {
		if !strings.Contains(m.Text, "hunk") {
			t.Fatalf("row %q does not match the query", m.Text)
		}
	}
}

// Accepting a row closes the picker and does nothing else: the rows are key
// sequences, and the file finder's default would have tried to open one as a
// path under the project root.
func TestNavHelpAcceptOnlyCloses(t *testing.T) {
	st, _, _ := testAppWithText("", "Plain Text")
	st.vimState = vim.NewState()
	st.fuzzyFinder = ui.NewFuzzyFinder()
	st.executeNavAction(vim.Action{Kind: vim.ActionNavHelp})

	tabs := st.tabBar.TabCount()
	st.openFuzzySelection()
	if st.fuzzyFinder.Visible {
		t.Fatal("Enter left the picker open")
	}
	if got := st.tabBar.TabCount(); got != tabs {
		t.Fatalf("Enter opened %d tabs, want none", got-tabs)
	}
}

// The help picker's list and callback must not survive into the next file
// finder, which would show key rows under a project root and open one as a path.
func TestNavHelpDoesNotLeakIntoTheFileFinder(t *testing.T) {
	st, dir := finderApp(t)
	st.executeNavAction(vim.Action{Kind: vim.ActionNavHelp})
	st.fuzzyFinder.Close()

	openFinder(t, st)
	if st.fuzzyFinder.OnAccept != nil {
		t.Fatal("the help picker's callback outlived it")
	}
	if st.fuzzyFinder.Prompt() != "> " {
		t.Fatalf("prompt = %q, want the plain file-finder prompt", st.fuzzyFinder.Prompt())
	}
	for _, m := range st.fuzzyFinder.Results {
		if strings.Contains(m.Text, "<Space>") {
			t.Fatalf("the file finder is showing help rows: %q", m.Text)
		}
	}
	if got := st.fuzzyFinder.SelectedPath(); !strings.HasPrefix(got, dir) {
		t.Fatalf("SelectedPath = %q, want a path under %q", got, dir)
	}
}
