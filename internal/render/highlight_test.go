package render

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/kristianweb/zephyr/internal/config"
	"github.com/kristianweb/zephyr/internal/highlight"
)

func TestTokenColorMapUsesThemeColors(t *testing.T) {
	theme := config.DarkTheme()
	got := TokenColorMap(theme)
	if got[highlight.TokenKeyword] != theme.Keyword || got[highlight.TokenVariable] != theme.Variable {
		t.Fatalf("TokenColorMap did not preserve theme colors: %#v", got)
	}
}

func TestTokensToColorSpansClampsTabsAndPrioritizesEarlierTokens(t *testing.T) {
	keyword := color.NRGBA{R: 1, A: 255}
	stringColor := color.NRGBA{G: 2, A: 255}
	fallback := color.NRGBA{B: 3, A: 255}
	line := "a\tbcd"
	tokens := []highlight.Token{
		{StartByte: 99, EndByte: 104, Type: highlight.TokenKeyword},
		{StartByte: 101, EndByte: 106, Type: highlight.TokenString},
		{StartByte: 104, EndByte: 105, Type: highlight.TokenType("unknown")},
	}
	got := TokensToColorSpans(tokens, 100, 106, line, highlight.TokenColorMap{
		highlight.TokenKeyword: keyword,
		highlight.TokenString:  stringColor,
	}, fallback, 4)
	want := []ColorSpan{
		{Start: 0, End: 6, Color: keyword},
		{Start: 6, End: 7, Color: stringColor},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TokensToColorSpans = %#v, want %#v", got, want)
	}
	if spans := TokensToColorSpans(nil, 0, 0, "", nil, fallback, 0); spans != nil {
		t.Fatalf("empty tokens produced %#v", spans)
	}
}
