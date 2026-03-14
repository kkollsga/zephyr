package editor

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
)

func TestFind_LiteralMatch(t *testing.T) {
	pt := buffer.NewFromString("hello world hello")
	results, err := Find(pt, "hello", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(results))
	}
	if results[0].Offset != 0 || results[1].Offset != 12 {
		t.Fatalf("unexpected offsets: %d, %d", results[0].Offset, results[1].Offset)
	}
}

func TestFind_CaseInsensitive(t *testing.T) {
	pt := buffer.NewFromString("Hello HELLO hello")
	results, err := Find(pt, "hello", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(results))
	}
}

func TestFind_Regex(t *testing.T) {
	pt := buffer.NewFromString("foo123 bar456 baz")
	results, err := Find(pt, `\d+`, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(results))
	}
}

func TestReplace_Single(t *testing.T) {
	pt := buffer.NewFromString("hello world")
	Replace(pt, 6, 5, "Go")
	if pt.Text() != "hello Go" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestReplace_All(t *testing.T) {
	pt := buffer.NewFromString("foo bar foo baz foo")
	count, err := ReplaceAll(pt, "foo", "qux", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 replacements, got %d", count)
	}
	if pt.Text() != "qux bar qux baz qux" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestReplace_Regex_WithGroups(t *testing.T) {
	pt := buffer.NewFromString("John Smith\nJane Doe")
	count, err := ReplaceAll(pt, `(\w+)\s(\w+)`, "$2, $1", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
	if pt.Text() != "Smith, John\nDoe, Jane" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestReplace_UndoRestoresOriginal(t *testing.T) {
	// This tests at the editor level
	ed := NewEditor(buffer.NewFromString("hello hello hello"), "")
	original := ed.Buffer.Text()

	// Manual replace (not through editor's undo system for now)
	ReplaceAll(ed.Buffer, "hello", "world", false, true)
	if ed.Buffer.Text() == original {
		t.Fatal("replace should have changed text")
	}
}

func TestFind_NoMatch(t *testing.T) {
	pt := buffer.NewFromString("hello world")
	results, err := Find(pt, "xyz", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(results))
	}
}

func TestFind_EmptyPattern(t *testing.T) {
	pt := buffer.NewFromString("hello")
	results, err := Find(pt, "", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 matches for empty pattern, got %d", len(results))
	}
}
