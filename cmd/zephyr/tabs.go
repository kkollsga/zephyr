package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/highlight"
	"github.com/kristianweb/zephyr/internal/ipc"
)

// maxTabTitleChars is the maximum visible characters for the filename stem.
// The extension (up to 3 chars + dot) is appended separately.
const maxTabTitleChars = 14

func (st *appState) tabMetrics() tabLayout {
	if st.dp == nil {
		return tabLayout{8, 6, 10, 6, 2, 28, 16}
	}
	return tabLayout{
		leftPad:  st.dp(8),
		innerGap: st.dp(6),
		closeW:   st.dp(10),
		rightPad: st.dp(6),
		tabGap:   st.dp(2),
		plusW:     st.dp(28),
		titleGap: st.dp(16),
	}
}

// clipTabTitle truncates a tab title to maxTabTitleChars for the stem,
// preserving the extension (up to ".xxx"). E.g.:
//
//	"very_long_filename.go" → "very_long_file….go"
//	"short.md" → "short.md" (unchanged)
func clipTabTitle(title string) string {
	ext := filepath.Ext(title)
	stem := title[:len(title)-len(ext)]

	// Clamp extension to dot + 3 chars max
	extRunes := []rune(ext)
	if len(extRunes) > 4 {
		extRunes = extRunes[:4]
		ext = string(extRunes)
	}

	stemRunes := []rune(stem)
	if len(stemRunes) <= maxTabTitleChars {
		return string(stemRunes) + ext
	}
	return string(stemRunes[:maxTabTitleChars]) + "\u2026" + ext
}

// tabWidth computes the pixel width of a tab given its title.
// Layout: [leftPad] title [innerGap] closeBtn [rightPad]
func (st *appState) tabWidth(title string) int {
	tr := st.tabRend
	if tr == nil {
		return 0
	}
	m := st.tabMetrics()
	display := clipTabTitle(title)
	return m.leftPad + utf8.RuneCountInString(display)*tr.CharWidth + m.innerGap + m.closeW + m.rightPad
}

func (st *appState) newTab() {
	ed := editor.NewEmptyEditor()
	st.tabBar.OpenEditor(ed, "Untitled")
	st.activeTabState() // init tab state
	st.updateWindowTitle()
}

func (st *appState) switchTab(idx int) {
	// Evict highlight resources from the old tab to save memory.
	if ts := st.activeTabState(); ts != nil {
		if ts.highlighter != nil {
			ts.highlighter.Evict()
			ts.sourceBuf = nil
		}
	}

	st.tabBar.SwitchToTab(idx)
	ts := st.activeTabState() // ensure state exists

	// Force viewport sync for the new tab
	if ts != nil {
		ts.lastCursorLine = -1
		ts.lastCursorCol = -1
	}

	// Reparse the new tab if its tree was evicted.
	if ts != nil && ts.highlighter != nil && ts.highlighter.NeedsParse() {
		if ed := st.activeEd(); ed != nil {
			ts.sourceBuf = ed.Buffer.TextBytes(ts.sourceBuf)
			ts.highlighter.Parse(ts.sourceBuf)
		}
	}
	st.updateWindowTitle()
}

