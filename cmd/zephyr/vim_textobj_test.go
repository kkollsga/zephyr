package main

import (
	"strings"
	"testing"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/git"
	"github.com/kristianweb/zephyr/internal/vim"
)

// hunkFixture is one hunk over a five-line file: line 1 context, lines 2-3
// changed, line 4 context, line 5 changed. The interior context line is the
// point of the fixture — a hunk is not a run of changed lines.
func hunkFixture() *git.FileDiff {
	return &git.FileDiff{
		Path:   "sample.txt",
		Status: 'M',
		Hunks: []git.Hunk{{
			OldStart: 1, OldCount: 5, NewStart: 1, NewCount: 5,
			Lines: []git.DiffLine{
				{Type: git.DiffLineContext, Content: "one"},
				{Type: git.DiffLineAdd, Content: "two"},
				{Type: git.DiffLineAdd, Content: "three"},
				{Type: git.DiffLineContext, Content: "four"},
				{Type: git.DiffLineAdd, Content: "five"},
			},
		}},
	}
}

func TestHunkChangedLineRange(t *testing.T) {
	fd := hunkFixture()
	// A second file whose only hunk sits far below the cursor lines probed.
	far := &git.FileDiff{Hunks: []git.Hunk{{
		OldStart: 40, OldCount: 1, NewStart: 40, NewCount: 1,
		Lines: []git.DiffLine{{Type: git.DiffLineAdd, Content: "x"}},
	}}}

	tests := []struct {
		name               string
		fd                 *git.FileDiff
		line               int
		wantStart, wantEnd int
		wantOK             bool
	}{
		{"first changed line of a run", fd, 2, 2, 3, true},
		{"last changed line of a run", fd, 3, 2, 3, true},
		{"context line inside the hunk", fd, 4, 0, 0, false},
		{"context line at the hunk start", fd, 1, 0, 0, false},
		{"single changed line at EOF", fd, 5, 5, 5, true},
		{"line past the hunk", fd, 6, 0, 0, false},
		{"line outside any hunk", far, 3, 0, 0, false},
		{"no diff for this file", nil, 3, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, ok := hunkChangedLineRange(tt.fd, tt.line)
			if ok != tt.wantOK || start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("hunkChangedLineRange(line %d) = (%d,%d,%v), want (%d,%d,%v)",
					tt.line, start, end, ok, tt.wantStart, tt.wantEnd, tt.wantOK)
			}
		})
	}
}

// hunkObjApp builds an app whose buffer matches hunkFixture's five lines.
func hunkObjApp(t *testing.T, text string) (*appState, *editor.Editor, *tabState) {
	t.Helper()
	st, ed, ts := testAppWithText(text, "Plain Text")
	st.vimState = vim.NewState()
	ts.gitDiff = hunkFixture()
	return st, ed, ts
}

