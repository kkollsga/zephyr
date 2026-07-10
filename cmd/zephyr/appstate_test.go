package main

import (
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/config"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/fileio"
	"github.com/kristianweb/zephyr/internal/highlight"
	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/ui"
)

func testAppWithText(text, lang string) (*appState, *editor.Editor, *tabState) {
	ed := editor.NewEditor(buffer.NewFromString(text), "")
	tb := ui.NewTabBar()
	tb.OpenEditor(ed, "test")
	ts := &tabState{
		langLabel: lang,
		viewport:  render.NewViewport(),
		foldState: render.NewFoldState(),
	}
	st := &appState{
		tabBar:      tb,
		tabStates:   map[*editor.Editor]*tabState{ed: ts},
		textRend:    &render.TextRenderer{CharWidth: 8, CharAdvance: 8, LineHeightPx: 20},
		gutterRend:  &render.GutterRenderer{CharWidth: 8, LineHeight: 20},
		tabRend:     &render.TextRenderer{CharWidth: 8, LineHeightPx: 18},
		theme:       config.DarkTheme(),
		themeBundle: config.DefaultBundle(),
		fontCfg:     config.DefaultFontConfig(),
		findBar:     ui.NewFindReplaceBar(),
		langSel:     ui.NewLanguageSelector(),
	}
	return st, ed, ts
}

func TestPointerToLineColIntegratesPixelOffsetTabsWrapAndFolds(t *testing.T) {
	st, _, ts := testAppWithText("a\tbcd\nsecond\nthird\nfourth\n", "Go")
	// Three lines means a five-character gutter (40px), plus one character pad.
	textX := float32(40 + 8)
	st.tabBarHeight = 28
	ts.viewport.FirstLine = 1
	ts.viewport.PixelOffset = 19
	line, col := st.pointerToLineCol(f32.Pt(textX+2*8, float32(28+editorTopPad+1)))
	if line != 2 || col != 2 {
		t.Fatalf("fractional-scroll pointer = (%d,%d), want (2,2)", line, col)
	}

	ts.viewport.FirstLine = 0
	ts.viewport.PixelOffset = 0
	line, col = st.pointerToLineCol(f32.Pt(textX+2*8, float32(28+editorTopPad)))
	if line != 0 || col != 1 {
		t.Fatalf("pointer in first half of expanded tab = (%d,%d), want (0,1)", line, col)
	}
	line, col = st.pointerToLineCol(f32.Pt(textX+3*8, float32(28+editorTopPad)))
	if line != 0 || col != 2 {
		t.Fatalf("pointer in second half of expanded tab = (%d,%d), want (0,2)", line, col)
	}

	ts.wrapMap = buildWrapMap([]string{"abcdefghij", "second", "third", "fourth"}, 5, 4)
	line, col = st.pointerToLineCol(f32.Pt(textX+2*8, float32(28+editorTopPad+20)))
	if line != 0 || col != 5 {
		t.Fatalf("wrapped pointer = (%d,%d), want (0,5)", line, col)
	}

	ts.wrapMap = nil
	ts.foldState.SetRegions([]render.FoldRegion{{StartLine: 0, EndLine: 2}}, 4)
	ts.foldState.Toggle(0, 4)
	line, _ = st.pointerToLineCol(f32.Pt(textX, float32(28+editorTopPad+20)))
	if line != 3 {
		t.Fatalf("folded display line maps to buffer line %d, want 3", line)
	}
}

func TestCancelPointerGestureClearsAllDragState(t *testing.T) {
	st, _, ts := testAppWithText("text", "Plain Text")
	st.pointerActive = true
	st.dragging = true
	st.tabDrag.active = true
	st.tabDrag.started = true
	st.tabDrag.fromDropdown = true
	ts.mdSelActive = true

	st.cancelPointerGesture()

	if st.pointerActive || st.dragging || st.tabDrag.active || st.tabDrag.started || st.tabDrag.fromDropdown || ts.mdSelActive {
		t.Fatalf("gesture state survived cancellation: pointer=%v drag=%+v markdown=%v", st.pointerActive, st.tabDrag, ts.mdSelActive)
	}
}

