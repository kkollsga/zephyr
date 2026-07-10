package render

import (
	"strings"
	"testing"
)

func TestParseMarkdownCoversRichDocumentStructure(t *testing.T) {
	source := []byte("# Heading\n\n\n" +
		"Paragraph with **bold**, *italic*, and `code`.\n\n" +
		"1. first\n2. second\n\n" +
		"- [x] done\n- [ ] todo\n  - nested\n\n" +
		"> quoted text\n\n" +
		"```go\nfmt.Println(1)\n```\n\n" +
		"---\n\n" +
		"| A | B |\n|---|---|\n| 1 | 2 |\n")

	doc := ParseMarkdown(source)
	if doc == nil || len(doc.Blocks) == 0 {
		t.Fatal("ParseMarkdown returned no blocks")
	}

	kinds := map[BlockKind]int{}
	var sawBold, sawItalic, sawInlineCode, sawChecked, sawUnchecked, sawNested bool
	for _, block := range doc.Blocks {
		kinds[block.Kind]++
		if block.BlankLinesBefore > 0 {
			// At least one intentionally doubled gap should survive parsing.
			sawNested = sawNested || block.Level > 0
		}
		if block.Kind == BlockListItem && block.Level > 0 {
			sawNested = true
		}
		for _, span := range block.Spans {
			sawBold = sawBold || span.Bold
			sawItalic = sawItalic || span.Italic
			sawInlineCode = sawInlineCode || span.Code
			sawChecked = sawChecked || span.Checkbox == 2
			sawUnchecked = sawUnchecked || span.Checkbox == 1
		}
	}

	for _, kind := range []BlockKind{BlockHeading, BlockParagraph, BlockCodeBlock, BlockBlockquote, BlockListItem, BlockThematicBreak, BlockTable} {
		if kinds[kind] == 0 {
			t.Errorf("missing parsed block kind %v", kind)
		}
	}
	if !sawBold || !sawItalic || !sawInlineCode || !sawChecked || !sawUnchecked || !sawNested {
		t.Fatalf("inline/list flags bold=%v italic=%v code=%v checked=%v unchecked=%v nested=%v", sawBold, sawItalic, sawInlineCode, sawChecked, sawUnchecked, sawNested)
	}

	var code, table bool
	for _, block := range doc.Blocks {
		if block.Kind == BlockCodeBlock {
			code = block.CodeLang == "go" && strings.Contains(block.CodeText, "fmt.Println")
		}
		if block.Kind == BlockTable {
			table = len(block.TableCells) == 2 && len(block.TableCells[0]) == 2
		}
	}
	if !code || !table {
		t.Fatalf("code metadata=%v table shape=%v", code, table)
	}
}

func TestMarkdownSmallHelpers(t *testing.T) {
	if got := countBlankLines([]byte("\n\n\n")); got != 2 {
		t.Fatalf("countBlankLines = %d, want 2", got)
	}
	if got := itoa(12034); got != "12034" {
		t.Fatalf("itoa = %q", got)
	}
	spans := []InlineSpan{{Text: "[x"}, {Text: "] hello", Bold: true}}
	trimmed := trimLeadingChars(spans, 3)
	if len(trimmed) != 1 || trimmed[0].Text != "hello" || !trimmed[0].Bold {
		t.Fatalf("trimLeadingChars = %#v", trimmed)
	}
}
