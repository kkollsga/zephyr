package buffer

import "testing"

func TestOffsetToLineCol_FirstLine(t *testing.T) {
	pt := NewFromString("hello\nworld\nfoo")
	lc, err := pt.OffsetToLineCol(3) // 'l' in "hello"
	if err != nil {
		t.Fatal(err)
	}
	if lc.Line != 0 || lc.Col != 3 {
		t.Fatalf("got %+v, want {0, 3}", lc)
	}
}

func TestOffsetToLineCol_MiddleLine(t *testing.T) {
	pt := NewFromString("hello\nworld\nfoo")
	lc, err := pt.OffsetToLineCol(8) // 'r' in "world"
	if err != nil {
		t.Fatal(err)
	}
	if lc.Line != 1 || lc.Col != 2 {
		t.Fatalf("got %+v, want {1, 2}", lc)
	}
}

func TestOffsetToLineCol_LastLine(t *testing.T) {
	pt := NewFromString("hello\nworld\nfoo")
	lc, err := pt.OffsetToLineCol(14) // 'o' (last char) in "foo"
	if err != nil {
		t.Fatal(err)
	}
	if lc.Line != 2 || lc.Col != 2 {
		t.Fatalf("got %+v, want {2, 2}", lc)
	}
}

func TestOffsetToLineCol_AtNewline(t *testing.T) {
	pt := NewFromString("hello\nworld")
	lc, err := pt.OffsetToLineCol(5) // the '\n' itself
	if err != nil {
		t.Fatal(err)
	}
	if lc.Line != 0 || lc.Col != 5 {
		t.Fatalf("got %+v, want {0, 5}", lc)
	}
}

func TestOffsetToLineCol_EndOfFile(t *testing.T) {
	pt := NewFromString("hello")
	lc, err := pt.OffsetToLineCol(5) // one past last char
	if err != nil {
		t.Fatal(err)
	}
	if lc.Line != 0 || lc.Col != 5 {
		t.Fatalf("got %+v, want {0, 5}", lc)
	}
}

func TestLineColToOffset_Roundtrip(t *testing.T) {
	pt := NewFromString("hello\nworld\nfoo bar\nbaz")
	offsets := []int{0, 3, 5, 6, 10, 12, 15, 20}
	for _, off := range offsets {
		lc, err := pt.OffsetToLineCol(off)
		if err != nil {
			t.Fatalf("OffsetToLineCol(%d): %v", off, err)
		}
		got, err := pt.LineColToOffset(lc)
		if err != nil {
			t.Fatalf("LineColToOffset(%+v): %v", lc, err)
		}
		if got != off {
			t.Errorf("roundtrip offset %d -> %+v -> %d", off, lc, got)
		}
	}
}

func TestLineColToOffset_InvalidLine(t *testing.T) {
	pt := NewFromString("hello\nworld")
	_, err := pt.LineColToOffset(LineCol{Line: 5, Col: 0})
	if err == nil {
		t.Fatal("expected error for invalid line")
	}
}

func TestLineColToOffset_InvalidCol(t *testing.T) {
	pt := NewFromString("hello\nworld")
	_, err := pt.LineColToOffset(LineCol{Line: 0, Col: 100})
	if err == nil {
		t.Fatal("expected error for column beyond line length")
	}
}

func TestLineColToOffset_NegativeValues(t *testing.T) {
	pt := NewFromString("hello")
	_, err := pt.LineColToOffset(LineCol{Line: -1, Col: 0})
	if err == nil {
		t.Fatal("expected error for negative line")
	}
}
