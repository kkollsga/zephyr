// Package format implements buffer reformatting used by the status-bar
// Compact/Expanded toggle and the fix-indent-on-Enter behavior.
//
// JSON conversions delegate to encoding/json (Indent/Compact) so key order
// and number formatting are preserved. JavaScript/TypeScript re-indentation
// is done with a small token-aware scanner that adjusts only the leading
// whitespace of each line based on bracket/brace/paren nesting, never touching
// content inside strings, template literals, regex literals, or comments.
package format

import (
	"bytes"
	"encoding/json"
	"strings"
)

// JSONIndent reformats raw JSON bytes with the given indent unit, preserving
// key order and number formatting (it does not unmarshal into interface{}).
// Returns (formatted, true) on success or (nil, false) if src is not valid JSON.
func JSONIndent(src []byte, indent string) ([]byte, bool) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, bytes.TrimSpace(src), "", indent); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// JSONCompact minifies raw JSON bytes onto a single line. Returns (nil, false)
// if src is not valid JSON.
func JSONCompact(src []byte) ([]byte, bool) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, bytes.TrimSpace(src)); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// IsSingleLine reports whether s, trimmed of surrounding whitespace, contains no
// newline. For JSON this reliably distinguishes compact (single-line) documents
// from expanded ones, since JSON strings cannot contain raw newlines.
func IsSingleLine(s string) bool {
	return !strings.Contains(strings.TrimSpace(s), "\n")
}

// scanner states.
const (
	stNormal = iota
	stLineComment
	stBlockComment
	stString
	stTemplate
)

// frame is one open bracket on the nesting stack.
type frame struct {
	open     byte // '{', '(' or '['
	isSwitch bool // true when this brace opens a switch body
	caseOpen bool // true once a case/default label has been seen in this switch
}

// lineInfo describes how one physical line should be indented.
type lineInfo struct {
	indent string // leading whitespace to apply (meaningful only when !skip)
	skip   bool   // true when the line begins inside a multi-line template
	//              literal or block comment and must be left untouched
}

// JSReindent re-indents JavaScript/TypeScript (or JSON) source using unit as
// one indentation level. Only leading whitespace is rewritten; the remainder of
// each line is preserved. The operation is idempotent.
func JSReindent(src, unit string) string {
	if src == "" {
		return src
	}
	infos := computeLineInfos(src, unit)
	lines := strings.Split(src, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if infos[i].skip {
			b.WriteString(line)
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue // blank line: no leading whitespace
		}
		b.WriteString(infos[i].indent)
		b.WriteString(trimmed)
	}
	return b.String()
}

// LineIndent returns the leading whitespace that the line at lineIdx should
// have. For blank lines this is the indent a statement at that position would
// get (useful for indenting a freshly inserted line); JSReindent still strips
// whitespace-only lines. skip is true when the line begins inside a template
// literal or block comment (callers must not rewrite it); ok is false when
// lineIdx is out of range.
func LineIndent(src, unit string, lineIdx int) (indent string, skip, ok bool) {
	if lineIdx < 0 {
		return "", false, false
	}
	infos := computeLineInfos(src, unit)
	if lineIdx >= len(infos) {
		return "", false, false
	}
	return infos[lineIdx].indent, infos[lineIdx].skip, true
}

// computeLineInfos scans the whole source once, tracking bracket depth and
// lexical state, and returns per-line indentation info.
func computeLineInfos(src, unit string) []lineInfo {
	lines := strings.Split(src, "\n")
	infos := make([]lineInfo, len(lines))

	var frames []frame
	state := stNormal
	var quote byte     // active quote in stString
	tmplExprBrace := 0 // brace depth inside the active template's ${...}
	pendingSwitch := false
	var lastWord string // last identifier scanned in normal state
	var prevSig byte    // last significant (non-space) char in normal code

	for li, line := range lines {
		if state == stTemplate || state == stBlockComment {
			infos[li] = lineInfo{skip: true}
		} else {
			trimmed := strings.TrimLeft(line, " \t")
			if trimmed == "" {
				// Blank line: record the position's indent so LineIndent can
				// indent a freshly inserted line (JSReindent ignores it).
				infos[li] = lineInfo{indent: strings.Repeat(unit, stackIndentUnits(frames))}
			} else {
				closers := 0
				for closers < len(trimmed) {
					c := trimmed[closers]
					if c == '}' || c == ')' || c == ']' {
						closers++
					} else {
						break
					}
				}
				label := isCaseLabel(trimmed)

				tmp := make([]frame, len(frames))
				copy(tmp, frames)
				for i := 0; i < closers && len(tmp) > 0; i++ {
					tmp = tmp[:len(tmp)-1]
				}
				units := stackIndentUnits(tmp)
				if label {
					if si := innermostSwitch(tmp); si >= 0 && tmp[si].caseOpen {
						units-- // labels sit one level out from the case body
					}
				}
				if units < 0 {
					units = 0
				}
				infos[li] = lineInfo{indent: strings.Repeat(unit, units)}

				// A label opens (or re-opens) the case body for following lines.
				if label {
					if si := innermostSwitch(frames); si >= 0 {
						frames[si].caseOpen = true
					}
				}
			}
		}

		scanLine(line, &state, &quote, &tmplExprBrace, &frames, &pendingSwitch, &lastWord, &prevSig)

		// Single-line-only constructs never carry into the next line.
		if state == stLineComment || state == stString {
			state = stNormal
		}
	}
	return infos
}

