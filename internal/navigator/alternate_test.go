package navigator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAlternateFile_Go(t *testing.T) {
	dir := t.TempDir()
	// Create both impl and test files
	os.WriteFile(filepath.Join(dir, "handler.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "handler_test.go"), []byte("package main"), 0644)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"impl to test", filepath.Join(dir, "handler.go"), filepath.Join(dir, "handler_test.go")},
		{"test to impl", filepath.Join(dir, "handler_test.go"), filepath.Join(dir, "handler.go")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AlternateFile(tt.in)
			if got != tt.want {
				t.Errorf("AlternateFile(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestAlternateFile_JS(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Button.tsx"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "Button.test.tsx"), []byte(""), 0644)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"impl to test", filepath.Join(dir, "Button.tsx"), filepath.Join(dir, "Button.test.tsx")},
		{"test to impl", filepath.Join(dir, "Button.test.tsx"), filepath.Join(dir, "Button.tsx")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AlternateFile(tt.in)
			if got != tt.want {
				t.Errorf("AlternateFile(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestAlternateFile_Python(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "handler.py"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "test_handler.py"), []byte(""), 0644)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"impl to test", filepath.Join(dir, "handler.py"), filepath.Join(dir, "test_handler.py")},
		{"test to impl", filepath.Join(dir, "test_handler.py"), filepath.Join(dir, "handler.py")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AlternateFile(tt.in)
			if got != tt.want {
				t.Errorf("AlternateFile(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestAlternateFile_NoMatch(t *testing.T) {
	got := AlternateFile("/some/dir/data.json")
	if got != "" {
		t.Errorf("expected empty for unsupported extension, got %q", got)
	}
}

func TestAlternateFile_NonExistent(t *testing.T) {
	// Should still return a path (the first candidate) even if file doesn't exist
	got := AlternateFile("/nonexistent/handler.go")
	if got == "" {
		t.Error("expected a candidate path even for non-existent files")
	}
	if !filepath.IsAbs(got) {
		t.Error("expected absolute path")
	}
}
