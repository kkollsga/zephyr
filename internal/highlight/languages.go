package highlight

import (
	"sort"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
)

// LanguageInfo holds the tree-sitter language and its highlight query.
type LanguageInfo struct {
	Name     string
	Language *sitter.Language
	Query    string
}

// registry maps file extensions to language info.
var registry = map[string]*LanguageInfo{}

func init() {
	Register(".go", &LanguageInfo{Name: "Go", Language: golang.GetLanguage(), Query: goHighlightQuery})
	Register(".py", &LanguageInfo{Name: "Python", Language: python.GetLanguage(), Query: pythonHighlightQuery})
	Register(".js", &LanguageInfo{Name: "JavaScript", Language: javascript.GetLanguage(), Query: jsHighlightQuery})
	Register(".jsx", &LanguageInfo{Name: "JavaScript", Language: javascript.GetLanguage(), Query: jsHighlightQuery})
	Register(".rs", &LanguageInfo{Name: "Rust", Language: rust.GetLanguage(), Query: rustHighlightQuery})
}

// Register adds a language to the registry.
func Register(ext string, info *LanguageInfo) {
	registry[ext] = info
}

// ForExtension returns the language info for a file extension, or nil.
func ForExtension(ext string) *LanguageInfo {
	return registry[ext]
}

// ForName returns the language info for a language name, or nil.
func ForName(name string) *LanguageInfo {
	for _, info := range registry {
		if info.Name == name {
			return info
		}
	}
	return nil
}

// LanguageNames returns a sorted list of available language names.
func LanguageNames() []string {
	seen := map[string]bool{}
	for _, info := range registry {
		seen[info.Name] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
