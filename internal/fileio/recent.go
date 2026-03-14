package fileio

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// MaxRecentFiles is the maximum number of recent files to track.
const MaxRecentFiles = 20

// RecentFiles manages a list of recently opened files.
type RecentFiles struct {
	Files    []string
	filePath string
}

// NewRecentFiles loads the recent files list from the config directory.
func NewRecentFiles() *RecentFiles {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "zephyr")
	os.MkdirAll(configDir, 0755)
	filePath := filepath.Join(configDir, "recent.json")

	rf := &RecentFiles{filePath: filePath}
	rf.load()
	return rf
}

// Add adds a file to the front of the recent list.
func (rf *RecentFiles) Add(path string) {
	absPath, _ := filepath.Abs(path)

	// Remove if already present
	filtered := make([]string, 0, len(rf.Files))
	for _, f := range rf.Files {
		if f != absPath {
			filtered = append(filtered, f)
		}
	}

	// Prepend
	rf.Files = append([]string{absPath}, filtered...)
	if len(rf.Files) > MaxRecentFiles {
		rf.Files = rf.Files[:MaxRecentFiles]
	}

	rf.save()
}

func (rf *RecentFiles) load() {
	data, err := os.ReadFile(rf.filePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &rf.Files)
}

func (rf *RecentFiles) save() {
	data, _ := json.MarshalIndent(rf.Files, "", "  ")
	os.WriteFile(rf.filePath, data, 0644)
}
