package git

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
