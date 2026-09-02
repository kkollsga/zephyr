package editor

import (
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/buffer"
)

// The replay net: every editor method that changes the buffer must record
// exactly one history entry doing it, and undoing those entries one at a time
// must walk the document back through the same states it passed through
// forward. An unrecorded mutation breaks both halves at once — the state it
// produced is never restored, and every older entry on the stack is left
// holding byte offsets computed against a buffer that has since changed length.

type opKind int

const (
	opInsert opKind = iota
	opBackspace
	opDeleteForward
	opSelectType
	opSelectDelete
	opIndent
	opDedent
	opMultiInsert
	opJoin
	opReplaceRune
	opTransact
	numOpKinds
)

var opKindNames = [...]string{
	"insert", "backspace", "delete-forward", "select+type", "select+delete",
	"indent", "dedent", "multi-cursor insert", "join", "replace-rune",
	"transact",
}

// op is one scripted editor command. Positions are clamped to the buffer when
// applied, so a script stays valid as the document shrinks under it.
type op struct {
	kind    opKind
	line    int
	col     int
	endLine int
	endCol  int
	n       int // extra cursors for opMultiInsert
	text    string
	r       rune
}

func (o op) String() string {
	return fmt.Sprintf("%s@%d:%d text=%q", opKindNames[o.kind], o.line, o.col, o.text)
}

func setCursor(ed *Editor, line, col int) {
	ed.Selection.Clear()
	ed.Cursor.SetPosition(ed.Buffer, line, col)
}

func applyOp(ed *Editor, o op) {
	switch o.kind {
	case opInsert:
		setCursor(ed, o.line, o.col)
		ed.InsertText(o.text)
	case opBackspace:
		setCursor(ed, o.line, o.col)
		ed.DeleteBackward()
	case opDeleteForward:
		setCursor(ed, o.line, o.col)
		ed.DeleteForward()
	case opSelectType:
		ed.Selection.Clear()
		selectRange(ed, o.line, o.col, o.endLine, o.endCol)
		ed.InsertText(o.text)
	case opSelectDelete:
		ed.Selection.Clear()
		selectRange(ed, o.line, o.col, o.endLine, o.endCol)
		ed.DeleteSelection()
	case opIndent:
		setCursor(ed, o.line, 0)
		ed.IndentLine(ed.Cursor.Line)
	case opDedent:
		setCursor(ed, o.line, 0)
		ed.DedentLine(ed.Cursor.Line)
	case opMultiInsert:
		setCursor(ed, o.line, o.col)
		for i := 0; i < o.n; i++ {
			ed.AddCursorBelow()
		}
		ed.InsertTextAtAllCursors(o.text)
	case opJoin:
		setCursor(ed, o.line, o.col)
		ed.JoinLines()
	case opReplaceRune:
		setCursor(ed, o.line, o.col)
		ed.ReplaceRuneAtCursor(o.r)
	case opTransact:
		setCursor(ed, o.line, o.col)
		ed.Transact(func() {
			ed.InsertText(o.text)
			ed.DeleteBackward()
			ed.InsertText(o.text)
		})
	}
	ed.Selection.Clear()
	ed.ClearExtraCursors()
}

func checkCursorInBounds(t *testing.T, ed *Editor, where string) {
	t.Helper()
	if ed.Cursor.Line < 0 || ed.Cursor.Line >= ed.Buffer.LineCount() {
		t.Fatalf("%s: cursor line %d outside [0, %d)", where, ed.Cursor.Line, ed.Buffer.LineCount())
	}
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		t.Fatalf("%s: Line(%d): %v", where, ed.Cursor.Line, err)
	}
	if n := utf8.RuneCountInString(line); ed.Cursor.Col < 0 || ed.Cursor.Col > n {
		t.Fatalf("%s: cursor col %d outside [0, %d] on %q", where, ed.Cursor.Col, n, line)
	}
}

