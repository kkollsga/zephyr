package highlight

import (
	"path/filepath"
	"sort"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	tree_sitter_markdown "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
)

// LanguageInfo holds the tree-sitter language and its highlight query.
type LanguageInfo struct {
	Name         string
	Language     *sitter.Language
	Query        string
	initLang     func() *sitter.Language                     // lazy loader; nil once Language is set
	SimpleTokens func(source []byte, startRow, endRow int) []Token // fallback tokenizer when no tree-sitter grammar
}

// ensureLoaded initializes the Language field on first access.
func (li *LanguageInfo) ensureLoaded() {
	if li.Language == nil && li.initLang != nil {
		li.Language = li.initLang()
		li.initLang = nil
	}
}

// registry maps file extensions to language info.
var registry = map[string]*LanguageInfo{}

func init() {
	Register(".go", &LanguageInfo{Name: "Go", initLang: golang.GetLanguage, Query: goHighlightQuery})
	Register(".py", &LanguageInfo{Name: "Python", initLang: python.GetLanguage, Query: pythonHighlightQuery})
	Register(".js", &LanguageInfo{Name: "JavaScript", initLang: javascript.GetLanguage, Query: jsHighlightQuery})
	Register(".jsx", &LanguageInfo{Name: "JavaScript", initLang: javascript.GetLanguage, Query: jsHighlightQuery})
	Register(".rs", &LanguageInfo{Name: "Rust", initLang: rust.GetLanguage, Query: rustHighlightQuery})
	Register(".md", &LanguageInfo{Name: "Markdown", initLang: tree_sitter_markdown.GetLanguage, Query: markdownHighlightQuery})
	Register(".json", &LanguageInfo{Name: "JSON", SimpleTokens: jsonTokenize})
}

// Register adds a language to the registry.
func Register(ext string, info *LanguageInfo) {
	registry[ext] = info
}

// ForExtension returns the language info for a file extension, or nil.
func ForExtension(ext string) *LanguageInfo {
	info := registry[ext]
	if info != nil {
		info.ensureLoaded()
	}
	return info
}

// ForName returns the language info for a language name, or nil.
func ForName(name string) *LanguageInfo {
	for _, info := range registry {
		if info.Name == name {
			info.ensureLoaded()
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

// DetectLanguage returns the language name for a file path based on extension.
func DetectLanguage(path string) string {
	if path == "" {
		return "Plain Text"
	}
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".js":
		return "JavaScript"
	case ".ts":
		return "TypeScript"
	case ".rs":
		return "Rust"
	case ".c", ".h":
		return "C"
	case ".cpp", ".hpp", ".cc":
		return "C++"
	case ".java":
		return "Java"
	case ".rb":
		return "Ruby"
	case ".lua":
		return "Lua"
	case ".md":
		return "Markdown"
	case ".json":
		return "JSON"
	case ".yaml", ".yml":
		return "YAML"
	case ".toml":
		return "TOML"
	case ".html", ".htm":
		return "HTML"
	case ".css":
		return "CSS"
	case ".sh", ".bash", ".zsh":
		return "Shell"
	case ".txt":
		return "Plain Text"
	default:
		return "Plain Text"
	}
}

// ExtensionForLanguage returns a file extension for a language name.
func ExtensionForLanguage(lang string) string {
	switch lang {
	case "Go":
		return ".go"
	case "Python":
		return ".py"
	case "JavaScript":
		return ".js"
	case "TypeScript":
		return ".ts"
	case "Rust":
		return ".rs"
	case "C":
		return ".c"
	case "C++":
		return ".cpp"
	case "Java":
		return ".java"
	case "Ruby":
		return ".rb"
	case "Lua":
		return ".lua"
	case "Markdown":
		return ".md"
	case "JSON":
		return ".json"
	case "YAML":
		return ".yaml"
	case "TOML":
		return ".toml"
	case "HTML":
		return ".html"
	case "CSS":
		return ".css"
	case "Shell":
		return ".sh"
	default:
		return ".txt"
	}
}