// handleTabBarPress handles a pointer press in the tab bar.
// Close button and "+" fire immediately; tab body press starts a potential drag.
func (st *appState) handleTabBarPress(x, y int) {
	tr := st.tabRend
	if tr == nil {
		return
	}

	// If overflow dropdown is open, check presses inside it first.
	if st.overflowOpen {
		if st.handleOverflowDropdownPress(x, y) {
			return // press was inside dropdown, drag started
		}
	}

	m := st.tabMetrics()

	// Check overflow ">" button
	if len(st.dropdownTabIdxs) > 0 {
		if x >= st.overflowBtnX && x < st.overflowBtnX+st.overflowBtnW {
			st.overflowOpen = !st.overflowOpen
			return
		}
	}

	// Check bar tabs using barTabIdxs
	tabX := st.trafficLightPx
	for _, ti := range st.barTabIdxs {
		tab := st.tabBar.Tabs[ti]
		tabW := st.tabWidth(tab.Title)
		if x >= tabX && x < tabX+tabW {
			// Close button fires immediately
			if x >= tabX+tabW-m.closeW-m.rightPad {
				st.closeTabAt(ti)
				st.overflowOpen = false
				return
			}
			// Start potential drag (keep dropdown open during drag)
			st.tabDrag.active = true
			st.tabDrag.tabIdx = ti
			st.tabDrag.startX = x
			st.tabDrag.startY = y
			st.tabDrag.currentX = x
			st.tabDrag.currentY = y
			st.tabDrag.started = false
			st.tabDrag.fromDropdown = false
			st.tabDrag.dropTargetIdx = ti
			st.tabDrag.dropInBar = true
			st.tabDrag.dropSlot = 0
			return
		}
		tabX += tabW
	}

	// Check "+" button
	plusX := st.plusBtnX()
	if x >= plusX && x < plusX+m.plusW {
		st.newTab()
		st.overflowOpen = false
		return
	}

	// Check theme toggle icon (upper-right corner)
	if st.lastMaxX > 0 {
		toggleX := st.themeToggleX(st.lastMaxX)
		_, hitW := st.themeToggleSize()
		if x >= toggleX && x < toggleX+hitW {
			st.toggleTheme()
			return
		}
	}

	// Click on empty tab bar space → close dropdown and start native window drag
	st.overflowOpen = false
	if y >= 0 && y < st.tabBarHeight {
		startWindowDrag()
	}
}

// handleTabBarDrag handles pointer drag while a tab drag is active.
// Computes visual drop targets in both the bar and dropdown regions
// so drawTabBar can render the Chrome-style drag animation.
func (st *appState) handleTabBarDrag(x, y int) {
	if !st.tabDrag.active {
		return
	}
	st.tabDrag.currentX = x
	st.tabDrag.currentY = y

	dx := x - st.tabDrag.startX
	dy := y - st.tabDrag.startY

	// Start dragging once threshold is exceeded
	if !st.tabDrag.started {
		if dx > 5 || dx < -5 || dy > 5 || dy < -5 {
			st.tabDrag.started = true
		} else {
			return
		}
	}

	// Detect drag outside window → write IPC offer for cross-instance transfer
	if pointerOutsideWindow() {
		idx := st.tabDrag.tabIdx
		st.tabDrag.active = false
		st.tabDrag.started = false
		go st.offerTabTransfer(idx)
		return
	}

	dragIdx := st.tabDrag.tabIdx

	// Check if cursor is in the dropdown items region (below header)
	inDropdown := false
	if len(st.dropdownTabIdxs) > 0 && st.tabRend != nil {
		itemH := st.tabRend.LineHeightPx + 8
		ddVisCount := 0
		for _, ti := range st.dropdownTabIdxs {
			if ti != dragIdx {
				ddVisCount++
			}
		}
		headerItems := 0
		if st.dropdownHeader >= 0 {
			headerItems = 1
		}
		dropdownH := (ddVisCount + 1) * itemH // +1 for potential gap
		dropdownW := st.overflowDropdownWidth()
		dropdownX := st.overflowBtnX + st.overflowBtnW - dropdownW
		if dropdownX < 0 {
			dropdownX = 0
		}
		itemsY := st.tabBarHeight + headerItems*itemH
		if x >= dropdownX && x <= dropdownX+dropdownW && y >= itemsY && y < itemsY+dropdownH {
			inDropdown = true
		}
	}

	// Auto-show/hide dropdown during drag based on cursor position.
	// Open when cursor passes the right edge of the last bar tab.
	// Close only when cursor moves left of the last bar tab's LEFT edge
	// (hysteresis of one tab width prevents flickering at the boundary).
	if len(st.dropdownTabIdxs) > 0 {
		lastBarRight := st.trafficLightPx
		lastBarTabLeft := st.trafficLightPx
		for _, ti := range st.barTabIdxs {
			if ti != dragIdx {
				w := st.tabWidth(st.tabBar.Tabs[ti].Title)
				lastBarTabLeft = lastBarRight
				lastBarRight += w
			}
		}
		if x >= lastBarRight || inDropdown {
			st.overflowOpen = true
		} else if x < lastBarTabLeft {
			st.overflowOpen = false
		}
		// Between lastBarTabLeft and lastBarRight: keep current state
	}

	if inDropdown {
		// Compute drop position in dropdown (vertical midpoints)
		itemH := st.tabRend.LineHeightPx + 8
		headerItems := 0
		if st.dropdownHeader >= 0 {
			headerItems = 1
		}
		dropdownY := st.tabBarHeight + headerItems*itemH

		barSlots := 0
		for _, ti := range st.barTabIdxs {
			if ti != dragIdx {
				barSlots++
			}
		}

		slot := 0
		visualTarget := barSlots // default: start of dropdown
		for _, ti := range st.dropdownTabIdxs {
			if ti == dragIdx {
				continue
			}
			itemY := dropdownY + slot*itemH
			mid := itemY + itemH/2
			if y > mid {
				visualTarget = barSlots + slot + 1
			}
			slot++
		}

		st.tabDrag.dropTargetIdx = st.visualSlotToFlat(visualTarget, dragIdx)
		st.tabDrag.dropInBar = false
		st.tabDrag.dropSlot = visualTarget - barSlots
	} else {
		// Compute drop position in bar (horizontal midpoints)
		tabX := st.trafficLightPx
		slot := 0
		target := 0
		for _, ti := range st.barTabIdxs {
			if ti == dragIdx {
				continue
			}
			w := st.tabWidth(st.tabBar.Tabs[ti].Title)
			mid := tabX + w/2
			if x > mid {
				target = slot + 1
			}
			tabX += w
			slot++
		}

		st.tabDrag.dropTargetIdx = st.visualSlotToFlat(target, dragIdx)
		st.tabDrag.dropInBar = true
		st.tabDrag.dropSlot = target
	}
}

