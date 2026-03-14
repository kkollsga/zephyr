package highlight

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestHighlighter_ParseGo_Keywords(t *testing.T) {
	h := NewHighlighter("test.go")
	if h == nil {
		t.Fatal("expected highlighter for .go")
	}
	defer h.Close()

	src := []byte("package main\n\nfunc main() {}\n")
	h.Parse(src)
	tokens := h.Tokens()

	hasKeyword := false
	for _, tok := range tokens {
		if tok.Type == TokenKeyword {
			text := string(src[tok.StartByte:tok.EndByte])
			if text == "package" || text == "func" {
				hasKeyword = true
			}
		}
	}
	if !hasKeyword {
		t.Fatal("expected keyword tokens for 'package' or 'func'")
	}
}

func TestHighlighter_ParseGo_Strings(t *testing.T) {
	h := NewHighlighter("test.go")
	if h == nil {
		t.Fatal("expected highlighter")
	}
	defer h.Close()

	src := []byte("package main\nvar s = \"hello world\"\n")
	h.Parse(src)
	tokens := h.Tokens()

	hasString := false
	for _, tok := range tokens {
		if tok.Type == TokenString {
			hasString = true
		}
	}
	if !hasString {
		t.Fatal("expected string token")
	}
}

func TestHighlighter_ParseGo_Comments(t *testing.T) {
	h := NewHighlighter("test.go")
	if h == nil {
		t.Fatal("expected highlighter")
	}
	defer h.Close()

	src := []byte("package main\n// this is a comment\n")
	h.Parse(src)
	tokens := h.Tokens()

	hasComment := false
	for _, tok := range tokens {
		if tok.Type == TokenComment {
			hasComment = true
		}
	}
	if !hasComment {
		t.Fatal("expected comment token")
	}
}

func TestHighlighter_ParsePython_Keywords(t *testing.T) {
	h := NewHighlighter("test.py")
	if h == nil {
		t.Fatal("expected highlighter for .py")
	}
	defer h.Close()

	src := []byte("def hello():\n    return 42\n")
	h.Parse(src)
	tokens := h.Tokens()

	hasKeyword := false
	for _, tok := range tokens {
		if tok.Type == TokenKeyword {
			text := string(src[tok.StartByte:tok.EndByte])
			if text == "def" || text == "return" {
				hasKeyword = true
			}
		}
	}
	if !hasKeyword {
		t.Fatal("expected keyword tokens")
	}
}

func TestHighlighter_ParseJS_ArrowFunction(t *testing.T) {
	h := NewHighlighter("test.js")
	if h == nil {
		t.Fatal("expected highlighter for .js")
	}
	defer h.Close()

	src := []byte("const add = (a, b) => a + b;\n")
	h.Parse(src)
	tokens := h.Tokens()

	hasKeyword := false
	for _, tok := range tokens {
		if tok.Type == TokenKeyword {
			text := string(src[tok.StartByte:tok.EndByte])
			if text == "const" {
				hasKeyword = true
			}
		}
	}
	if !hasKeyword {
		t.Fatal("expected 'const' keyword")
	}
}

func TestHighlighter_IncrementalParse(t *testing.T) {
	h := NewHighlighter("test.go")
	if h == nil {
		t.Fatal("expected highlighter")
	}
	defer h.Close()

	src := []byte("package main\nfunc main() {}\n")
	h.Parse(src)

	newSrc := []byte("package main\n// comment\nfunc main() {}\n")
	h.Update(newSrc, sitter.EditInput{
		StartIndex:  13,
		OldEndIndex: 13,
		NewEndIndex: 24,
		StartPoint:  sitter.Point{Row: 1, Column: 0},
		OldEndPoint: sitter.Point{Row: 1, Column: 0},
		NewEndPoint: sitter.Point{Row: 2, Column: 0},
	})

	tokens := h.Tokens()
	hasComment := false
	for _, tok := range tokens {
		if tok.Type == TokenComment {
			hasComment = true
		}
	}
	if !hasComment {
		t.Fatal("expected comment token after incremental update")
	}
}

func TestHighlighter_TokensForRange(t *testing.T) {
	h := NewHighlighter("test.go")
	if h == nil {
		t.Fatal("expected highlighter")
	}
	defer h.Close()

	src := []byte("package main\nvar x = 42\n")
	h.Parse(src)
	tokens := h.TokensForLineRange(13, 24)
	if len(tokens) == 0 {
		t.Fatal("expected tokens for line range")
	}
}

func TestHighlighter_UnknownLanguage(t *testing.T) {
	h := NewHighlighter("test.txt")
	if h != nil {
		t.Fatal("expected nil highlighter for .txt")
	}
}

func TestHighlighter_EmptyFile(t *testing.T) {
	h := NewHighlighter("test.go")
	if h == nil {
		t.Fatal("expected highlighter")
	}
	defer h.Close()

	h.Parse([]byte(""))
	tokens := h.Tokens()
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens for empty file, got %d", len(tokens))
	}
}

func TestHighlighter_SyntaxError(t *testing.T) {
	h := NewHighlighter("test.go")
	if h == nil {
		t.Fatal("expected highlighter")
	}
	defer h.Close()

	src := []byte("package main\nfunc {{{{ broken\n")
	h.Parse(src)
	_ = h.Tokens() // should not panic
}

// --- Benchmarks ---

func BenchmarkHighlighter_ParseLargeGoFile(b *testing.B) {
	src := generateLargeGoFile(1000)
	h := NewHighlighter("test.go")
	if h == nil {
		b.Fatal("expected highlighter")
	}
	defer h.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Parse(src)
	}
}

func BenchmarkHighlighter_IncrementalReparse(b *testing.B) {
	src := generateLargeGoFile(1000)
	h := NewHighlighter("test.go")
	if h == nil {
		b.Fatal("expected highlighter")
	}
	defer h.Close()
	h.Parse(src)

	newSrc := append([]byte("// new comment\n"), src...)
	edit := sitter.EditInput{
		StartIndex:  0,
		OldEndIndex: 0,
		NewEndIndex: 15,
		StartPoint:  sitter.Point{Row: 0, Column: 0},
		OldEndPoint: sitter.Point{Row: 0, Column: 0},
		NewEndPoint: sitter.Point{Row: 1, Column: 0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Update(newSrc, edit)
	}
}

func generateLargeGoFile(funcs int) []byte {
	var buf []byte
	buf = append(buf, "package main\n\nimport \"fmt\"\n\n"...)
	for i := 0; i < funcs; i++ {
		buf = append(buf, []byte("func function"+itoa(i)+"() {\n\tfmt.Println(\"hello world\")\n\tx := 42\n\t_ = x\n}\n\n")...)
	}
	return buf
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
