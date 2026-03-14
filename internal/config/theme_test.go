package config

import (
	"image/color"
	"testing"
)

func TestTheme_LoadFromJSON(t *testing.T) {
	jsonData := []byte(`{
		"name": "Test",
		"background": "#1e1e1e",
		"foreground": "#d4d4d4",
		"gutter": "#6e6e6e",
		"gutterBg": "#1e1e1e",
		"cursor": "#d4d4d4",
		"selection": "#3c5a8c80",
		"lineHighlight": "#282828",
		"statusBg": "#191919",
		"statusFg": "#969696",
		"tokens": {
			"keyword": "#c586c0",
			"string": "#ce9178",
			"comment": "#6a9955"
		}
	}`)
	theme, err := LoadThemeFromJSON(jsonData)
	if err != nil {
		t.Fatal(err)
	}
	if theme.Name != "Test" {
		t.Fatalf("got name %q", theme.Name)
	}
	if theme.Background != (color.NRGBA{R: 0x1e, G: 0x1e, B: 0x1e, A: 255}) {
		t.Fatalf("unexpected background: %+v", theme.Background)
	}
	if theme.Selection.A != 0x80 {
		t.Fatalf("expected selection alpha 128, got %d", theme.Selection.A)
	}
}

func TestTheme_DefaultTheme_AllTokensCovered(t *testing.T) {
	theme := DarkTheme()
	zero := color.NRGBA{}
	if theme.Keyword == zero {
		t.Fatal("keyword color not set")
	}
	if theme.String == zero {
		t.Fatal("string color not set")
	}
	if theme.Comment == zero {
		t.Fatal("comment color not set")
	}
	if theme.Function == zero {
		t.Fatal("function color not set")
	}
	if theme.Type == zero {
		t.Fatal("type color not set")
	}
	if theme.Number == zero {
		t.Fatal("number color not set")
	}
}

func TestTheme_MissingTokenType_FallsBackToForeground(t *testing.T) {
	// When a token type is not specified in the theme, it should be zero-value
	jsonData := []byte(`{
		"name": "Minimal",
		"background": "#000000",
		"foreground": "#ffffff",
		"gutter": "#888888",
		"gutterBg": "#000000",
		"cursor": "#ffffff",
		"selection": "#444444",
		"lineHighlight": "#111111",
		"statusBg": "#000000",
		"statusFg": "#888888",
		"tokens": {}
	}`)
	theme, err := LoadThemeFromJSON(jsonData)
	if err != nil {
		t.Fatal(err)
	}
	// Keyword should be zero since not specified
	zero := color.NRGBA{}
	if theme.Keyword != zero {
		t.Fatal("expected zero keyword color for unspecified token")
	}
}

func TestColorToHex(t *testing.T) {
	tests := []struct {
		color color.NRGBA
		want  string
	}{
		{color.NRGBA{R: 255, G: 0, B: 0, A: 255}, "#ff0000"},
		{color.NRGBA{R: 0, G: 255, B: 0, A: 128}, "#00ff0080"},
		{color.NRGBA{R: 30, G: 30, B: 30, A: 255}, "#1e1e1e"},
	}
	for _, tt := range tests {
		got := ColorToHex(tt.color)
		if got != tt.want {
			t.Errorf("ColorToHex(%+v) = %q, want %q", tt.color, got, tt.want)
		}
	}
}
