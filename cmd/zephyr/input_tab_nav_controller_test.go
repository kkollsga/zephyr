package main

import (
	"reflect"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/vim"
)

func TestHandleTextInputRoutesOverlaysPairsAndPlainText(t *testing.T) {
	st, ed, _ := testAppWithText("", "Plain Text")
	st.handleTextInput("(")
	if ed.Buffer.Text() != "()" || ed.Cursor.Col != 1 || !st.reparsePending {
		t.Fatalf("auto pair text=%q cursor=%+v pending=%v", ed.Buffer.Text(), ed.Cursor, st.reparsePending)
	}
	st.handleTextInput(")")
	if ed.Buffer.Text() != "()" || ed.Cursor.Col != 2 {
		t.Fatalf("closer skip text=%q cursor=%+v", ed.Buffer.Text(), ed.Cursor)
	}
	st.handleTextInput(" plain")
	if ed.Buffer.Text() != "() plain" {
		t.Fatalf("plain input text=%q", ed.Buffer.Text())
	}

	st.findBar.Visible = true
	st.findBar.Query = ""
	st.findBar.CursorPos = 0
	st.handleTextInput("needle")
	if st.findBar.Query != "needle" || ed.Buffer.Text() != "() plain" {
		t.Fatalf("find routing query=%q editor=%q", st.findBar.Query, ed.Buffer.Text())
	}
	st.findBar.Visible = false

	st.saveMenu.visible = true
	st.saveMenu.tabIdx = 0
	st.saveMenu.selectAll = true
	st.handleTextInput("saved.go")
	if got := string(st.saveMenu.filename); got != "saved.go" {
		t.Fatalf("save routing filename=%q", got)
	}
}

func TestHandleKeyEditingNavigationAndOverlayBranches(t *testing.T) {
	st, ed, _ := testAppWithText("abc\n    second\nthird", "Plain Text")
	ed.Cursor.SetPosition(ed.Buffer, 0, 1)
	st.handleKey(key.Event{Name: key.NameRightArrow})
	st.handleKey(key.Event{Name: key.NameEnd})
	if ed.Cursor.Col != 3 {
		t.Fatalf("right/end cursor=%+v", ed.Cursor)
	}
	st.handleKey(key.Event{Name: key.NameDownArrow})
	st.handleKey(key.Event{Name: key.NameHome})
	if ed.Cursor.Line != 1 || ed.Cursor.Col != 0 {
		t.Fatalf("down/home cursor=%+v", ed.Cursor)
	}
	st.handleKey(key.Event{Name: key.NameEnd})
	st.handleKey(key.Event{Name: key.NameDeleteBackward})
	st.handleKey(key.Event{Name: key.NameReturn})
	st.handleKey(key.Event{Name: key.NameTab})
	if !ed.Modified || !st.reparsePending || ed.Cursor.Line != 2 {
		t.Fatalf("editing state modified=%v pending=%v cursor=%+v", ed.Modified, st.reparsePending, ed.Cursor)
	}
	st.handleKey(key.Event{Name: "Z", Modifiers: key.ModShortcut})
	st.handleKey(key.Event{Name: "Z", Modifiers: key.ModShortcut | key.ModShift})
	st.handleKey(key.Event{Name: key.NameUpArrow, Modifiers: key.ModShortcut})
	if ed.Cursor.Line != 0 {
		t.Fatalf("file-start cursor=%+v", ed.Cursor)
	}
	st.handleKey(key.Event{Name: key.NameDownArrow, Modifiers: key.ModShortcut})
	if ed.Cursor.Line != ed.Buffer.LineCount()-1 {
		t.Fatalf("file-end cursor=%+v lines=%d", ed.Cursor, ed.Buffer.LineCount())
	}

	st.findBar.Visible = true
	st.findBar.ShowReplace = true
	st.findBar.Query = "abc"
	st.findBar.CursorPos = 3
	st.handleKey(key.Event{Name: key.NameDeleteBackward})
	st.handleKey(key.Event{Name: key.NameEscape})
	if st.findBar.Visible {
		t.Fatal("find overlay did not consume Escape")
	}

	st.langSel.Open([]string{"Go", "Markdown"})
	st.handleKey(key.Event{Name: key.NameDownArrow})
	st.handleKey(key.Event{Name: key.NameReturn})
	if st.langSel.Visible || st.activeTabState().langLabel != "Go" {
		t.Fatalf("language overlay visible=%v lang=%q", st.langSel.Visible, st.activeTabState().langLabel)
	}
	if st.activeTabState().highlighter != nil {
		st.activeTabState().highlighter.Close()
	}
}

