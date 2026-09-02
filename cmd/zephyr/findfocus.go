package main

import "gioui.org/io/key"

// findBarHasKeys reports whether the find bar owns the keyboard. The bar can be
// visible without owning it: a press in the editor hands the keyboard back
// while the matches stay highlighted.
func (st *appState) findBarHasKeys() bool {
	return st.findBar.Visible && st.findBar.Focused
}

// dispatchKey routes one key press to the overlay, vim, or editor handler.
func (st *appState) dispatchKey(ke key.Event) {
	if st.handleUnfocusedFindBarKey(ke) {
		return
	}
	if st.vimEnabled && st.vimState != nil &&
		!st.saveMenu.visible && !st.langSel.Visible && !st.findBarHasKeys() {
		st.handleVimKeyEvent(ke)
		return
	}
	st.handleKey(ke)
}

// handleUnfocusedFindBarKey handles the keys a visible-but-unfocused find bar
// still answers, and reports whether it consumed the event. It runs ahead of
// the vim handler so Escape closes the bar rather than being swallowed as
// "back to normal mode", matching how the save menu and language selector
// already take precedence over vim.
func (st *appState) handleUnfocusedFindBarKey(ke key.Event) bool {
	if !st.findBar.Visible || st.findBar.Focused ||
		st.saveMenu.visible || st.langSel.Visible {
		return false
	}
	switch {
	case ke.Name == key.NameEscape:
		st.findBar.Close()
		return true
	case ke.Name == "F" && ke.Modifiers == key.ModShortcut:
		st.openFindBar(false)
		return true
	case ke.Name == "H" && ke.Modifiers == key.ModShortcut:
		st.openFindBar(true)
		return true
	}
	return false
}

// blurFindBarForEditorPress releases the find bar's keyboard focus when a press
// lands in the editor. The bar stays open with its matches highlighted so the
// result set survives an edit; Cmd+F or a click in a field takes focus back.
func (st *appState) blurFindBarForEditorPress() {
	if !st.findBarHasKeys() {
		return
	}
	st.findBar.Blur()
	st.invalidate()
}
