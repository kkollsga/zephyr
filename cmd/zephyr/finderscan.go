package main

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/kristianweb/zephyr/internal/git"
	"github.com/kristianweb/zephyr/internal/ui"
)

// gitFinderScan is the file finder's scanner: git's own file list when root is
// inside a repository, the plain walk otherwise. Going through git is what
// makes the finder honour .gitignore, which a walk cannot do without
// reimplementing the exclude rules.
//
// internal/ui must not reach internal/git, so the finder takes this as an
// injected Scan rather than owning it.
func gitFinderScan(root string, stop <-chan struct{}) []string {
	if files, ok := gitListFiles(root); ok {
		return files
	}
	return ui.WalkFiles(root, stop)
}

// gitListFiles asks git for the tracked and untracked-but-not-ignored files
// under root, and reports whether git could answer at all. Run from root, git
// prints paths relative to it and limited to its subtree, which is exactly the
// list the finder wants.
//
// It does not watch the finder's stop channel: git.Run has its own timeout, so
// a scan the user has already abandoned lingers until that fires. The finder's
// generation check discards the late result — the cost is latency, not a stale
// list.
func gitListFiles(root string) ([]string, bool) {
	out, err := git.Run(root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, false
	}
	entries := bytes.Split(out, []byte{0})
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if len(e) == 0 {
			continue
		}
		rel := string(e)
		// The index outlives the worktree: a tracked file deleted on disk is
		// still listed, and a submodule is listed as a gitlink directory.
		// Neither is a file the finder can open, so only regular files survive.
		// Lstat, not Stat: a symlink is followed nowhere.
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, rel)
	}
	return ui.ToFinderPaths(files), true
}
