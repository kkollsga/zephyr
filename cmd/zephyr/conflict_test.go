package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/fileio"
	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/ui"
)

// conflictTestApp opens path in a one-tab appState wired the way run() does.
func conflictTestApp(t *testing.T, path string) (*appState, *editor.Editor, *tabState) {
	t.Helper()
	ed, err := editor.NewEditorFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tb := ui.NewTabBar()
	tb.OpenEditor(ed, filepath.Base(path))
	ts := &tabState{viewport: render.NewViewport(), foldState: render.NewFoldState(), lastCursorLine: -1}
	st := &appState{tabBar: tb, tabStates: map[*editor.Editor]*tabState{ed: ts}}
	return st, ed, ts
}

// A file deleted underneath a tab must never be reported as reloaded, and the
// user's text must survive as unsaved work.
func TestExternalDeleteKeepsBufferAndNeverClaimsReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gone.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, ed, _ := conflictTestApp(t, path)
	before := ed.Buffer.Text()

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	st.handleExternalFileChange(path)

	if strings.HasPrefix(st.notification, "Reloaded:") {
		t.Fatalf("delete reported as a reload: %q", st.notification)
	}
	if st.notification == "" {
		t.Fatal("delete produced no notification")
	}
	if got := ed.Buffer.Text(); got != before {
		t.Fatalf("buffer lost text on delete: %q", got)
	}
	if !ed.Modified {
		t.Fatal("buffer holding the only copy of the file was not marked modified")
	}
}

// A reload that cannot read the file must not claim a reload happened.
func TestFailedReloadDoesNotClaimReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "swapped.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, ed, _ := conflictTestApp(t, path)
	before := ed.Buffer.Text()

	// Replace the file with a directory: the path still exists, but reading it fails.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	st.handleExternalFileChange(path)

	if strings.HasPrefix(st.notification, "Reloaded:") {
		t.Fatalf("failed reload reported as success: %q", st.notification)
	}
	if got := ed.Buffer.Text(); got != before {
		t.Fatalf("buffer changed after a failed reload: %q", got)
	}
}

// Our own save must not look like an external change afterwards: a late
// directory-watch event for a save we performed must raise no warning.
func TestOwnSaveRefreshesSnapshotSoLateEventIsSilent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "own.go")
	if err := os.WriteFile(path, []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, ed, _ := conflictTestApp(t, path)
	tab := st.tabBar.Tabs[0]

	ed.Cursor.SetPosition(ed.Buffer, 0, 0)
	ed.InsertText("x")
	if !st.saveTab(tab) {
		t.Fatal("saveTab failed")
	}
	saved := ed.Buffer.Text()

	ed.Cursor.SetPosition(ed.Buffer, 0, 0)
	ed.InsertText("y")
	if !ed.Modified {
		t.Fatal("buffer not modified after edit")
	}

	// A late event for our own write: disk still holds exactly what we saved.
	st.handleExternalFileChange(path)
	if st.notification != "" {
		t.Fatalf("own save raised a false external-change warning: %q", st.notification)
	}

	if err := os.WriteFile(path, []byte("external\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st.handleExternalFileChange(path)
	if !strings.HasPrefix(st.notification, "File changed externally:") {
		t.Fatalf("real external change not reported: %q", st.notification)
	}
	if got, _ := os.ReadFile(path); string(got) == saved {
		t.Fatal("test setup: external write did not land")
	}
}

// diskChangedSinceLoad must answer from the file, not from watcher events, so
// a write that landed inside our own-save suppression window is still caught.
func TestDiskChangedSinceLoadSeesUnreportedWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "silent.go")
	if err := os.WriteFile(path, []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, ed, ts := conflictTestApp(t, path)
	tab := st.tabBar.Tabs[0]
	if snap, err := fileio.TakeSnapshot(path); err == nil {
		ts.diskSnap = snap
	}
	if st.diskChangedSinceLoad(tab) {
		t.Fatal("freshly loaded file reported as changed")
	}

	ed.Cursor.SetPosition(ed.Buffer, 0, 0)
	ed.InsertText("x")
	if !st.saveTab(tab) {
		t.Fatal("saveTab failed")
	}
	if st.diskChangedSinceLoad(tab) {
		t.Fatal("our own save reported as an external change")
	}

	// No watcher event at all: the write is only visible by comparison.
	if err := os.WriteFile(path, []byte("someone else\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !st.diskChangedSinceLoad(tab) {
		t.Fatal("unreported external write was not detected")
	}
}

// An atomic replace arrives as Remove then Create. The delete must not leave a
// clean buffer permanently marked modified once the file comes back.
func TestDeleteThenRecreateReloadsCleanBuffer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "replaced.go")
	if err := os.WriteFile(path, []byte("package old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, ed, ts := conflictTestApp(t, path)
	if snap, err := fileio.TakeSnapshot(path); err == nil {
		ts.diskSnap = snap
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	st.handleExternalFileChange(path)
	if ts.conflict != conflictDeleted || !ed.Modified {
		t.Fatalf("delete state conflict=%v modified=%v", ts.conflict, ed.Modified)
	}

	if err := os.WriteFile(path, []byte("package new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st.handleExternalFileChange(path)
	if ts.conflict != conflictNone {
		t.Fatalf("recreated file left conflict=%v", ts.conflict)
	}
	if ed.Buffer.Text() != "package new\n" || ed.Modified {
		t.Fatalf("clean buffer did not reload: text=%q modified=%v", ed.Buffer.Text(), ed.Modified)
	}
}

// The watcher goroutine must wake an idle window and hand every event to the
// frame loop's drain.
func TestWatcherPumpWakesWindowAndQueuesEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "woken.go")
	if err := os.WriteFile(path, []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, ed, _ := conflictTestApp(t, path)

	events := make(chan fileio.FileEvent, 4)
	woken := make(chan struct{}, 4)
	startWatcherPump(events, &st.watcherPending, func() { woken <- struct{}{} })

	events <- fileio.FileEvent{Path: path, Op: fsnotify.Write}
	select {
	case <-woken:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher event did not wake the window")
	}

	if err := os.WriteFile(path, []byte("second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st.pollFileWatcher()
	if ed.Buffer.Text() != "second\n" {
		t.Fatalf("queued event was not applied by the frame drain: %q", ed.Buffer.Text())
	}
	if len(st.watcherPending.drain()) != 0 {
		t.Fatal("drained events were left pending")
	}
	close(events)
}

// Repeated events for one path coalesce, so a busy writer cannot grow the
// pending list without bound.
func TestPendingWatchEventsCoalescePerPath(t *testing.T) {
	var p pendingWatchEvents
	for i := 0; i < 100; i++ {
		p.push(fileio.FileEvent{Path: "/a", Op: fsnotify.Write})
		p.push(fileio.FileEvent{Path: "/b", Op: fsnotify.Remove})
	}
	got := p.drain()
	if len(got) != 2 {
		t.Fatalf("pending list length = %d, want 2", len(got))
	}
	if got[0].Op&fsnotify.Write == 0 || got[1].Op&fsnotify.Remove == 0 {
		t.Fatalf("coalesced ops lost information: %+v", got)
	}
}
