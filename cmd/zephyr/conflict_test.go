package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

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
	if snap, err := fileio.TakeSnapshot(path); err == nil {
		ts.diskSnap = snap
	}
	st := &appState{
		tabBar: tb, tabStates: map[*editor.Editor]*tabState{ed: ts},
		tabRend:    &render.TextRenderer{CharWidth: 8, LineHeightPx: 18},
		barTabIdxs: []int{0}, lastMaxX: 900, tabBarHeight: 28,
	}
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

// A save must never silently overwrite a change made on disk while the buffer
// was dirty: the external bytes stay until the user chooses what happens.
func TestSaveRefusesToClobberExternalChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contested.go")
	if err := os.WriteFile(path, []byte("original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, ed, _ := conflictTestApp(t, path)
	tab := st.tabBar.Tabs[0]

	ed.Cursor.SetPosition(ed.Buffer, 0, 0)
	ed.InsertText("mine ")
	if err := os.WriteFile(path, []byte("theirs\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st.handleExternalFileChange(path)

	saved := st.saveTab(tab)
	if got, _ := os.ReadFile(path); string(got) != "theirs\n" {
		t.Fatalf("external change was clobbered: disk = %q", got)
	}
	if saved {
		t.Fatal("saveTab reported success while a conflict was unresolved")
	}
	if !ed.Modified {
		t.Fatal("refused save cleared the modified flag")
	}
}

// conflictedTab leaves path holding external bytes while the tab holds unsaved
// edits, with the resulting conflict already recorded.
func conflictedTab(t *testing.T, mine, theirs string) (*appState, *editor.Editor, *tabState, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "contested.go")
	if err := os.WriteFile(path, []byte("original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, ed, ts := conflictTestApp(t, path)
	ed.Cursor.SetPosition(ed.Buffer, 0, 0)
	ed.InsertText(mine)
	if err := os.WriteFile(path, []byte(theirs), 0o600); err != nil {
		t.Fatal(err)
	}
	st.handleExternalFileChange(path)
	if ts.conflict != conflictModified {
		t.Fatalf("conflict state = %v, want conflictModified", ts.conflict)
	}
	return st, ed, ts, path
}

func TestClobberPromptRaisedInsteadOfWriting(t *testing.T) {
	st, _, ts, _ := conflictedTab(t, "mine ", "theirs\n")
	if st.saveTabWithPrompt(st.tabBar.Tabs[0], false, false) {
		t.Fatal("save was not refused")
	}
	if !st.saveMenu.visible || !st.saveMenu.confirmClobber {
		t.Fatalf("clobber prompt not raised: %+v", st.saveMenu)
	}
	if st.saveMenuRowCount() != 2 {
		t.Fatalf("clobber sub-state row count = %d, want 2", st.saveMenuRowCount())
	}
	if ts.conflict != conflictModified {
		t.Fatalf("conflict cleared by the prompt: %v", ts.conflict)
	}
}

func TestClobberOverwriteWritesBufferAndClearsConflict(t *testing.T) {
	st, ed, ts, path := conflictedTab(t, "mine ", "theirs\n")
	tab := st.tabBar.Tabs[0]
	st.saveTab(tab)

	st.clobberOverwrite()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != ed.Buffer.Text() {
		t.Fatalf("disk = %q, buffer = %q", got, ed.Buffer.Text())
	}
	if ts.conflict != conflictNone {
		t.Fatalf("conflict survived Overwrite: %v", ts.conflict)
	}
	if st.diskChangedSinceLoad(tab) {
		t.Fatal("snapshot was not refreshed by Overwrite")
	}
	if ed.Modified || st.saveMenu.visible || st.saveMenu.confirmClobber {
		t.Fatalf("post-Overwrite state modified=%v menu=%+v", ed.Modified, st.saveMenu)
	}
}

func TestClobberReloadTakesDiskAndOneUndoRestoresMine(t *testing.T) {
	st, ed, ts, path := conflictedTab(t, "mine ", "theirs\n")
	mine := ed.Buffer.Text()
	st.saveTab(st.tabBar.Tabs[0])

	st.clobberReload()

	if ed.Buffer.Text() != "theirs\n" {
		t.Fatalf("buffer after Reload = %q", ed.Buffer.Text())
	}
	if got, _ := os.ReadFile(path); string(got) != "theirs\n" {
		t.Fatalf("Reload wrote to disk: %q", got)
	}
	if ed.Modified {
		t.Fatal("buffer still modified after Reload")
	}
	if ts.conflict != conflictNone {
		t.Fatalf("conflict survived Reload: %v", ts.conflict)
	}
	if !ed.Undo() || ed.Buffer.Text() != mine {
		t.Fatalf("one undo did not restore the user's text: %q", ed.Buffer.Text())
	}
}

func TestClobberCancelLeavesDiskAndBufferUntouched(t *testing.T) {
	st, ed, ts, path := conflictedTab(t, "mine ", "theirs\n")
	mine := ed.Buffer.Text()
	st.saveTab(st.tabBar.Tabs[0])

	st.dismissClobberPrompt()

	if got, _ := os.ReadFile(path); string(got) != "theirs\n" {
		t.Fatalf("Cancel changed the file: %q", got)
	}
	if ed.Buffer.Text() != mine || !ed.Modified {
		t.Fatalf("Cancel changed the buffer: %q modified=%v", ed.Buffer.Text(), ed.Modified)
	}
	if ts.conflict != conflictModified {
		t.Fatalf("Cancel resolved the conflict: %v", ts.conflict)
	}
	if st.saveMenu.visible || st.saveMenu.confirmClobber {
		t.Fatalf("Cancel left the prompt open: %+v", st.saveMenu)
	}
}

// The prompt's three choices must be reachable by clicking the row the layout
// actually draws.
func TestClobberClickHitsOverwriteReloadAndCancel(t *testing.T) {
	for _, tc := range []struct {
		name   string
		third  int
		wantOn string
	}{
		{"overwrite", 0, "mine original\n"},
		{"reload", 1, "theirs\n"},
		{"cancel", 2, "theirs\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, _, _, path := conflictedTab(t, "mine ", "theirs\n")
			st.saveTab(st.tabBar.Tabs[0])
			dx, dy, dw, _, itemH := st.saveMenuRect()
			x := dx + dw/3*tc.third + dw/6
			st.handleSaveMenuClick(x, dy+itemH+itemH/2)
			got, _ := os.ReadFile(path)
			if string(got) != tc.wantOn {
				t.Fatalf("disk after %s = %q, want %q", tc.name, got, tc.wantOn)
			}
			if st.saveMenu.confirmClobber {
				t.Fatalf("%s left the prompt open", tc.name)
			}
		})
	}
}

// Escape and Return both cancel: neither may be the key that loses a version.
func TestClobberKeysCancelOnly(t *testing.T) {
	for _, name := range []key.Name{key.NameEscape, key.NameReturn} {
		st, ed, ts, path := conflictedTab(t, "mine ", "theirs\n")
		st.saveTab(st.tabBar.Tabs[0])
		st.handleKey(key.Event{Name: name})
		if got, _ := os.ReadFile(path); string(got) != "theirs\n" {
			t.Fatalf("key %v wrote to disk: %q", name, got)
		}
		if !ed.Modified || ts.conflict != conflictModified {
			t.Fatalf("key %v resolved the conflict", name)
		}
		if st.saveMenu.visible {
			t.Fatalf("key %v left the prompt open", name)
		}
	}
}

// Pre-mortem 3: a save refused inside the quit flow must leave the tab open
// and must not let the quit complete.
func TestRefusedSaveDuringQuitLeavesTabOpen(t *testing.T) {
	st, ed, ts, path := conflictedTab(t, "mine ", "theirs\n")
	st.startQuitFlow()
	if !st.saveMenu.visible || !st.saveMenu.forQuit || !st.saveMenu.closeAfterSave {
		t.Fatalf("quit prompt = %+v", st.saveMenu)
	}

	// Click Save on the quit prompt.
	dx, dy, dw, dropdownH, itemH := st.saveMenuRect()
	st.handleSaveMenuClick(dx+dw/6, dy+dropdownH-itemH/2)

	if !st.saveMenu.confirmClobber {
		t.Fatalf("save during quit did not raise the clobber prompt: %+v", st.saveMenu)
	}
	if !st.saveMenu.forQuit || !st.saveMenu.closeAfterSave {
		t.Fatal("clobber prompt dropped the quit flow flags")
	}
	if got, _ := os.ReadFile(path); string(got) != "theirs\n" {
		t.Fatalf("quit-flow save clobbered the file: %q", got)
	}
	if st.exitPending {
		t.Fatal("quit completed over an unresolved conflict")
	}

	st.dismissClobberPrompt()
	if len(st.tabBar.Tabs) != 1 || st.tabBar.Tabs[0].Editor != ed {
		t.Fatal("cancelling the clobber prompt closed the tab")
	}
	if st.exitPending || st.quitInProgress || ts.conflict != conflictModified {
		t.Fatalf("cancel state exit=%v quit=%v conflict=%v", st.exitPending, st.quitInProgress, ts.conflict)
	}
}

// A conflict must show up in the tab bar as well as the status bar.
func TestConflictBadgeSurfaces(t *testing.T) {
	st, _, ts, _ := conflictedTab(t, "mine ", "theirs\n")
	tab := st.tabBar.Tabs[0]
	if st.tabDotColor(tab, false) != warningColor() {
		t.Fatal("tab dot did not carry the conflict badge")
	}
	if ts.conflict.label() != "changed on disk" {
		t.Fatalf("status badge label = %q", ts.conflict.label())
	}
	ts.conflict = conflictDeleted
	if ts.conflict.label() != "deleted on disk" || st.clobberWarning(tab) != "Deleted on disk" {
		t.Fatalf("deleted labels = %q / %q", ts.conflict.label(), st.clobberWarning(tab))
	}
	ts.conflict = conflictNone
	if st.tabDotColor(tab, false) == warningColor() {
		t.Fatal("badge survived conflict resolution")
	}
}

// Every open tab's file is registered with the watcher, not only the one the
// window started with: a second tab used to get no conflict detection at all.
func TestEveryOpenTabsFileIsWatched(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(dir, "first.go")
	second := filepath.Join(dir, "second.go")
	for _, p := range []string{first, second} {
		if err := os.WriteFile(p, []byte("package p\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	st, _, _ := testAppWithText("", "Plain Text")
	st.tabBar = ui.NewTabBar()
	st.tabStates = map[*editor.Editor]*tabState{}
	fw, err := fileio.NewWatcher()
	if err != nil {
		t.Skipf("watcher unavailable: %v", err)
	}
	defer fw.Close()
	st.watcher = fw
	startWatcherPump(fw.Events, &st.watcherPending, nil)

	for _, p := range []string{first, second} {
		if _, err := st.openFileInTab(p); err != nil {
			t.Fatal(err)
		}
	}

	secondTab := st.tabBar.Tabs[1]
	// Unsaved edits in the tab: an external change to a clean buffer reloads
	// it, and the standing conflict is what a save has to be refused against.
	secondTab.Editor.Modified = true

	if err := os.WriteFile(second, []byte("package p // changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st.pollFileWatcher()
		if ts := st.tabStates[secondTab.Editor]; ts != nil && ts.conflict != conflictNone {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("an external change to the second tab's file was never reported")
}

// The tab-transfer offer copies the buffer to a temp file in plaintext. When
// nobody claims the offer, that copy has to go with it.
func TestUnclaimedTabTransferRemovesItsContentFile(t *testing.T) {
	t.Setenv("ZEPHYR_GUI_STATE_DIR", t.TempDir())
	st, ed, _ := testAppWithText("secret working text\n", "Plain Text")
	_ = ed

	before, err := filepath.Glob(filepath.Join(os.TempDir(), "zephyr-transfer-*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// One tab, so the no-claim path stops before spawning a second instance.
	st.offerTabTransferWithin(0, 10*time.Millisecond)

	after, err := filepath.Glob(filepath.Join(os.TempDir(), "zephyr-transfer-*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("the unclaimed offer left %d transfer file(s) behind: %v", len(after)-len(before), after)
	}
}
