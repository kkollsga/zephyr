package navigator

import (
	"regexp"
	"strings"
)

// ImportEntry represents a single import in a source file.
type ImportEntry struct {
	Path  string // import path as written
	Line  int    // 0-based line number
}

// Go import patterns
var (
	goSingleImportRe = regexp.MustCompile(`^import\s+"([^"]+)"`)
	goBlockImportRe  = regexp.MustCompile(`^\s*(?:\w+\s+)?"([^"]+)"`)
)

// ExtractGoImports extracts import paths from Go source code.
func ExtractGoImports(source string) []ImportEntry {
	lines := strings.Split(source, "\n")
	var imports []ImportEntry
	inBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Single import
		if matches := goSingleImportRe.FindStringSubmatch(trimmed); matches != nil {
			imports = append(imports, ImportEntry{Path: matches[1], Line: i})
			continue
		}

		// Start of import block
		if trimmed == "import (" {
			inBlock = true
			continue
		}

		// End of import block
		if inBlock && trimmed == ")" {
			inBlock = false
			continue
		}

		// Import within block
		if inBlock {
			if matches := goBlockImportRe.FindStringSubmatch(trimmed); matches != nil {
				imports = append(imports, ImportEntry{Path: matches[1], Line: i})
			}
		}
	}
	return imports
}

// ResolveGoImport attempts to resolve a Go import path to a local file.
// For standard library imports, returns empty string.
// For relative imports within the module, returns the file path.
func ResolveGoImport(importPath, modulePath, repoRoot string) string {
	// Check if this is a module-local import
	if !strings.HasPrefix(importPath, modulePath) {
		return ""
	}
	// Convert module path to file system path
	relPath := strings.TrimPrefix(importPath, modulePath+"/")
	if relPath == importPath {
		return ""
	}
	return relPath
}
