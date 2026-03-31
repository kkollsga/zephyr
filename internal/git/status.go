package git

import "strings"

// FileStatus represents one entry from git status --porcelain.
type FileStatus struct {
	Path     string // file path relative to repo root
	Index    rune   // status in the index (staged): ' ', 'M', 'A', 'D', 'R', '?'
	Worktree rune   // status in the worktree (unstaged): ' ', 'M', 'D', '?'
	OrigPath string // non-empty for renames (the original path)
}

// Status returns the list of changed files in the repository.
func (r *Repo) Status() ([]FileStatus, error) {
	out, err := Run(r.Root, "status", "--porcelain=v1", "-z")
	if err != nil {
		return nil, err
	}
	return ParseStatus(out), nil
}

// ParseStatus parses the output of git status --porcelain=v1 -z.
// The -z flag uses NUL bytes as separators and doesn't quote filenames.
func ParseStatus(data []byte) []FileStatus {
	if len(data) == 0 {
		return nil
	}

	// Split on NUL bytes
	parts := strings.Split(string(data), "\x00")
	var result []FileStatus

	for i := 0; i < len(parts); i++ {
		entry := parts[i]
		if len(entry) < 3 {
			continue
		}

		index := rune(entry[0])
		worktree := rune(entry[1])
		path := entry[3:] // skip "XY "

		fs := FileStatus{
			Path:     path,
			Index:    index,
			Worktree: worktree,
		}

		// Renames have an extra entry for the original path
		if index == 'R' || worktree == 'R' {
			if i+1 < len(parts) {
				i++
				fs.OrigPath = parts[i]
			}
		}

		result = append(result, fs)
	}

	return result
}
