package config

import (
	"image/color"
	"os"

	"gopkg.in/yaml.v3"
)

// ThemeBundle holds both dark and light variants from a single theme file.
type ThemeBundle struct {
	Name     string
	Subtitle string // custom tagline, e.g. "The caffeinated editor"
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

// themeYAML is the top-level YAML structure for a unified theme file.
type themeYAML struct {
	Name     string       `yaml:"name"`
	Subtitle string       `yaml:"subtitle"`
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
	return ThemeBundle{
		Name:     ty.Name,
		Subtitle: ty.Subtitle,
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
	}
	for key, ptr := range uiFields {
		if c, ok := v.UI[key]; ok {
			*ptr = parseColor(c)
		}
	}

	return t
}
