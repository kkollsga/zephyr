package config

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
)

// Theme defines the color scheme for the editor.
type Theme struct {
	Name          string
	Background    color.NRGBA
	Foreground    color.NRGBA
	Gutter        color.NRGBA
	GutterBg      color.NRGBA
	Cursor        color.NRGBA
	Selection     color.NRGBA
	LineHighlight color.NRGBA
	StatusBg      color.NRGBA
	StatusFg      color.NRGBA

	// Find/replace highlight colors
	FindMatch   color.NRGBA
	FindCurrent color.NRGBA

	// Syntax token colors
	Keyword  color.NRGBA
	String   color.NRGBA
	Comment  color.NRGBA
	Function color.NRGBA
	Type     color.NRGBA
	Number   color.NRGBA
	Operator color.NRGBA
	Variable color.NRGBA

	// Markdown preview colors
	MdHeading color.NRGBA // headings and blockquote bar
	MdAccent  color.NRGBA // bold, italic, inline code text

	// UI element colors
	TabBarBg       color.NRGBA
	TabActiveBg    color.NRGBA
	TabBorder      color.NRGBA
	TabDimFg       color.NRGBA
	TabModifiedDot color.NRGBA
	TabCloseBtn    color.NRGBA
	TabCloseHover  color.NRGBA
	TabPlusHover   color.NRGBA
	TabAccent      color.NRGBA // accent line at top of active tab
	TabBarGradTop  color.NRGBA // tab bar gradient top color
	TabBarGradBot  color.NRGBA // tab bar gradient bottom color
	TitleFg        color.NRGBA
	SubtitleFg     color.NRGBA
	StatusBorder   color.NRGBA
	GutterSep      color.NRGBA
	ScrollbarThumb color.NRGBA
	FindBarBg      color.NRGBA
	FindBarBorder  color.NRGBA
	FindBarInputBg color.NRGBA
	FindBarFocus   color.NRGBA
	FindBarText    color.NRGBA
	FindBarDim     color.NRGBA
	DropdownBg     color.NRGBA
	DropdownSel    color.NRGBA
}

// Appearance represents the system appearance mode.
type Appearance int

const (
	Dark Appearance = iota
	Light
)

// themeJSON is the JSON representation of a theme.
type themeJSON struct {
	Name          string            `json:"name"`
	Background    string            `json:"background"`
	Foreground    string            `json:"foreground"`
	Gutter        string            `json:"gutter"`
	GutterBg      string            `json:"gutterBg"`
	Cursor        string            `json:"cursor"`
	Selection     string            `json:"selection"`
	LineHighlight string            `json:"lineHighlight"`
	StatusBg      string            `json:"statusBg"`
	StatusFg      string            `json:"statusFg"`
	Tokens        map[string]string `json:"tokens"`
	Find          map[string]string `json:"find"`
	UI            map[string]string `json:"ui"`
}

// LoadThemeFromFile loads a theme from a JSON file.
func LoadThemeFromFile(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, err
	}
	return LoadThemeFromJSON(data)
}

// LoadThemeFromJSON parses a theme from JSON bytes.
func LoadThemeFromJSON(data []byte) (Theme, error) {
	var tj themeJSON
	if err := json.Unmarshal(data, &tj); err != nil {
		return Theme{}, err
	}

	t := Theme{
		Name:          tj.Name,
		Background:    parseColor(tj.Background),
		Foreground:    parseColor(tj.Foreground),
		Gutter:        parseColor(tj.Gutter),
		GutterBg:      parseColor(tj.GutterBg),
		Cursor:        parseColor(tj.Cursor),
		Selection:     parseColor(tj.Selection),
		LineHighlight: parseColor(tj.LineHighlight),
		StatusBg:      parseColor(tj.StatusBg),
		StatusFg:      parseColor(tj.StatusFg),
	}

	if c, ok := tj.Tokens["keyword"]; ok {
		t.Keyword = parseColor(c)
	}
	if c, ok := tj.Tokens["string"]; ok {
		t.String = parseColor(c)
	}
	if c, ok := tj.Tokens["comment"]; ok {
		t.Comment = parseColor(c)
	}
	if c, ok := tj.Tokens["function"]; ok {
		t.Function = parseColor(c)
	}
	if c, ok := tj.Tokens["type"]; ok {
		t.Type = parseColor(c)
	}
	if c, ok := tj.Tokens["number"]; ok {
		t.Number = parseColor(c)
	}
	if c, ok := tj.Tokens["operator"]; ok {
		t.Operator = parseColor(c)
	}
	if c, ok := tj.Tokens["variable"]; ok {
		t.Variable = parseColor(c)
	}

	// Find colors
	if c, ok := tj.Find["match"]; ok {
		t.FindMatch = parseColor(c)
	}
	if c, ok := tj.Find["current"]; ok {
		t.FindCurrent = parseColor(c)
	}

	// UI element colors
	uiFields := map[string]*color.NRGBA{
		"tabBarBg":       &t.TabBarBg,
		"tabActiveBg":    &t.TabActiveBg,
		"tabBorder":      &t.TabBorder,
		"tabDimFg":       &t.TabDimFg,
		"tabModifiedDot": &t.TabModifiedDot,
		"tabCloseBtn":    &t.TabCloseBtn,
		"tabCloseHover":  &t.TabCloseHover,
		"tabPlusHover":   &t.TabPlusHover,
		"tabAccent":      &t.TabAccent,
		"tabBarGradTop":  &t.TabBarGradTop,
		"tabBarGradBot":  &t.TabBarGradBot,
		"titleFg":        &t.TitleFg,
		"subtitleFg":     &t.SubtitleFg,
		"statusBorder":   &t.StatusBorder,
		"gutterSep":      &t.GutterSep,
		"scrollbarThumb": &t.ScrollbarThumb,
		"findBarBg":      &t.FindBarBg,
		"findBarBorder":  &t.FindBarBorder,
		"findBarInputBg": &t.FindBarInputBg,
		"findBarFocus":   &t.FindBarFocus,
		"findBarText":    &t.FindBarText,
		"findBarDim":     &t.FindBarDim,
		"dropdownBg":     &t.DropdownBg,
		"dropdownSel":    &t.DropdownSel,
	}
	for key, ptr := range uiFields {
		if c, ok := tj.UI[key]; ok {
			*ptr = parseColor(c)
		}
	}

	// Fallback for new fields not present in older theme files
	if t.TabAccent.A == 0 {
		t.TabAccent = color.NRGBA{R: 0x4e, G: 0xc9, B: 0xb0, A: 255}
	}
	if t.TabBarGradTop.A == 0 {
		t.TabBarGradTop = t.TabBarBg
	}
	if t.TabBarGradBot.A == 0 {
		t.TabBarGradBot = t.TabBarBg
	}

	return t, nil
}

// parseColor parses a hex color string like "#ff0000" or "#ff000080" to NRGBA.
func parseColor(hex string) color.NRGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 && len(hex) != 8 {
		return color.NRGBA{}
	}

	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	a := uint64(255)
	if len(hex) == 8 {
		a, _ = strconv.ParseUint(hex[6:8], 16, 8)
	}

	return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}
}

// ColorToHex converts an NRGBA color to a hex string.
func ColorToHex(c color.NRGBA) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}
