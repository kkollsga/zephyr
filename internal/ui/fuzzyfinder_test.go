package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// openAndWait opens the finder and blocks until the scan it started has been
// folded in, which the app does not do — it repaints when the scan lands.
func openAndWait(t *testing.T, ff *FuzzyFinder, dir string) {
	t.Helper()
	ff.Open(dir)
	if !ff.WaitForScan(10 * time.Second) {
		t.Fatal("the scan did not land")
	}
}

func setupFinderDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "editor.go"), []byte("package src"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "buffer.go"), []byte("package src"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0644)
	return dir
}

func TestFuzzyFinder_Open(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	openAndWait(t, ff, dir)
	if !ff.Visible {
		t.Fatal("expected visible")
	}
	if len(ff.Files) == 0 {
		t.Fatal("expected files to be scanned")
	}
}

func TestFuzzyFinder_Filter(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	openAndWait(t, ff, dir)
	ff.UpdateQuery("editor")
	if len(ff.Results) == 0 {
		t.Fatal("expected matches for 'editor'")
	}
	if ff.Results[0].Text != "src/editor.go" {
		t.Fatalf("top result = %q, want src/editor.go", ff.Results[0].Text)
	}
}

// The scan lists slash paths on every platform. filepath.Join builds the
// nested path with the host separator, so on Windows the walk hands the finder
// "src\editor.go"; listing that verbatim breaks a query typed with "/",
// because the matcher is a plain subsequence test over the displayed string.
func TestFuzzyFinder_ListsSlashPathsAndOpensHostPaths(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	openAndWait(t, ff, dir)
	nested := filepath.Join("src", "editor.go")
	if slices.Contains(ff.Files, nested) && nested != "src/editor.go" {
		t.Fatalf("scan listed the host-separator path %q", nested)
	}
	if !slices.Contains(ff.Files, "src/editor.go") {
		t.Fatalf("scanned files = %v, want src/editor.go listed", ff.Files)
	}

	ff.UpdateQuery("src/ed")
	if len(ff.Results) == 0 || ff.Results[0].Text != "src/editor.go" {
		t.Fatalf("query \"src/ed\" gave %+v, want src/editor.go on top", ff.Results)
	}
	if got, want := ff.SelectedPath(), filepath.Join(dir, "src", "editor.go"); got != want {
		t.Fatalf("SelectedPath = %q, want the host-separator path %q", got, want)
	}
}

// The changed-file list reaches the same matcher, so it is normalised the same
// way and still opens to a host path.
func TestFuzzyFinder_ChangedFilesUseSlashPaths(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	ff.OpenChanged(dir, []string{filepath.Join("src", "editor.go")})
	if len(ff.Results) != 1 || ff.Results[0].Text != "src/editor.go" {
		t.Fatalf("changed-file results = %+v, want src/editor.go", ff.Results)
	}
	if got, want := ff.SelectedPath(), filepath.Join(dir, "src", "editor.go"); got != want {
		t.Fatalf("SelectedPath = %q, want %q", got, want)
	}
}

func TestToFinderPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		sep  rune
		want string
	}{
		{"windows separator becomes a slash", `src\editor.go`, '\\', "src/editor.go"},
		{"slash paths are untouched", "src/editor.go", '/', "src/editor.go"},
		{"a backslash is a filename byte on posix", `we\ird.go`, '/', `we\ird.go`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := toFinderPath(tc.in, tc.sep); got != tc.want {
				t.Fatalf("toFinderPath(%q, %q) = %q, want %q", tc.in, tc.sep, got, tc.want)
			}
		})
	}
}

func TestFuzzyFinder_Navigation(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	openAndWait(t, ff, dir)
	ff.MoveDown()
	if ff.Selected != 1 {
		t.Fatalf("expected selected=1, got %d", ff.Selected)
	}
}

func TestFuzzyFinder_SelectedPath(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	openAndWait(t, ff, dir)
	path := ff.SelectedPath()
	if path == "" {
		t.Fatal("expected selected path")
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got %s", path)
	}
}

func TestFuzzyFinder_Close(t *testing.T) {
	ff := NewFuzzyFinder()
	openAndWait(t, ff, t.TempDir())
	ff.Close()
	if ff.Visible {
		t.Fatal("expected not visible")
	}
}

// TestFuzzyFinder_ChangedThenAllFilesRescans pins the cache interaction: a
// changed-files open replaces Files with the changed list, and the next
// all-files open on the same root must not reuse it.
func TestFuzzyFinder_ChangedThenAllFilesRescans(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	ff.OpenChanged(dir, []string{"main.go"})
	if len(ff.Results) != 1 {
		t.Fatalf("changed-files open listed %d results, want 1", len(ff.Results))
	}
	ff.Close()
	openAndWait(t, ff, dir)
	if len(ff.Files) < 4 {
		t.Fatalf("all-files open listed %d files, want the whole scan", len(ff.Files))
	}
}

