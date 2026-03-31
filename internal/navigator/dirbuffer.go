package navigator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kristianweb/zephyr/internal/git"
)

// DirEntry represents a file or directory in a directory buffer.
type DirEntry struct {
	Name      string
	Path      string
	IsDir     bool
	GitStatus rune // 'M', 'A', 'D', '?', ' '
	Added     int
	Deleted   int
}

// DirBuffer holds the data for an oil-style directory buffer.
type DirBuffer struct {
	DirPath    string
	Entries    []DirEntry
	ShowHidden bool
	headerLines int // number of header lines (path + separator)
}

// NewDirBuffer creates a directory buffer by scanning the directory.
func NewDirBuffer(dirPath string, statuses []git.FileStatus, diffStat map[string][2]int, repoRoot string) *DirBuffer {
	db := &DirBuffer{
		DirPath:     dirPath,
		headerLines: 2,
	}
	db.loadEntries(statuses, diffStat, repoRoot)
	return db
}

func (db *DirBuffer) loadEntries(statuses []git.FileStatus, diffStat map[string][2]int, repoRoot string) {
	entries, err := os.ReadDir(db.DirPath)
	if err != nil {
		return
	}

	// Build a map of git statuses relative to this directory
	statusMap := make(map[string]rune)
	for _, s := range statuses {
		absPath := filepath.Join(repoRoot, s.Path)
		if dir := filepath.Dir(absPath); dir == db.DirPath {
			name := filepath.Base(absPath)
			switch {
			case s.Index == 'A' || (s.Index == '?' && s.Worktree == '?'):
				statusMap[name] = 'A'
			case s.Index == 'M' || s.Worktree == 'M':
				statusMap[name] = 'M'
			case s.Index == 'D' || s.Worktree == 'D':
				statusMap[name] = 'D'
			case s.Index == '?' || s.Worktree == '?':
				statusMap[name] = '?'
			}
		}
	}

	var dirs, files []DirEntry
	for _, e := range entries {
		name := e.Name()
		// Skip hidden files unless ShowHidden is true
		if !db.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}
		// Skip common noise directories
		if e.IsDir() && (name == "node_modules" || name == "__pycache__" || name == "vendor") {
			if !db.ShowHidden {
				continue
			}
		}

		entry := DirEntry{
			Name:      name,
			Path:      filepath.Join(db.DirPath, name),
			IsDir:     e.IsDir(),
			GitStatus: statusMap[name],
		}
		if entry.GitStatus == 0 {
			entry.GitStatus = ' '
		}

		// Look up diff stats
		if repoRoot != "" {
			relPath, err := filepath.Rel(repoRoot, entry.Path)
			if err == nil {
				if stats, ok := diffStat[relPath]; ok {
					entry.Added = stats[0]
					entry.Deleted = stats[1]
				}
			}
		}

		if e.IsDir() {
			entry.Name += "/"
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	// Sort: directories first (alphabetical), then files (alphabetical)
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	db.Entries = append(dirs, files...)
}

// GenerateText produces the buffer content for the directory.
func (db *DirBuffer) GenerateText() string {
	var b strings.Builder

	// Header: directory path
	b.WriteString(db.DirPath)
	b.WriteString("/\n")
	// Separator
	b.WriteString(strings.Repeat("─", 40))
	b.WriteString("\n")

	// Entries
	for _, e := range db.Entries {
		statusChar := ' '
		if e.GitStatus != ' ' {
			statusChar = e.GitStatus
		}

		line := fmt.Sprintf("%c %s", statusChar, e.Name)

		// Right-align diff stats
		if e.Added > 0 || e.Deleted > 0 {
			stats := fmt.Sprintf("+%d -%d", e.Added, e.Deleted)
			padding := 40 - len(line) - len(stats)
			if padding < 2 {
				padding = 2
			}
			line += strings.Repeat(" ", padding) + stats
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// EntryAtLine returns the DirEntry at the given buffer line (0-based), or nil.
func (db *DirBuffer) EntryAtLine(line int) *DirEntry {
	idx := line - db.headerLines
	if idx < 0 || idx >= len(db.Entries) {
		return nil
	}
	return &db.Entries[idx]
}

// Refresh reloads the directory entries.
func (db *DirBuffer) Refresh(statuses []git.FileStatus, diffStat map[string][2]int, repoRoot string) {
	db.Entries = nil
	db.loadEntries(statuses, diffStat, repoRoot)
}
