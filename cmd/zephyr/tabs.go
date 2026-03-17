package main

import (
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/highlight"
)

func (st *appState) tabMetrics() tabLayout {
	if st.dp == nil {
		return tabLayout{8, 2, 10, 6, 2, 28, 16}
	}
	return tabLayout{
		leftPad:  st.dp(8),
		innerGap: st.dp(2),
		closeW:   st.dp(10),
		rightPad: st.dp(6),
		tabGap:   st.dp(2),
		plusW:     st.dp(28),
		titleGap: st.dp(16),
	}
}

// tabWidth computes the pixel width of a tab given its title.
// Layout: [leftPad] title [innerGap] closeBtn [rightPad]
func (st *appState) tabWidth(title string) int {
	tr := st.tabRend
	if tr == nil {
		return 0
	}
	m := st.tabMetrics()
	return m.leftPad + len(title)*tr.CharWidth + m.innerGap + m.closeW + m.rightPad
}

func (st *appState) newTab() {
	ed := editor.NewEmptyEditor()
	st.tabBar.OpenEditor(ed, "Untitled")
	st.activeTabState() // init tab state
	st.updateWindowTitle()
}

func (st *appState) switchTab(idx int) {
	st.tabBar.SwitchToTab(idx)
	st.activeTabState() // ensure state exists
	st.updateWindowTitle()
}

func (st *appState) handleTabBarClick(x int) {
	tr := st.tabRend
	if tr == nil {
		return
	}

	m := st.tabMetrics()

	// Calculate tab positions
	tabX := st.trafficLightPx
	for i, tab := range st.tabBar.Tabs {
		tabW := st.tabWidth(tab.Title)
		if x >= tabX && x < tabX+tabW {
			// Check if click is on the close button area
			if x >= tabX+tabW-m.closeW-m.rightPad {
				st.closeTabAt(i)
			} else {
				st.switchTab(i)
			}
			return
		}
		tabX += tabW
	}

	// Check if click is on the "+" button
	plusX := tabX + m.tabGap
	if x >= plusX && x < plusX+m.plusW {
		st.newTab()
	}
}

func (st *appState) closeCurrentTab() {
	idx := st.tabBar.ActiveIdx
	if idx < 0 {
		return
	}
	st.closeTabAt(idx)
}

// closeTabAt closes the tab at idx. If the tab has unsaved changes it
// shows a Save/Save As/Discard/Cancel prompt via osascript.
func (st *appState) closeTabAt(idx int) {
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return
	}
	tab := st.tabBar.Tabs[idx]
	if tab.Editor.Modified {
		go st.promptAndCloseTab(idx)
		return
	}
	st.forceCloseTab(idx)
}

// forceCloseTab tears down tab state and removes the tab.
func (st *appState) forceCloseTab(idx int) {
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return
	}
	tab := st.tabBar.Tabs[idx]
	if ts, ok := st.tabStates[tab.Editor]; ok {
		if ts.highlighter != nil {
			ts.highlighter.Close()
		}
		delete(st.tabStates, tab.Editor)
	}
	st.tabBar.ForceCloseTab(idx)
	if st.tabBar.TabCount() == 0 {
		st.newTab()
	}
	st.updateWindowTitle()
}

// promptAndCloseTab shows the unified save dialog and closes the tab
// based on the user's choice. Runs in a goroutine because osascript blocks.
func (st *appState) promptAndCloseTab(idx int) {
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return
	}
	tab := st.tabBar.Tabs[idx]
	result := st.promptSaveDialog(tab.Title)
	switch result {
	case saveResultCancel:
		return
	case saveResultDiscard:
		st.forceCloseTab(idx)
		st.window.Invalidate()
	case saveResultSave:
		if st.saveTab(tab) {
			st.forceCloseTab(idx)
			st.window.Invalidate()
		}
	}
}

func langToExtension(lang string) string {
	return highlight.ExtensionForLanguage(lang)
}
