package highlight

import (
	"strings"
	"testing"
)

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestErrorLines_ValidJS(t *testing.T) {
	h := NewHighlighterForLanguage("JavaScript")
	if h == nil {
		t.Fatal("expected JavaScript highlighter")
	}
	defer h.Close()

	h.Parse([]byte("let x = 1;\nfunction f() { return x + 1; }\n"))
	if got := h.ErrorLines(50); len(got) != 0 {
		t.Fatalf("valid JS reported errors: %v", got)
	}
}

func TestErrorLines_StrayToken(t *testing.T) {
	h := NewHighlighterForLanguage("JavaScript")
	if h == nil {
		t.Fatal("expected JavaScript highlighter")
	}
	defer h.Close()

	// Garbage on line index 2.
	h.Parse([]byte("let a = 1;\nlet b = 2;\n@@@ %%% garbage\nlet c = 3;\n"))
	got := h.ErrorLines(50)
	if len(got) == 0 {
		t.Fatal("expected an error for stray tokens")
	}
	if !containsInt(got, 2) {
		t.Fatalf("expected error on line 2, got %v", got)
	}
}

func TestErrorLines_MissingBrace(t *testing.T) {
	h := NewHighlighterForLanguage("JavaScript")
	if h == nil {
		t.Fatal("expected JavaScript highlighter")
	}
	defer h.Close()

	// Unclosed function body yields a MISSING "}" node.
	h.Parse([]byte("function f() {\n  return 1;\n"))
	if got := h.ErrorLines(50); len(got) == 0 {
		t.Fatal("expected an error for the unclosed brace")
	}
}

func TestErrorLines_CapRespected(t *testing.T) {
	h := NewHighlighterForLanguage("JavaScript")
	if h == nil {
		t.Fatal("expected JavaScript highlighter")
	}
	defer h.Close()

	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("let x = 1;\n") // valid statement...
		b.WriteString("@@@\n")        // ...separating a distinct stray-token error
	}
	h.Parse([]byte(b.String()))
	got := h.ErrorLines(50)
	if len(got) > 50 {
		t.Fatalf("cap not respected: got %d markers", len(got))
	}
	if len(got) == 0 {
		t.Fatal("expected errors for 200 malformed lines")
	}
	// Results must be sorted ascending.
	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] {
			t.Fatalf("results not sorted: %v", got)
		}
	}
}

func TestErrorLines_SimpleLanguageNoTree(t *testing.T) {
	// JSON uses a simple tokenizer (no parse tree); ErrorLines must return nil
	// rather than panic.
	h := NewHighlighterForLanguage("JSON")
	if h == nil {
		t.Fatal("expected JSON highlighter")
	}
	defer h.Close()
	h.Parse([]byte("{ not valid"))
	if got := h.ErrorLines(50); got != nil {
		t.Fatalf("simple language should yield no tree-walk markers, got %v", got)
	}
}
