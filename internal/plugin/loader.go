package plugin

import (
	"os"
	"path/filepath"
	"strings"
)

// PluginDir returns the path to the plugins directory.
func PluginDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "zephyr", "plugins")
}

// Discover finds all .lua plugin files in the plugins directory.
func Discover() []string {
	dir := PluginDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var plugins []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".lua") {
			plugins = append(plugins, filepath.Join(dir, entry.Name()))
		}
	}
	return plugins
}

// LoadAll discovers and loads all plugins.
func LoadAll(api *EditorAPI) ([]*LuaPlugin, []error) {
	paths := Discover()
	var plugins []*LuaPlugin
	var errors []error

	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".lua")
		p := NewLuaPlugin(name, path, api)
		if err := p.Load(); err != nil {
			errors = append(errors, err)
			continue
		}
		plugins = append(plugins, p)
	}

	return plugins, errors
}
