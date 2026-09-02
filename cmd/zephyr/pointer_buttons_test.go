package main

import (
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/kristianweb/zephyr/internal/render"
)

// mouseEvent builds a mouse event whose Buttons is the set of buttons still
// down after it, which is how Gio's macOS and Windows backends report them:
// os_macos.go clears the released bit from w.pointerBtns before dispatching, so
// a secondary release during a primary drag arrives with Buttons ==
// ButtonPrimary.
func mouseEvent(kind pointer.Kind, btns pointer.Buttons, x, y int) pointer.Event {
	return pointer.Event{
		Kind:     kind,
		Source:   pointer.Mouse,
		Buttons:  btns,
		Position: f32.Pt(float32(x), float32(y)),
	}
}

func TestSecondaryButtonReleaseDoesNotEndTextDrag(t *testing.T) {
	st, ed, _ := testAppWithText("hello world line one\nsecond line here with plenty more text\n", "Plain Text")
	st.cursorRend = &render.CursorRenderer{}
	st.lastMaxX = 900
	st.lastMaxY = 600
	st.tabBarHeight = 28
	textY := st.tabBarHeight + 40

	st.handlePointer(mouseEvent(pointer.Press, pointer.ButtonPrimary, 100, textY))
	st.handlePointer(mouseEvent(pointer.Drag, pointer.ButtonPrimary, 180, textY))
	if !st.dragging || ed.Selection.IsEmpty() {
		t.Fatalf("primary press+drag did not select: dragging=%v selection=%+v", st.dragging, ed.Selection)
	}
	want := ed.Selection

	// Secondary button goes down, then up, while primary stays held.
	st.handlePointer(mouseEvent(pointer.Press, pointer.ButtonPrimary|pointer.ButtonSecondary, 180, textY))
	if !st.dragging || ed.Selection != want {
		t.Fatalf("secondary press ended the drag: dragging=%v selection=%+v want %+v", st.dragging, ed.Selection, want)
	}
	st.handlePointer(mouseEvent(pointer.Release, pointer.ButtonPrimary, 180, textY))
	if !st.dragging || !st.pointerActive || ed.Selection != want {
		t.Fatalf("secondary release ended the drag: dragging=%v active=%v selection=%+v want %+v",
			st.dragging, st.pointerActive, ed.Selection, want)
	}

	// The gesture is still live: dragging further keeps extending the selection.
	st.handlePointer(mouseEvent(pointer.Drag, pointer.ButtonPrimary, 260, textY))
	if ed.Selection.Head.Col <= want.Head.Col {
		t.Fatalf("drag after the secondary release did not extend: %+v", ed.Selection)
	}

	// Releasing the primary button ends it.
	st.handlePointer(mouseEvent(pointer.Release, 0, 260, textY))
	if st.dragging || st.pointerActive {
		t.Fatalf("primary release did not end the drag: dragging=%v active=%v", st.dragging, st.pointerActive)
	}
}

func TestSecondaryButtonReleaseDoesNotCommitTabDrag(t *testing.T) {
	st, _, _ := testAppWithText("", "Plain Text")
	st.tabBarHeight = 28
	st.newTab()
	st.barTabIdxs = []int{0, 1}
	st.tabBar.ActiveIdx = 0
	firstWidth := st.tabWidth(st.tabBar.Tabs[0].Title)
	first := st.tabBar.Tabs[0]

	st.handlePointer(mouseEvent(pointer.Press, pointer.ButtonPrimary, 10, 5))
	st.handlePointer(mouseEvent(pointer.Drag, pointer.ButtonPrimary, firstWidth*2, 5))
	if !st.tabDrag.active || !st.tabDrag.started || st.tabDrag.dropTargetIdx != 1 {
		t.Fatalf("tab drag did not start: %+v", st.tabDrag)
	}

	st.handlePointer(mouseEvent(pointer.Press, pointer.ButtonPrimary|pointer.ButtonSecondary, firstWidth*2, 5))
	st.handlePointer(mouseEvent(pointer.Release, pointer.ButtonPrimary, firstWidth*2, 5))
	if !st.tabDrag.active || st.tabBar.Tabs[0] != first {
		t.Fatalf("secondary button committed the tab drag: drag=%+v order[0]=%v", st.tabDrag, st.tabBar.Tabs[0].Title)
	}

	st.handlePointer(mouseEvent(pointer.Release, 0, firstWidth*2, 5))
	if st.tabDrag.active || st.tabBar.Tabs[1] != first {
		t.Fatalf("primary release did not commit the move: drag=%+v order=%v/%v",
			st.tabDrag, st.tabBar.Tabs[0].Title, st.tabBar.Tabs[1].Title)
	}
}