func TestHunkTextObjectIsLinewiseOverChangedLinesOnly(t *testing.T) {
	const src = "one\ntwo\nthree\nfour\nfive\n"
	dih := vim.Action{Kind: vim.ActionDelete, TextObj: 'h', TextObjType: 'i'}

	t.Run("dih removes whole changed lines", func(t *testing.T) {
		st, ed, _ := hunkObjApp(t, src)
		ed.Cursor.SetPosition(ed.Buffer, 1, 0) // buffer line 1 == new-file line 2
		st.executeVimAction(dih)
		if got := ed.Buffer.Text(); got != "one\nfour\nfive\n" {
			t.Fatalf("dih text = %q", got)
		}
		if reg := st.vimState.Registers.Get('"'); reg != "two\nthree\n" {
			t.Fatalf("dih register = %q", reg)
		}
		st.undoSteps(1)
		if got := ed.Buffer.Text(); got != src {
			t.Fatalf("one undo after dih = %q, want the original text", got)
		}
	})

	t.Run("cursor on a context line inside the hunk is a no-op", func(t *testing.T) {
		st, ed, _ := hunkObjApp(t, src)
		ed.Cursor.SetPosition(ed.Buffer, 3, 0) // "four", context
		st.executeVimAction(dih)
		if got := ed.Buffer.Text(); got != src {
			t.Fatalf("dih on a context line changed the buffer: %q", got)
		}
		if ed.History.CanUndo() {
			t.Fatal("dih on a context line recorded an undo step")
		}
	})

	t.Run("cursor outside any hunk is a no-op", func(t *testing.T) {
		st, ed, ts := hunkObjApp(t, src)
		ts.gitDiff = &git.FileDiff{Hunks: []git.Hunk{{
			OldStart: 40, OldCount: 1, NewStart: 40, NewCount: 1,
			Lines: []git.DiffLine{{Type: git.DiffLineAdd, Content: "x"}},
		}}}
		ed.Cursor.SetPosition(ed.Buffer, 1, 0)
		st.executeVimAction(dih)
		if got := ed.Buffer.Text(); got != src {
			t.Fatalf("dih outside a hunk changed the buffer: %q", got)
		}
	})

	t.Run("hunk at EOF leaves no stranded newline", func(t *testing.T) {
		st, ed, _ := hunkObjApp(t, src)
		ed.Cursor.SetPosition(ed.Buffer, 4, 0) // "five", last real line
		st.executeVimAction(dih)
		if got := ed.Buffer.Text(); got != "one\ntwo\nthree\nfour\n" {
			t.Fatalf("dih at EOF = %q", got)
		}
	})

	t.Run("hunk at EOF without a trailing newline", func(t *testing.T) {
		st, ed, _ := hunkObjApp(t, "one\ntwo\nthree\nfour\nfive")
		ed.Cursor.SetPosition(ed.Buffer, 4, 0)
		st.executeVimAction(dih)
		if got := ed.Buffer.Text(); got != "one\ntwo\nthree\nfour" {
			t.Fatalf("dih at EOF without trailing newline = %q", got)
		}
	})

	t.Run("yih yanks linewise so p pastes on its own line", func(t *testing.T) {
		st, ed, _ := hunkObjApp(t, src)
		ed.Cursor.SetPosition(ed.Buffer, 2, 0)
		st.executeVimAction(vim.Action{Kind: vim.ActionYank, TextObj: 'h', TextObjType: 'i'})
		if got := ed.Buffer.Text(); got != src {
			t.Fatalf("yih changed the buffer: %q", got)
		}
		reg := st.vimState.Registers.Get('"')
		if reg != "two\nthree\n" || !strings.HasSuffix(reg, "\n") {
			t.Fatalf("yih register = %q, want a linewise register", reg)
		}
		ed.Cursor.SetPosition(ed.Buffer, 3, 0) // "four"
		// Register 0 rather than the unnamed one: vimPut reads the system
		// clipboard for ", which would make the test depend on the host.
		st.vimPut(ed, vim.Action{Kind: vim.ActionPut, Register: '0'}, false)
		if got := ed.Buffer.Text(); got != "one\ntwo\nthree\nfour\ntwo\nthree\nfive\n" {
			t.Fatalf("p after yih = %q", got)
		}
	})

	t.Run("cih clears the lines and opens insert on an empty line", func(t *testing.T) {
		st, ed, _ := hunkObjApp(t, src)
		ed.Cursor.SetPosition(ed.Buffer, 1, 0)
		st.executeVimAction(vim.Action{Kind: vim.ActionChange, TextObj: 'h', TextObjType: 'i'})
		if got := ed.Buffer.Text(); got != "one\n\nfour\nfive\n" {
			t.Fatalf("cih text = %q", got)
		}
		if ed.Cursor.Line != 1 || ed.Cursor.Col != 0 {
			t.Fatalf("cih cursor = %+v, want line 1 col 0", ed.Cursor)
		}
		if st.vimState.Mode != vim.ModeInsert {
			t.Fatalf("cih mode = %v, want insert", st.vimState.Mode)
		}
		st.undoSteps(1)
		if got := ed.Buffer.Text(); got != src {
			t.Fatalf("one undo after cih = %q", got)
		}
	})
}
