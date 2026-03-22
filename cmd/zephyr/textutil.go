package main

import (
	"strings"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/highlight"
	"github.com/kristianweb/zephyr/internal/render"
)

var autoPairs = map[string]string{
	"(": ")", "{": "}", "[": "]", `"`: `"`, "'": "'", "`": "`",
}

var closerSet = map[string]bool{
	")": true, "}": true, "]": true, `"`: true, "'": true, "`": true,
}

func (st *appState) deleteAutoPair() bool {
	ed := st.activeEd()
	if ed == nil || ed.Cursor.Col == 0 {
		return false
	}
	if ed.Selection.Active && !ed.Selection.IsEmpty() {
		return false
	}
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil || ed.Cursor.Col >= len([]rune(line)) {
		return false
	}
	runes := []rune(line)
	col := ed.Cursor.Col
	before := string(runes[col-1])
	after := string(runes[col])
	if closer, ok := autoPairs[before]; ok && closer == after {
		ed.DeleteForward()
		ed.DeleteBackward()
		return true
	}
	return false
}

func (st *appState) softTabBackspace() bool {
	ed := st.activeEd()
	if ed == nil {
		return false
	}
	col := ed.Cursor.Col
	if col == 0 || (ed.Selection.Active && !ed.Selection.IsEmpty()) {
		return false
	}
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return false
	}
	if col > len(line) {
		return false
	}
	prefix := line[:col]
	if strings.TrimLeft(prefix, " ") != "" {
		return false
	}
	remove := len(prefix) % 4
	if remove == 0 {
		remove = 4
	}
	ed.DeleteBackwardN(remove)
	return true
}

func (st *appState) computeAutoIndent() string {
	ed := st.activeEd()
	ts := st.activeTabState()
	if ed == nil {
		return ""
	}
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return ""
	}

	indent := ""
	for _, r := range line {
		if r == ' ' || r == '\t' {
			indent += string(r)
		} else {
			break
		}
	}

	trimmed := strings.TrimRight(line, " \t")
	if len(trimmed) == 0 {
		if len(indent) >= 4 {
			return indent[:len(indent)-4]
		}
		return ""
	}

	last := trimmed[len(trimmed)-1]
	if last == '{' || last == '(' || last == '[' {
		return indent + "    "
	}

	lang := ""
	if ts != nil {
		lang = ts.langLabel
	}
	if lang == "Python" && last == ':' {
		return indent + "    "
	}

	word := lastWord(trimmed)
	switch lang {
	case "Python":
		switch word {
		case "return", "break", "continue", "pass", "raise":
			return dedent(indent)
		}
	case "Go", "Rust", "JavaScript":
		switch word {
		case "return", "break", "continue":
			return dedent(indent)
		}
	}

	return indent
}

func dedent(indent string) string {
	if len(indent) >= 4 {
		return indent[:len(indent)-4]
	}
	return ""
}

func (st *appState) autoDedentClosingBracket() {
	ed := st.activeEd()
	if ed == nil {
		return
	}
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	trimmed := strings.TrimLeft(line, " ")
	if len(trimmed) != 1 {
		return
	}
	indent := len(line) - len(trimmed)
	if indent < 4 {
		return
	}
	savedCol := ed.Cursor.Col
	ed.Cursor.Col = 4
	ed.Cursor.PreferredCol = -1
	ed.DeleteBackwardN(4)
	ed.Cursor.Col = savedCol - 4
	ed.Cursor.PreferredCol = -1
}

func lastWord(s string) string {
	s = strings.TrimRight(s, " \t")
	i := strings.LastIndexAny(s, " \t")
	if i >= 0 {
		return s[i+1:]
	}
	return s
}

// Delegate to render package functions
func runeColToDisplayCol(lineText string, runeCol, tabSize int) int {
	return render.RuneColToDisplayCol(lineText, runeCol, tabSize)
}

func matchDisplayLen(lineText string, runeCol, byteLen, tabSize int) int {
	return render.MatchDisplayLen(lineText, runeCol, byteLen, tabSize)
}

func expandTabs(s string, tabSize int) string {
	return render.ExpandTabs(s, tabSize)
}

// runeColToDisplayCol2 is a convenience that fetches the line text from the editor.
func runeColToDisplayCol2(ed *editor.Editor, line, runeCol, tabSize int) int {
	lineText, err := ed.Buffer.Line(line)
	if err != nil {
		return runeCol
	}
	return runeColToDisplayCol(lineText, runeCol, tabSize)
}

func tokensForRange(tokens []highlight.Token, startByte, endByte int) []highlight.Token {
	// Binary search for first token that ends after startByte
	lo, hi := 0, len(tokens)
	for lo < hi {
		mid := (lo + hi) / 2
		if tokens[mid].EndByte <= startByte {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	var result []highlight.Token
	for i := lo; i < len(tokens); i++ {
		if tokens[i].StartByte >= endByte {
			break
		}
		result = append(result, tokens[i])
	}
	return result
}
