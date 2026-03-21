package render

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// BlockKind identifies the type of rendered block.
type BlockKind int

const (
	BlockParagraph    BlockKind = iota
	BlockHeading                // Level stored in Block.Level (1-6)
	BlockCodeBlock              // Fenced or indented code block
	BlockBlockquote             // Blockquote container
	BlockListItem               // A single list item (ordered or unordered)
	BlockThematicBreak          // Horizontal rule
	BlockTable                  // Pipe table
)

// InlineSpan represents a styled run of text within a block.
type InlineSpan struct {
	Text     string
	Bold     bool
	Italic   bool
	Code     bool // inline code span
	Checkbox int  // 0 = not a checkbox, 1 = unchecked, 2 = checked
}

// Block is a single rendered element in the markdown document.
type Block struct {
	Kind             BlockKind
	Level            int           // heading level (1-6) or list indent depth
	Spans            []InlineSpan  // inline content for text blocks
	CodeText         string        // full text for code blocks
	CodeLang         string        // language hint for code blocks
	Children         []Block       // nested blocks (blockquotes, sub-lists)
	Ordered          bool          // true for ordered list items
	Marker           string        // list marker text ("•", "1.", etc.)
	TableCells       [][]string       // rows × cols for tables
	TableAlign       []east.Alignment // per-column alignment
	BlankLinesBefore int           // number of blank lines before this block in source
	SourceOffset     int           // byte offset in source for checkbox toggling
}

// MarkdownDoc holds the parsed block structure of a markdown file.
type MarkdownDoc struct {
	Blocks []Block
}

// ParseMarkdown parses markdown source into a MarkdownDoc.
func ParseMarkdown(source []byte) *MarkdownDoc {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.TaskList),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	var blocks []Block
	walkBlocks(doc, source, &blocks, 0)

	// Compute blank lines between blocks from source byte positions.
	computeBlankLines(doc, source, blocks)

	return &MarkdownDoc{Blocks: blocks}
}

// computeBlankLines counts blank lines between top-level AST nodes
// and assigns BlankLinesBefore to the corresponding Block entries.
func computeBlankLines(doc ast.Node, source []byte, blocks []Block) {
	type nodeInfo struct {
		startByte  int
		endByte    int
		blockCount int // how many Block entries this AST node produces
	}

	// Collect positions and block counts for each top-level AST node
	var nodes []nodeInfo
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		start := nodeStartByte(child, source)
		end := nodeEndByte(child, source)
		count := 1
		if list, ok := child.(*ast.List); ok {
			count = 0
			for item := list.FirstChild(); item != nil; item = item.NextSibling() {
				count++
				countNestedListItems(item, &count)
			}
		}
		nodes = append(nodes, nodeInfo{start, end, count})
	}

	// Assign blank line counts to blocks
	bi := 0
	for ni, info := range nodes {
		if bi >= len(blocks) {
			break
		}
		if ni > 0 {
			prevEnd := nodes[ni-1].endByte
			curStart := info.startByte
			if curStart > prevEnd && prevEnd > 0 && curStart <= len(source) {
				blanks := countBlankLines(source[prevEnd:curStart])
				blocks[bi].BlankLinesBefore = blanks
			}
		}
		bi += info.blockCount
	}
}

func countNestedListItems(n ast.Node, count *int) {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if list, ok := child.(*ast.List); ok {
			for item := list.FirstChild(); item != nil; item = item.NextSibling() {
				*count++
				countNestedListItems(item, count)
			}
		}
	}
}

func nodeStartByte(n ast.Node, source []byte) int {
	if n.Lines().Len() > 0 {
		return n.Lines().At(0).Start
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if s := nodeStartByte(c, source); s > 0 {
			return s
		}
	}
	return 0
}

func nodeEndByte(n ast.Node, source []byte) int {
	if n.Lines().Len() > 0 {
		last := n.Lines().At(n.Lines().Len() - 1)
		return last.Stop
	}
	var end int
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if e := nodeEndByte(c, source); e > end {
			end = e
		}
	}
	return end
}

