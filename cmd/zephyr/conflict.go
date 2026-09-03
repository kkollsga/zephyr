package main

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"gioui.org/io/key"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/fileio"
	"github.com/kristianweb/zephyr/internal/git"
	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/ui"
	"github.com/kristianweb/zephyr/internal/vim"
)

// conflictState records an unresolved disagreement between a tab's buffer and
// the file on disk.
type conflictState int

const (
	conflictNone     conflictState = iota
	conflictModified               // the file was rewritten while the buffer had unsaved edits
	conflictDeleted                // the file was removed or moved away
)

// label returns the status-bar wording for an unresolved conflict.
func (c conflictState) label() string {
	switch c {
	case conflictModified:
		return "changed on disk"
	case conflictDeleted:
		return "deleted on disk"
	}
	return ""
}

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
		// Disk holds exactly what we last loaded or wrote. If a conflict was
		// standing, the change has been undone on disk and it is resolved.
		st.clearDeletedConflict(tab, ts)
		ts.conflict = conflictNone
		return
	}
	st.clearDeletedConflict(tab, ts)

	if tab.Editor.Modified || (ts != nil && ts.bufType == bufOriginal) {
		// The buffer holds edits the file no longer contains. Record it as a
		// standing conflict rather than only announcing it: the badge and the
		// save-time guard both outlive the notification. A HEAD view is in the
		// same position for a different reason — reloading would overwrite the
		// displayed HEAD content and strand the stashed working buffer.
		if ts != nil {
			ts.conflict = conflictModified
		}
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

// --- Save-time clobber guard ---

// tabIndexOf returns tab's index in the tab bar, or -1.
func (st *appState) tabIndexOf(tab *ui.Tab) int {
	for i, t := range st.tabBar.Tabs {
		if t == tab {
			return i
		}
	}
	return -1
}

// saveWouldClobber reports whether writing tab would destroy a change made on
// disk since the buffer was loaded or last saved. It consults the file as well
// as the recorded conflict, so a change the watcher never reported — one made
// inside the suppression window around our own save — still stops the write.
func (st *appState) saveWouldClobber(tab *ui.Tab) bool {
	if tab == nil || tab.Editor == nil || tab.Editor.FilePath == "" {
		return false
	}
	ts := st.tabStates[tab.Editor]
	if ts != nil && ts.conflict != conflictNone {
		return true
	}
	if !st.diskChangedSinceLoad(tab) {
		return false
	}
	if ts != nil {
		ts.conflict = conflictModified
	}
	return true
}

// raiseClobberPrompt opens the save menu on the clobber sub-state, carrying
// through the close-tab and quit flags of the flow that asked for the save so
// a refusal inside a quit leaves the tab open and the quit incomplete.
func (st *appState) raiseClobberPrompt(idx int, closeAfter, forQuit bool) {
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return
	}
	st.saveMenu.visible = true
	st.saveMenu.tabIdx = idx
	st.saveMenu.closeAfterSave = closeAfter
	st.saveMenu.forQuit = forQuit
	st.saveMenu.saveAsExpanded = false
	st.saveMenu.confirmOverwrite = false
	st.saveMenu.confirmClobber = true
	// Every path back to the prompt — Compare's own exit, a fresh Cmd+S while
	// comparing — ends the overlay here, so no caller can leave one standing
	// behind the prompt.
	if ts := st.tabStates[st.tabBar.Tabs[idx].Editor]; ts != nil {
		ts.compareDiff = nil
	}
	st.switchTab(idx)
	st.invalidate()
}

// clobberOverwrite resolves the prompt by writing the buffer over the file.
func (st *appState) clobberOverwrite() {
	idx := st.saveMenu.tabIdx
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		st.dismissClobberPrompt()
		return
	}
	tab := st.tabBar.Tabs[idx]
	closeAfter, forQuit := st.saveMenu.closeAfterSave, st.saveMenu.forQuit
	st.clearConflict(tab)
	st.saveMenu.confirmClobber = false
	st.saveMenu.visible = false
	if !st.forceSaveTab(tab) {
		return
	}
	st.showSaveNotification(tab.Editor.FilePath)
	if closeAfter {
		st.forceCloseTab(idx)
	}
	if forQuit {
		st.continueQuitFlow()
	}
	st.updateWindowTitle()
}

