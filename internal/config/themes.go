package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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

// ListThemes scans the theme directory for .yaml files and returns metadata.
func ListThemes() []ThemeMeta {
	dir := ThemeDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var themes []ThemeMeta
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") {
			themes = append(themes, ThemeMeta{
				Name: strings.TrimSuffix(name, ".yaml"),
				Path: filepath.Join(dir, name),
			})
		} else if strings.HasSuffix(name, ".yml") {
			themes = append(themes, ThemeMeta{
				Name: strings.TrimSuffix(name, ".yml"),
				Path: filepath.Join(dir, name),
			})
		}
	}
	return themes
}

// LoadBundleByName loads a theme bundle by name from the theme directory.
func LoadBundleByName(name string) (ThemeBundle, error) {
	path := filepath.Join(ThemeDir(), name+".yaml")
	return LoadBundleFromFile(path)
}

// LoadThemeByName loads a theme by name from the theme directory (legacy JSON).
func LoadThemeByName(name string) (Theme, error) {
	path := filepath.Join(ThemeDir(), name+".json")
	return LoadThemeFromFile(path)
}

// EnsureDefaultThemes writes the built-in default.yaml to the theme directory
// if it doesn't exist or its version is older than the embedded default.
func EnsureDefaultThemes() error {
	dir := ThemeDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "default.yaml")

	// Check if existing file needs updating
	if data, err := os.ReadFile(path); err == nil {
		var header struct {
			Version int `yaml:"version"`
		}
		if yaml.Unmarshal(data, &header) == nil && header.Version >= DefaultThemeVersion {
			return nil // up to date
		}
		// Outdated version — regenerate
	}

	return os.WriteFile(path, defaultThemeYAML, 0644)
}
