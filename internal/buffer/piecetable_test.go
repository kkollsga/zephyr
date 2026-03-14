package buffer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPieceTable_NewFromString(t *testing.T) {
	pt := NewFromString("hello")
	if pt.Text() != "hello" {
		t.Fatalf("got %q, want %q", pt.Text(), "hello")
	}
	if pt.Length() != 5 {
		t.Fatalf("got length %d, want 5", pt.Length())
	}
}

func TestPieceTable_NewFromString_Empty(t *testing.T) {
	pt := NewFromString("")
	if pt.Text() != "" {
		t.Fatalf("got %q, want empty", pt.Text())
	}
	if pt.Length() != 0 {
		t.Fatalf("got length %d, want 0", pt.Length())
	}
}

func TestPieceTable_NewFromFile(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "small.txt")
	pt, err := NewFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pt.Length() == 0 {
		t.Fatal("expected non-empty file")
	}
	if pt.LineCount() < 2 {
		t.Fatalf("expected multiple lines, got %d", pt.LineCount())
	}
}

func TestPieceTable_InsertAtBeginning(t *testing.T) {
	pt := NewFromString("world")
	if err := pt.Insert(0, "hello "); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "hello world" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestPieceTable_InsertAtEnd(t *testing.T) {
	pt := NewFromString("hello")
	if err := pt.Insert(5, " world"); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "hello world" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestPieceTable_InsertAtMiddle(t *testing.T) {
	pt := NewFromString("helo")
	if err := pt.Insert(2, "l"); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "hello" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestPieceTable_InsertEmpty(t *testing.T) {
	pt := NewFromString("hello")
	if err := pt.Insert(0, ""); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "hello" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestPieceTable_InsertAtInvalidPos(t *testing.T) {
	pt := NewFromString("hello")
	if err := pt.Insert(-1, "x"); err == nil {
		t.Fatal("expected error for negative offset")
	}
	if err := pt.Insert(100, "x"); err == nil {
		t.Fatal("expected error for offset beyond length")
	}
}

func TestPieceTable_DeleteFromBeginning(t *testing.T) {
	pt := NewFromString("hello world")
	if err := pt.Delete(0, 6); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "world" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestPieceTable_DeleteFromEnd(t *testing.T) {
	pt := NewFromString("hello world")
	if err := pt.Delete(5, 6); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "hello" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestPieceTable_DeleteFromMiddle(t *testing.T) {
	pt := NewFromString("hello world")
	if err := pt.Delete(5, 1); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "helloworld" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestPieceTable_DeleteEntireContent(t *testing.T) {
	pt := NewFromString("hello")
	if err := pt.Delete(0, 5); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "" {
		t.Fatalf("got %q, want empty", pt.Text())
	}
	if pt.Length() != 0 {
		t.Fatalf("got length %d, want 0", pt.Length())
	}
}

func TestPieceTable_DeleteZeroLength(t *testing.T) {
	pt := NewFromString("hello")
	if err := pt.Delete(2, 0); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "hello" {
		t.Fatalf("got %q", pt.Text())
	}
}

func TestPieceTable_DeleteInvalidRange(t *testing.T) {
	pt := NewFromString("hello")
	if err := pt.Delete(-1, 1); err == nil {
		t.Fatal("expected error for negative offset")
	}
	if err := pt.Delete(0, 100); err == nil {
		t.Fatal("expected error for length beyond content")
	}
	if err := pt.Delete(3, 5); err == nil {
		t.Fatal("expected error for range beyond content")
	}
}

func TestPieceTable_SequentialInserts(t *testing.T) {
	pt := NewFromString("")
	for i := 0; i < 100; i++ {
		if err := pt.Insert(pt.Length(), fmt.Sprintf("line %d\n", i)); err != nil {
			t.Fatal(err)
		}
	}
	if pt.LineCount() != 101 { // 100 lines + trailing empty line after last \n
		t.Fatalf("got %d lines, want 101", pt.LineCount())
	}
}

func TestPieceTable_SequentialDeletes(t *testing.T) {
	pt := NewFromString("abcdefghij")
	// Delete one char at a time from the beginning
	for i := 0; i < 10; i++ {
		if err := pt.Delete(0, 1); err != nil {
			t.Fatal(err)
		}
	}
	if pt.Text() != "" {
		t.Fatalf("got %q, want empty", pt.Text())
	}
}

func TestPieceTable_InsertThenDelete(t *testing.T) {
	pt := NewFromString("hello world")
	if err := pt.Insert(5, " beautiful"); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "hello beautiful world" {
		t.Fatalf("after insert: got %q", pt.Text())
	}
	if err := pt.Delete(5, 10); err != nil {
		t.Fatal(err)
	}
	if pt.Text() != "hello world" {
		t.Fatalf("after delete: got %q", pt.Text())
	}
}

func TestPieceTable_LineCount(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 1},
		{"hello", 1},
		{"hello\n", 2},
		{"a\nb\nc", 3},
		{"a\nb\nc\n", 4},
		{"\n", 2},
		{"\n\n", 3},
	}
	for _, tt := range tests {
		pt := NewFromString(tt.input)
		if got := pt.LineCount(); got != tt.want {
			t.Errorf("LineCount(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestPieceTable_LineByNumber(t *testing.T) {
	pt := NewFromString("first\nsecond\nthird")
	tests := []struct {
		line int
		want string
	}{
		{0, "first"},
		{1, "second"},
		{2, "third"},
	}
	for _, tt := range tests {
		got, err := pt.Line(tt.line)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("Line(%d) = %q, want %q", tt.line, got, tt.want)
		}
	}

	// Out of range
	if _, err := pt.Line(-1); err == nil {
		t.Fatal("expected error for negative line")
	}
	if _, err := pt.Line(100); err == nil {
		t.Fatal("expected error for line beyond count")
	}
}

func TestPieceTable_Substring(t *testing.T) {
	pt := NewFromString("hello world")
	got, err := pt.Substring(6, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got != "world" {
		t.Fatalf("got %q, want %q", got, "world")
	}

	// Empty substring
	got, err = pt.Substring(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}

	// Invalid range
	if _, err := pt.Substring(-1, 1); err == nil {
		t.Fatal("expected error")
	}
	if _, err := pt.Substring(0, 100); err == nil {
		t.Fatal("expected error")
	}
}

func TestPieceTable_LargeFile(t *testing.T) {
	// Build a 100K-line string
	var b strings.Builder
	for i := 0; i < 100_000; i++ {
		fmt.Fprintf(&b, "Line %d: some content here for testing\n", i)
	}
	content := b.String()
	pt := NewFromString(content)

	if pt.LineCount() != 100_001 { // 100K lines + trailing empty line
		t.Fatalf("got %d lines, want 100001", pt.LineCount())
	}

	// Insert in the middle
	mid := pt.Length() / 2
	if err := pt.Insert(mid, "INSERTED\n"); err != nil {
		t.Fatal(err)
	}
	if pt.LineCount() != 100_002 {
		t.Fatalf("after insert: got %d lines", pt.LineCount())
	}

	// Delete the inserted text
	if err := pt.Delete(mid, 9); err != nil {
		t.Fatal(err)
	}
}

func TestPieceTable_Unicode(t *testing.T) {
	pt := NewFromString("Hello, 世界! 🌍")
	if pt.Text() != "Hello, 世界! 🌍" {
		t.Fatalf("got %q", pt.Text())
	}

	// Insert after the Chinese chars (byte offsets, not rune offsets)
	// "Hello, 世界" is 7 + 6 = 13 bytes
	if err := pt.Insert(13, "🎉"); err != nil {
		t.Fatal(err)
	}
	want := "Hello, 世界🎉! 🌍"
	if pt.Text() != want {
		t.Fatalf("got %q, want %q", pt.Text(), want)
	}
}

func TestPieceTable_ConcurrentReads(t *testing.T) {
	pt := NewFromString("hello world\nsecond line\nthird line\n")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pt.Text()
			_ = pt.Length()
			_ = pt.LineCount()
			_, _ = pt.Line(0)
			_, _ = pt.Substring(0, 5)
		}()
	}
	wg.Wait()
}

// --- Benchmarks ---

func BenchmarkPieceTable_InsertSingle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pt := NewFromString("hello world")
		_ = pt.Insert(5, "X")
	}
}

func BenchmarkPieceTable_InsertLargeFile(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 100_000; i++ {
		fmt.Fprintf(&sb, "Line %d: some content\n", i)
	}
	content := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt := NewFromString(content)
		_ = pt.Insert(pt.Length()/2, "inserted text")
	}
}

func BenchmarkPieceTable_DeleteLargeFile(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 100_000; i++ {
		fmt.Fprintf(&sb, "Line %d: some content\n", i)
	}
	content := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt := NewFromString(content)
		_ = pt.Delete(pt.Length()/2, 10)
	}
}

func BenchmarkPieceTable_LineAccess(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 100_000; i++ {
		fmt.Fprintf(&sb, "Line %d: some content\n", i)
	}
	content := sb.String()
	pt := NewFromString(content)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pt.Line(50_000)
	}
}

// Generate testdata/large.txt for manual testing
func TestGenerateLargeTestFile(t *testing.T) {
	if os.Getenv("GENERATE_TESTDATA") == "" {
		t.Skip("set GENERATE_TESTDATA=1 to generate large test file")
	}
	var b strings.Builder
	for i := 0; i < 100_000; i++ {
		fmt.Fprintf(&b, "Line %06d: The quick brown fox jumps over the lazy dog\n", i)
	}
	path := filepath.Join("..", "..", "testdata", "large.txt")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
}
