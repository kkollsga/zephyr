package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_LoadDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.FontSize != 14 {
		t.Fatalf("expected fontSize 14, got %f", cfg.FontSize)
	}
	if cfg.TabSize != 4 {
		t.Fatalf("expected tabSize 4, got %d", cfg.TabSize)
	}
}

func TestConfig_LoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(`{"fontSize": 18, "tabSize": 2}`), 0644)

	cfg := DefaultConfig()
	data, _ := os.ReadFile(path)
	json.Unmarshal(data, &cfg)
	if cfg.FontSize != 18 {
		t.Fatalf("expected fontSize 18, got %f", cfg.FontSize)
	}
	if cfg.TabSize != 2 {
		t.Fatalf("expected tabSize 2, got %d", cfg.TabSize)
	}
}

func TestConfig_PartialOverride(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FontSize = 20
	if cfg.TabSize != 4 {
		t.Fatalf("expected tabSize 4 (default), got %d", cfg.TabSize)
	}
}