// A scan of a large tree takes seconds. Open used to run it inline, so the key
// that opened the finder froze the window until the walk finished.
func TestFuzzyFinder_OpenDoesNotWaitForTheScan(t *testing.T) {
	release := make(chan struct{})
	ff := NewFuzzyFinder()
	ff.Scan = func(root string, stop <-chan struct{}) []string {
		<-release
		return []string{"slow.go"}
	}

	start := time.Now()
	ff.Open("/root")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("Open blocked for %v", elapsed)
	}
	if !ff.Visible {
		t.Fatal("the overlay did not open")
	}
	if len(ff.Files) != 0 {
		t.Fatalf("Files = %v before the scan finished", ff.Files)
	}
	if !ff.Scanning() {
		t.Fatal("the overlay did not report that it is scanning")
	}

	close(release)
	if !ff.WaitForScan(10 * time.Second) {
		t.Fatal("the scan never landed")
	}
	if len(ff.Results) != 1 || ff.Results[0].Text != "slow.go" {
		t.Fatalf("results after the scan = %+v, want slow.go", ff.Results)
	}
	if ff.Scanning() {
		t.Fatal("still scanning after the result landed")
	}
}

// Closing the overlay before the scan finishes must leave nothing behind: the
// list belongs to a finder the user has dismissed.
func TestFuzzyFinder_CloseDiscardsAnInFlightScan(t *testing.T) {
	release := make(chan struct{})
	done := make(chan struct{})
	ff := NewFuzzyFinder()
	ff.Scan = func(root string, stop <-chan struct{}) []string {
		<-release
		return []string{"late.go"}
	}
	ff.OnResults = func() { close(done) }

	ff.Open("/root")
	ff.Close()
	close(release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the scan goroutine never finished")
	}

	if ff.Sync() {
		t.Fatal("a scan from a closed finder was applied")
	}
	if len(ff.Files) != 0 || len(ff.Results) != 0 {
		t.Fatalf("closed finder holds files=%v results=%+v", ff.Files, ff.Results)
	}
}

// A second Open supersedes the first: whichever scan finishes last, the list
// describes the root the finder is actually showing.
func TestFuzzyFinder_ASupersededScanIsDropped(t *testing.T) {
	releaseFirst := make(chan struct{})
	var mu sync.Mutex
	landed := 0
	ff := NewFuzzyFinder()
	ff.Scan = func(root string, stop <-chan struct{}) []string {
		if root == "/first" {
			<-releaseFirst
			return []string{"first.go"}
		}
		return []string{"second.go"}
	}
	ff.OnResults = func() {
		mu.Lock()
		landed++
		mu.Unlock()
	}

	ff.Open("/first")
	ff.Open("/second")
	if !ff.WaitForScan(10 * time.Second) {
		t.Fatal("the second scan never landed")
	}
	close(releaseFirst)
	for i := 0; i < 100; i++ {
		mu.Lock()
		n := landed
		mu.Unlock()
		if n == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	ff.Sync()
	if len(ff.Files) != 1 || ff.Files[0] != "second.go" {
		t.Fatalf("files = %v, want only the second root's scan", ff.Files)
	}
}

// The list is rebuilt on every open, so a file deleted since the last open is
// gone from it — and the previous list stays on screen until the new one lands.
func TestFuzzyFinder_OpenRescansTheRoot(t *testing.T) {
	dir := setupFinderDir(t)
	ff := NewFuzzyFinder()
	openAndWait(t, ff, dir)
	before := len(ff.Files)

	if err := os.Remove(filepath.Join(dir, "src", "editor.go")); err != nil {
		t.Fatal(err)
	}
	ff.Close()
	ff.Open(dir)
	if len(ff.Files) != before {
		t.Fatalf("the reopened finder shows %d files before the rescan lands, want the previous %d", len(ff.Files), before)
	}
	if !ff.WaitForScan(10 * time.Second) {
		t.Fatal("the rescan did not land")
	}
	if slices.Contains(ff.Files, "src/editor.go") {
		t.Fatalf("the deleted file is still listed: %v", ff.Files)
	}
}

// Backspace cuts a rune. One byte off "é" leaves an invalid fragment that
// matches nothing.
func TestFuzzyFinder_BackspaceRemovesARune(t *testing.T) {
	ff := NewFuzzyFinder()
	ff.Files = []string{"café.go"}
	ff.UpdateQuery("café")
	ff.BackspaceQuery()
	if ff.Query != "caf" {
		t.Fatalf("query after backspace = %q, want %q", ff.Query, "caf")
	}
	ff.UpdateQuery("é")
	ff.BackspaceQuery()
	if ff.Query != "" {
		t.Fatalf("query after backspacing a lone rune = %q, want empty", ff.Query)
	}
}

// The skip rules apply inside the tree, not to the tree itself: a project
// checked out in ~/.dotfiles or in a directory called vendor listed nothing.
func TestFuzzyFinder_ScansARootTheSkipRulesWouldReject(t *testing.T) {
	for _, name := range []string{".dotfiles", "vendor", "node_modules"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), name)
			if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
				t.Fatal(err)
			}
			for _, rel := range []string{"main.go", "src/editor.go"} {
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			ff := NewFuzzyFinder()
			openAndWait(t, ff, root)
			if !slices.Contains(ff.Files, "main.go") || !slices.Contains(ff.Files, "src/editor.go") {
				t.Fatalf("a root named %q listed %v, want both files", name, ff.Files)
			}
		})
	}
}

// The skip rules still apply below the root.
func TestFuzzyFinder_SkipsExcludedDirectoriesBelowTheRoot(t *testing.T) {
	dir := setupFinderDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "vendor", "dep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vendor", "dep", "lib.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ff := NewFuzzyFinder()
	openAndWait(t, ff, dir)
	for _, f := range ff.Files {
		if strings.HasPrefix(f, "vendor/") {
			t.Fatalf("vendor below the root was listed: %v", ff.Files)
		}
	}
}
