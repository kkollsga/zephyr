package main

import (
	"image"
	"path/filepath"
	"strconv"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// fuzzyMaxRows caps the visible result rows; the finder itself caps the result
// list at 100.
const fuzzyMaxRows = 12

// fuzzyLayout is the overlay's geometry for one frame. Draw and click
// hit-testing both derive it from the same function so a click always lands on
// the row it looks like it lands on.
type fuzzyLayout struct {
	x, y, w, h int
	itemH      int
	rows       int // result rows actually shown
	first      int // index of the first result row shown
	listY      int // y of the first result row, relative to the window
}

// fuzzyFinderLayout centres a panel of nResults rows in the window, scrolled so
// that the selected result is one of them.
func fuzzyFinderLayout(maxX, maxY, tabBarHeight, itemH, nResults, selected int) fuzzyLayout {
	rows := nResults
	if rows > fuzzyMaxRows {
		rows = fuzzyMaxRows
	}
	first := 0
	if selected >= rows {
		first = selected - rows + 1
	}
	w := maxX * 2 / 3
	if w < 280 {
		w = 280
	}
	if w > maxX-16 {
		w = maxX - 16
	}
	// Query line, the rows, and the match count.
	h := itemH*(rows+2) + 8
	x := (maxX - w) / 2
	if x < 0 {
		x = 0
	}
	y := tabBarHeight + (maxY-tabBarHeight-h)/3
	if y < tabBarHeight+4 {
		y = tabBarHeight + 4
	}
	return fuzzyLayout{x: x, y: y, w: w, h: h, itemH: itemH, rows: rows, first: first, listY: y + itemH + 4}
}

// overlayVisible reports whether another overlay owns the keyboard, in which
// case the fuzzy finder neither opens nor draws — one overlay at a time.
func (st *appState) overlayVisible() bool {
	return st.saveMenu.visible || st.langSel.Visible || st.findBarHasKeys() ||
		st.navRootDropdown.open
}

// openFuzzyFinder opens the file finder over the navigator root, or over the
// repository's changed files when changedOnly is set.
func (st *appState) openFuzzyFinder(changedOnly bool) {
	if st.fuzzyFinder == nil || st.overlayVisible() {
		return
	}
	if st.navRoot == "" {
		st.detectNavRoot()
	}
	if changedOnly {
		if st.gitRepo == nil || st.gitCache == nil {
			st.notify("No git repository for this folder", 5*time.Second)
			return
		}
		statuses, err := st.gitCache.Status()
		if err != nil || len(statuses) == 0 {
			st.notify("No changed files", 5*time.Second)
			return
		}
		paths := make([]string, 0, len(statuses))
		for _, s := range statuses {
			paths = append(paths, s.Path)
		}
		st.fuzzyFinder.OpenChanged(st.gitRepo.Root, paths)
	} else {
		if st.navRoot == "" {
			st.notify("No project folder", 5*time.Second)
			return
		}
		st.fuzzyFinder.Open(st.navRoot)
	}
	st.invalidate()
}

// handleFuzzyFinderKey routes one key press to the open finder and reports
// whether it consumed the event.
func (st *appState) handleFuzzyFinderKey(ke key.Event) bool {
	if st.fuzzyFinder == nil || !st.fuzzyFinder.Visible {
		return false
	}
	ctrl := ke.Modifiers&key.ModCtrl != 0
	switch {
	case ke.Name == key.NameEscape:
		st.fuzzyFinder.Close()
	case ke.Name == key.NameUpArrow, ctrl && ke.Name == "P":
		st.fuzzyFinder.MoveUp()
	case ke.Name == key.NameDownArrow, ctrl && ke.Name == "N":
		st.fuzzyFinder.MoveDown()
	case ke.Name == key.NameReturn:
		st.openFuzzySelection()
	case ke.Name == key.NameDeleteBackward:
		st.fuzzyFinder.BackspaceQuery()
	}
	st.invalidate()
	return true
}

// fuzzyFinderInsert appends typed text to the query.
func (st *appState) fuzzyFinderInsert(text string) {
	if st.fuzzyFinder == nil || !st.fuzzyFinder.Visible {
		return
	}
	st.fuzzyFinder.UpdateQuery(st.fuzzyFinder.Query + text)
	st.invalidate()
}

// openFuzzySelection accepts the highlighted row and closes the finder: the
// picker's own callback when it has one, otherwise the file finder's default of
// opening the path as a tab.
func (st *appState) openFuzzySelection() {
	if accept := st.fuzzyFinder.OnAccept; accept != nil {
		item := st.fuzzyFinder.SelectedItem()
		st.fuzzyFinder.Close()
		if item != "" {
			accept(item)
		}
		return
	}
	path := st.fuzzyFinder.SelectedPath()
	st.fuzzyFinder.Close()
	if path == "" {
		return
	}
	if _, err := st.openFileInTab(path); err != nil {
		st.notify("Could not open "+filepath.Base(path), 5*time.Second)
		return
	}
	st.activeTabState()
	st.updateWindowTitle()
}

// handleFuzzyFinderClick opens the row under the pointer, or closes the finder
// when the click lands outside it. Reports whether it consumed the click.
func (st *appState) handleFuzzyFinderClick(px, py int) bool {
	if st.fuzzyFinder == nil || !st.fuzzyFinder.Visible {
		return false
	}
	lay := st.fuzzyLayoutNow()
	if px < lay.x || px >= lay.x+lay.w || py < lay.y || py >= lay.y+lay.h {
		st.fuzzyFinder.Close()
		st.invalidate()
		return true
	}
	if row := (py - lay.listY) / lay.itemH; py >= lay.listY && row < lay.rows {
		st.fuzzyFinder.Selected = lay.first + row
		st.openFuzzySelection()
	}
	st.invalidate()
	return true
}

// fuzzyLayoutNow recomputes the overlay geometry from the last frame's size.
func (st *appState) fuzzyLayoutNow() fuzzyLayout {
	itemH := 20
	if st.statusRend != nil {
		itemH = st.statusRend.LineHeightPx + 4
	}
	ff := st.fuzzyFinder
	return fuzzyFinderLayout(st.lastMaxX, st.lastMaxY, st.tabBarHeight, itemH, len(ff.Results), ff.Selected)
}

func (st *appState) drawFuzzyFinder(gtx layout.Context) {
	sr := st.statusRend
	if sr == nil || st.fuzzyFinder == nil {
		return
	}
	ff := st.fuzzyFinder
	// One drain per frame: a scan that landed since the last frame becomes
	// visible here, and the geometry a click is tested against is the geometry
	// this frame drew.
	ff.Sync()
	itemH := sr.LineHeightPx + 4
	lay := fuzzyFinderLayout(gtx.Constraints.Max.X, gtx.Constraints.Max.Y, st.tabBarHeight, itemH, len(ff.Results), ff.Selected)

	off := op.Offset(image.Pt(lay.x, lay.y)).Push(gtx.Ops)
	border := clip.Rect{Max: image.Pt(lay.w, lay.h)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.TabBorder}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	border.Pop()
	// Two fills: the theme's dropdown colour carries alpha, and the editor text
	// showing through a floating panel makes both unreadable.
	bg := clip.Rect{Min: image.Pt(1, 1), Max: image.Pt(lay.w-1, lay.h-1)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.Background}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	paint.ColorOp{Color: st.theme.DropdownBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bg.Pop()

	prompt := ff.Prompt()
	sr.RenderGlyphs(gtx.Ops, gtx, prompt, sr.CharWidth, 2, st.theme.Foreground)

	for i := 0; i < lay.rows; i++ {
		iy := itemH + 4 + i*itemH
		if lay.first+i == ff.Selected {
			selRect := clip.Rect{
				Min: image.Pt(1, iy),
				Max: image.Pt(lay.w-1, iy+itemH),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: st.theme.DropdownSel}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			selRect.Pop()
		}
		text := truncateTailRunes(ff.Results[lay.first+i].Text, lay.w/sr.CharWidth-2)
		sr.RenderGlyphs(gtx.Ops, gtx, text, sr.CharWidth, iy+2, st.theme.Foreground)
	}

	count := strconv.Itoa(len(ff.Results)) + " of " + strconv.Itoa(len(ff.Files))
	switch {
	case ff.Scanning() && len(ff.Files) == 0:
		count = "scanning…"
	case ff.Scanning():
		count = count + " — scanning…"
	case len(ff.Results) == 0:
		count = "no matches"
	}
	sr.RenderGlyphs(gtx.Ops, gtx, count, sr.CharWidth, lay.h-itemH, st.theme.SubtitleFg)
	off.Pop()
}

// truncateTailRunes keeps the last maxChars characters of text. Characters,
// not bytes: a path trimmed mid-rune draws as a replacement glyph.
func truncateTailRunes(text string, maxChars int) string {
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	r := []rune(text)
	if len(r) <= maxChars {
		return text
	}
	return string(r[len(r)-maxChars:])
}
