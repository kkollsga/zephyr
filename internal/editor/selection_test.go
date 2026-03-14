package editor

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
)

func TestSelection_ShiftRight_SelectsOneChar(t *testing.T) {
	pt := buffer.NewFromString("hello\nworld")
	s := NewSelection()
	c := Cursor{Line: 0, Col: 0}
	s.Start(c)
	c.Col = 1
	s.Update(c)
	got := s.Text(pt)
	if got != "h" {
		t.Fatalf("got %q, want %q", got, "h")
	}
}

func TestSelection_ShiftDown_SelectsMultipleLines(t *testing.T) {
	pt := buffer.NewFromString("hello\nworld\nfoo")
	s := NewSelection()
	s.Start(Cursor{Line: 0, Col: 2})
	s.Update(Cursor{Line: 1, Col: 3})
	got := s.Text(pt)
	if got != "llo\nwor" {
		t.Fatalf("got %q, want %q", got, "llo\nwor")
	}
}

func TestSelection_SelectAll(t *testing.T) {
	pt := buffer.NewFromString("hello\nworld")
	s := NewSelection()
	s.SelectAll(pt)
	got := s.Text(pt)
	if got != "hello\nworld" {
		t.Fatalf("got %q, want %q", got, "hello\nworld")
	}
}

func TestSelection_SelectedText_SingleLine(t *testing.T) {
	pt := buffer.NewFromString("hello world")
	s := NewSelection()
	s.Start(Cursor{Line: 0, Col: 0})
	s.Update(Cursor{Line: 0, Col: 5})
	got := s.Text(pt)
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestSelection_SelectedText_MultiLine(t *testing.T) {
	pt := buffer.NewFromString("first\nsecond\nthird")
	s := NewSelection()
	s.Start(Cursor{Line: 0, Col: 3})
	s.Update(Cursor{Line: 2, Col: 2})
	got := s.Text(pt)
	if got != "st\nsecond\nth" {
		t.Fatalf("got %q, want %q", got, "st\nsecond\nth")
	}
}

func TestSelection_DeleteSelected_RemovesText(t *testing.T) {
	pt := buffer.NewFromString("hello world")
	ed := NewEditor(pt, "")
	ed.Selection.Start(Cursor{Line: 0, Col: 5})
	ed.Selection.Update(Cursor{Line: 0, Col: 11})
	ed.DeleteSelection()
	if ed.Buffer.Text() != "hello" {
		t.Fatalf("got %q, want %q", ed.Buffer.Text(), "hello")
	}
}

func TestSelection_ReplaceSelected_WithTyping(t *testing.T) {
	pt := buffer.NewFromString("hello world")
	ed := NewEditor(pt, "")
	ed.Selection.Start(Cursor{Line: 0, Col: 6})
	ed.Selection.Update(Cursor{Line: 0, Col: 11})
	ed.InsertText("Go")
	if ed.Buffer.Text() != "hello Go" {
		t.Fatalf("got %q, want %q", ed.Buffer.Text(), "hello Go")
	}
}

func TestSelection_DoubleClick_SelectsWord(t *testing.T) {
	pt := buffer.NewFromString("hello world")
	s := NewSelection()
	s.SelectWord(pt, Cursor{Line: 0, Col: 7})
	got := s.Text(pt)
	if got != "world" {
		t.Fatalf("got %q, want %q", got, "world")
	}
}

func TestSelection_TripleClick_SelectsLine(t *testing.T) {
	pt := buffer.NewFromString("hello\nworld\nfoo")
	s := NewSelection()
	s.SelectLine(pt, 1)
	got := s.Text(pt)
	if got != "world" {
		t.Fatalf("got %q, want %q", got, "world")
	}
}