func TestTabControllerClickDragOverflowAndTitles(t *testing.T) {
	st, ed, _ := testAppWithText("", "Plain Text")
	st.barTabIdxs = []int{0}
	firstWidth := st.tabWidth(st.tabBar.Tabs[0].Title)
	st.handleTabBarPress(10, 5)
	if !st.tabDrag.active || st.tabDrag.tabIdx != 0 {
		t.Fatalf("tab press drag=%+v", st.tabDrag)
	}
	st.handleTabBarRelease(10, 5)
	if st.tabDrag.active || st.tabBar.ActiveIdx != 0 {
		t.Fatal("tab click release did not settle")
	}

	st.handleTabBarPress(firstWidth+st.tabMetrics().tabGap+2, 5)
	if st.tabBar.TabCount() != 2 {
		t.Fatal("plus button did not create a tab")
	}
	st.barTabIdxs = []int{0, 1}
	st.tabBar.ActiveIdx = 0
	st.handleTabBarPress(10, 5)
	st.handleTabBarDrag(firstWidth*2, 5)
	if !st.tabDrag.started || st.tabDrag.dropTargetIdx != 1 {
		t.Fatalf("tab drag target=%+v", st.tabDrag)
	}
	st.handleTabBarRelease(firstWidth*2, 5)
	if st.tabBar.ActiveIdx != 1 {
		t.Fatalf("active tab did not follow move: %d", st.tabBar.ActiveIdx)
	}

	st.tabBar.ActiveIdx = 0
	active := st.tabBar.ActiveTab()
	active.IsUntitled = true
	active.Editor = ed
	active.Editor.Buffer = renderBuffer("12345678 and more")
	st.updateUntitledTitle()
	if active.Title != "12345678…" {
		t.Fatalf("dynamic untitled title=%q", active.Title)
	}
	st.afterEdit()
	st.reparseDeadline = time.Now().Add(-time.Millisecond)
	st.flushReparse()
	if st.reparsePending {
		t.Fatal("flushReparse left work pending")
	}

	st.dropdownHeader = -1
	st.dropdownTabIdxs = []int{1}
	st.overflowBtnX = 200
	st.overflowBtnW = 28
	if st.overflowDropdownWidth() <= 24 {
		t.Fatal("overflow dropdown width did not include title")
	}
	if st.handleOverflowDropdownPress(0, 0) {
		t.Fatal("outside overflow press was consumed")
	}
	dropW := st.overflowDropdownWidth()
	dropX := st.overflowBtnX + st.overflowBtnW - dropW
	if dropX < 0 {
		dropX = 0
	}
	if !st.handleOverflowDropdownPress(dropX+2, st.tabBarHeight+2) || !st.tabDrag.fromDropdown {
		t.Fatal("overflow item did not start dropdown drag")
	}
}

func renderBuffer(text string) *buffer.PieceTable {
	return buffer.NewFromString(text)
}

func TestCloseTabControllerPromptsModifiedAndClosesClean(t *testing.T) {
	st, first, _ := testAppWithText("first", "Plain Text")
	second := editor.NewEmptyEditor()
	st.tabBar.OpenEditor(second, "second")
	st.tabStates[second] = &tabState{viewport: render.NewViewport(), foldState: render.NewFoldState()}
	first.Modified = true
	st.closeTabAt(0)
	if !st.saveMenu.visible || st.saveMenu.tabIdx != 0 || !st.saveMenu.closeAfterSave {
		t.Fatalf("modified close prompt=%+v", st.saveMenu)
	}
	st.saveMenu.visible = false
	first.Modified = false
	st.closeCurrentTab()
	if st.tabBar.TabCount() != 1 {
		t.Fatalf("clean close left %d tabs", st.tabBar.TabCount())
	}
	st.closeTabAt(-1)
	st.forceCloseTab(99)
}

