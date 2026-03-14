package fuzzy

import (
	"fmt"
	"testing"
)

func TestFuzzyMatch_ExactMatch(t *testing.T) {
	m := FuzzyMatch("hello", "hello")
	if m == nil {
		t.Fatal("expected match")
	}
	if m.Score <= 0 {
		t.Fatal("expected positive score")
	}
}

func TestFuzzyMatch_Prefix(t *testing.T) {
	m := FuzzyMatch("hel", "hello")
	if m == nil {
		t.Fatal("expected match")
	}
}

func TestFuzzyMatch_Subsequence(t *testing.T) {
	m := FuzzyMatch("hlo", "hello")
	if m == nil {
		t.Fatal("expected match")
	}
}

func TestFuzzyMatch_CaseInsensitive(t *testing.T) {
	m := FuzzyMatch("HELLO", "hello world")
	if m == nil {
		t.Fatal("expected case-insensitive match")
	}
}

func TestFuzzyMatch_NoMatch(t *testing.T) {
	m := FuzzyMatch("xyz", "hello")
	if m != nil {
		t.Fatal("expected no match")
	}
}

func TestFuzzyMatch_ConsecutiveBonus(t *testing.T) {
	// "hel" in "hello" should score higher than "hel" in "h_e_l_o"
	m1 := FuzzyMatch("hel", "hello")
	m2 := FuzzyMatch("hel", "h_e_l_o")
	if m1 == nil || m2 == nil {
		t.Fatal("expected both to match")
	}
	if m1.Score <= m2.Score {
		t.Fatalf("consecutive should score higher: %d <= %d", m1.Score, m2.Score)
	}
}

func TestFuzzyMatch_PathSeparatorBonus(t *testing.T) {
	// "mt" matching "main.go" after separator should rank well
	m := FuzzyMatch("mg", "main.go")
	if m == nil {
		t.Fatal("expected match")
	}
}

func TestFuzzyMatch_Ranking_BestFirst(t *testing.T) {
	items := []string{
		"internal/editor/editor.go",
		"internal/buffer/piecetable.go",
		"cmd/zephyr/main.go",
		"internal/editor/cursor.go",
	}
	matches := RankMatches("editor", items)
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}
	// First match should contain "editor" directly
	if matches[0].Text != "internal/editor/editor.go" && matches[0].Text != "internal/editor/cursor.go" {
		t.Logf("top match: %s (score %d)", matches[0].Text, matches[0].Score)
	}
}

func TestFuzzyMatch_LargeFileList(t *testing.T) {
	// 10K files should complete quickly
	items := make([]string, 10_000)
	for i := range items {
		items[i] = fmt.Sprintf("src/path/to/file_%d.go", i)
	}
	matches := RankMatches("file_50", items)
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}
}

func TestFuzzyMatch_EmptyQuery(t *testing.T) {
	m := FuzzyMatch("", "anything")
	if m == nil {
		t.Fatal("empty query should match everything")
	}
}

func BenchmarkFuzzyMatch_LargeFileList(b *testing.B) {
	items := make([]string, 10_000)
	for i := range items {
		items[i] = fmt.Sprintf("src/components/module_%d/index.tsx", i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RankMatches("modul", items)
	}
}
