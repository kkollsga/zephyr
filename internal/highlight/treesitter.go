package highlight

import (
	"path/filepath"
	"sort"

	sitter "github.com/smacker/go-tree-sitter"
)

// Highlighter manages tree-sitter parsing and token extraction for a file.
type Highlighter struct {
	parser   *sitter.Parser
	tree     *sitter.Tree
	langInfo *LanguageInfo
	query    *sitter.Query
	source   []byte
}

// NewHighlighter creates a highlighter for the given file extension.
// Returns nil if the language is not supported.
func NewHighlighter(filePath string) *Highlighter {
	ext := filepath.Ext(filePath)
	info := ForExtension(ext)
	if info == nil {
		return nil
	}

	parser := sitter.NewParser()
	parser.SetLanguage(info.Language)

	query, err := sitter.NewQuery([]byte(info.Query), info.Language)
	if err != nil {
		return nil
	}

	return &Highlighter{
		parser:   parser,
		langInfo: info,
		query:    query,
	}
}

// NewHighlighterForLanguage creates a highlighter for a language name (e.g. "Go", "Python").
// Returns nil if the language is not supported.
func NewHighlighterForLanguage(name string) *Highlighter {
	info := ForName(name)
	if info == nil {
		return nil
	}

	parser := sitter.NewParser()
	parser.SetLanguage(info.Language)

	query, err := sitter.NewQuery([]byte(info.Query), info.Language)
	if err != nil {
		return nil
	}

	return &Highlighter{
		parser:   parser,
		langInfo: info,
		query:    query,
	}
}

// Parse parses the source code and builds the syntax tree.
func (h *Highlighter) Parse(source []byte) {
	h.source = source
	h.tree = h.parser.Parse(h.tree, source)
}

// Update performs an incremental update after an edit.
func (h *Highlighter) Update(source []byte, edit sitter.EditInput) {
	if h.tree != nil {
		h.tree.Edit(edit)
	}
	h.source = source
	h.tree = h.parser.Parse(h.tree, source)
}

// Tokens returns all highlighted tokens in the source.
func (h *Highlighter) Tokens() []Token {
	if h.tree == nil || h.query == nil {
		return nil
	}

	cursor := sitter.NewQueryCursor()
	cursor.Exec(h.query, h.tree.RootNode())

	var tokens []Token
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			name := h.query.CaptureNameForId(capture.Index)
			tokenType := CaptureNameToTokenType(name)
			if tokenType == "" {
				continue
			}
			tokens = append(tokens, Token{
				StartByte: int(capture.Node.StartByte()),
				EndByte:   int(capture.Node.EndByte()),
				Type:      tokenType,
			})
		}
	}

	// Sort by start position, higher-priority (earlier in query) tokens come first
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].StartByte != tokens[j].StartByte {
			return tokens[i].StartByte < tokens[j].StartByte
		}
		return tokens[i].EndByte < tokens[j].EndByte
	})

	return tokens
}

// TokensForLineRange returns tokens that overlap the given byte range.
func (h *Highlighter) TokensForLineRange(startByte, endByte int) []Token {
	all := h.Tokens()
	var result []Token
	for _, t := range all {
		if t.EndByte <= startByte {
			continue
		}
		if t.StartByte >= endByte {
			break
		}
		result = append(result, t)
	}
	return result
}

// Language returns the name of the detected language.
func (h *Highlighter) Language() string {
	if h.langInfo != nil {
		return h.langInfo.Name
	}
	return ""
}

// Close releases resources.
func (h *Highlighter) Close() {
	if h.parser != nil {
		h.parser.Close()
	}
}