// clobberReload resolves the prompt by taking the file's version. The buffer's
// text is not lost: Reload records the swap in history, so one undo brings it
// back.
func (st *appState) clobberReload() {
	idx := st.saveMenu.tabIdx
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		st.dismissClobberPrompt()
		return
	}
	tab := st.tabBar.Tabs[idx]
	st.saveMenu.confirmClobber = false
	st.saveMenu.visible = false
	st.quitInProgress = false
	if err := st.reloadEditorFromDisk(tab.Editor); err != nil {
		st.notify("Could not reload "+filepath.Base(tab.Editor.FilePath)+": "+err.Error(), 10*time.Second)
		return
	}
	st.refreshGitDiffForEditor(tab.Editor)
	st.notify("Reloaded: "+filepath.Base(tab.Editor.FilePath), 5*time.Second)
	st.updateWindowTitle()
}

// clobberCompare stands the prompt down and marks the buffer's divergence from
// the file on disk in the gutter. The conflict itself stays unresolved: only
// the prompt, which Compare stashes the flags of, can end it.
//
// The diff runs disk-to-buffer, so a '+' or '~' marks a line the buffer has
// that disk does not, and a deletion marker sits where disk has lines the
// buffer does not.
func (st *appState) clobberCompare() {
	idx := st.saveMenu.tabIdx
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		st.dismissClobberPrompt()
		return
	}
	tab := st.tabBar.Tabs[idx]
	ts := st.tabStates[tab.Editor]
	if ts == nil {
		st.dismissClobberPrompt()
		return
	}
	// An unreadable file — the deleted-on-disk conflict — compares as empty,
	// which is the truth: every line in the buffer is one disk no longer has.
	disk, err := os.ReadFile(tab.Editor.FilePath)
	if err != nil {
		disk = nil
	}
	ts.compareDiff = git.LineDiff(string(disk), tab.Editor.Buffer.Text())
	ts.compareCloseAfter = st.saveMenu.closeAfterSave
	ts.compareForQuit = st.saveMenu.forQuit
	// The menu goes away but quitInProgress does not: the quit is still
	// waiting on this decision, and the prompt comes back to finish it.
	st.saveMenu.confirmClobber = false
	st.saveMenu.visible = false
	st.invalidate()
}

// comparingTabState returns the active tab's state while its compare overlay
// is up, or nil.
func (st *appState) comparingTabState() *tabState {
	ts := st.activeTabState()
	if ts == nil || ts.compareDiff == nil {
		return nil
	}
	return ts
}

// endCompare closes the overlay by putting the prompt back, carrying the flags
// of the flow that raised it. Re-raising is the only way out of compare: the
// conflict is still unresolved and something has to resolve it.
func (st *appState) endCompare(ts *tabState) {
	st.raiseClobberPrompt(st.tabIndexOf(st.tabBar.ActiveTab()), ts.compareCloseAfter, ts.compareForQuit)
}

// compareCKeyExits reports whether c leaves the compare overlay rather than
// reaching the buffer. It does so only where a bare c is a command in its own
// right: vim's normal mode with nothing pending. That excludes text (insert
// mode, and every mode when vim is off) and the second half of a sequence —
// ]c and [c step through the very hunks compare is showing, so c cannot be
// taken while a bracket, an operator or a count is waiting for it.
func (st *appState) compareCKeyExits() bool {
	return st.vimEnabled && st.vimState != nil && st.vimIsIdleInNormalMode()
}

// handleCompareKey routes the compare overlay's Escape and reports whether it
// consumed the event. Every other key still edits, because the buffer on
// screen is the working file; re-raising the prompt is the only way out, since
// the conflict behind it is still unresolved. Any other overlay outranks it.
func (st *appState) handleCompareKey(ke key.Event) bool {
	if ke.Name != key.NameEscape || st.overlayOwnsKeys() {
		return false
	}
	ts := st.comparingTabState()
	if ts == nil {
		return false
	}
	// Escape belongs to insert mode, and to any pending sequence it would
	// cancel, before it belongs to the overlay.
	if st.vimEnabled && st.vimState != nil && !st.vimIsIdleInNormalMode() {
		return false
	}
	st.endCompare(ts)
	return true
}