func TestMarkdownModeRoundTrip(t *testing.T) {
	st, _, ts := testAppWithText("# Heading\n\nBody", "Markdown")
	ts.mode = viewEdit
	ts.mdScrollY = 99

	st.toggleMarkdownPreview()
	if ts.mode != viewMarkdownRead || ts.mdDoc == nil || len(ts.mdDoc.Blocks) != 2 || ts.mdScrollY != 0 {
		t.Fatalf("read transition state = mode %v doc %#v scroll %v", ts.mode, ts.mdDoc, ts.mdScrollY)
	}
	st.toggleMarkdownPreview()
	if ts.mode != viewEdit || ts.lastCursorLine != -1 || ts.lastCursorCol != -1 {
		t.Fatalf("edit transition state = mode %v last=(%d,%d)", ts.mode, ts.lastCursorLine, ts.lastCursorCol)
	}
	ts.langLabel = "Go"
	st.toggleMarkdownPreview()
	if ts.mode != viewEdit {
		t.Fatal("non-Markdown tab changed preview mode")
	}
}

func TestMarkdownSelectionHelpers(t *testing.T) {
	blocks := []mdSelBlock{
		{y: 10, h: 40, x: 20, textOff: 3, textLen: 11, lineH: 20, charW: 5, lineLens: []int{5, 5}},
	}
	tests := []struct {
		px, py, want int
	}{{0, 0, 0}, {20, 10, 3}, {35, 10, 6}, {25, 35, 10}, {100, 35, 14}, {0, 60, 14}}
	for _, tt := range tests {
		if got := mdCharOffset(blocks, tt.px, tt.py); got != tt.want {
			t.Errorf("mdCharOffset(%d,%d) = %d, want %d", tt.px, tt.py, got, tt.want)
		}
	}
	if got := mdSelectedText("0123456789", 8, 3); got != "34567" {
		t.Fatalf("reverse selection = %q", got)
	}
	if got := mdSelectedText("012345", -2, 99); got != "012345" {
		t.Fatalf("clamped selection = %q", got)
	}
}

func TestMarkdownPlainTextIncludesContainerContent(t *testing.T) {
	blocks := []render.Block{
		{Kind: render.BlockParagraph, Spans: []render.InlineSpan{{Text: "intro"}}},
		{Kind: render.BlockCodeBlock, CodeText: "code()"},
		{Kind: render.BlockTable, TableCells: [][]string{{"a", "b"}}},
	}
	quote := render.Block{Kind: render.BlockBlockquote, Children: blocks}
	if got := blockPlainText(&quote); got != "intro\ncode()\na\tb" {
		t.Fatalf("blockquote plain text = %q", got)
	}
}

