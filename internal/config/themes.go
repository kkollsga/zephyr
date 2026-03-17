package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ThemeMeta describes a theme file found on disk.
type ThemeMeta struct {
	Name string
	Path string
}

// ThemeDir returns the directory where custom themes are stored.
func ThemeDir() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = os.Getenv("HOME")
	}
	return filepath.Join(cfgDir, "zephyr", "themes")
}

// ListThemes scans the theme directory for .json files and returns metadata.
func ListThemes() []ThemeMeta {
	dir := ThemeDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var themes []ThemeMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		themes = append(themes, ThemeMeta{
			Name: name,
			Path: filepath.Join(dir, e.Name()),
		})
	}
	return themes
}

// LoadThemeByName loads a theme by name from the theme directory.
func LoadThemeByName(name string) (Theme, error) {
	path := filepath.Join(ThemeDir(), name+".json")
	return LoadThemeFromFile(path)
}

// EnsureDefaultThemes writes the built-in dark.json and light.json to the
// theme directory if they don't already exist.
func EnsureDefaultThemes() error {
	dir := ThemeDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	defaults := map[string]Theme{
		"dark":  DarkTheme(),
		"light": LightTheme(),
	}
	for name, theme := range defaults {
		path := filepath.Join(dir, name+".json")
		if _, err := os.Stat(path); err == nil {
			continue // already exists
		}
		data, err := marshalThemeJSON(theme)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// marshalThemeJSON converts a Theme to its JSON representation.
func marshalThemeJSON(t Theme) ([]byte, error) {
	tj := themeJSON{
		Name:          t.Name,
		Background:    ColorToHex(t.Background),
		Foreground:    ColorToHex(t.Foreground),
		Gutter:        ColorToHex(t.Gutter),
		GutterBg:      ColorToHex(t.GutterBg),
		Cursor:        ColorToHex(t.Cursor),
		Selection:     ColorToHex(t.Selection),
		LineHighlight: ColorToHex(t.LineHighlight),
		StatusBg:      ColorToHex(t.StatusBg),
		StatusFg:      ColorToHex(t.StatusFg),
		Tokens: map[string]string{
			"keyword":  ColorToHex(t.Keyword),
			"string":   ColorToHex(t.String),
			"comment":  ColorToHex(t.Comment),
			"function": ColorToHex(t.Function),
			"type":     ColorToHex(t.Type),
			"number":   ColorToHex(t.Number),
			"operator": ColorToHex(t.Operator),
			"variable": ColorToHex(t.Variable),
		},
		Find: map[string]string{
			"match":   ColorToHex(t.FindMatch),
			"current": ColorToHex(t.FindCurrent),
		},
		UI: map[string]string{
			"tabBarBg":       ColorToHex(t.TabBarBg),
			"tabActiveBg":    ColorToHex(t.TabActiveBg),
			"tabBorder":      ColorToHex(t.TabBorder),
			"tabDimFg":       ColorToHex(t.TabDimFg),
			"tabModifiedDot": ColorToHex(t.TabModifiedDot),
			"tabCloseBtn":    ColorToHex(t.TabCloseBtn),
			"tabCloseHover":  ColorToHex(t.TabCloseHover),
			"tabPlusHover":   ColorToHex(t.TabPlusHover),
			"titleFg":        ColorToHex(t.TitleFg),
			"subtitleFg":     ColorToHex(t.SubtitleFg),
			"statusBorder":   ColorToHex(t.StatusBorder),
			"gutterSep":      ColorToHex(t.GutterSep),
			"scrollbarThumb": ColorToHex(t.ScrollbarThumb),
			"findBarBg":      ColorToHex(t.FindBarBg),
			"findBarBorder":  ColorToHex(t.FindBarBorder),
			"findBarInputBg": ColorToHex(t.FindBarInputBg),
			"findBarFocus":   ColorToHex(t.FindBarFocus),
			"findBarText":    ColorToHex(t.FindBarText),
			"findBarDim":     ColorToHex(t.FindBarDim),
			"dropdownBg":     ColorToHex(t.DropdownBg),
			"dropdownSel":    ColorToHex(t.DropdownSel),
		},
	}
	return json.MarshalIndent(tj, "", "  ")
}
