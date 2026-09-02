package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kristianweb/zephyr/internal/navigator"
)

// dirBufferTab wires a tab onto a real directory so the listing rebuilds have
// something to regenerate.
func dirBufferTab(t *testing.T) (*appState, *tabState) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"alpha.go", "beta.go", ".hidden"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	db := navigator.NewDirBuffer(dir, nil, nil, "")
	st, ed, ts := testAppWithText(db.GenerateText(), "Plain Text")
	ts.bufType = bufDirectory
	ts.dirBuf = db
	ed.Cursor.SetPosition(ed.Buffer, 2, 0)
	return st, ts
}

// TestNavToggleHidden_ResyncsDerivedState pins the buffer-swap contract for the
// directory listing rebuild: the replaced buffer leaves no pending incremental
// edits behind, and the viewport's cursor cache is invalidated so the next
// frame re-derives it against the new document.
func TestNavToggleHidden_ResyncsDerivedState(t *testing.T) {
	st, ts := dirBufferTab(t)
	ed := st.activeEd()
	before := ed.Buffer
	ts.lastCursorLine = 7
	ts.lastCursorCol = 3

	st.navToggleHidden()

	if ed.Buffer == before {
		t.Fatal("navToggleHidden did not rebuild the listing buffer")
	}
	if edits := ed.Buffer.DrainEdits(); len(edits) != 0 {
		t.Fatalf("listing rebuild left %d pending edits on the new buffer", len(edits))
	}
	if ts.lastCursorLine != -1 || ts.lastCursorCol != -1 {
		t.Fatalf("listing rebuild left lastCursor (%d,%d), want (-1,-1)",
			ts.lastCursorLine, ts.lastCursorCol)
	}
	if ts.foldState.DisplayLineCount() != ed.Buffer.LineCount() {
		t.Fatalf("fold state covers %d lines, buffer has %d",
			ts.foldState.DisplayLineCount(), ed.Buffer.LineCount())
	}
}
