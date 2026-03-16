package main

import (
	"fmt"
	"strings"
	"time"
)

// Zephyr — The caffeinated editor
// A fast, GPU-accelerated text editor written in Go.

type Editor struct {
	Name       string
	Version    [3]int
	Tabs       []Tab
	Theme      Theme
	Modified   bool
	StartedAt  time.Time
}

type Tab struct {
	Title    string
	FilePath string
	Language string
	Lines    int
}

type Theme struct {
	Name       string
	Background string
	Foreground string
	Keywords   string
	Strings    string
	Comments   string
}

// NewEditor creates a fresh editor instance with sensible defaults.
func NewEditor(name string) *Editor {
	return &Editor{
		Name:      name,
		Version:   [3]int{0, 1, 0},
		Theme:     DarkTheme(),
		StartedAt: time.Now(),
	}
}

func DarkTheme() Theme {
	return Theme{
		Name:       "Zephyr Dark",
		Background: "#1e1e1e",
		Foreground: "#d4d4d4",
		Keywords:   "#569cd6",
		Strings:    "#ce9178",
		Comments:   "#6a9955",
	}
}

// OpenFile loads a file into a new tab.
func (e *Editor) OpenFile(path string) error {
	lang := detectLanguage(path)
	tab := Tab{
		Title:    lastSegment(path, "/"),
		FilePath: path,
		Language: lang,
	}
	e.Tabs = append(e.Tabs, tab)
	return nil
}

func detectLanguage(path string) string {
	extensions := map[string]string{
		".go":   "Go",
		".py":   "Python",
		".rs":   "Rust",
		".js":   "JavaScript",
		".ts":   "TypeScript",
		".lua":  "Lua",
		".html": "HTML",
		".css":  "CSS",
	}
	for ext, lang := range extensions {
		if strings.HasSuffix(path, ext) {
			return lang
		}
	}
	return "Plain Text"
}

func lastSegment(path, sep string) string {
	parts := strings.Split(path, sep)
	return parts[len(parts)-1]
}

func main() {
	editor := NewEditor("Zephyr")
	editor.OpenFile("main.go")
	editor.OpenFile("README.md")

	fmt.Printf("%s v%d.%d.%d\n",
		editor.Name,
		editor.Version[0],
		editor.Version[1],
		editor.Version[2],
	)
	fmt.Printf("Theme: %s\n", editor.Theme.Name)
	fmt.Printf("Open tabs: %d\n", len(editor.Tabs))
	fmt.Printf("Ready in %v\n", time.Since(editor.StartedAt))
}