// replayOps applies ops to a fresh editor, then undoes one history entry at a
// time asserting the buffer walks back through the states it passed through,
// then redoes forward asserting the same sequence. Coalescing is switched off
// (a negative window makes the elapsed-time test fail even for two entries
// recorded within the same clock tick, which a zero window would not), so one
// buffer-changing op maps to exactly one history entry and the test never
// depends on how fast the machine runs.
func replayOps(t *testing.T, initial string, ops []op) {
	t.Helper()
	ed := NewEditor(buffer.NewFromString(initial), "")
	ed.History.SetCoalesceWindow(-1)

	states := make([]string, 0, len(ops))
	for i, o := range ops {
		before := ed.Buffer.Text()
		depth := len(ed.History.undoStack)
		applyOp(ed, o)
		after := ed.Buffer.Text()
		recorded := len(ed.History.undoStack) - depth

		switch {
		case after != before && recorded != 1:
			t.Fatalf("op %d (%s) changed the buffer but recorded %d history entries, want 1;\n"+
				"an unrecorded edit is unundoable and leaves every older entry holding stale offsets\n"+
				" before %q\n  after %q", i, o, recorded, before, after)
		case after == before && recorded != 0:
			t.Fatalf("op %d (%s) recorded %d history entries without changing the buffer", i, o, recorded)
		case after != before:
			states = append(states, after)
		}
		checkCursorInBounds(t, ed, fmt.Sprintf("after op %d (%s)", i, o))
	}

	for i := len(states) - 1; i >= 0; i-- {
		want := initial
		if i > 0 {
			want = states[i-1]
		}
		if !ed.Undo() {
			t.Fatalf("undo of entry %d refused with %d entries left", i, len(ed.History.undoStack))
		}
		if got := ed.Buffer.Text(); got != want {
			t.Fatalf("after undoing entry %d:\n got %q\nwant %q", i, got, want)
		}
		checkCursorInBounds(t, ed, fmt.Sprintf("after undoing entry %d", i))
	}
	if ed.History.CanUndo() {
		t.Fatalf("%d history entries left after undoing every recorded state", len(ed.History.undoStack))
	}
	if got := ed.Buffer.Text(); got != initial {
		t.Fatalf("full undo did not restore the initial text:\n got %q\nwant %q", got, initial)
	}

	for i, want := range states {
		if !ed.Redo() {
			t.Fatalf("redo of entry %d refused", i)
		}
		if got := ed.Buffer.Text(); got != want {
			t.Fatalf("after redoing entry %d:\n got %q\nwant %q", i, got, want)
		}
		checkCursorInBounds(t, ed, fmt.Sprintf("after redoing entry %d", i))
	}
	if ed.History.CanRedo() {
		t.Fatal("redo stack not empty at the top of the history")
	}
}

func TestEditor_HistoryReplay(t *testing.T) {
	scripts := []struct {
		name    string
		initial string
		ops     []op
	}{
		{
			name:    "typing and deleting",
			initial: "alpha\nbeta\ngamma",
			ops: []op{
				{kind: opInsert, line: 0, col: 5, text: " one"},
				{kind: opInsert, line: 1, col: 0, text: "X"},
				{kind: opBackspace, line: 1, col: 1},
				{kind: opDeleteForward, line: 2, col: 0},
				{kind: opInsert, line: 2, col: 4, text: "\nnew line"},
			},
		},
		{
			name:    "replace over selections",
			initial: "one two three\nfour five",
			ops: []op{
				{kind: opSelectType, line: 0, col: 4, endLine: 0, endCol: 7, text: "TWO"},
				{kind: opSelectDelete, line: 1, col: 0, endLine: 1, endCol: 5},
				{kind: opSelectType, line: 0, col: 0, endLine: 1, endCol: 4, text: "merged"},
				{kind: opSelectType, line: 0, col: 0, endLine: 0, endCol: 6, text: ""},
			},
		},
		{
			name:    "indent, join and replace-rune",
			initial: "    if x:\n        body\n    tail\n",
			ops: []op{
				{kind: opIndent, line: 1},
				{kind: opDedent, line: 0},
				{kind: opDedent, line: 0}, // no indentation left: a no-op
				{kind: opJoin, line: 1, col: 0},
				{kind: opReplaceRune, line: 0, col: 0, r: 'I'},
				{kind: opReplaceRune, line: 0, col: 99, r: 'Z'}, // past the line: a no-op
			},
		},
		{
			name:    "multibyte text",
			initial: "héllo wörld\nnäher\n🙂 emoji line",
			ops: []op{
				{kind: opInsert, line: 0, col: 11, text: "!"},
				{kind: opReplaceRune, line: 1, col: 2, r: 'ö'},
				{kind: opSelectType, line: 0, col: 1, endLine: 1, endCol: 2, text: "ü"},
				{kind: opJoin, line: 0, col: 0},
				{kind: opBackspace, line: 1, col: 2},
				{kind: opInsert, line: 1, col: 99, text: "日本語"},
			},
		},
		{
			name:    "multi-cursor and transactions",
			initial: "aa\nbb\ncc\ndd",
			ops: []op{
				{kind: opMultiInsert, line: 0, col: 1, n: 3, text: "-"},
				{kind: opTransact, line: 1, col: 0, text: "zz"},
				{kind: opMultiInsert, line: 2, col: 0, n: 1, text: "世"},
				{kind: opTransact, line: 3, col: 2, text: ""},
			},
		},
		{
			name:    "empty buffer",
			initial: "",
			ops: []op{
				{kind: opBackspace, line: 0, col: 0},     // nothing to delete
				{kind: opDeleteForward, line: 0, col: 0}, // nothing to delete
				{kind: opInsert, line: 0, col: 0, text: "first"},
				{kind: opJoin, line: 0, col: 0}, // no line below
				{kind: opInsert, line: 0, col: 5, text: "\nsecond"},
				{kind: opJoin, line: 0, col: 0},
			},
		},
	}

	for _, s := range scripts {
		t.Run(s.name, func(t *testing.T) {
			replayOps(t, s.initial, s.ops)
		})
	}
}

