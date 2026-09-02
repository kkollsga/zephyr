package main

import (
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/kristianweb/zephyr/internal/render"
)

// editorPress builds a primary press at a point inside the editor text area.
func editorPress(x, y int) pointer.Event {
	return pointer.Event{
		Kind:     pointer.Press,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(float32(x), float32(y)),
	}
}

func findFocusApp(t *testing.T) *appState {
	t.Helper()
	st, _, _ := testAppWithText("one one one\nsecond line\n", "Plain Text")
	st.cursorRend = &render.CursorRenderer{}
	st.lastMaxX = 900
	st.lastMaxY = 600
	st.tabBarHeight = 28
	return st
}

func TestFindBarFocusRoutesKeysToBarOrBuffer(t *testing.T) {
	st := findFocusApp(t)
	ed := st.activeEd()
	st.openFindBar(false)
	st.findBar.Query = "one"
	st.findBar.CursorPos = 3
	st.updateSearchResults()
	if !st.findBar.Focused || st.findBar.MatchCount != 3 {
		t.Fatalf("opened bar focused=%v matches=%d", st.findBar.Focused, st.findBar.MatchCount)
	}

	// Focused: typing edits the query and Return steps through matches.
	st.handleTextInput("x")
	if st.findBar.Query != "onex" || strings.Contains(ed.Buffer.Text(), "onex") {
		t.Fatalf("focused typing went to the buffer: query=%q text=%q", st.findBar.Query, ed.Buffer.Text())
	}
	st.findBar.Query = "one"
	st.findBar.CursorPos = 3
	st.updateSearchResults()
	before := st.findBar.CurrentMatch
	st.dispatchKey(key.Event{Name: key.NameReturn, State: key.Press})
	if st.findBar.CurrentMatch == before {
		t.Fatalf("focused Return did not advance the match (still %d)", before)
	}

	// A press in the editor text area hands the keyboard back to the buffer.
	st.handlePointer(editorPress(400, st.tabBarHeight+40))
	if !st.findBar.Visible || st.findBar.Focused {
		t.Fatalf("editor press left bar visible=%v focused=%v", st.findBar.Visible, st.findBar.Focused)
	}
	if len(st.findBar.Matches) != 3 {
		t.Fatalf("unfocusing dropped the highlighted matches: %d", len(st.findBar.Matches))
	}

	// Unfocused: typing edits the buffer and Return inserts a newline.
	st.handleTextInput("Z")
	if st.findBar.Query != "one" || !strings.Contains(ed.Buffer.Text(), "Z") {
		t.Fatalf("unfocused typing went to the bar: query=%q text=%q", st.findBar.Query, ed.Buffer.Text())
	}
	lines := ed.Buffer.LineCount()
	st.dispatchKey(key.Event{Name: key.NameReturn, State: key.Press})
	if ed.Buffer.LineCount() != lines+1 {
		t.Fatalf("unfocused Return did not reach the buffer: lines %d -> %d", lines, ed.Buffer.LineCount())
	}

	// Cmd+F takes focus back without closing or clearing the bar.
	st.dispatchKey(key.Event{Name: "F", Modifiers: key.ModShortcut, State: key.Press})
	if !st.findBar.Visible || !st.findBar.Focused {
		t.Fatalf("Cmd+F did not refocus: visible=%v focused=%v", st.findBar.Visible, st.findBar.Focused)
	}
}

func TestUnfocusedFindBarEscapeClosesIt(t *testing.T) {
	st := findFocusApp(t)
	st.openFindBar(false)
	st.findBar.Query = "one"
	st.updateSearchResults()
	if !st.findBar.Focused {
		t.Fatal("openFindBar left the bar unfocused")
	}
	st.handlePointer(editorPress(400, st.tabBarHeight+40))
	if st.findBar.Focused {
		t.Fatal("editor press did not unfocus the bar")
	}
	st.dispatchKey(key.Event{Name: key.NameEscape, State: key.Press})
	if st.findBar.Visible {
		t.Fatal("Escape on an unfocused bar did not close it")
	}
}

func TestFindBarClickRefocusesAndSelectsField(t *testing.T) {
	st := findFocusApp(t)
	st.openFindBar(true)
	st.findBar.Query = "one"
	st.findBar.Replacement = "two"
	st.updateSearchResults()
	st.findBar.Blur()

	g := computeFindBarGeom(st.lastMaxX, true)
	windowY := func(relative int) int { return st.tabBarHeight + g.barY + relative }

	if !st.handleFindBarClick(g.barX+g.inputX+10, windowY(g.rowY2+2)) {
		t.Fatal("replace field click was not consumed")
	}
	if !st.findBar.Focused || st.findBar.FocusField != 1 {
		t.Fatalf("replace field click focused=%v field=%d", st.findBar.Focused, st.findBar.FocusField)
	}

	st.findBar.Blur()
	if !st.handleFindBarClick(g.barX+g.inputX+10, windowY(g.rowY+2)) {
		t.Fatal("query field click was not consumed")
	}
	if !st.findBar.Focused || st.findBar.FocusField != 0 {
		t.Fatalf("query field click focused=%v field=%d", st.findBar.Focused, st.findBar.FocusField)
	}
}