// visualSlotToFlat converts a visual slot (position in the combined
// barTabIdxs + dropdownTabIdxs order, excluding the dragged tab) to the
// flat index used by MoveTab.
func (st *appState) visualSlotToFlat(visualSlot, dragIdx int) int {
	var vo []int
	for _, ti := range st.barTabIdxs {
		if ti != dragIdx {
			vo = append(vo, ti)
		}
	}
	for _, ti := range st.dropdownTabIdxs {
		if ti != dragIdx {
			vo = append(vo, ti)
		}
	}

	if visualSlot >= len(vo) {
		return len(st.tabBar.Tabs) - 1
	}

	ti := vo[visualSlot]
	if ti > dragIdx {
		return ti - 1
	}
	return ti
}

// handleTabBarRelease handles pointer release after a tab bar press.
func (st *appState) handleTabBarRelease(x, y int) {
	if !st.tabDrag.active {
		return
	}

	if !st.tabDrag.started {
		// Was a click, not a drag — switch to the tab
		st.switchTab(st.tabDrag.tabIdx)
	} else {
		// Commit the move
		from := st.tabDrag.tabIdx
		to := st.tabDrag.dropTargetIdx
		if to != from {
			st.tabBar.MoveTab(from, to)
		}
	}

	st.tabDrag.active = false
	st.tabDrag.started = false
	st.tabDrag.fromDropdown = false
	st.overflowOpen = false
}

// plusBtnX returns the left X of the "+" button, accounting for overflow.
func (st *appState) plusBtnX() int {
	m := st.tabMetrics()
	tabX := st.trafficLightPx
	for _, ti := range st.barTabIdxs {
		tabX += st.tabWidth(st.tabBar.Tabs[ti].Title)
	}
	if len(st.dropdownTabIdxs) > 0 {
		return st.overflowBtnX + st.overflowBtnW + m.tabGap
	}
	return tabX + m.tabGap
}

