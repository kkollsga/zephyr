package git

import (
	"fmt"
	"strconv"
	"strings"
)

// Repo represents a discovered git repository.
type Repo struct {
	Root   string // absolute path to the working tree root
	GitDir string // absolute path to .git directory
}

// Discover finds the git repository containing the given path.
// Returns nil, nil if path is not inside a git repo.
func Discover(path string) (*Repo, error) {
	root, err := Run(path, "rev-parse", "--show-toplevel")
	if err != nil {
		// Not a git repo — not an error
		return nil, nil
	}
	gitDir, err := Run(path, "rev-parse", "--git-dir")
	if err != nil {
		return nil, err
	}
	return &Repo{
		Root:   strings.TrimSpace(string(root)),
		GitDir: strings.TrimSpace(string(gitDir)),
	}, nil
}

// Head returns the current branch name and abbreviated commit hash.
func (r *Repo) Head() (branch string, hash string, err error) {
	b, err := Run(r.Root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", "", err
	}
	h, err := Run(r.Root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(string(b)), strings.TrimSpace(string(h)), nil
}

// Upstream returns the remote tracking info: remote name, commits ahead, commits behind.
// Returns empty remote and zero counts if no upstream is configured.
func (r *Repo) Upstream() (remote string, ahead int, behind int, err error) {
	rem, err := Run(r.Root, "rev-parse", "--abbrev-ref", "@{upstream}")
	if err != nil {
		// No upstream configured
		return "", 0, 0, nil
	}
	remote = strings.TrimSpace(string(rem))

	out, err := Run(r.Root, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return remote, 0, 0, nil
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 2 {
		ahead, _ = strconv.Atoi(parts[0])
		behind, _ = strconv.Atoi(parts[1])
	}
	return remote, ahead, behind, nil
}

// ChangedFiles returns the list of file paths that have changes (staged or unstaged).
func (r *Repo) ChangedFiles() ([]string, error) {
	statuses, err := r.Status()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, s := range statuses {
		paths = append(paths, s.Path)
	}
	return paths, nil
}

// DiffStat returns per-file +/- line counts for the given ref comparison.
func (r *Repo) DiffStat(ref string) (map[string][2]int, error) {
	out, err := Run(r.Root, "diff", "--numstat", ref)
	if err != nil {
		return nil, err
	}
	result := make(map[string][2]int)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		deleted, _ := strconv.Atoi(parts[1])
		path := parts[2]
		// Handle renames: "old => new"
		if idx := strings.Index(path, " => "); idx >= 0 {
			path = path[idx+4:]
		}
		result[path] = [2]int{added, deleted}
	}
	return result, nil
}

// Show retrieves the content of a file at a given ref.
func (r *Repo) Show(ref, path string) ([]byte, error) {
	return Run(r.Root, "show", fmt.Sprintf("%s:%s", ref, path))
}