// countBlankLines counts blank lines in a gap between blocks.
func countBlankLines(gap []byte) int {
	newlines := 0
	for _, b := range gap {
		if b == '\n' {
			newlines++
		}
	}
	if newlines <= 1 {
		return 0
	}
	return newlines - 1
}

// walkBlocks recursively converts goldmark AST nodes to Block slices.
func walkBlocks(n ast.Node, source []byte, blocks *[]Block, depth int) {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch node := child.(type) {
		case *ast.Heading:
			b := Block{Kind: BlockHeading, Level: node.Level}
			b.Spans = collectInlines(node, source)
			*blocks = append(*blocks, b)

		case *ast.Paragraph:
			b := Block{Kind: BlockParagraph}
			b.Spans = collectInlines(node, source)
			srcOff := nodeStartByte(node, source)
			b.SourceOffset = srcOff
			// Detect standalone [ ] or [x] at start of paragraph
			if plain := spansPlainPrefix(b.Spans); len(plain) >= 3 &&
				plain[0] == '[' && (plain[1] == ' ' || plain[1] == 'x' || plain[1] == 'X') && plain[2] == ']' {
				cb := 1
				if plain[1] != ' ' {
					cb = 2
				}
				b.Kind = BlockListItem
				// Check raw source for leading spaces to determine indent
				indent := 0
				for i := srcOff - 1; i >= 0 && i < len(source) && source[i] == ' '; i-- {
					indent++
				}
				if indent > 0 {
					b.Level = 0 // indented, no marker
				} else {
					b.Level = -1 // no indent, no marker
				}
				b.Spans = trimLeadingChars(b.Spans, 3)
				b.Spans = append([]InlineSpan{{Checkbox: cb}}, b.Spans...)
			}
			*blocks = append(*blocks, b)

		case *ast.FencedCodeBlock:
			b := Block{Kind: BlockCodeBlock}
			b.CodeLang = string(node.Language(source))
			var buf bytes.Buffer
			for i := 0; i < node.Lines().Len(); i++ {
				line := node.Lines().At(i)
				buf.Write(line.Value(source))
			}
			b.CodeText = strings.TrimRight(buf.String(), "\n")
			*blocks = append(*blocks, b)

		case *ast.CodeBlock:
			b := Block{Kind: BlockCodeBlock}
			var buf bytes.Buffer
			for i := 0; i < node.Lines().Len(); i++ {
				line := node.Lines().At(i)
				buf.Write(line.Value(source))
			}
			b.CodeText = strings.TrimRight(buf.String(), "\n")
			*blocks = append(*blocks, b)

		case *ast.Blockquote:
			b := Block{Kind: BlockBlockquote}
			walkBlocks(node, source, &b.Children, depth+1)
			*blocks = append(*blocks, b)

		case *ast.List:
			flattenList(node, source, blocks, depth)

		case *ast.ThematicBreak:
			*blocks = append(*blocks, Block{Kind: BlockThematicBreak})

		case *east.Table:
			b := Block{Kind: BlockTable}
			// Collect alignment
			for _, align := range node.Alignments {
				b.TableAlign = append(b.TableAlign, align)
			}
			// Walk header and body rows
			for row := node.FirstChild(); row != nil; row = row.NextSibling() {
				var cells []string
				for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
					cellText := collectPlainText(cell, source)
					cells = append(cells, cellText)
				}
				b.TableCells = append(b.TableCells, cells)
			}
			*blocks = append(*blocks, b)
		}
	}
}

