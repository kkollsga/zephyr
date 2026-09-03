package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/fuzzy"
)

// finderMaxResults caps the ranked list handed to the overlay.
const finderMaxResults = 100

// FuzzyFinder manages the file finder overlay state.
//
// Every exported field belongs to the UI goroutine. The directory scan runs off
// it — a walk of a large tree takes seconds, and doing it inside the key
// handler froze the window — and hands its result back through the
// mutex-protected pending slot that Sync drains. The scan goroutine touches
// nothing else.
type FuzzyFinder struct {
	Visible  bool
	Query    string
	Results  []fuzzy.Match
	Selected int
	Files    []string
	RootDir  string

	// Scan lists the files under root, relative to it and in slash form. It
	// runs on its own goroutine and must return promptly once stop is closed.
	// Replaceable so a test can drive the handoff without a real tree.
	Scan func(root string, stop <-chan struct{}) []string

	// OnResults is called from the scan goroutine once a result is waiting, so
	// the app can repaint; the next frame's Sync folds the result in. It must
	// be safe to call from another goroutine.
	OnResults func()

	changedOnly bool
	scanning    bool
	// gen rises on every Open and Close. A scan carries the generation it
	// started under, so the result of a superseded or closed scan is dropped
	// instead of replacing a newer list.
	gen  int
	stop chan struct{}

	mu      sync.Mutex
	pending *scanResult
	landed  chan struct{}
}

type scanResult struct {
	gen   int
	root  string
	files []string
}

// NewFuzzyFinder creates a new fuzzy file finder with the plain directory
// walk as its scanner. A host that can do better — cmd/zephyr replaces it with
// a git-aware scanner — overwrites Scan after construction.
func NewFuzzyFinder() *FuzzyFinder {
	return &FuzzyFinder{Scan: WalkFiles}
}

// Open shows the fuzzy finder over rootDir and starts a fresh scan of it. It
// returns at once: the previous list for the same root stays on screen until
// the new one lands, so a delete or a rename is picked up without the overlay
// ever waiting on the walk.
func (ff *FuzzyFinder) Open(rootDir string) {
	ff.Visible = true
	ff.Query = ""
	ff.Selected = 0
	if ff.RootDir != rootDir || ff.changedOnly {
		// Another root's files, or the changed-file list, describe something
		// else; showing them under this root would be a lie, not a stale cache.
		ff.Files = nil
	}
	ff.RootDir = rootDir
	ff.changedOnly = false
	ff.rank()
	ff.startScan(rootDir)
}

// OpenChanged shows the fuzzy finder with only changed files.
func (ff *FuzzyFinder) OpenChanged(rootDir string, changedFiles []string) {
	ff.cancelScan()
	ff.Visible = true
	ff.Query = ""
	ff.Selected = 0
	ff.RootDir = rootDir
	// git prints its status paths with forward slashes already; normalising
	// keeps the invariant one line rather than one per producer, since
	// SelectedPath converts back on the way out.
	ff.Files = ToFinderPaths(changedFiles)
	ff.changedOnly = true
	ff.rank()
}

// Close hides the fuzzy finder and drops whatever the in-flight scan produces.
func (ff *FuzzyFinder) Close() {
	ff.cancelScan()
	ff.Visible = false
	ff.Query = ""
	ff.Results = nil
}

// Scanning reports whether a scan is still running with nothing to show yet.
func (ff *FuzzyFinder) Scanning() bool {
	return ff.scanning
}

// startScan launches the walk of root and supersedes any scan already running.
func (ff *FuzzyFinder) startScan(root string) {
	ff.cancelScan()
	gen := ff.gen
	stop := make(chan struct{})
	ff.stop = stop
	ff.scanning = true
	landed := make(chan struct{})
	ff.mu.Lock()
	ff.landed = landed
	ff.mu.Unlock()

	scan, notify := ff.Scan, ff.OnResults
	if scan == nil {
		scan = WalkFiles
	}
	go func() {
		files := scan(root, stop)
		ff.mu.Lock()
		ff.pending = &scanResult{gen: gen, root: root, files: files}
		ff.mu.Unlock()
		close(landed)
		if notify != nil {
			notify()
		}
	}()
}

// cancelScan asks the running scan to stop and moves past its generation, so
// a result already on its way is discarded rather than applied late.
func (ff *FuzzyFinder) cancelScan() {
	if ff.stop != nil {
		close(ff.stop)
		ff.stop = nil
	}
	ff.gen++
	ff.scanning = false
	ff.mu.Lock()
	ff.pending = nil
	ff.mu.Unlock()
}

// Sync folds a completed scan into the visible list and reports whether it
// changed anything. Only the UI goroutine calls it.
func (ff *FuzzyFinder) Sync() bool {
	ff.mu.Lock()
	p := ff.pending
	ff.pending = nil
	ff.mu.Unlock()
	if p == nil || p.gen != ff.gen || p.root != ff.RootDir {
		return false
	}
	ff.scanning = false
	ff.Files = p.files
	ff.rank()
	return true
}

// WaitForScan blocks until the in-flight scan has been folded in, and reports
// whether one was. The app never calls it — it repaints from OnResults — but a
// caller that needs the list synchronously has no other join point.
func (ff *FuzzyFinder) WaitForScan(d time.Duration) bool {
	ff.mu.Lock()
	landed := ff.landed
	ff.mu.Unlock()
	if landed == nil {
		return false
	}
	select {
	case <-landed:
	case <-time.After(d):
		return false
	}
	return ff.Sync()
}

// UpdateQuery filters files based on the query.
func (ff *FuzzyFinder) UpdateQuery(query string) {
	ff.Sync()
	ff.Query = query
	ff.Selected = 0
	ff.rank()
}

// BackspaceQuery drops the last character of the query. It is a rune, not a
// byte: cutting one byte off "é" leaves an invalid fragment that matches
// nothing and draws as a replacement glyph.
func (ff *FuzzyFinder) BackspaceQuery() {
	if ff.Query == "" {
		return
	}
	_, size := utf8.DecodeLastRuneInString(ff.Query)
	ff.UpdateQuery(ff.Query[:len(ff.Query)-size])
}

// rank re-ranks Files against the current query, keeping the selection inside
// the result list.
func (ff *FuzzyFinder) rank() {
	ff.Results = fuzzy.RankMatches(ff.Query, ff.Files)
	if len(ff.Results) > finderMaxResults {
		ff.Results = ff.Results[:finderMaxResults]
	}
	if ff.Selected >= len(ff.Results) {
		ff.Selected = len(ff.Results) - 1
	}
	if ff.Selected < 0 {
		ff.Selected = 0
	}
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

// ToFinderPaths normalises a batch of relative paths built with the host
// separator into the slash form the finder displays and matches.
func ToFinderPaths(paths []string) []string {
	if filepath.Separator == '/' {
		return paths
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = toFinderPath(p, filepath.Separator)
	}
	return out
}

// errScanStopped ends the walk early; filepath.Walk has no other way out.
var errScanStopped = errors.New("scan stopped")

// WalkFiles walks root off the UI goroutine, listing every file the finder
// offers, relative to root and in slash form. It is the default Scan and the
// fallback for a scanner that cannot list a particular root.
func WalkFiles(root string, stop <-chan struct{}) []string {
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-stop:
			return errScanStopped
		default:
		}
		name := info.Name()
		if info.IsDir() {
			// The skip rules describe what lies inside the tree, not the tree
			// itself: a project that lives in ~/.dotfiles or in a directory
			// called vendor still lists its own files.
			if path == root {
				return nil
			}
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, toFinderPath(rel, filepath.Separator))
		return nil
	})
	return files
}
