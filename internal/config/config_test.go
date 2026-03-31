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

func TestConfig_AddRecentRoot(t *testing.T) {
	cfg := DefaultConfig()

	// Add first root
	cfg.AddRecentRoot("/project/a")
	if len(cfg.RecentRoots) != 1 || cfg.RecentRoots[0] != "/project/a" {
		t.Fatalf("expected [/project/a], got %v", cfg.RecentRoots)
	}

	// Add second root — prepended
	cfg.AddRecentRoot("/project/b")
	if len(cfg.RecentRoots) != 2 || cfg.RecentRoots[0] != "/project/b" {
		t.Fatalf("expected [/project/b, /project/a], got %v", cfg.RecentRoots)
	}

	// Add duplicate — deduplicates and moves to front
	cfg.AddRecentRoot("/project/a")
	if len(cfg.RecentRoots) != 2 || cfg.RecentRoots[0] != "/project/a" {
		t.Fatalf("expected [/project/a, /project/b], got %v", cfg.RecentRoots)
	}

	// Add 10 more — capped at 10
	for i := 0; i < 12; i++ {
		cfg.AddRecentRoot("/project/" + string(rune('c'+i)))
	}
	if len(cfg.RecentRoots) != 10 {
		t.Fatalf("expected 10 roots, got %d", len(cfg.RecentRoots))
	}
}

func TestConfig_RecentRoots_JSON(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AddRecentRoot("/project/x")
	cfg.AddRecentRoot("/project/y")

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var loaded Config
	json.Unmarshal(data, &loaded)
	if len(loaded.RecentRoots) != 2 {
		t.Fatalf("expected 2 roots after round-trip, got %d", len(loaded.RecentRoots))
	}
	if loaded.RecentRoots[0] != "/project/y" {
		t.Errorf("first root = %q, want /project/y", loaded.RecentRoots[0])
	}
}
