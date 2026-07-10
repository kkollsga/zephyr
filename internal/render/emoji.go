package render

import "strings"

// genericFamilies are CSS-style generic font keywords that always resolve to
// some concrete face and therefore must stay last in a fallback list.
var genericFamilies = map[string]bool{
	"serif":      true,
	"sans-serif": true,
	"monospace":  true,
	"cursive":    true,
	"fantasy":    true,
	"system-ui":  true,
	"emoji":      true,
}

// AppendEmojiFamily inserts the platform's color-emoji font family into a
// comma-separated Gio typeface fallback list so that emoji runes not covered
// by the primary font resolve to a color font instead of .notdef (tofu).
//
// The emoji family is placed before any trailing generic keyword (e.g.
// "serif") because a generic keyword always resolves to a concrete face and
// would otherwise shadow later entries. If the emoji family is already present
// (case-insensitive) or the platform has no known emoji family, the input is
// returned unchanged.
func AppendEmojiFamily(typeface string) string {
	if emojiFamily == "" {
		return typeface
	}
	parts := strings.Split(typeface, ",")
	insertAt := len(parts)
	for i, p := range parts {
		name := strings.TrimSpace(p)
		if strings.EqualFold(name, emojiFamily) {
			return typeface // already present
		}
		if genericFamilies[strings.ToLower(name)] && insertAt == len(parts) {
			insertAt = i
		}
	}
	out := make([]string, 0, len(parts)+1)
	out = append(out, parts[:insertAt]...)
	out = append(out, " "+emojiFamily)
	out = append(out, parts[insertAt:]...)
	return strings.Join(out, ",")
}