func TestNavigatorPureHelpersAndMarkdownScroll(t *testing.T) {
	if got := findNextInList([]string{"a", "b", "c"}, "c"); got != "a" {
		t.Fatalf("next list wrap=%q", got)
	}
	if got := findPrevInList([]string{"a", "b", "c"}, "a"); got != "c" {
		t.Fatalf("prev list wrap=%q", got)
	}
	if findNextInList(nil, "") != "" || findPrevInList(nil, "") != "" {
		t.Fatal("empty list navigation returned a value")
	}
	quoted := []struct {
		line string
		col  int
		want string
	}{{`import "pkg/file.go"`, 10, "pkg/file.go"}, {"x := 'v'", 6, "v"}, {"x := `raw`", 7, "raw"}, {"no quote", 3, ""}}
	for _, tt := range quoted {
		if got := extractQuotedString(tt.line, tt.col); got != tt.want {
			t.Errorf("extractQuotedString(%q,%d)=%q want %q", tt.line, tt.col, got, tt.want)
		}
	}

	st, _, ts := testAppWithText("# doc", "Markdown")
	ts.mode = viewMarkdownRead
	ts.mdTotalH = 1000
	st.lastMaxY = 600
	st.tabBarHeight = 28
	st.mdScroll(ts, 300, 572)
	st.mdScroll(ts, 1000, 572)
	if ts.mdScrollY != 428 {
		t.Fatalf("markdown bottom clamp=%v", ts.mdScrollY)
	}
	st.mdScroll(ts, -1000, 572)
	if ts.mdScrollY != 0 {
		t.Fatalf("markdown top clamp=%v", ts.mdScrollY)
	}
	st.executeMdReadAction(vim.Action{Kind: vim.ActionMovePageDown}, ts)
	if ts.mdScrollY != 428 {
		t.Fatalf("page-down scroll=%v", ts.mdScrollY)
	}
	st.executeMdReadAction(vim.Action{Kind: vim.ActionMoveFileStart}, ts)
	if ts.mdScrollY != 0 {
		t.Fatal("file-start action did not reset Markdown scroll")
	}
	st.executeMdReadAction(vim.Action{Kind: vim.ActionMoveFileEnd}, ts)
	if ts.mdScrollY != 428 {
		t.Fatal("file-end action did not clamp Markdown scroll")
	}

	if !st.handleStatusBufferAction(vim.Action{Kind: vim.ActionDelete}) {
		t.Fatal("status buffer did not consume editing action")
	}
	if st.handleStatusBufferAction(vim.Action{Kind: vim.ActionMoveDown}) {
		t.Fatal("status buffer consumed normal movement")
	}
	if !st.executeNavAction(vim.Action{Kind: vim.ActionNavToggleOriginal}) {
		t.Fatal("navigator stub action was not handled")
	}
}

func TestGioKeyToVimInputMappings(t *testing.T) {
	tests := []struct {
		name key.Name
		want string
	}{{key.NameEscape, vim.NameEscape}, {key.NameReturn, vim.NameReturn}, {key.NameDeleteBackward, vim.NameDeleteBackward}, {key.NameUpArrow, vim.NameUpArrow}, {key.NameTab, vim.NameTab}}
	for _, tt := range tests {
		got := gioKeyToVimInput(key.Event{Name: tt.name, Modifiers: key.ModCtrl | key.ModShift})
		if got.Name != tt.want || !got.Ctrl || !got.Shift {
			t.Errorf("key %q -> %+v, want name %q with modifiers", tt.name, got, tt.want)
		}
	}
	got := gioKeyToVimInput(key.Event{Name: "X", Modifiers: key.ModAlt | key.ModShortcut})
	if !reflect.DeepEqual(got, vim.KeyInput{Char: 'x', Alt: true, Shortcut: true}) {
		t.Fatalf("printable key mapping=%+v", got)
	}
}
