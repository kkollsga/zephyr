package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/ui"
)

func TestSavePromptStateAndFilenameEditing(t *testing.T) {
	st, _, ts := testAppWithText("content", "Go")
	st.barTabIdxs = []int{0}
	st.lastMaxX = 900
	st.tabBarHeight = 28
	ts.langLabel = "Go"

	st.showSaveMenu(0, true, true)
	if !st.saveMenu.visible || st.saveMenu.tabIdx != 0 || !st.saveMenu.closeAfterSave || !st.saveMenu.forQuit {
		t.Fatalf("save menu flags = %+v", st.saveMenu)
	}
	if got := string(st.saveMenu.filename); got != "Untitled.go" || !st.saveMenu.selectAll || st.saveMenu.dir == "" {
		t.Fatalf("default Save As fields name=%q select=%v dir=%q", got, st.saveMenu.selectAll, st.saveMenu.dir)
	}
	if !st.saveMenuShowSaveAs() || !st.saveMenuCanSave() {
		t.Fatal("untitled save prompt did not expose a valid Save As form")
	}

	st.saveAsInsertText("renamed.go")
	st.saveAsInsertText("x")
	if got := string(st.saveMenu.filename); got != "renamed.gox" {
		t.Fatalf("insert filename = %q", got)
	}
	st.saveAsDeleteBack()
	st.saveMenu.cursor = 0
	st.saveAsDeleteForward()
	if got := string(st.saveMenu.filename); got != "enamed.go" {
		t.Fatalf("edited filename = %q", got)
	}
	st.saveMenu.selectAll = true
	st.saveAsDeleteBack()
	if len(st.saveMenu.filename) != 0 || st.saveMenu.cursor != 0 || st.saveMenu.selectAll {
		t.Fatalf("select-all backspace state = %+v", st.saveMenu)
	}

	st.saveMenu.filename = []rune("name.go")
	x, y, w, h, itemH := st.saveMenuRect()
	if w == 0 || h == 0 || itemH == 0 || y != st.tabBarHeight {
		t.Fatalf("save menu rect=(%d,%d,%d,%d) item=%d", x, y, w, h, itemH)
	}
	st.handleSaveMenuClick(x-1, y)
	if st.saveMenu.visible || st.quitInProgress {
		t.Fatal("outside click did not cancel save prompt")
	}
}

func TestFileBackedSavePromptRowsAndClicks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.go")
	if err := os.WriteFile(path, []byte("package p"), 0o600); err != nil {
		t.Fatal(err)
	}
	ed, err := editor.NewEditorFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tb := ui.NewTabBar()
	tb.OpenEditor(ed, "file.go")
	ts := &tabState{viewport: render.NewViewport(), foldState: render.NewFoldState(), langLabel: "Go"}
	st := &appState{
		tabBar: tb, tabStates: map[*editor.Editor]*tabState{ed: ts},
		tabRend:    &render.TextRenderer{CharWidth: 8, LineHeightPx: 18},
		barTabIdxs: []int{0}, lastMaxX: 900, tabBarHeight: 28,
	}

	st.showSaveMenu(0, false, false)
	if st.saveMenuShowSaveAs() || !st.saveMenuCanSave() || st.saveMenuRowCount() != 2 {
		t.Fatalf("collapsed file prompt showAs=%v canSave=%v rows=%d", st.saveMenuShowSaveAs(), st.saveMenuCanSave(), st.saveMenuRowCount())
	}
	dx, dy, dw, _, itemH := st.saveMenuRect()
	st.handleSaveMenuClick(dx+dw/2, dy+itemH/2)
	if !st.saveMenu.saveAsExpanded || !st.saveMenuShowSaveAs() {
		t.Fatal("Save As radio row did not expand")
	}
	rows := st.saveMenuRowCount()
	if rows < 4 {
		t.Fatalf("expanded row count = %d", rows)
	}
	st.saveMenu.confirmOverwrite = true
	if st.saveMenuRowCount() != rows+2 {
		t.Fatal("overwrite confirmation rows not included")
	}

	// Name-row click moves the rune cursor and clears select-all.
	dx, dy, _, _, itemH = st.saveMenuRect()
	st.handleSaveMenuClick(dx+80, dy+itemH/2)
	if st.saveMenu.selectAll {
		t.Fatal("name-row click retained select-all")
	}
}