// fuzzPayloads keeps the generated text valid UTF-8 and mixes in newlines,
// whitespace and multibyte runes, which is where the byte/rune boundaries the
// editor has to keep straight actually differ.
var fuzzPayloads = []string{"x", "", " ", "\n", "ab\ncd", "    ", "é", "世界", "🙂", "\t"}

var fuzzRunes = []rune{'a', ' ', 'é', '世', '🙂', '\t'}

// decodeOps reads a fuzz-generated script. Six bytes per op; a trailing partial
// op is ignored.
func decodeOps(script []byte) []op {
	ops := make([]op, 0, len(script)/6)
	for i := 0; i+5 < len(script); i += 6 {
		ops = append(ops, op{
			kind:    opKind(int(script[i]) % int(numOpKinds)),
			line:    int(script[i+1] % 32),
			col:     int(script[i+2] % 32),
			endLine: int(script[i+3] % 32),
			endCol:  int(script[i+4] % 32),
			n:       int(script[i+4] % 4),
			text:    fuzzPayloads[int(script[i+5])%len(fuzzPayloads)],
			r:       fuzzRunes[int(script[i+5])%len(fuzzRunes)],
		})
	}
	return ops
}

// Inputs that once failed are kept under testdata/fuzz as regression seeds and
// run by plain `go test`, so a fix cannot silently come undone.
func FuzzEditorCommandSequence(f *testing.F) {
	f.Add([]byte("alpha\nbeta\n"), []byte{0, 0, 3, 0, 0, 4, 1, 1, 1, 0, 0, 0})
	f.Add([]byte("héllo wörld\n"), []byte{3, 0, 1, 0, 5, 7, 9, 0, 2, 0, 0, 6})
	f.Add([]byte(""), []byte{0, 0, 0, 0, 0, 4, 8, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0})
	f.Add([]byte("    a\n    b\n    c\n"), []byte{5, 1, 0, 0, 0, 0, 6, 0, 0, 0, 0, 0, 7, 0, 0, 0, 2, 1})
	f.Add([]byte("aa\nbb\ncc\n"), []byte{10, 0, 1, 0, 0, 4, 4, 0, 0, 2, 1, 0})

	f.Fuzz(func(t *testing.T, initial, script []byte) {
		// The editor's rune/byte arithmetic assumes the buffer holds valid
		// UTF-8, which is what the file loader guarantees.
		if len(initial) > 256 || len(script) > 300 || !utf8.Valid(initial) {
			t.Skip()
		}
		replayOps(t, string(initial), decodeOps(script))
	})
}
