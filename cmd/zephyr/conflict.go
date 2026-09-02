package main

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/fileio"
	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/ui"
)

// conflictState records an unresolved disagreement between a tab's buffer and
// the file on disk.
type conflictState int

const (
	conflictNone    conflictState = iota
	conflictDeleted               // the file was removed or moved away
)

// pendingWatchEvents buffers watcher events between the watcher goroutine and
// the frame loop that consumes them. Entries are coalesced per path, so the
// list is bounded by the number of watched files however fast a writer churns.
type pendingWatchEvents struct {
	mu   sync.Mutex
	list []fileio.FileEvent
}

func (p *pendingWatchEvents) push(evt fileio.FileEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.list {
		if p.list[i].Path == evt.Path {
			p.list[i].Op |= evt.Op
			return
		}
	}
	p.list = append(p.list, evt)
}

func (p *pendingWatchEvents) drain() []fileio.FileEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.list
	p.list = nil
	return out
}

// startWatcherPump makes a goroutine the sole consumer of the watcher's event
// channel, stashing each event for the frame loop and calling notify so an
// idle window wakes up and runs pollFileWatcher. Without this an external
// change is invisible until something else happens to produce a frame.
//
// The goroutine consumes rather than forwards for two reasons: a second
// consumer racing the frame loop on one channel would steal events the frame
// loop needs, and the watcher's 16-slot channel blocks its producer once full,
// which would drop the wakeup too. Draining it immediately into an unbounded
// per-path-coalesced list keeps the watcher running and loses nothing.
func startWatcherPump(events <-chan fileio.FileEvent, pending *pendingWatchEvents, notify func()) {
	go func() {
		for evt := range events {
			pending.push(evt)
			if notify != nil {
				notify()
			}
		}
	}()
}

// pollFileWatcher applies the watcher events collected since the last frame.
func (st *appState) pollFileWatcher() {
	for _, evt := range st.watcherPending.drain() {
		st.handleExternalFileChange(evt.Path)
	}
}

// --- Disk snapshots ---

// snapshotEditorFile records the on-disk state a tab's content now agrees
// with. Called after a load, a reload and every successful save, so a later
// event or save can tell our own write from someone else's.
func (st *appState) snapshotEditorFile(ed *editor.Editor) {
	ts := st.tabStates[ed]
	if ts == nil || ed == nil || ed.FilePath == "" {
		return
	}
	snap, err := fileio.TakeSnapshot(ed.FilePath)
	if err != nil {
		// An unreadable path is not "unchanged": leave the zero snapshot so the
		// next comparison reports a difference rather than a false agreement.
		ts.diskSnap = fileio.Snapshot{}
		return
	}
	ts.diskSnap = snap
}

// diskChangedSinceLoad reports whether the file backing tab differs from the
// state it was last loaded from or saved to. It answers from the file itself,
// so it catches a write the watcher never reported — including one made inside
// the settle window that suppresses our own save's events.
func (st *appState) diskChangedSinceLoad(tab *ui.Tab) bool {
	if tab == nil || tab.Editor == nil || tab.Editor.FilePath == "" {
		return false
	}
	ts := st.tabStates[tab.Editor]
	if ts == nil || !ts.diskSnap.Exists {
		return false
	}
	snap, err := fileio.TakeSnapshot(tab.Editor.FilePath)
	if err != nil {
		return true
	}
	return !snap.Equal(ts.diskSnap)
}

// --- External change handling ---

// handleExternalFileChange processes a file change detected by the watcher.
func (st *appState) handleExternalFileChange(path string) {
	for _, tab := range st.tabBar.Tabs {
		if tab.Editor.FilePath != path {
			continue
		}
		st.applyExternalChange(tab, st.tabStates[tab.Editor], path)
		st.refreshGitDiffForEditor(tab.Editor)
		break
	}
}