func TestExecuteSaveAsOverwriteAndSuccessOutcomes(t *testing.T) {
	st, ed, ts := testAppWithText("save me", "Plain Text")
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(existing, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	st.saveMenu.visible = true
	st.saveMenu.tabIdx = 0
	st.saveMenu.dir = dir
	st.saveMenu.filename = []rune("existing.txt")
	st.executeSaveAs()
	gotExisting, _ := os.ReadFile(existing)
	if !st.saveMenu.confirmOverwrite || string(gotExisting) != "old" {
		t.Fatal("existing target was overwritten without confirmation")
	}

	st.saveMenu.confirmOverwrite = false
	st.saveMenu.filename = []rune("created.go")
	st.executeSaveAs()
	created := filepath.Join(dir, "created.go")
	if st.saveMenu.visible || ed.FilePath != created || ed.Modified || ts.langLabel != "Go" {
		t.Fatalf("successful Save As state visible=%v path=%q modified=%v lang=%q", st.saveMenu.visible, ed.FilePath, ed.Modified, ts.langLabel)
	}
	if got, err := os.ReadFile(created); err != nil || string(got) != "save me" {
		t.Fatalf("created content=%q err=%v", got, err)
	}
	if !strings.Contains(st.notification, "created.go") || st.notificationUntil.Before(time.Now()) {
		t.Fatalf("save notification=%q until=%v", st.notification, st.notificationUntil)
	}
	if ts.highlighter != nil {
		ts.highlighter.Close()
	}
}

func TestQuitFlowAdvancesAcrossUnsavedTabs(t *testing.T) {
	st, first, _ := testAppWithText("first", "Plain Text")
	second := editor.NewEditor(buffer.NewFromString("second"), "")
	st.tabBar.OpenEditor(second, "second")
	st.tabStates[second] = &tabState{viewport: render.NewViewport(), foldState: render.NewFoldState()}
	first.Modified = true
	second.Modified = true

	if !st.hasUnsavedChanges() {
		t.Fatal("modified tabs were not detected")
	}
	st.startQuitFlow()
	if !st.quitInProgress || !st.saveMenu.visible || st.saveMenu.tabIdx != 0 || !st.saveMenu.forQuit {
		t.Fatalf("initial quit prompt = %+v", st.saveMenu)
	}
	first.Modified = false
	st.continueQuitFlow()
	if st.saveMenu.tabIdx != 1 {
		t.Fatalf("quit flow did not advance to second tab: %+v", st.saveMenu)
	}
	second.Modified = false
	st.continueQuitFlow()
	if !st.exitPending || st.notification != "Closing…" || st.hasUnsavedChanges() {
		t.Fatalf("completed quit state pending=%v notification=%q", st.exitPending, st.notification)
	}
	deadline := st.exitDeadline
	st.gracefulExit()
	if st.exitDeadline != deadline {
		t.Fatal("repeated gracefulExit reset the deadline")
	}
}

func TestExternalFileChangeReloadWarnAndNoopBranches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watched.go")
	if err := os.WriteFile(path, []byte("package old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ed, err := editor.NewEditorFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tb := ui.NewTabBar()
	tb.OpenEditor(ed, "watched.go")
	ts := &tabState{viewport: render.NewViewport(), foldState: render.NewFoldState(), lastCursorLine: 9, lastCursorCol: 9}
	st := &appState{tabBar: tb, tabStates: map[*editor.Editor]*tabState{ed: ts}}

	st.handleExternalFileChange(path)
	if st.notification != "" {
		t.Fatalf("identical disk event produced notification %q", st.notification)
	}
	if err := os.WriteFile(path, []byte("package fresh\nfunc f() {\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st.handleExternalFileChange(path)
	if ed.Buffer.Text() != "package fresh\nfunc f() {\n}\n" || !strings.HasPrefix(st.notification, "Reloaded:") || ts.lastCursorLine != -1 || ts.foldState.RegionAt(1) == nil {
		t.Fatalf("reload state text=%q notification=%q last=%d folds=%#v", ed.Buffer.Text(), st.notification, ts.lastCursorLine, ts.foldState.Regions)
	}

	ed.Cursor.SetPosition(ed.Buffer, 0, 0)
	ed.InsertText("local ")
	local := ed.Buffer.Text()
	if err := os.WriteFile(path, []byte("external\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st.handleExternalFileChange(path)
	if ed.Buffer.Text() != local || !strings.HasPrefix(st.notification, "File changed externally:") {
		t.Fatalf("modified editor was overwritten: text=%q notification=%q", ed.Buffer.Text(), st.notification)
	}
	st.handleExternalFileChange(filepath.Join(dir, "other.go"))
}

func TestShortenDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	if got := shortenDir(filepath.Join(home, "project")); got != filepath.Join("~", "project") {
		t.Fatalf("shortenDir = %q", got)
	}
	if got := shortenDir("/definitely/not/home"); got != "/definitely/not/home" {
		t.Fatalf("non-home path changed to %q", got)
	}
}