// computeOverflow determines which tabs overflow the available width.
// Builds barTabIdxs and dropdownTabIdxs. If the active tab is in the
// overflow region it is swapped into the last bar slot.
func (st *appState) computeOverflow(maxWidth int) {
	m := st.tabMetrics()
	overflowBtnW := m.plusW
	st.overflowBtnW = overflowBtnW

	n := len(st.tabBar.Tabs)
	reservedRight := m.tabGap + m.plusW
	reservedWithOverflow := m.tabGap + overflowBtnW + m.tabGap + m.plusW

	availW := maxWidth - st.trafficLightPx - reservedRight
	availWOverflow := maxWidth - st.trafficLightPx - reservedWithOverflow

	// Check if all tabs fit without overflow button
	totalW := 0
	for _, tab := range st.tabBar.Tabs {
		totalW += st.tabWidth(tab.Title)
	}
	if totalW <= availW {
		st.overflowStartIdx = n
		st.barTabIdxs = st.barTabIdxs[:0]
		for i := 0; i < n; i++ {
			st.barTabIdxs = append(st.barTabIdxs, i)
		}
		st.dropdownTabIdxs = st.dropdownTabIdxs[:0]
		st.dropdownHeader = -1
		return
	}

	// Find natural overflow point
	cumW := 0
	overflow := n
	for i, tab := range st.tabBar.Tabs {
		w := st.tabWidth(tab.Title)
		if cumW+w > availWOverflow {
			overflow = i
			break
		}
		cumW += w
	}
	if overflow < 1 {
		overflow = 1
	}
	st.overflowStartIdx = overflow

	// Build display lists
	activeIdx := st.tabBar.ActiveIdx
	activeInOverflow := activeIdx >= overflow

	st.barTabIdxs = st.barTabIdxs[:0]
	st.dropdownTabIdxs = st.dropdownTabIdxs[:0]

	if !activeInOverflow {
		for i := 0; i < overflow; i++ {
			st.barTabIdxs = append(st.barTabIdxs, i)
		}
		for i := overflow; i < n; i++ {
			st.dropdownTabIdxs = append(st.dropdownTabIdxs, i)
		}
		st.dropdownHeader = overflow - 1
	} else {
		// Active tab goes in bar, displacing the last natural bar tab.
		// Shrink the bar if the active tab is wider than the displaced tab.
		activeW := st.tabWidth(st.tabBar.Tabs[activeIdx].Title)
		for overflow > 1 {
			barW := 0
			for i := 0; i < overflow-1; i++ {
				barW += st.tabWidth(st.tabBar.Tabs[i].Title)
			}
			if barW+activeW <= availWOverflow {
				break
			}
			overflow--
		}
		st.overflowStartIdx = overflow

		for i := 0; i < overflow-1; i++ {
			st.barTabIdxs = append(st.barTabIdxs, i)
		}
		st.barTabIdxs = append(st.barTabIdxs, activeIdx)

		// Displaced tab first in dropdown, then remaining overflow tabs
		st.dropdownTabIdxs = append(st.dropdownTabIdxs, overflow-1)
		for i := overflow; i < n; i++ {
			if i != activeIdx {
				st.dropdownTabIdxs = append(st.dropdownTabIdxs, i)
			}
		}
		st.dropdownHeader = -1
	}
}

// handleOverflowDropdownPress handles a press inside the overflow dropdown.
// Starts a potential drag (same as bar tabs). Returns true if the press was consumed.
func (st *appState) handleOverflowDropdownPress(x, y int) bool {
	if st.tabRend == nil || len(st.dropdownTabIdxs) == 0 {
		return false
	}
	itemH := st.tabRend.LineHeightPx + 8
	hasHeader := st.dropdownHeader >= 0
	headerItems := 0
	if hasHeader {
		headerItems = 1
	}
	count := len(st.dropdownTabIdxs) + headerItems
	dropdownW := st.overflowDropdownWidth()
	dropdownH := count * itemH

	dropdownX := st.overflowBtnX + st.overflowBtnW - dropdownW
	if dropdownX < 0 {
		dropdownX = 0
	}
	dropdownY := st.tabBarHeight

	if x < dropdownX || x > dropdownX+dropdownW || y < dropdownY || y >= dropdownY+dropdownH {
		return false
	}
	idx := (y - dropdownY) / itemH

	// Header item — switch to that tab (it's already in the bar)
	if hasHeader && idx == 0 {
		st.switchTab(st.dropdownHeader)
		st.overflowOpen = false
		return true
	}

	ddIdx := idx - headerItems
	if ddIdx < 0 || ddIdx >= len(st.dropdownTabIdxs) {
		return false
	}

	tabIdx := st.dropdownTabIdxs[ddIdx]

	// Start potential drag
	st.tabDrag.active = true
	st.tabDrag.tabIdx = tabIdx
	st.tabDrag.startX = x
	st.tabDrag.startY = y
	st.tabDrag.currentX = x
	st.tabDrag.currentY = y
	st.tabDrag.started = false
	st.tabDrag.fromDropdown = true
	st.tabDrag.dropTargetIdx = tabIdx
	st.tabDrag.dropInBar = false
	st.tabDrag.dropSlot = 0
	return true
}

