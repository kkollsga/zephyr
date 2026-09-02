package editor

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
)

func TestJoinLines_DeletesTheNextLineByBytes(t *testing.T) {
	const src = "first\n  héllo wörld\nthird"
	ed := NewEditor(buffer.NewFromString(src), "")
	ed.History.SetCoalesceWindow(0)
	if !ed.JoinLines() {
		t.Fatal("expected a join")
	}
	if got := ed.Buffer.Text(); got != "first héllo wörld\nthird" {
		t.Fatalf("join = %q", got)
	}
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("undo of join = %q", ed.Buffer.Text())
	}
}

func TestJoinLines_LastLineIsANoOp(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("only"), "")
	if ed.JoinLines() {
		t.Fatal("joined past the last line")
	}
	if ed.History.CanUndo() {
		t.Fatal("a no-op join recorded history")
	}
}

func TestJoinLines_BlankNextLineLeavesNoSpace(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("first\n   \nthird"), "")
	if !ed.JoinLines() {
		t.Fatal("expected a join")
	}
	if got := ed.Buffer.Text(); got != "first\nthird" {
		t.Fatalf("join of a blank line = %q", got)
	}
}

func TestReplaceRunes_MultibyteAndClamping(t *testing.T) {
	const src = "aébc"
	ed := NewEditor(buffer.NewFromString(src), "")
	ed.History.SetCoalesceWindow(0)
	ed.Cursor.SetPosition(ed.Buffer, 0, 1)
	if !ed.ReplaceRunes(2, 'X') {
		t.Fatal("expected a replacement")
	}
	if got := ed.Buffer.Text(); got != "aXXc" {
		t.Fatalf("2rX over a multibyte rune = %q, want %q", got, "aXXc")
	}
	// A count past the end of the line replaces only what is left there.
	if !ed.Undo() || ed.Buffer.Text() != src {
		t.Fatalf("undo = %q", ed.Buffer.Text())
	}
	ed.Cursor.SetPosition(ed.Buffer, 0, 3)
	ed.ReplaceRunes(9, 'Z')
	if got := ed.Buffer.Text(); got != "aébZ" {
		t.Fatalf("clamped replace = %q, want %q", got, "aébZ")
	}
}

func TestReplaceRunes_AtEndOfLineRecordsNothing(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("ab\ncd"), "")
	ed.Cursor.SetPosition(ed.Buffer, 0, 2)
	if ed.ReplaceRunes(1, 'X') {
		t.Fatal("replaced past the end of the line")
	}
	if ed.Buffer.Text() != "ab\ncd" || ed.History.CanUndo() {
		t.Fatalf("buffer or history touched: %q", ed.Buffer.Text())
	}
}

func TestIndentDedentLine(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("alpha\n  beta\n\tgamma"), "")
	ed.History.SetCoalesceWindow(0)

	if !ed.IndentLine(0) {
		t.Fatal("indent did nothing")
	}
	if line, _ := ed.Buffer.Line(0); line != "    alpha" {
		t.Fatalf("indent = %q", line)
	}
	if !ed.DedentLine(0) {
		t.Fatal("dedent did nothing")
	}
	if line, _ := ed.Buffer.Line(0); line != "alpha" {
		t.Fatalf("dedent = %q", line)
	}
	// A partial indent is removed whole.
	if !ed.DedentLine(1) {
		t.Fatal("dedent of a two-space line did nothing")
	}
	if line, _ := ed.Buffer.Line(1); line != "beta" {
		t.Fatalf("dedent of two spaces = %q", line)
	}
	// A tab-indented line has no leading spaces to remove.
	if ed.DedentLine(2) {
		t.Fatal("dedent claimed to change a tab-indented line")
	}
}

// Overwriting a rune with itself changes nothing, so it must not leave an undo
// step behind. Surfaced by the history replay fuzzer.
func TestReplaceRuneAtCursor_SameRuneRecordsNothing(t *testing.T) {
	ed := NewEditor(buffer.NewFromString("🙂 hello"), "")
	ed.Cursor.SetPosition(ed.Buffer, 0, 0)

	ed.ReplaceRuneAtCursor('🙂')

	if got := ed.Buffer.Text(); got != "🙂 hello" {
		t.Fatalf("buffer changed: %q", got)
	}
	if ed.History.CanUndo() {
		t.Fatal("replacing a rune with itself recorded an undo step")
	}
}
