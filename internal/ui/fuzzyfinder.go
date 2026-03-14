package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kristianweb/zephyr/internal/fuzzy"
)

// FuzzyFinder manages the file finder overlay state.
type FuzzyFinder struct {
	Visible  bool
	Query    string
	Results  []fuzzy.Match
	Selected int
	Files    []string
	RootDir  string
}

// NewFuzzyFinder creates a new fuzzy file finder.
func NewFuzzyFinder() *FuzzyFinder {
	return &FuzzyFinder{}
}

// Open shows the fuzzy finder. Scans the directory for files if not already loaded.
func (ff *FuzzyFinder) Open(rootDir string) {
	ff.Visible = true
	ff.Query = ""
	ff.Selected = 0
	if ff.RootDir != rootDir || len(ff.Files) == 0 {
		ff.RootDir = rootDir
		ff.scanFiles()
	}
	ff.Results = fuzzy.RankMatches("", ff.Files)
	if len(ff.Results) > 100 {
		ff.Results = ff.Results[:100]
	}
}

// Close hides the fuzzy finder.
func (ff *FuzzyFinder) Close() {
	ff.Visible = false
	ff.Query = ""
	ff.Results = nil
}

// UpdateQuery filters files based on the query.
func (ff *FuzzyFinder) UpdateQuery(query string) {
	ff.Query = query
	ff.Results = fuzzy.RankMatches(query, ff.Files)
	if len(ff.Results) > 100 {
		ff.Results = ff.Results[:100]
	}
	ff.Selected = 0
}

// MoveUp moves selection up.
func (ff *FuzzyFinder) MoveUp() {
	if ff.Selected > 0 {
		ff.Selected--
	}
}

// MoveDown moves selection down.
func (ff *FuzzyFinder) MoveDown() {
	if ff.Selected < len(ff.Results)-1 {
		ff.Selected++
	}
}

// SelectedPath returns the full path of the selected file, or empty string.
func (ff *FuzzyFinder) SelectedPath() string {
	if ff.Selected < 0 || ff.Selected >= len(ff.Results) {
		return ""
	}
	return filepath.Join(ff.RootDir, ff.Results[ff.Selected].Text)
}

func (ff *FuzzyFinder) scanFiles() {
	ff.Files = nil
	filepath.Walk(ff.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := info.Name()

		// Skip hidden dirs, node_modules, .git, etc.
		if info.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files and binary-looking files
		if strings.HasPrefix(name, ".") {
			return nil
		}

		rel, _ := filepath.Rel(ff.RootDir, path)
		ff.Files = append(ff.Files, rel)
		return nil
	})
}
