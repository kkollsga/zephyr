package fuzzy

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode"
)

var rankedMatchesSink []Match

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

func TestRankMatches_PreservesReferenceBehavior(t *testing.T) {
	items := []string{
		"internal/editor/editor.go",
		"InternalEditor.go",
		"src/components/module_50/index.tsx",
		"src/components/module_500/index.tsx",
		"a_b-c.d/e f",
		"Ångström.go",
		"İstanbul.txt",
		"Straße.md",
		"Καλημέρα.go",
		"unrelated",
		"",
	}
	for i := 0; i < 200; i++ {
		items = append(items, fmt.Sprintf("src/ModuleGroup_%03d/file-name.go", i))
	}

	queries := []string{"", "editor", "IE", "modul", "mgf", "a/c", "Åg", "İs", "Stra", "xyz"}
	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			want := referenceRankMatches(query, items)
			got := RankMatches(query, items)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("ranking changed for query %q\n got: %#v\nwant: %#v", query, got, want)
			}
		})
	}
}

func TestRankMatches_LargeListAllocations(t *testing.T) {
	items := make([]string, 10_000)
	for i := range items {
		items[i] = fmt.Sprintf("src/components/module_%d/index.tsx", i)
	}

	allocs := testing.AllocsPerRun(5, func() {
		rankedMatchesSink = RankMatches("modul", items)
	})
	if allocs > 10 {
		t.Fatalf("RankMatches allocated %.0f times; want at most 10", allocs)
	}
	if len(rankedMatchesSink) != len(items) {
		t.Fatalf("got %d matches, want %d", len(rankedMatchesSink), len(items))
	}
}

func TestRankMatches_MatchedIndicesHaveIndependentCapacity(t *testing.T) {
	matches := RankMatches("ab", []string{"ab-one", "ab-two"})
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	wantSecond := append([]int(nil), matches[1].MatchedIdx...)
	matches[0].MatchedIdx = append(matches[0].MatchedIdx, 99)
	if !reflect.DeepEqual(matches[1].MatchedIdx, wantSecond) {
		t.Fatalf("appending to one match changed another: got %v, want %v", matches[1].MatchedIdx, wantSecond)
	}
}

// referenceRankMatches is the pre-optimization implementation. Keeping it in
// tests guards scores, byte indices, Unicode behavior, and ranking order while
// the production path reuses batch storage.
func referenceRankMatches(query string, items []string) []Match {
	var matches []Match
	for _, item := range items {
		m := referenceFuzzyMatch(query, item)
		if m != nil {
			matches = append(matches, *m)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})
	return matches
}

func referenceFuzzyMatch(query, text string) *Match {
	if query == "" {
		return &Match{Text: text, Score: 1}
	}

	lowerQuery := strings.ToLower(query)
	lowerText := strings.ToLower(text)
	qi := 0
	score := 0
	var matched []int
	prevMatched := false
	prevWasSeparator := true

	for ti := 0; ti < len(lowerText) && qi < len(lowerQuery); ti++ {
		if lowerText[ti] == lowerQuery[qi] {
			matched = append(matched, ti)
			baseScore := 1
			if prevMatched {
				baseScore += 8
			}
			if prevWasSeparator {
				baseScore += 5
			}
			if ti < len(text) && qi < len(query) && text[ti] == query[qi] {
				baseScore++
			}
			if ti == 0 {
				baseScore += 10
			}
			if ti > 0 && unicode.IsUpper(rune(text[ti])) && unicode.IsLower(rune(text[ti-1])) {
				baseScore += 5
			}
			score += baseScore
			qi++
			prevMatched = true
		} else {
			prevMatched = false
		}
		prevWasSeparator = isSeparator(lowerText[ti])
	}

	if qi < len(lowerQuery) {
		return nil
	}
	score -= len(text) / 10
	return &Match{Text: text, Score: score, MatchedIdx: matched}
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