func TestSaveTabToPathLifecycle(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.go")
	if err := os.WriteFile(oldPath, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	ed, err := editor.NewEditorFromFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	ed.Cursor.Col = len("before")
	ed.InsertText(" after")
	tb := ui.NewTabBar()
	tb.OpenEditor(ed, "old.txt")
	ts := &tabState{langLabel: "Plain Text"}
	watcher, err := fileio.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = watcher.Close() })
	if err := watcher.Watch(oldPath); err != nil {
		t.Fatal(err)
	}
	st := &appState{tabBar: tb, tabStates: map[*editor.Editor]*tabState{ed: ts}, watcher: watcher}

	if !st.saveTabToPath(tb.Tabs[0], newPath) {
		t.Fatal("saveTabToPath failed")
	}
	if ed.FilePath != newPath || ed.Modified || tb.Tabs[0].Title != "new.go" || tb.Tabs[0].IsUntitled || ts.langLabel != "Go" || ts.highlighter == nil {
		t.Fatalf("save state path=%q modified=%v tab=%+v lang=%q highlighter=%v", ed.FilePath, ed.Modified, tb.Tabs[0], ts.langLabel, ts.highlighter)
	}
	t.Cleanup(func() {
		if ts.highlighter != nil {
			ts.highlighter.Close()
		}
	})
	if got, err := os.ReadFile(newPath); err != nil || string(got) != "before after" {
		t.Fatalf("saved content %q, err=%v", got, err)
	}
	if err := os.WriteFile(oldPath, []byte("external old write"), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(150 * time.Millisecond)
oldWatchCheck:
	for {
		select {
		case ev := <-watcher.Events:
			if ev.Path == oldPath {
				t.Fatalf("old path remained watched after Save As: %+v", ev)
			}
		case <-deadline:
			break oldWatchCheck
		}
	}
	if err := os.WriteFile(newPath, []byte("external new write"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-watcher.Events:
		if ev.Path != newPath {
			t.Fatalf("watcher reported %q, want %q", ev.Path, newPath)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("new path was not watched after Save As")
	}

	plainPath := filepath.Join(dir, "plain.txt")
	if !st.saveTabToPath(tb.Tabs[0], plainPath) {
		t.Fatal("second Save As failed")
	}
	if ts.langLabel != "Plain Text" || ts.highlighter != nil || ts.sourceBuf != nil {
		t.Fatalf("plain-text Save As retained highlighting: lang=%q highlighter=%v source=%d bytes", ts.langLabel, ts.highlighter, len(ts.sourceBuf))
	}
}

func TestSaveTabToPathFailureRetainsState(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(oldPath, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	ed, err := editor.NewEditorFromFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	ed.Cursor.Col = len("before")
	ed.InsertText(" changed")
	tb := ui.NewTabBar()
	tb.OpenEditor(ed, "old.txt")
	ts := &tabState{langLabel: "Plain Text"}
	st := &appState{tabBar: tb, tabStates: map[*editor.Editor]*tabState{ed: ts}}

	badPath := filepath.Join(dir, "missing", "new.go")
	if st.saveTabToPath(tb.Tabs[0], badPath) {
		t.Fatal("save unexpectedly succeeded")
	}
	if ed.FilePath != oldPath || !ed.Modified || tb.Tabs[0].Title != "old.txt" || ts.langLabel != "Plain Text" || ts.highlighter != nil {
		t.Fatalf("failure mutated state: path=%q modified=%v tab=%+v lang=%q highlighter=%v", ed.FilePath, ed.Modified, tb.Tabs[0], ts.langLabel, ts.highlighter)
	}
}

func TestTabAndWrapDisplayMath(t *testing.T) {
	if got := clipTabTitle("short.md"); got != "short.md" {
		t.Fatalf("short title = %q", got)
	}
	if got := clipTabTitle("very_long_filename.go"); got != "very_long_file….go" {
		t.Fatalf("long title = %q", got)
	}
	if got := clipTabTitle("abcdefghijklmnop.abcdef"); got != "abcdefghijklmn….abc" {
		t.Fatalf("long extension title = %q", got)
	}

	wm := buildWrapMap([]string{"short", "a\tbcdefgh", ""}, 5, 4)
	if wm.visualLines() != 5 || wm.segmentCount(1) != 3 || wm.segmentCount(99) != 1 {
		t.Fatalf("wrap counts total=%d line1=%d", wm.visualLines(), wm.segmentCount(1))
	}
	if line, seg := wm.bufferLineForVisual(2); line != 1 || seg != 1 {
		t.Fatalf("visual mapping = (%d,%d)", line, seg)
	}
	if line, col := wm.bufferToVisual(1, 7); line != 2 || col != 2 {
		t.Fatalf("buffer mapping = (%d,%d)", line, col)
	}
	if start, end := wm.segmentRange(1, 1); start != 5 || end != 10 {
		t.Fatalf("segment range = [%d,%d)", start, end)
	}
}

func TestTabOverflowKeepsActiveTabVisibleAndSlotMappingStable(t *testing.T) {
	st, _, _ := testAppWithText("", "Plain Text")
	for i := 1; i < 6; i++ {
		st.tabBar.OpenEditor(editor.NewEmptyEditor(), "file-number.go")
	}
	st.computeOverflow(2000)
	if len(st.barTabIdxs) != 6 || len(st.dropdownTabIdxs) != 0 || st.overflowStartIdx != 6 {
		t.Fatalf("wide overflow state bar=%v dropdown=%v start=%d", st.barTabIdxs, st.dropdownTabIdxs, st.overflowStartIdx)
	}

	st.tabBar.ActiveIdx = 5
	st.computeOverflow(260)
	if len(st.dropdownTabIdxs) == 0 || !containsInt(st.barTabIdxs, 5) {
		t.Fatalf("active overflow tab not visible: bar=%v dropdown=%v", st.barTabIdxs, st.dropdownTabIdxs)
	}
	all := append(append([]int(nil), st.barTabIdxs...), st.dropdownTabIdxs...)
	sorted := append([]int(nil), all...)
	sort.Ints(sorted)
	if !reflect.DeepEqual(sorted, []int{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("overflow partition lost or duplicated tabs: %v", all)
	}

	st.barTabIdxs = []int{0, 3}
	st.dropdownTabIdxs = []int{1, 2, 4, 5}
	if got := st.visualSlotToFlat(0, 3); got != 0 {
		t.Fatalf("first visual slot = %d", got)
	}
	if got := st.visualSlotToFlat(4, 3); got != 4 {
		t.Fatalf("last visual slot = %d", got)
	}
}

func containsInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestDisplayColumnWrappersAndTokenRange(t *testing.T) {
	line := "α\tb"
	if got := expandTabs(line, 4); got != "α   b" {
		t.Fatalf("expandTabs = %q", got)
	}
	if got := runeColToDisplayCol(line, 2, 4); got != 4 {
		t.Fatalf("runeColToDisplayCol = %d", got)
	}
	if got := displayColToRuneCol(line, 3, 4); got != 2 {
		t.Fatalf("displayColToRuneCol = %d", got)
	}
	if got := matchDisplayLen(line, 1, 1, 4); got != 3 {
		t.Fatalf("matchDisplayLen = %d", got)
	}
	tokens := []highlight.Token{{StartByte: 0, EndByte: 2}, {StartByte: 3, EndByte: 5}, {StartByte: 6, EndByte: 9}}
	if got := tokensForRange(tokens, 2, 7); !reflect.DeepEqual(got, tokens[1:]) {
		t.Fatalf("tokensForRange = %#v", got)
	}
}

func TestPureLayoutHelpers(t *testing.T) {
	geom := computeFindBarGeom(900, true)
	if geom.barW != 320 || geom.barH != 68 || geom.rowY2 <= geom.rowY || geom.inputW <= 0 {
		t.Fatalf("find geometry = %+v", geom)
	}
	if got := dedent("\t  "); got != "" {
		t.Fatalf("dedent tab = %q", got)
	}
	if got := dedent("      "); got != "  " {
		t.Fatalf("dedent spaces = %q", got)
	}
	if got := lastWord("hello foo_bar42"); got != "foo_bar42" {
		t.Fatalf("lastWord = %q", got)
	}
	if got := shiftColor(color.NRGBA{R: 250, G: 5, B: 100, A: 7}, 20); got != (color.NRGBA{R: 255, G: 25, B: 120, A: 7}) {
		t.Fatalf("shiftColor = %v", got)
	}
	theme := config.DarkTheme()
	if codeLineColor("// comment", "go", theme) != theme.Comment {
		t.Fatal("codeLineColor did not identify comment")
	}
}

func TestIsPrimaryTouchPress(t *testing.T) {
	if !isPrimaryPointerPress(pointer.Event{Source: pointer.Touch}) {
		t.Fatal("touch press should act as primary")
	}
}