// applyExternalChange resolves one watched file's new disk state against the
// tab holding it. The file is inspected rather than the event's Op trusted: a
// Remove is routinely the first half of an atomic replace, and an event can be
// drained after several more have landed, so only the file itself is current.
func (st *appState) applyExternalChange(tab *ui.Tab, ts *tabState, path string) {
	snap, err := fileio.TakeSnapshot(path)
	if err == nil && !snap.Exists {
		st.markFileDeleted(tab, ts)
		return
	}
	if err == nil && ts != nil && ts.diskSnap.Exists && snap.Equal(ts.diskSnap) {
		return // disk holds exactly what we last loaded or wrote
	}
	st.clearDeletedConflict(tab, ts)

	if tab.Editor.Modified {
		st.notify("File changed externally: "+filepath.Base(path), 10*time.Second)
		return
	}
	if err == nil && snap.Equal(fileio.SnapshotOfBytes(tab.Editor.Buffer.TextBytes(nil))) {
		// Buffer and disk already agree — adopt the new disk state without a
		// reload, which would push a no-op entry onto the undo stack.
		if ts != nil {
			ts.diskSnap = snap
		}
		return
	}
	if err := st.reloadEditorFromDisk(tab.Editor); err != nil {
		st.notify("Could not reload "+filepath.Base(path)+": "+err.Error(), 10*time.Second)
		return
	}
	st.notify("Reloaded: "+filepath.Base(path), 5*time.Second)
}

// markFileDeleted keeps the buffer as the last surviving copy of a file that
// disappeared: the text stays, and Modified goes true so no flow treats the
// tab as safe to close.
func (st *appState) markFileDeleted(tab *ui.Tab, ts *tabState) {
	if ts != nil && ts.conflict != conflictDeleted {
		ts.deleteForcedModified = !tab.Editor.Modified
	}
	if ts != nil {
		ts.conflict = conflictDeleted
		ts.diskSnap = fileio.Snapshot{}
	}
	tab.Editor.Modified = true
	st.notify("File deleted on disk: "+filepath.Base(tab.Editor.FilePath), 10*time.Second)
	st.updateWindowTitle()
}

// clearDeletedConflict retires a delete that turned out to be the first half
// of a replace, restoring the buffer's real modified state so a clean tab
// still reloads silently instead of prompting.
func (st *appState) clearDeletedConflict(tab *ui.Tab, ts *tabState) {
	if ts == nil || ts.conflict != conflictDeleted {
		return
	}
	if ts.deleteForcedModified {
		tab.Editor.Modified = false
		ts.deleteForcedModified = false
	}
	ts.conflict = conflictNone
	if st.notification == "File deleted on disk: "+filepath.Base(tab.Editor.FilePath) {
		st.notification = ""
	}
	st.updateWindowTitle()
}

// reloadEditorFromDisk reloads a file from disk, re-parses syntax, and
// refreshes derived per-tab state.
func (st *appState) reloadEditorFromDisk(ed *editor.Editor) error {
	if err := ed.Reload(); err != nil {
		return err
	}
	if ts, ok := st.tabStates[ed]; ok {
		if ts.highlighter != nil {
			ts.sourceBuf = ed.Buffer.TextBytes(ts.sourceBuf)
			ts.highlighter.Parse(ts.sourceBuf)
		}
		source := ed.Buffer.TextBytes(nil)
		regions := render.ComputeFoldRegions(string(source))
		ts.foldState.SetRegions(regions, ed.Buffer.LineCount())
		ts.lastCursorLine = -1
		ts.lastCursorCol = -1
		ts.conflict = conflictNone
		ts.deleteForcedModified = false
	}
	st.snapshotEditorFile(ed)
	// Registers the path if it was never watched (a tab whose file appeared
	// after the tab was opened). Directory watching survives the atomic rename
	// of a save, so an already-watched path needs no reattachment and Watch
	// returns immediately.
	if st.watcher != nil {
		st.watcher.Watch(ed.FilePath)
	}
	return nil
}

// --- Watcher registration ---

// watchEditorFile adds a file to the watcher.
func (st *appState) watchEditorFile(ed *editor.Editor) {
	if st.watcher != nil && ed != nil && ed.FilePath != "" {
		st.watcher.Watch(ed.FilePath)
	}
}

// unwatchEditorFile removes a file from the watcher.
func (st *appState) unwatchEditorFile(ed *editor.Editor) {
	if st.watcher != nil && ed != nil && ed.FilePath != "" {
		st.watcher.Unwatch(ed.FilePath)
	}
}

// notify shows a footer message for the given duration.
func (st *appState) notify(msg string, d time.Duration) {
	st.notification = msg
	st.notificationUntil = time.Now().Add(d)
}
