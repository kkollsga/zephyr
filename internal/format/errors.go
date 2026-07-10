package format

import (
	"encoding/json"
	"strings"
)

// JSONErrorLine reports the 0-based line of the first JSON syntax error in src.
// It returns ok=false when src is valid JSON or contains only whitespace.
//
// Validation is delegated to encoding/json (no DOM is built): the source is
// unmarshaled into a json.RawMessage, which makes the standard-library scanner
// walk the whole value and report a *json.SyntaxError with a byte Offset for the
// first malformed token. The line is the number of newlines preceding that
// offset.
func JSONErrorLine(src []byte) (line int, ok bool) {
	if strings.TrimSpace(string(src)) == "" {
		return 0, false // empty / whitespace-only buffer: no error
	}
	var raw json.RawMessage
	err := json.Unmarshal(src, &raw)
	if err == nil {
		return 0, false
	}
	se, isSyntax := err.(*json.SyntaxError)
	if !isSyntax {
		// RawMessage never triggers type errors, but guard defensively so an
		// unexpected error kind still surfaces a marker on the first line.
		return 0, true
	}
	off := int(se.Offset)
	if off < 0 {
		off = 0
	}
	if off > len(src) {
		off = len(src)
	}
	return strings.Count(string(src[:off]), "\n"), true
}
