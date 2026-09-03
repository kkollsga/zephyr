package main

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/vim"
)

// vimFeed drives keys through the state machine and the executor the way the
// key handler does, so a test exercises the parse and the execution together.
func vimFeed(st *appState, keys string) {
	for _, r := range keys {
		st.executeVimAction(st.vimState.HandleKey(vim.KeyInput{Char: r}))
		switch st.vimState.Mode {
		case vim.ModeVisual, vim.ModeVisualLine, vim.ModeVisualBlock:
			st.updateVimVisualSelection()
		}
	}
}

func visualApp(t *testing.T, text string, line, col int) (*appState, *editor.Editor, *tabState) {
	t.Helper()
	st, ed, ts := testAppWithText(text, "Plain Text")
	st.vimState = vim.NewState()
	ed.Cursor.SetPosition(ed.Buffer, line, col)
	return st, ed, ts
}

// The visual span an inner object selects is head-exclusive, which is the same
// span the operator form deletes: viwd and diw must agree exactly.
func TestVisualTextObjectMatchesOperatorForm(t *testing.T) {
	const src = "alpha beta gamma\n"
	for _, keys := range []struct{ visual, operator string }{
		{"viwd", "diw"},
		{"vawd", "daw"},
	} {
		t.Run(keys.visual, func(t *testing.T) {
			stV, edV, _ := visualApp(t, src, 0, 7)
			vimFeed(stV, keys.visual)
			stO, edO, _ := visualApp(t, src, 0, 7)
			vimFeed(stO, keys.operator)
			if edV.Buffer.Text() != edO.Buffer.Text() {
				t.Fatalf("%s = %q, %s = %q", keys.visual, edV.Buffer.Text(), keys.operator, edO.Buffer.Text())
			}
			if edV.Buffer.Text() == src {
				t.Fatalf("%s deleted nothing", keys.visual)
			}
		})
	}
}

func TestVisualTextObjectInsideQuotes(t *testing.T) {
	st, ed, _ := visualApp(t, "say \"hello world\" now\n", 0, 8)
	vimFeed(st, "vi\"")
	if got := ed.SelectedText(); got != "hello world" {
		t.Fatalf(`vi" selected %q, want "hello world"`, got)
	}
	if st.vimState.Mode != vim.ModeVisual {
		t.Fatalf(`vi" mode = %v, want visual`, st.vimState.Mode)
	}
}

func TestVisualTextObjectInsideParens(t *testing.T) {
	st, ed, _ := visualApp(t, "f(a, b)\n", 0, 3)
	vimFeed(st, "vi(")
	if got := ed.SelectedText(); got != "a, b" {
		t.Fatalf("vi( selected %q, want \"a, b\"", got)
	}
}

// vih is linewise: it selects the whole changed lines and switches to V-LINE.
func TestVisualHunkObjectIsLinewise(t *testing.T) {
	st, ed, _ := hunkObjApp(t, "one\ntwo\nthree\nfour\nfive\n")
	st.vimState = vim.NewState()
	ed.Cursor.SetPosition(ed.Buffer, 1, 1) // buffer line 1 == new-file line 2
	vimFeed(st, "vih")
	if st.vimState.Mode != vim.ModeVisualLine {
		t.Fatalf("vih mode = %v, want V-LINE", st.vimState.Mode)
	}
	if got := ed.SelectedText(); got != "two\nthree" {
		t.Fatalf("vih selected %q, want the whole changed lines", got)
	}
}

func TestVisualTextObjectNotFoundLeavesSelectionAlone(t *testing.T) {
	st, ed, _ := visualApp(t, "no quotes here\n", 0, 3)
	vimFeed(st, "vl") // v then l: a one-character selection
	before := ed.SelectedText()
	if before == "" {
		t.Fatal("setup selected nothing, so the assertion below cannot fail")
	}
	vimFeed(st, "i\"")
	if got := ed.SelectedText(); got != before {
		t.Fatalf(`vi" with no quotes changed the selection: %q, want %q`, got, before)
	}
	if st.vimState.Mode != vim.ModeVisual {
		t.Fatalf("mode = %v, want visual", st.vimState.Mode)
	}
}

// p is not a text object. Before the visual i/a branch existed the p of vip
// ran the visual put, which replaced the selection and left visual mode.
func TestVisualParagraphObjectIsRejected(t *testing.T) {
	const src = "alpha beta\n"
	st, ed, _ := visualApp(t, src, 0, 0)
	vimFeed(st, "vip")
	if st.vimState.Mode != vim.ModeVisual {
		t.Fatalf("vip mode = %v, want visual", st.vimState.Mode)
	}
	if got := ed.Buffer.Text(); got != src {
		t.Fatalf("vip changed the buffer: %q", got)
	}
}