// overflowDropdownWidth computes the width of the overflow dropdown.
func (st *appState) overflowDropdownWidth() int {
	tr := st.tabRend
	if tr == nil {
		return 0
	}
	maxW := 0
	if st.dropdownHeader >= 0 && st.dropdownHeader < len(st.tabBar.Tabs) {
		w := utf8.RuneCountInString(st.tabBar.Tabs[st.dropdownHeader].Title) * tr.CharWidth
		if w > maxW {
			maxW = w
		}
	}
	for _, ti := range st.dropdownTabIdxs {
		w := utf8.RuneCountInString(st.tabBar.Tabs[ti].Title) * tr.CharWidth
		if w > maxW {
			maxW = w
		}
	}
	return maxW + 24
}

func (st *appState) closeCurrentTab() {
	idx := st.tabBar.ActiveIdx
	if idx < 0 {
		return
	}
	st.closeTabAt(idx)
}

// closeTabAt closes the tab at idx. If the tab has unsaved changes it
// shows the in-app save prompt dropdown.
func (st *appState) closeTabAt(idx int) {
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return
	}
	tab := st.tabBar.Tabs[idx]
	if tab.Editor.Modified {
		st.showSaveMenu(idx, true, false)
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
	st.unwatchEditorFile(tab.Editor)
	if ts, ok := st.tabStates[tab.Editor]; ok {
		if ts.highlighter != nil {
			ts.highlighter.Close()
		}
		delete(st.tabStates, tab.Editor)
	}
	st.tabBar.ForceCloseTab(idx)
	if st.tabBar.TabCount() == 0 {
		os.Exit(0)
	}
	st.updateWindowTitle()
}

// updateUntitledTitle sets a dynamic title for untitled tabs based on content.
// Shows "Untitled" until 8+ non-whitespace characters are typed on the first line,
// then shows the first 8 characters followed by "…".
func (st *appState) updateUntitledTitle() {
	tab := st.tabBar.ActiveTab()
	if tab == nil || !tab.IsUntitled {
		return
	}
	firstLine, err := tab.Editor.Buffer.Line(0)
	if err != nil {
		return
	}
	trimmed := strings.TrimSpace(firstLine)
	if utf8.RuneCountInString(trimmed) < 8 {
		tab.Title = "Untitled"
	} else {
		runes := []rune(trimmed)
		tab.Title = string(runes[:8]) + "\u2026"
	}
	st.updateWindowTitle()
}

// afterEdit performs common post-edit bookkeeping: schedules a debounced
// reparse and updates the untitled tab title.
func (st *appState) afterEdit() {
	st.reparsePending = true
	st.reparseDeadline = time.Now().Add(50 * time.Millisecond)
	st.updateUntitledTitle()
}

// flushReparse fires the pending reparse if the deadline has passed.
// Called once per frame before draw.
func (st *appState) flushReparse() {
	if st.reparsePending && !time.Now().Before(st.reparseDeadline) {
		st.reparsePending = false
		st.reparseHighlight()
	}
}

