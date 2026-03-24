package highlight

import (
	"path/filepath"
	"sort"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/kristianweb/zephyr/internal/buffer"
)

// Highlighter manages tree-sitter parsing and token extraction for a file.
type Highlighter struct {
	parser   *sitter.Parser
	tree     *sitter.Tree
	langInfo *LanguageInfo
	query    *sitter.Query
	source   []byte
	simple   bool // true when using SimpleTokens instead of tree-sitter
}

// NewHighlighter creates a highlighter for the given file extension.
// Returns nil if the language is not supported.
func NewHighlighter(filePath string) *Highlighter {
	ext := filepath.Ext(filePath)
	info := ForExtension(ext)
	if info == nil {
		return nil
	}

	// Simple tokenizer mode (no tree-sitter grammar needed)
	if info.Language == nil && info.SimpleTokens != nil {
		return &Highlighter{
			langInfo: info,
			simple:   true,
		}
	}

	if info.Language == nil {
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

	if info.Language == nil && info.SimpleTokens != nil {
		return &Highlighter{
			langInfo: info,
			simple:   true,
		}
	}

	if info.Language == nil {
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
// Always does a full reparse since we don't track edit byte offsets.
// Passing the old tree without calling Tree.Edit() first causes tree-sitter
// to produce incorrect tokens when the text has changed.
func (h *Highlighter) Parse(source []byte) {
	h.source = source
	if h.simple {
		return
	}
	h.tree = h.parser.Parse(nil, source)
}

// Update performs an incremental update after an edit.
func (h *Highlighter) Update(source []byte, edit sitter.EditInput) {
	if h.tree != nil {
		h.tree.Edit(edit)
	}
	h.source = source
	h.tree = h.parser.Parse(h.tree, source)
}

// UpdateMulti applies multiple sequential edits and reparses incrementally.
func (h *Highlighter) UpdateMulti(source []byte, edits []sitter.EditInput) {
	if h.tree != nil {
		for _, edit := range edits {
			h.tree.Edit(edit)
		}
	}
	h.source = source
	h.tree = h.parser.Parse(h.tree, source)
}

// UpdateFromEdits converts buffer.EditInfo slices to tree-sitter EditInputs
// and performs an incremental reparse. If edits is empty, does a full parse.
func (h *Highlighter) UpdateFromEdits(source []byte, edits []buffer.EditInfo) {
	if h.simple {
		h.source = source
		return
	}
	if len(edits) == 0 {
		h.Parse(source)
		return
	}
	sEdits := make([]sitter.EditInput, len(edits))
	for i, e := range edits {
		sEdits[i] = sitter.EditInput{
			StartIndex:  uint32(e.StartByte),
			OldEndIndex: uint32(e.OldEndByte),
			NewEndIndex: uint32(e.NewEndByte),
			StartPoint:  sitter.Point{Row: uint32(e.StartRow), Column: uint32(e.StartCol)},
			OldEndPoint: sitter.Point{Row: uint32(e.OldEndRow), Column: uint32(e.OldEndCol)},
			NewEndPoint: sitter.Point{Row: uint32(e.NewEndRow), Column: uint32(e.NewEndCol)},
		}
	}
	h.UpdateMulti(source, sEdits)
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

// TokensInRange returns highlighted tokens that overlap the given row range.
// Uses tree-sitter's SetPointRange for efficient querying of visible lines only.
// Results are in document order (tree-sitter guarantees this with SetPointRange).
func (h *Highlighter) TokensInRange(startRow, endRow int) []Token {
	if h.simple && h.langInfo != nil && h.langInfo.SimpleTokens != nil {
		return h.langInfo.SimpleTokens(h.source, startRow, endRow)
	}

	if h.tree == nil || h.query == nil {
		return nil
	}

	cursor := sitter.NewQueryCursor()
	cursor.Exec(h.query, h.tree.RootNode())
	cursor.SetPointRange(
		sitter.Point{Row: uint32(startRow), Column: 0},
		sitter.Point{Row: uint32(endRow + 1), Column: 0},
	)

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

// Evict releases the source buffer and parsed tree to free memory.
// The parser and query are retained so a subsequent Parse can restore them.
func (h *Highlighter) Evict() {
	if h.tree != nil {
		h.tree.Close()
		h.tree = nil
	}
	h.source = nil
}

// NeedsParse returns true if the highlighter has been evicted and needs a full reparse.
func (h *Highlighter) NeedsParse() bool {
	if h.simple {
		return h.source == nil
	}
	return h.tree == nil
}

// Close releases resources.
func (h *Highlighter) Close() {
	if h.tree != nil {
		h.tree.Close()
	}
	if h.parser != nil {
		h.parser.Close()
	}
}
