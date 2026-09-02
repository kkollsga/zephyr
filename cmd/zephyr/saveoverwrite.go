package main

import "gioui.org/io/key"

// handleOverwriteKey routes a key while the Save As overwrite prompt is up.
//
// Return confirms here, unlike the clobber prompt where Return cancels. The
// difference is whose decision the prompt is questioning: an overwrite asks
// about a file the user just named themselves, so confirming loses only the
// version they deliberately targeted. A clobber prompt reports a change Zephyr
// found underneath them, where either answer discards work they have not seen.
//
// Escape goes back to the filename editor with the menu still open and the
// target untouched — the same as the prompt's Back button.
func (st *appState) handleOverwriteKey(ke key.Event) {
	switch ke.Name {
	case key.NameReturn:
		st.saveMenu.confirmOverwrite = false
		st.forceExecuteSaveAs()
	case key.NameEscape:
		st.saveMenu.confirmOverwrite = false
	}
}