// handleConflictText routes the c that enters the compare overlay from the
// prompt and the c that leaves it again, reporting whether it consumed the
// text. c is a character, so it arrives as an edit event; the key event that
// accompanies it deliberately does nothing with c, because acting on both
// would toggle compare straight back off.
func (st *appState) handleConflictText(text string) bool {
	if text != "c" {
		return false
	}
	if st.saveMenu.visible {
		if st.saveMenu.confirmClobber {
			st.clobberCompare()
			return true
		}
		return false
	}
	if ts := st.comparingTabState(); ts != nil && st.compareCKeyExits() {
		st.endCompare(ts)
		return true
	}
	return false
}

// vimIsIdleInNormalMode reports whether vim is in normal mode with no count,
// operator or multi-key sequence waiting for its next key.
func (st *appState) vimIsIdleInNormalMode() bool {
	v := st.vimState
	return v.Mode == vim.ModeNormal && v.PendingBuf == "" && v.Operator == vim.OpNone &&
		v.Count == 0 && !v.WaitingForChar && !v.WaitingForTextObj
}

// dismissClobberPrompt closes the prompt leaving the conflict unresolved: no
// write, no reload, the tab still open and any quit flow abandoned.
func (st *appState) dismissClobberPrompt() {
	st.saveMenu.confirmClobber = false
	st.saveMenu.visible = false
	st.quitInProgress = false
	st.invalidate()
}

// clearConflict marks a tab's disagreement with disk as resolved.
func (st *appState) clearConflict(tab *ui.Tab) {
	ts := st.tabStates[tab.Editor]
	if ts == nil {
		return
	}
	ts.conflict = conflictNone
	ts.deleteForcedModified = false
}

// tabConflict returns the unresolved conflict state of a tab, if any.
func (st *appState) tabConflict(tab *ui.Tab) conflictState {
	if tab == nil || tab.Editor == nil {
		return conflictNone
	}
	if ts := st.tabStates[tab.Editor]; ts != nil {
		return ts.conflict
	}
	return conflictNone
}

// --- Clobber prompt input ---

// handleClobberClick routes a click in the clobber sub-state, whose two rows
// are a warning line and an [Overwrite | Reload | Compare | Cancel] split.
func (st *appState) handleClobberClick(x, y, dy, dw, itemH int) {
	rowY := dy + itemH // the warning line is not clickable
	if y < rowY || y >= rowY+itemH {
		return
	}
	dx, _, _, _, _ := st.saveMenuRect()
	cellW := dw / clobberCellCount
	switch {
	case x < dx+cellW:
		st.clobberOverwrite()
	case x < dx+cellW*2:
		st.clobberReload()
	case x < dx+cellW*3:
		st.clobberCompare()
	default:
		st.dismissClobberPrompt()
	}
}

// handleClobberKey routes a key in the clobber sub-state. Return is Cancel,
// not Overwrite or Reload: both of those discard one of the two versions of
// the file, and a reflexive Return must not be the thing that loses work.
// Compare answers to c, which is a character and so arrives at
// handleConflictText instead.
func (st *appState) handleClobberKey(ke key.Event) {
	switch ke.Name {
	case key.NameEscape, key.NameReturn:
		st.dismissClobberPrompt()
	}
}

// statusConflictBadge is the status-bar text for the active tab's standing
// disagreement with disk, or "" when there is none. While the compare overlay
// is up it names the overlay and its orientation instead of the conflict: the
// gutter is marking what the buffer has that disk does not.
func (st *appState) statusConflictBadge() string {
	if st.comparingTabState() != nil {
		exit := "Esc"
		if st.compareCKeyExits() {
			exit = "c/Esc"
		}
		return "Comparing with disk — markers are the buffer's; " + exit + " returns"
	}
	if c := st.tabConflict(st.tabBar.ActiveTab()); c != conflictNone {
		return "! " + c.label()
	}
	return ""
}

// clobberWarning is the prompt's headline for a tab's conflict.
func (st *appState) clobberWarning(tab *ui.Tab) string {
	if st.tabConflict(tab) == conflictDeleted {
		return "Deleted on disk"
	}
	return "Changed on disk"
}