// flattenList converts a List node into flat BlockListItem entries at the given depth.
func flattenList(list *ast.List, source []byte, blocks *[]Block, depth int) {
	marker := 1
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		li, ok := item.(*ast.ListItem)
		if !ok {
			continue
		}
		b := Block{Kind: BlockListItem, Level: depth, Ordered: list.IsOrdered()}
		b.SourceOffset = nodeStartByte(li, source)
		if list.IsOrdered() {
			b.Marker = itoa(marker) + "."
			marker++
		} else {
			b.Marker = "•"
		}
		var nested []*ast.List
		for liChild := li.FirstChild(); liChild != nil; liChild = liChild.NextSibling() {
			switch lc := liChild.(type) {
			case *ast.Paragraph, *ast.TextBlock:
				b.Spans = append(b.Spans, collectInlines(liChild, source)...)
			case *ast.List:
				nested = append(nested, lc)
			}
		}
		*blocks = append(*blocks, b)
		for _, nl := range nested {
			flattenList(nl, source, blocks, depth+1)
		}
	}
}

// collectInlines gathers inline content from a block node into InlineSpans.
func collectInlines(n ast.Node, source []byte) []InlineSpan {
	var spans []InlineSpan
	walkInlines(n, source, &spans, false, false, false)
	return spans
}

func walkInlines(n ast.Node, source []byte, spans *[]InlineSpan, bold, italic, code bool) {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch node := child.(type) {
		case *ast.Text:
			t := string(node.Segment.Value(source))
			*spans = append(*spans, InlineSpan{Text: t, Bold: bold, Italic: italic, Code: code})
			if node.HardLineBreak() || node.SoftLineBreak() {
				*spans = append(*spans, InlineSpan{Text: "\n"})
			}
		case *ast.String:
			*spans = append(*spans, InlineSpan{Text: string(node.Value), Bold: bold, Italic: italic, Code: code})
		case *ast.CodeSpan:
			t := collectPlainText(node, source)
			*spans = append(*spans, InlineSpan{Text: t, Bold: bold, Italic: italic, Code: true})
		case *ast.Emphasis:
			if node.Level == 2 {
				walkInlines(node, source, spans, true, italic, code)
			} else {
				walkInlines(node, source, spans, bold, true, code)
			}
		case *east.TaskCheckBox:
			cb := 1
			if node.IsChecked {
				cb = 2
			}
			*spans = append(*spans, InlineSpan{Checkbox: cb})
		case *ast.Link:
			walkInlines(node, source, spans, bold, italic, code)
		case *ast.Image:
			altText := collectPlainText(node, source)
			if altText == "" {
				altText = "[image]"
			}
			*spans = append(*spans, InlineSpan{Text: altText, Bold: bold, Italic: true, Code: code})
		case *ast.AutoLink:
			*spans = append(*spans, InlineSpan{Text: string(node.URL(source)), Bold: bold, Italic: italic, Code: code})
		default:
			// Recurse for unknown inline containers
			walkInlines(child, source, spans, bold, italic, code)
		}
	}
}

// collectPlainText returns the plain text content of a node and its children.
func collectPlainText(n ast.Node, source []byte) string {
	var buf bytes.Buffer
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch node := child.(type) {
		case *ast.Text:
			buf.Write(node.Segment.Value(source))
		case *ast.String:
			buf.Write(node.Value)
		case *ast.CodeSpan:
			buf.WriteString(collectPlainText(node, source))
		default:
			buf.WriteString(collectPlainText(child, source))
		}
	}
	return buf.String()
}

// spansPlainPrefix returns the first few characters of the concatenated span text.
func spansPlainPrefix(spans []InlineSpan) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
		if b.Len() >= 4 {
			break
		}
	}
	return b.String()
}

// trimLeadingChars removes the first n characters from the span text,
// potentially consuming or shortening the first span(s).
func trimLeadingChars(spans []InlineSpan, n int) []InlineSpan {
	for n > 0 && len(spans) > 0 {
		t := spans[0].Text
		if len(t) <= n {
			n -= len(t)
			spans = spans[1:]
		} else {
			spans[0].Text = t[n:]
			n = 0
		}
	}
	// Trim leading space from the remaining first span
	if len(spans) > 0 {
		spans[0].Text = strings.TrimLeft(spans[0].Text, " ")
	}
	return spans
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
