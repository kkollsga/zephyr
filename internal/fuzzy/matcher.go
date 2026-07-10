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
	matched := make([]int, 0, len(lowerQuery))
	score, matched, ok := fuzzyMatch(query, lowerQuery, text, matched)
	if !ok {
		return nil
	}
	return &Match{
		Text:       text,
		Score:      score,
		MatchedIdx: matched,
	}
}

// fuzzyMatch appends matched character indices to matched and returns the
// resulting slice. Callers that process a batch can share one backing array
// across matches, avoiding a short allocation for every candidate.
func fuzzyMatch(query, lowerQuery, text string, matched []int) (int, []int, bool) {
	if query == "" {
		return 1, matched, true
	}

	lowerText := strings.ToLower(text)

	matchedStart := len(matched)
	qi := 0
	score := 0
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
		return 0, matched[:matchedStart], false // not all query chars matched
	}

	// Penalty for longer texts (prefer shorter matches)
	score -= len(text) / 10

	return score, matched, true
}

func isSeparator(b byte) bool {
	return b == '/' || b == '\\' || b == '.' || b == '_' || b == '-' || b == ' '
}

// RankMatches performs fuzzy matching of query against all items and returns
// matches sorted by score (best first).
func RankMatches(query string, items []string) []Match {
	lowerQuery := strings.ToLower(query)

	// Every successful match has exactly len(lowerQuery) byte indices. Reserve
	// one shared backing array for the batch when the multiplication is safe.
	// Unsuccessful candidates roll their temporary indices back in fuzzyMatch.
	var matchedIdx []int
	maxInt := int(^uint(0) >> 1)
	if len(items) > 0 && len(lowerQuery) > 0 && len(lowerQuery) <= maxInt/len(items) {
		matchedIdx = make([]int, 0, len(items)*len(lowerQuery))
	}

	var matches []Match
	for _, item := range items {
		start := len(matchedIdx)
		score, nextMatchedIdx, ok := fuzzyMatch(query, lowerQuery, item, matchedIdx)
		matchedIdx = nextMatchedIdx
		if ok {
			if matches == nil {
				matches = make([]Match, 0, len(items))
			}
			matches = append(matches, Match{
				Text:       item,
				Score:      score,
				MatchedIdx: matchedIdx[start:len(matchedIdx):len(matchedIdx)],
			})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches
}
