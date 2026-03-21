package config

import (
	"image/color"
	"os"

	"gopkg.in/yaml.v3"
)

// FontConfig holds the font families for different contexts.
type FontConfig struct {
	Monospace string // editor, code blocks (e.g. "Menlo, monospace")
	Heading   string // headings in markdown read mode
	Body      string // body text in markdown read mode
}

// DefaultFontConfig returns the default font configuration.
func DefaultFontConfig() FontConfig {
	return FontConfig{
		Monospace: "Menlo, monospace",
		Heading:   "Menlo, monospace",
		Body:      "Menlo, monospace",
	}
}

// ThemeBundle holds both dark and light variants from a single theme file.
type ThemeBundle struct {
	Name     string
	Subtitle string // custom tagline, e.g. "The caffeinated editor"
	Fonts    FontConfig
	Dark     Theme
	Light    Theme
}

// Theme returns the variant matching the given appearance.
func (b ThemeBundle) Theme(dark bool) Theme {
	if dark {
		return b.Dark
	}
	return b.Light
}

// fontsYAML is the YAML representation of font configuration.
type fontsYAML struct {
	Monospace string `yaml:"monospace"`
	Heading   string `yaml:"heading"`
	Body      string `yaml:"body"`
}

// themeYAML is the top-level YAML structure for a unified theme file.
// DefaultThemeVersion is bumped when the built-in default theme changes.
// EnsureDefaultThemes regenerates default.yaml when the on-disk version is older.
const DefaultThemeVersion = 2

type themeYAML struct {
	Version  int          `yaml:"version"`
	Name     string       `yaml:"name"`
	Subtitle string       `yaml:"subtitle"`
	Fonts    fontsYAML    `yaml:"fonts"`
	Dark     themeVariant `yaml:"dark"`
	Light    themeVariant `yaml:"light"`
}

// themeVariant holds one appearance variant's colors.
type themeVariant struct {
	Background    string            `yaml:"background"`
	Foreground    string            `yaml:"foreground"`
	Gutter        string            `yaml:"gutter"`
	GutterBg      string            `yaml:"gutter-bg"`
	Cursor        string            `yaml:"cursor"`
	Selection     string            `yaml:"selection"`
	LineHighlight string            `yaml:"line-highlight"`
	StatusBg      string            `yaml:"status-bg"`
	StatusFg      string            `yaml:"status-fg"`
	Tokens        map[string]string `yaml:"tokens"`
	Find          map[string]string `yaml:"find"`
	UI            map[string]string `yaml:"ui"`
}

// LoadBundleFromFile reads a .yaml theme file and returns both variants.
func LoadBundleFromFile(path string) (ThemeBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ThemeBundle{}, err
	}
	return LoadBundleFromYAML(data)
}

// LoadBundleFromYAML parses YAML bytes into a ThemeBundle.
func LoadBundleFromYAML(data []byte) (ThemeBundle, error) {
	var ty themeYAML
	if err := yaml.Unmarshal(data, &ty); err != nil {
		return ThemeBundle{}, err
	}
	fonts := DefaultFontConfig()
	if ty.Fonts.Monospace != "" {
		fonts.Monospace = ty.Fonts.Monospace
	}
	if ty.Fonts.Heading != "" {
		fonts.Heading = ty.Fonts.Heading
	}
	if ty.Fonts.Body != "" {
		fonts.Body = ty.Fonts.Body
	}

	return ThemeBundle{
		Name:     ty.Name,
		Subtitle: ty.Subtitle,
		Fonts:    fonts,
		Dark:     variantToTheme(ty.Name, ty.Dark),
		Light:    variantToTheme(ty.Name, ty.Light),
	}, nil
}

// variantToTheme converts a YAML variant into a Theme struct.
func variantToTheme(name string, v themeVariant) Theme {
	t := Theme{
		Name:          name,
		Background:    parseColor(v.Background),
		Foreground:    parseColor(v.Foreground),
		Gutter:        parseColor(v.Gutter),
		GutterBg:      parseColor(v.GutterBg),
		Cursor:        parseColor(v.Cursor),
		Selection:     parseColor(v.Selection),
		LineHighlight: parseColor(v.LineHighlight),
		StatusBg:      parseColor(v.StatusBg),
		StatusFg:      parseColor(v.StatusFg),
	}

	// Syntax tokens
	tokenFields := map[string]*color.NRGBA{
		"keyword":  &t.Keyword,
		"string":   &t.String,
		"comment":  &t.Comment,
		"function": &t.Function,
		"type":     &t.Type,
		"number":   &t.Number,
		"operator": &t.Operator,
		"variable": &t.Variable,
	}
	for key, ptr := range tokenFields {
		if c, ok := v.Tokens[key]; ok {
			*ptr = parseColor(c)
		}
	}

	// Find colors
	if c, ok := v.Find["match"]; ok {
		t.FindMatch = parseColor(c)
	}
	if c, ok := v.Find["current"]; ok {
		t.FindCurrent = parseColor(c)
	}

	// UI element colors (kebab-case keys)
	uiFields := map[string]*color.NRGBA{
		"tab-bar-bg":       &t.TabBarBg,
		"tab-active-bg":    &t.TabActiveBg,
		"tab-border":       &t.TabBorder,
		"tab-dim-fg":       &t.TabDimFg,
		"tab-modified-dot": &t.TabModifiedDot,
		"tab-close-btn":    &t.TabCloseBtn,
		"tab-close-hover":  &t.TabCloseHover,
		"tab-plus-hover":   &t.TabPlusHover,
		"title-fg":         &t.TitleFg,
		"subtitle-fg":      &t.SubtitleFg,
		"status-border":    &t.StatusBorder,
		"gutter-sep":       &t.GutterSep,
		"scrollbar-thumb":  &t.ScrollbarThumb,
		"find-bar-bg":      &t.FindBarBg,
		"find-bar-border":  &t.FindBarBorder,
		"find-bar-input-bg": &t.FindBarInputBg,
		"find-bar-focus":   &t.FindBarFocus,
		"find-bar-text":    &t.FindBarText,
		"find-bar-dim":     &t.FindBarDim,
		"dropdown-bg":      &t.DropdownBg,
		"dropdown-sel":     &t.DropdownSel,
		"md-heading":       &t.MdHeading,
		"md-accent":        &t.MdAccent,
	}
	for key, ptr := range uiFields {
		if c, ok := v.UI[key]; ok {
			*ptr = parseColor(c)
		}
	}

	// Fallback for new fields not present in older theme files
	if t.MdHeading.A == 0 {
		t.MdHeading = color.NRGBA{R: 200, G: 80, B: 80, A: 255}
	}
	if t.MdAccent.A == 0 {
		t.MdAccent = color.NRGBA{R: 100, G: 150, B: 210, A: 255}
	}

	return t
}
