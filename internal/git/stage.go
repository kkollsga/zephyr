package git

import "strings"

// Stage adds files to the git index.
func (r *Repo) Stage(paths ...string) error {
	args := append([]string{"add", "--"}, paths...)
	return RunSilent(r.Root, args...)
}

// Unstage removes files from the git index (unstages them).
func (r *Repo) Unstage(paths ...string) error {
	args := append([]string{"restore", "--staged", "--"}, paths...)
	return RunSilent(r.Root, args...)
}

// Discard reverts working tree changes for the given files.
func (r *Repo) Discard(paths ...string) error {
	args := append([]string{"checkout", "--"}, paths...)
	return RunSilent(r.Root, args...)
}

// Commit creates a git commit with the given message.
func (r *Repo) Commit(message string) error {
	return RunSilent(r.Root, "commit", "-m", message)
}

// StagedFiles returns the list of files in the staging area.
func (r *Repo) StagedFiles() ([]string, error) {
	out, err := Run(r.Root, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}
