package ui

import (
	"path/filepath"

	"github.com/kristianweb/zephyr/internal/editor"
)

// Tab represents an open editor tab.
type Tab struct {
	Editor     *editor.Editor
	Title      string
	IsUntitled bool   // true for new tabs that haven't been saved to a file
	LangLabel  string // manually selected language label
}

// TabBar manages open tabs.
type TabBar struct {
	Tabs      []*Tab
	ActiveIdx int
}

// NewTabBar creates a tab bar with no tabs.
func NewTabBar() *TabBar {
	return &TabBar{
		ActiveIdx: -1,
	}
}

// OpenFile opens a file in a new tab, or switches to it if already open.
// Returns the active editor.
func (tb *TabBar) OpenFile(path string) (*editor.Editor, error) {
	absPath, _ := filepath.Abs(path)

	// Check if already open
	for i, tab := range tb.Tabs {
		if tab.Editor.FilePath == absPath {
			tb.ActiveIdx = i
			return tab.Editor, nil
		}
	}

	// Open new
	ed, err := editor.NewEditorFromFile(absPath)
	if err != nil {
		return nil, err
	}

	tb.Tabs = append(tb.Tabs, &Tab{
		Editor: ed,
		Title:  filepath.Base(absPath),
	})
	tb.ActiveIdx = len(tb.Tabs) - 1
	return ed, nil
}

// OpenEditor adds an existing editor as a tab.
func (tb *TabBar) OpenEditor(ed *editor.Editor, title string) {
	tb.Tabs = append(tb.Tabs, &Tab{
		Editor:     ed,
		Title:      title,
		IsUntitled: ed.FilePath == "",
	})
	tb.ActiveIdx = len(tb.Tabs) - 1
}

// CloseTab closes the tab at the given index.
// Returns true if the tab was closed (not modified), false if it needs save prompt.
func (tb *TabBar) CloseTab(idx int) bool {
	if idx < 0 || idx >= len(tb.Tabs) {
		return false
	}
	if tb.Tabs[idx].Editor.Modified {
		return false // caller should prompt
	}
	tb.removeTab(idx)
	return true
}

// ForceCloseTab closes the tab without checking for modifications.
func (tb *TabBar) ForceCloseTab(idx int) {
	if idx < 0 || idx >= len(tb.Tabs) {
		return
	}
	tb.removeTab(idx)
}

func (tb *TabBar) removeTab(idx int) {
	tb.Tabs = append(tb.Tabs[:idx], tb.Tabs[idx+1:]...)
	if tb.ActiveIdx >= len(tb.Tabs) {
		tb.ActiveIdx = len(tb.Tabs) - 1
	}
}

// ActiveEditor returns the currently active editor, or nil if no tabs.
func (tb *TabBar) ActiveEditor() *editor.Editor {
	if tb.ActiveIdx < 0 || tb.ActiveIdx >= len(tb.Tabs) {
		return nil
	}
	return tb.Tabs[tb.ActiveIdx].Editor
}

// ActiveTab returns the currently active tab, or nil.
func (tb *TabBar) ActiveTab() *Tab {
	if tb.ActiveIdx < 0 || tb.ActiveIdx >= len(tb.Tabs) {
		return nil
	}
	return tb.Tabs[tb.ActiveIdx]
}

// TabCount returns the number of open tabs.
func (tb *TabBar) TabCount() int {
	return len(tb.Tabs)
}

// SwitchToTab activates the tab at the given index.
func (tb *TabBar) SwitchToTab(idx int) {
	if idx >= 0 && idx < len(tb.Tabs) {
		tb.ActiveIdx = idx
	}
}

// MoveTab moves a tab from index 'from' to index 'to', adjusting ActiveIdx.
func (tb *TabBar) MoveTab(from, to int) {
	if from == to || from < 0 || to < 0 || from >= len(tb.Tabs) || to >= len(tb.Tabs) {
		return
	}
	tab := tb.Tabs[from]
	// Remove from old position
	tb.Tabs = append(tb.Tabs[:from], tb.Tabs[from+1:]...)
	// Insert at new position
	tb.Tabs = append(tb.Tabs[:to], append([]*Tab{tab}, tb.Tabs[to:]...)...)
	// Update active index to follow the active tab
	if tb.ActiveIdx == from {
		tb.ActiveIdx = to
	} else if from < tb.ActiveIdx && to >= tb.ActiveIdx {
		tb.ActiveIdx--
	} else if from > tb.ActiveIdx && to <= tb.ActiveIdx {
		tb.ActiveIdx++
	}
}
