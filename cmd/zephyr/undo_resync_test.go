package main

import (
	"bytes"
	"testing"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/highlight"
)

// Undoing a format replaces ed.Buffer with a fresh piece table. Everything the
// tab derives from the old one — the tree-sitter tree and its source snapshot,
// the fold regions, the find bar's byte offsets, the last-synced cursor — then
// describes a document that no longer exists. The 50 ms deferred reparse cannot
// repair that: it applies incremental edits taken from the new buffer to a tree
// parsed from the old one.
func TestUndoOfFormatResyncsDerivedState(t *testing.T) {
	expanded := "{\n  \"alpha\": 1,\n  \"beta\": [\n    2,\n    3\n  ]\n}"
	st, ed, ts := testAppWithText(expanded, "JSON")
	st.indentWidth = 2
	ts.highlighter = highlight.NewHighlighterForLanguage("JSON")
	if ts.highlighter == nil {
		t.Fatal("no JSON highlighter available")
	}
	defer ts.highlighter.Close()
	ts.sourceBuf = ed.Buffer.TextBytes(ts.sourceBuf)
	ts.highlighter.Parse(ts.sourceBuf)

	st.findBar.Visible = true
	st.findBar.Query = "alpha"
	st.updateSearchResults()

	st.toggleJSONCompact()
	ed.Selection.Clear()
	ed.Cursor.SetPosition(ed.Buffer, 0, 0)
	ed.InsertText(" ")
	st.afterEdit()

	st.undoSteps(1) // the typed space
	st.undoSteps(1) // the format: swaps the buffer wholesale

	if got := ed.Buffer.Text(); got != expanded {
		t.Fatalf("undo did not restore the expanded JSON: %q", got)
	}
	if want := ed.Buffer.TextBytes(nil); !bytes.Equal(ts.sourceBuf, want) {
		t.Fatalf("highlighter source is stale after the swap:\n got %q\nwant %q", ts.sourceBuf, want)
	}
	if edits := ed.Buffer.DrainEdits(); len(edits) != 0 {
		t.Fatalf("%d edits left pending across the swap; the next incremental "+
			"reparse would apply them to a tree parsed from the old buffer", len(edits))
	}
	if ts.lastCursorLine != -1 || ts.lastCursorCol != -1 {
		t.Fatalf("last-synced cursor not reset: %d:%d", ts.lastCursorLine, ts.lastCursorCol)
	}

	want, _ := editor.Find(ed.Buffer, "alpha", st.findBar.UseRegex, st.findBar.CaseSensitive)
	if len(want) != 1 {
		t.Fatalf("fixture: %d matches for alpha, want 1", len(want))
	}
	if len(st.findBar.Matches) != 1 || st.findBar.Matches[0].Offset != want[0].Offset {
		t.Fatalf("find matches not recomputed against the restored buffer: %+v, want %+v",
			st.findBar.Matches, want)
	}
}

// A buffer swap leaves the extra cursors pointing into the replaced document,
// so undo drops them rather than editing at stale positions.
func TestUndoClearsExtraCursors(t *testing.T) {
	st, ed, _ := testAppWithText("aa\nbb\ncc", "Go")
	ed.AddCursorBelow()
	ed.AddCursorBelow()
	ed.InsertTextAtAllCursors("x")
	if !ed.HasMultipleCursors() {
		t.Fatal("fixture: expected extra cursors before undo")
	}

	st.undoSteps(1)

	if ed.Buffer.Text() != "aa\nbb\ncc" {
		t.Fatalf("undo did not restore the buffer: %q", ed.Buffer.Text())
	}
	if len(ed.Cursors) != 0 || len(ed.Selections) != 0 {
		t.Fatalf("extra cursors survived undo: %d cursors, %d selections",
			len(ed.Cursors), len(ed.Selections))
	}
}
