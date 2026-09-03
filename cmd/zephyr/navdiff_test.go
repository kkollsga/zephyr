package main

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/git"
	"github.com/kristianweb/zephyr/internal/navigator"
	"github.com/kristianweb/zephyr/internal/vim"
)

// A tab's git diff is loaded when the tab's state is first built, and that
// happens before the navigator is switched on — which is what creates the git
// cache. Turning the navigator on therefore has to load the diff for the tabs
// already open, or their gutters stay blank until something else (a save, a
// watcher event) happens to refresh them.
func TestNavigatorToggleLoadsDiffForOpenTabs(t *testing.T) {
	repo, path := headViewRepo(t)
	st, ed, ts := testAppWithText(workingText, "Plain Text")
	ed.FilePath = path
	st.vimEnabled = true
	st.vimState = vim.NewState()
	st.navigator = navigator.New()
	// Pre-set so the toggle skips root detection, which persists config.
	st.navRoot = repo.Root
	st.gitRepo = repo
	st.gitCache = git.NewCache(repo)

	if ts.gitDiff != nil {
		t.Fatal("tab already carried a diff before the navigator was on")
	}
	st.toggleNavigatorMode()

	if ts.gitDiff == nil {
		t.Fatal("navigator on: tab still has no diff, so its gutter shows nothing")
	}
	if got := ts.gitDiff.LineStatus(2); got != '~' {
		t.Errorf("LineStatus(2) = %q, want '~' for the working copy's changed line", got)
	}
}
