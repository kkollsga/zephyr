package fuzzy

import (
	"sort"
	"strings"
	"unicode"
)

// Match represents a fuzzy match result.
type Match struct {
	Text       string
	Score      int
	MatchedIdx []int // indices of matched characters
}

// FuzzyMatch performs fuzzy string matching of query against text.
// Returns a Match with score > 0 if the query is found as a subsequence in text.
// Returns nil if no match.
func FuzzyMatch(query, text string) *Match {
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

			// Bonus for consecutive matches
			if prevMatched {
				baseScore += 8
			}

			// Bonus for match after separator (/, ., _, -, space)
			if prevWasSeparator {
				baseScore += 5
			}

			// Bonus for case-exact match
			if ti < len(text) && qi < len(query) && text[ti] == query[qi] {
				baseScore += 1
			}

			// Bonus for match at start of text
			if ti == 0 {
				baseScore += 10
			}

			// Bonus for CamelCase match
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
		return nil // not all query chars matched
	}

	// Penalty for longer texts (prefer shorter matches)
	score -= len(text) / 10

	return &Match{
		Text:       text,
		Score:      score,
		MatchedIdx: matched,
	}
}

func isSeparator(b byte) bool {
	return b == '/' || b == '\\' || b == '.' || b == '_' || b == '-' || b == ' '
}

// RankMatches performs fuzzy matching of query against all items and returns
// matches sorted by score (best first).
func RankMatches(query string, items []string) []Match {
	var matches []Match
	for _, item := range items {
		m := FuzzyMatch(query, item)
		if m != nil {
			matches = append(matches, *m)
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches
}