// stackIndentUnits counts indentation levels for the current frame stack. A
// switch brace whose case body is open counts as two levels so the case body
// sits one level deeper than its labels.
func stackIndentUnits(frames []frame) int {
	n := 0
	for _, f := range frames {
		n++
		if f.isSwitch && f.caseOpen {
			n++
		}
	}
	return n
}

// innermostSwitch returns the index of the deepest switch frame, or -1.
func innermostSwitch(frames []frame) int {
	for i := len(frames) - 1; i >= 0; i-- {
		if frames[i].isSwitch {
			return i
		}
	}
	return -1
}

// isCaseLabel reports whether the trimmed line begins with a case/default label.
func isCaseLabel(t string) bool {
	if strings.HasPrefix(t, "case") {
		if len(t) == 4 {
			return false
		}
		c := t[4]
		return c == ' ' || c == '\t' || c == '('
	}
	if strings.HasPrefix(t, "default") {
		rest := strings.TrimLeft(t[len("default"):], " \t")
		return strings.HasPrefix(rest, ":")
	}
	return false
}

// scanLine advances the scanner state across one physical line.
func scanLine(line string, state *int, quote *byte, tmplExprBrace *int, frames *[]frame, pendingSwitch *bool, lastWord *string, prevSig *byte) {
	i, n := 0, len(line)
	for i < n {
		c := line[i]
		switch *state {
		case stLineComment:
			return // rest of the line is comment
		case stBlockComment:
			if c == '*' && i+1 < n && line[i+1] == '/' {
				*state = stNormal
				i += 2
				continue
			}
			i++
		case stString:
			if c == '\\' {
				i += 2
				continue
			}
			if c == *quote {
				*state = stNormal
			}
			i++
		case stTemplate:
			if c == '\\' {
				i += 2
				continue
			}
			if *tmplExprBrace > 0 {
				// Inside ${...}; best-effort brace matching to find its end.
				if c == '{' {
					*tmplExprBrace++
				} else if c == '}' {
					*tmplExprBrace--
				}
				i++
				continue
			}
			if c == '`' {
				*state = stNormal
				i++
				continue
			}
			if c == '$' && i+1 < n && line[i+1] == '{' {
				*tmplExprBrace = 1
				i += 2
				continue
			}
			i++
		default: // stNormal
			switch {
			case c == '/' && i+1 < n && line[i+1] == '/':
				*state = stLineComment
				return
			case c == '/' && i+1 < n && line[i+1] == '*':
				*state = stBlockComment
				i += 2
			case c == '/':
				if regexAllowed(*prevSig, *lastWord) {
					i = skipRegex(line, i)
				} else {
					i++
				}
				*prevSig = '/'
				*lastWord = ""
			case c == '\'' || c == '"':
				*state = stString
				*quote = c
				*prevSig = c
				*lastWord = ""
				i++
			case c == '`':
				*state = stTemplate
				*tmplExprBrace = 0
				*prevSig = c
				*lastWord = ""
				i++
			case c == '{' || c == '(' || c == '[':
				*frames = append(*frames, frame{open: c, isSwitch: c == '{' && *pendingSwitch})
				if c == '{' {
					*pendingSwitch = false
				}
				*prevSig = c
				*lastWord = ""
				i++
			case c == '}' || c == ')' || c == ']':
				if len(*frames) > 0 {
					*frames = (*frames)[:len(*frames)-1]
				}
				*prevSig = c
				*lastWord = ""
				i++
			case c == ' ' || c == '\t':
				i++
			case isIdentChar(c):
				j := i
				for j < n && isIdentChar(line[j]) {
					j++
				}
				word := line[i:j]
				*lastWord = word
				if word == "switch" {
					*pendingSwitch = true
				}
				*prevSig = line[j-1]
				i = j
			default:
				if c == ';' {
					*pendingSwitch = false
				}
				*prevSig = c
				*lastWord = ""
				i++
			}
		}
	}
}

// regexAllowed reports whether a '/' in normal code starts a regex literal
// rather than a division operator, based on the preceding significant token.
func regexAllowed(prevSig byte, lastWord string) bool {
	switch lastWord {
	case "return", "typeof", "instanceof", "in", "of", "new", "delete",
		"void", "do", "else", "yield", "await", "case":
		return true
	}
	switch prevSig {
	case 0, '(', ',', '=', ':', '[', '!', '&', '|', '?', '{', '}',
		';', '+', '-', '*', '%', '<', '>', '^', '~':
		return true
	}
	return false
}

// skipRegex returns the index just past a regex literal that starts at i ('/').
// Regex literals cannot span lines, so this never runs past the line end.
func skipRegex(line string, i int) int {
	n := len(line)
	i++ // past opening '/'
	inClass := false
	for i < n {
		c := line[i]
		if c == '\\' {
			i += 2
			continue
		}
		switch {
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		case c == '/' && !inClass:
			return i + 1
		}
		i++
	}
	return n
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '$'
}
