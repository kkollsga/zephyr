package navigator

import "testing"

func TestExtractGoImports_SingleImport(t *testing.T) {
	source := `package main

import "fmt"

func main() {}
`
	imports := ExtractGoImports(source)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	if imports[0].Path != "fmt" {
		t.Errorf("path = %q, want fmt", imports[0].Path)
	}
}

func TestExtractGoImports_Block(t *testing.T) {
	source := `package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kristianweb/zephyr/internal/editor"
	ed "github.com/kristianweb/zephyr/internal/editor/v2"
)
`
	imports := ExtractGoImports(source)
	if len(imports) != 5 {
		t.Fatalf("expected 5 imports, got %d", len(imports))
	}

	paths := make([]string, len(imports))
	for i, imp := range imports {
		paths[i] = imp.Path
	}

	expected := []string{
		"fmt",
		"os",
		"path/filepath",
		"github.com/kristianweb/zephyr/internal/editor",
		"github.com/kristianweb/zephyr/internal/editor/v2",
	}
	for i, want := range expected {
		if paths[i] != want {
			t.Errorf("import[%d] = %q, want %q", i, paths[i], want)
		}
	}
}

func TestExtractGoImports_Empty(t *testing.T) {
	source := `package main

func main() {}
`
	imports := ExtractGoImports(source)
	if len(imports) != 0 {
		t.Errorf("expected 0 imports, got %d", len(imports))
	}
}

func TestExtractGoImports_LineNumbers(t *testing.T) {
	source := `package main

import (
	"fmt"
	"os"
)
`
	imports := ExtractGoImports(source)
	if len(imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(imports))
	}
	if imports[0].Line != 3 {
		t.Errorf("fmt line = %d, want 3", imports[0].Line)
	}
	if imports[1].Line != 4 {
		t.Errorf("os line = %d, want 4", imports[1].Line)
	}
}

func TestResolveGoImport(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		modulePath string
		want       string
	}{
		{
			"local import",
			"github.com/kristianweb/zephyr/internal/editor",
			"github.com/kristianweb/zephyr",
			"internal/editor",
		},
		{
			"stdlib import",
			"fmt",
			"github.com/kristianweb/zephyr",
			"",
		},
		{
			"external import",
			"github.com/other/pkg",
			"github.com/kristianweb/zephyr",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveGoImport(tt.importPath, tt.modulePath, "/repo")
			if got != tt.want {
				t.Errorf("ResolveGoImport(%q) = %q, want %q", tt.importPath, got, tt.want)
			}
		})
	}
}
