package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kristianweb/zephyr/internal/fuzzy"
)

// FuzzyFinder manages the file finder overlay state.
type FuzzyFinder struct {
	Visible      bool
	Query        string
	Results      []fuzzy.Match
	Selected     int
	Files        []string
	RootDir      string
	ChangedFiles []string // git-changed files for boosted ranking

	// True when Files holds the changed-file list rather than a directory
	// scan. Without it a changed-file open would poison the cache the
	// all-files open reuses, and the file finder would list only the changed
	// files for as long as the root stayed the same.
	changedOnly bool
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
	if ff.RootDir != rootDir || len(ff.Files) == 0 || ff.changedOnly {
		ff.RootDir = rootDir
		ff.changedOnly = false
		ff.scanFiles()
	}
	ff.Results = fuzzy.RankMatches("", ff.Files)
	if len(ff.Results) > 100 {
		ff.Results = ff.Results[:100]
	}
}

// OpenChanged shows the fuzzy finder with only changed files.
func (ff *FuzzyFinder) OpenChanged(rootDir string, changedFiles []string) {
	ff.Visible = true
	ff.Query = ""
	ff.Selected = 0
	ff.RootDir = rootDir
	// git prints its status paths with forward slashes already; normalising
	// keeps the invariant one line rather than one per producer, since
	// SelectedPath converts back on the way out.
	ff.ChangedFiles = toFinderPaths(changedFiles)
	ff.Files = ff.ChangedFiles
	ff.changedOnly = true
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
	return filepath.Join(ff.RootDir, filepath.FromSlash(ff.Results[ff.Selected].Text))
}

// toFinderPath rewrites a relative path built with sep into slash form. The
// finder both displays and fuzzy-matches these strings, and the matcher is a
// plain subsequence test: a Windows walk producing "src\editor.go" makes a
// query of "src/ed" match nothing and shows the user a separator no other
// pane uses. sep is the separator the path was built with
// (filepath.Separator in production) — taking it as an argument keeps the
// rewrite exercisable on a host whose separator is already a slash, where
// filepath.ToSlash is a no-op. On such a host a backslash is an ordinary
// filename byte and is left alone.
func toFinderPath(p string, sep rune) string {
	if sep == '/' {
		return p
	}
	return strings.ReplaceAll(p, string(sep), "/")
}

func toFinderPaths(paths []string) []string {
	if filepath.Separator == '/' {
		return paths
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = toFinderPath(p, filepath.Separator)
	}
	return out
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
		ff.Files = append(ff.Files, toFinderPath(rel, filepath.Separator))
		return nil
	})
}