// offerTabTransfer writes an IPC offer for the tab and waits for another
// instance to claim it. If nobody claims within 500ms, falls back to
// spawning a new instance. Runs in a goroutine.
func (st *appState) offerTabTransfer(idx int) {
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return
	}
	tab := st.tabBar.Tabs[idx]

	// Write content to temp file for the offer
	tmpFile, err := os.CreateTemp("", "zephyr-transfer-*.txt")
	if err != nil {
		st.detachTabToNewInstance(idx)
		return
	}
	tab.Editor.Buffer.WriteTo(tmpFile)
	tmpFile.Close()

	ts := st.tabStates[tab.Editor]
	lang := ""
	if ts != nil {
		lang = ts.langLabel
	}

	offer := ipc.TabTransfer{
		ContentFile: tmpFile.Name(),
		Title:       tab.Title,
		Language:    lang,
		FilePath:    tab.Editor.FilePath,
		Modified:    tab.Editor.Modified,
	}
	if err := ipc.WriteOffer(offer); err != nil {
		os.Remove(tmpFile.Name())
		st.detachTabToNewInstance(idx)
		return
	}

	// Wait up to 500ms for another instance to claim the offer
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ipc.WasClaimed() {
			// Another instance took the tab — close it here
			st.forceCloseTab(idx)
			st.window.Invalidate()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Nobody claimed — clean up offer.
	ipc.CleanupOffer()
	// Don't spawn a new instance if this is the last tab (pointless restart).
	if len(st.tabBar.Tabs) <= 1 {
		st.window.Invalidate()
		return
	}
	st.detachTabToNewInstance(idx)
}

// checkIncomingTabTransfer checks for a pending IPC tab offer and imports it.
// Called when pointer activity is detected in the tab bar.
func (st *appState) checkIncomingTabTransfer() bool {
	offer := ipc.ClaimOffer()
	if offer == nil {
		return false
	}

	// Load the content from the temp file
	var ed *editor.Editor
	if offer.FilePath != "" {
		// File-backed tab — re-open from original path
		var err error
		ed, err = editor.NewEditorFromFile(offer.FilePath)
		if err != nil {
			// Fall back to temp content
			ed = st.loadEditorFromTemp(offer.ContentFile)
		} else {
			os.Remove(offer.ContentFile)
		}
	} else {
		ed = st.loadEditorFromTemp(offer.ContentFile)
	}
	if ed == nil {
		return false
	}

	title := offer.Title
	if title == "" {
		title = "Untitled"
	}
	st.tabBar.OpenEditor(ed, title)
	st.activeTabState()
	st.updateWindowTitle()
	return true
}

func (st *appState) loadEditorFromTemp(path string) *editor.Editor {
	ed, err := editor.NewEditorFromFile(path)
	if err != nil {
		return nil
	}
	ed.FilePath = "" // treat as untitled
	os.Remove(path)
	return ed
}

// detachTabToNewInstance removes a tab and opens it in a new Zephyr process.
func (st *appState) detachTabToNewInstance(idx int) {
	if idx < 0 || idx >= len(st.tabBar.Tabs) {
		return
	}
	tab := st.tabBar.Tabs[idx]

	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "detach: cannot find executable: %v\n", err)
		return
	}

	var cmd *exec.Cmd
	if tab.Editor.FilePath != "" {
		// File-backed tab: just open the file in a new instance
		cmd = exec.Command(exePath, tab.Editor.FilePath)
	} else {
		// Untitled tab: write content to a temp file
		tmpFile, err := os.CreateTemp("", "zephyr-tab-*.txt")
		if err != nil {
			fmt.Fprintf(os.Stderr, "detach: cannot create temp file: %v\n", err)
			return
		}
		if _, err := tab.Editor.Buffer.WriteTo(tmpFile); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			fmt.Fprintf(os.Stderr, "detach: write error: %v\n", err)
			return
		}
		tmpFile.Close()
		cmd = exec.Command(exePath, "--temp", tmpFile.Name(), "--title", tab.Title)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "detach: cannot start new instance: %v\n", err)
		return
	}

	// Close the tab in this instance
	st.forceCloseTab(idx)
}

func langToExtension(lang string) string {
	return highlight.ExtensionForLanguage(lang)
}
