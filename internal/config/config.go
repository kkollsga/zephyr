package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the user configuration.
type Config struct {
	FontSize   float64 `json:"fontSize"`
	TabSize    int     `json:"tabSize"`
	Theme      string  `json:"theme"`
	DarkMode   bool    `json:"darkMode"`
	LineHeight float64 `json:"lineHeight"`
	WordWrap   bool    `json:"wordWrap"`
	VimMode    bool    `json:"vimMode"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		FontSize:   14,
		TabSize:    4,
		Theme:      "default",
		DarkMode:   true,
		LineHeight: 1.5,
		WordWrap:   false,
	}
}

// ConfigDir returns the path to the configuration directory.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "zephyr")
}

// LoadConfig loads configuration from ~/.config/zephyr/settings.json.
// Falls back to defaults for missing values.
func LoadConfig() Config {
	cfg := DefaultConfig()

	path := filepath.Join(ConfigDir(), "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	json.Unmarshal(data, &cfg)

	// Ensure sensible values
	if cfg.FontSize < 6 {
		cfg.FontSize = 6
	}
	if cfg.FontSize > 72 {
		cfg.FontSize = 72
	}
	if cfg.TabSize < 1 {
		cfg.TabSize = 4
	}
	if cfg.LineHeight < 1.0 {
		cfg.LineHeight = 1.0
	}
	if cfg.LineHeight > 3.0 {
		cfg.LineHeight = 3.0
	}

	return cfg
}

// SaveConfig writes configuration to ~/.config/zephyr/settings.json.
func SaveConfig(cfg Config) error {
	dir := ConfigDir()
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "settings.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
