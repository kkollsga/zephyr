package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// KeymapEntry represents a single keybinding override.
type KeymapEntry struct {
	Key     string `json:"key"`
	Command string `json:"command"`
}

// LoadKeymap loads keybinding overrides from ~/.config/zephyr/keybindings.json.
func LoadKeymap() []KeymapEntry {
	path := filepath.Join(ConfigDir(), "keybindings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entries []KeymapEntry
	json.Unmarshal(data, &entries)
	return entries
}

// DefaultKeymapJSON returns the default keybindings as JSON.
func DefaultKeymapJSON() string {
	entries := []KeymapEntry{
		{Key: "Cmd+S", Command: "file.save"},
		{Key: "Cmd+O", Command: "file.open"},
		{Key: "Cmd+N", Command: "file.new"},
		{Key: "Cmd+W", Command: "file.close"},
		{Key: "Cmd+Z", Command: "edit.undo"},
		{Key: "Cmd+Shift+Z", Command: "edit.redo"},
		{Key: "Cmd+X", Command: "edit.cut"},
		{Key: "Cmd+C", Command: "edit.copy"},
		{Key: "Cmd+V", Command: "edit.paste"},
		{Key: "Cmd+A", Command: "edit.selectAll"},
		{Key: "Cmd+F", Command: "edit.find"},
		{Key: "Cmd+Shift+F", Command: "edit.findReplace"},
		{Key: "Cmd+B", Command: "view.sidebar"},
		{Key: "Cmd+Shift+P", Command: "view.commandPalette"},
		{Key: "Cmd+P", Command: "view.fuzzyFinder"},
		{Key: "Cmd+Q", Command: "app.quit"},
		{Key: "Cmd+D", Command: "edit.selectNextOccurrence"},
		{Key: "Cmd+Shift+L", Command: "edit.splitSelectionIntoLines"},
	}
	data, _ := json.MarshalIndent(entries, "", "  ")
	return string(data)
}
