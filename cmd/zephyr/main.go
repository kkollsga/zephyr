package main

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"image/color"

	"github.com/kristianweb/zephyr/internal/config"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/highlight"
	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/ui"
	"github.com/kristianweb/zephyr/pkg/clipboard"
)

func main() {
	setupTitlebar()
	go run()
	app.Main()
}

// tabState holds per-tab state that isn't part of the editor itself.
type tabState struct {
	viewport       *render.Viewport
	highlighter    *highlight.Highlighter
	langLabel      string
	lastCursorLine int // tracks cursor to detect movement; -1 = uninitialized
	lastCursorCol  int
}

type appState struct {
	tabBar    *ui.TabBar
	tabStates map[*editor.Editor]*tabState
	theme     config.Theme
	shaper    *text.Shaper
	textRend  *render.TextRenderer
	gutterRend *render.GutterRenderer
	cursorRend *render.CursorRenderer
	colorMap   highlight.TokenColorMap
	statusRend *render.TextRenderer
	tabRend    *render.TextRenderer // font for tab bar
	plusRend   *render.TextRenderer // larger font for "+" button
	tag        *bool
	langSel    *ui.LanguageSelector
	langLabelX int
	lastMaxY   int
	lastMaxX   int
	dragging       bool
	quitInProgress bool
	scrollAccum    float32 // accumulated fractional scroll delta
	window         *app.Window

	tabBarHeight   int      // computed from display scale to match native titlebar
	trafficLightPx int     // traffic light padding in pixels (scaled from Dp)
	hoverX, hoverY int     // last pointer position for hover effects
	dp             func(v unit.Dp) int // cached scale function from latest gtx
}
const editorTopPad = 10 // top margin above first line of text

// tabLayout holds scaled pixel values for tab bar layout.
type tabLayout struct {
	leftPad  int // space before title text
	innerGap int // space between title and close button
	closeW   int // close button / dot area width
	rightPad int // space after close button to tab edge
	tabGap   int // space between tab edge and "+" button
	plusW    int // "+" button width
	titleGap int // space before app title text
}

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

func (st *appState) activeEd() *editor.Editor {
	return st.tabBar.ActiveEditor()
}

func (st *appState) activeTabState() *tabState {
	ed := st.activeEd()
	if ed == nil {
		return nil
	}
	ts, ok := st.tabStates[ed]
	if !ok {
		ts = &tabState{
			viewport:       render.NewViewport(),
			langLabel:      detectLanguage(ed.FilePath),
			lastCursorLine: -1,
		}
		// Init highlighter
		if ed.FilePath != "" {
			ts.highlighter = highlight.NewHighlighter(ed.FilePath)
			if ts.highlighter != nil {
				ts.highlighter.Parse([]byte(ed.Buffer.Text()))
				ts.langLabel = ts.highlighter.Language()
			}
		}
		st.tabStates[ed] = ts
	}
	return ts
}

func run() {
	tabBar := ui.NewTabBar()

	if len(os.Args) > 1 {
		path, _ := filepath.Abs(os.Args[1])
		_, err := tabBar.OpenFile(path)
		if err != nil {
			ed := editor.NewEmptyEditor()
			tabBar.OpenEditor(ed, "Untitled")
		}
	} else {
		ed := editor.NewEmptyEditor()
		tabBar.OpenEditor(ed, "Untitled")
	}

	theme := config.DarkTheme()
	w := &app.Window{}

	st := &appState{
		tabBar:    tabBar,
		tabStates: make(map[*editor.Editor]*tabState),
		theme:     theme,
		colorMap:  render.TokenColorMap(theme),
		tag:       new(bool),
		langSel:   ui.NewLanguageSelector(),
		window:    w,
	}

	// Init tab state for first tab
	st.activeTabState()

	st.updateWindowTitle()
	w.Option(
		app.Decorated(false),
		app.Size(unit.Dp(900), unit.Dp(600)),
		app.MinSize(unit.Dp(400), unit.Dp(300)),
	)

	var ops op.Ops

	for {
		evt := w.Event()
		switch e := evt.(type) {
		case app.DestroyEvent:
			if st.saveAllBeforeQuit() {
				os.Exit(0)
			}
			// User cancelled — don't exit. Re-create the window.
			w = new(app.Window)
			w.Option(
				app.Decorated(false),
				app.Title("Zephyr"),
				app.Size(unit.Dp(1024), unit.Dp(768)),
				app.MinSize(unit.Dp(400), unit.Dp(300)),
			)
			st.window = w
			setupTitlebar()

		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			st.initRenderers(gtx)
			setUnsavedFlag(st.hasUnsavedChanges())
			if closeRequested() && !st.quitInProgress {
				st.quitInProgress = true
				go func() {
					if st.saveAllBeforeQuit() {
						os.Exit(0)
					}
					st.quitInProgress = false
					st.window.Invalidate()
				}()
			}
			st.handleEvents(gtx, w)
			st.draw(gtx, w)
			e.Frame(gtx.Ops)
		}
	}
}

func (st *appState) updateWindowTitle() {
	tab := st.tabBar.ActiveTab()
	if tab != nil {
		title := tab.Title
		if tab.Editor.Modified {
			title += " •"
		}
		title += " — Zephyr"
		st.window.Option(app.Title(title))
	} else {
		st.window.Option(app.Title("Zephyr — The caffeinated editor"))
	}
}

func (st *appState) initRenderers(gtx layout.Context) {
	if st.shaper != nil {
		return
	}
	st.shaper = text.NewShaper()
	st.textRend = render.NewTextRenderer(st.shaper, render.TextStyle{
		FontSize:   13,
		LineHeight: 1.4,
		Foreground: st.theme.Foreground,
	})
	st.textRend.ComputeMetrics(gtx)

	st.gutterRend = &render.GutterRenderer{
		Shaper:     st.shaper,
		FontSize:   11,
		FgColor:    st.theme.Gutter,
		BgColor:    st.theme.GutterBg,
		CharWidth:  st.textRend.CharWidth,
		LineHeight: st.textRend.LineHeightPx,
	}

	st.statusRend = render.NewTextRenderer(st.shaper, render.TextStyle{
		FontSize:   11,
		LineHeight: 1.4,
		Foreground: st.theme.StatusFg,
	})
	st.statusRend.ComputeMetrics(gtx)

	st.tabRend = render.NewTextRenderer(st.shaper, render.TextStyle{
		FontSize:   11,
		LineHeight: 1.3,
		Foreground: st.theme.Foreground,
	})
	st.tabRend.ComputeMetrics(gtx)

	st.plusRend = render.NewTextRenderer(st.shaper, render.TextStyle{
		FontSize:   18,
		LineHeight: 1.0,
		Foreground: st.theme.Foreground,
	})
	st.plusRend.ComputeMetrics(gtx)

	st.cursorRend = render.NewCursorRenderer(
		st.theme.Cursor,
		st.textRend.CharWidth,
		st.textRend.LineHeightPx,
	)

	// Match the native macOS titlebar height (28dp) plus a little padding.
	// On Retina (2×) this is ~76px; on 1× it's ~38px.
	st.dp = gtx.Dp
	st.tabBarHeight = gtx.Dp(32)
	st.trafficLightPx = gtx.Dp(trafficLightPaddingDp)
}

func (st *appState) handleEvents(gtx layout.Context, w *app.Window) {
	areaStack := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, st.tag)
	key.InputHintOp{Tag: st.tag, Hint: key.HintAny}.Add(gtx.Ops)
	areaStack.Pop()
	gtx.Source.Execute(key.FocusCmd{Tag: st.tag})

	// Compute dynamic scroll range based on viewport position.
	scrollRange := pointer.ScrollRange{Min: -10000, Max: 10000}
	if ts := st.activeTabState(); ts != nil && st.textRend != nil && st.textRend.LineHeightPx > 0 {
		up, down := ts.viewport.ScrollablePixels(st.textRend.LineHeightPx)
		scrollRange = pointer.ScrollRange{Min: -up, Max: down}
	}

	for {
		ev, ok := gtx.Source.Event(
			key.FocusFilter{Target: st.tag},
			key.Filter{Focus: st.tag, Optional: key.ModShortcut | key.ModShift},
			key.Filter{Focus: st.tag, Name: key.NameTab},
			key.Filter{Focus: st.tag, Name: key.NameTab, Optional: key.ModShift},
			pointer.Filter{Target: st.tag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Scroll | pointer.Move, ScrollY: scrollRange},
		)
		if !ok {
			break
		}
		switch ke := ev.(type) {
		case key.Event:
			if ke.State == key.Press {
				st.handleKey(ke)
			}
		case key.EditEvent:
			if st.langSel.Visible {
				break
			}
			st.handleTextInput(ke.Text)
		case pointer.Event:
			st.handlePointer(ke)
		}
	}
}

func (st *appState) handleKey(ke key.Event) {
	if st.langSel.Visible {
		switch ke.Name {
		case key.NameEscape:
			st.langSel.Close()
		case key.NameUpArrow:
			st.langSel.MoveUp()
		case key.NameDownArrow:
			st.langSel.MoveDown()
		case key.NameReturn:
			lang := st.langSel.SelectedLanguage()
			st.langSel.Close()
			st.setLanguage(lang)
		}
		return
	}

	ed := st.activeEd()
	if ed == nil {
		// Only handle new tab if no editor
		if ke.Name == "T" && ke.Modifiers == key.ModShortcut {
			st.newTab()
		}
		return
	}

	switch {
	// Tab management
	case ke.Name == "T" && ke.Modifiers == key.ModShortcut:
		st.newTab()
	case ke.Name == "W" && ke.Modifiers == key.ModShortcut:
		st.closeCurrentTab()

	case ke.Name == key.NameLeftArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveLeft(ed.Buffer)
	case ke.Name == key.NameRightArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveRight(ed.Buffer)
	case ke.Name == key.NameUpArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveUp(ed.Buffer)
	case ke.Name == key.NameDownArrow && ke.Modifiers == 0:
		ed.Selection.Clear()
		ed.Cursor.MoveDown(ed.Buffer)
	case ke.Name == key.NameUpArrow && ke.Modifiers == key.ModShortcut:
		ed.Selection.Clear()
		ed.Cursor.MoveToFileStart()
	case ke.Name == key.NameDownArrow && ke.Modifiers == key.ModShortcut:
		ed.Selection.Clear()
		ed.Cursor.MoveToFileEnd(ed.Buffer)
	case ke.Name == key.NameHome:
		ed.Selection.Clear()
		ed.Cursor.MoveToLineStart()
	case ke.Name == key.NameEnd:
		ed.Selection.Clear()
		ed.Cursor.MoveToLineEnd(ed.Buffer)
	case ke.Name == key.NamePageDown:
		ed.Selection.Clear()
		ed.Cursor.PageDown(ed.Buffer, st.activeTabState().viewport.VisibleLines)
	case ke.Name == key.NamePageUp:
		ed.Selection.Clear()
		ed.Cursor.PageUp(ed.Buffer, st.activeTabState().viewport.VisibleLines)
	case ke.Name == key.NameDeleteBackward && ke.Modifiers == 0:
		if st.deleteAutoPair() {
			st.reparseHighlight()
		} else if st.softTabBackspace() {
			st.reparseHighlight()
		} else {
			ed.DeleteBackward()
			st.reparseHighlight()
		}
	case ke.Name == key.NameDeleteForward && ke.Modifiers == 0:
		ed.DeleteForward()
		st.reparseHighlight()
	case ke.Name == key.NameReturn && ke.Modifiers == 0:
		indent := st.computeAutoIndent()
		ed.InsertText("\n" + indent)
		st.reparseHighlight()
	case ke.Name == key.NameTab && ke.Modifiers == 0:
		ed.InsertText("    ")
		st.reparseHighlight()
	case ke.Name == "Z" && ke.Modifiers == key.ModShortcut:
		ed.Undo()
		st.reparseHighlight()
	case ke.Name == "Z" && ke.Modifiers == key.ModShortcut|key.ModShift:
		ed.Redo()
		st.reparseHighlight()
	case ke.Name == "S" && ke.Modifiers == key.ModShortcut:
		tab := st.tabBar.ActiveTab()
		if tab != nil {
			if tab.Editor.FilePath == "" {
				go func() {
					st.saveTabAs(tab)
					st.updateWindowTitle()
					st.window.Invalidate()
				}()
			} else {
				st.saveTab(tab)
				st.updateWindowTitle()
			}
		}
	case ke.Name == "S" && ke.Modifiers == key.ModShortcut|key.ModShift:
		// Cmd+Shift+S = Save As
		tab := st.tabBar.ActiveTab()
		if tab != nil {
			go func() {
				st.saveTabAs(tab)
				st.updateWindowTitle()
				st.window.Invalidate()
			}()
		}
	case ke.Name == "A" && ke.Modifiers == key.ModShortcut:
		ed.Selection.SelectAll(ed.Buffer)
		_, end := ed.Selection.Ordered()
		ed.Cursor = end
		ed.Cursor.PreferredCol = -1
	case ke.Name == "C" && ke.Modifiers == key.ModShortcut:
		if text := ed.SelectedText(); text != "" {
			clipboard.Set(text)
		}
	case ke.Name == "X" && ke.Modifiers == key.ModShortcut:
		if text := ed.SelectedText(); text != "" {
			clipboard.Set(text)
			ed.DeleteSelection()
			st.reparseHighlight()
		}
	case ke.Name == "V" && ke.Modifiers == key.ModShortcut:
		if text := clipboard.Get(); text != "" {
			ed.InsertText(text)
			st.reparseHighlight()
		}
	case ke.Name == "Q" && ke.Modifiers == key.ModShortcut:
		if !st.quitInProgress {
			st.quitInProgress = true
			go func() {
				if st.saveAllBeforeQuit() {
					os.Exit(0)
				}
				st.quitInProgress = false
				st.window.Invalidate()
			}()
		}
	// Selection via shift+arrows
	case ke.Name == key.NameLeftArrow && ke.Modifiers == key.ModShift:
		if !ed.Selection.Active {
			ed.Selection.Start(ed.Cursor)
		}
		ed.Cursor.MoveLeft(ed.Buffer)
		ed.Selection.Update(ed.Cursor)
	case ke.Name == key.NameRightArrow && ke.Modifiers == key.ModShift:
		if !ed.Selection.Active {
			ed.Selection.Start(ed.Cursor)
		}
		ed.Cursor.MoveRight(ed.Buffer)
		ed.Selection.Update(ed.Cursor)
	case ke.Name == key.NameUpArrow && ke.Modifiers == key.ModShift:
		if !ed.Selection.Active {
			ed.Selection.Start(ed.Cursor)
		}
		ed.Cursor.MoveUp(ed.Buffer)
		ed.Selection.Update(ed.Cursor)
	case ke.Name == key.NameDownArrow && ke.Modifiers == key.ModShift:
		if !ed.Selection.Active {
			ed.Selection.Start(ed.Cursor)
		}
		ed.Cursor.MoveDown(ed.Buffer)
		ed.Selection.Update(ed.Cursor)
	}
	if st.cursorRend != nil {
		st.cursorRend.ResetBlink()
	}
}

func (st *appState) handlePointer(pe pointer.Event) {
	st.hoverX = int(pe.Position.X)
	st.hoverY = int(pe.Position.Y)

	switch pe.Kind {
	case pointer.Press:
		// Check tab bar clicks first
		if int(pe.Position.Y) < st.tabBarHeight {
			st.handleTabBarClick(int(pe.Position.X))
			return
		}

		sr := st.statusRend
		statusH := 0
		if sr != nil {
			statusH = sr.LineHeightPx + 6
		}
		statusY := st.lastMaxY - statusH

		if st.langSel.Visible && sr != nil {
			itemH := sr.LineHeightPx + 4
			dropdownH := len(st.langSel.Languages) * itemH
			dropdownW := st.langDropdownWidth()
			dropdownX := st.lastMaxX - dropdownW - 4
			dropdownY := statusY - dropdownH
			if dropdownX < 0 {
				dropdownX = 0
			}
			px, py := int(pe.Position.X), int(pe.Position.Y)
			if px >= dropdownX && px <= dropdownX+dropdownW && py >= dropdownY && py < statusY {
				idx := st.langSel.LanguageAtY(py-dropdownY, itemH)
				if idx >= 0 {
					st.langSel.Selected = idx
					lang := st.langSel.SelectedLanguage()
					st.langSel.Close()
					st.setLanguage(lang)
				}
				return
			}
			st.langSel.Close()
			return
		}

		if int(pe.Position.Y) >= statusY && int(pe.Position.X) >= st.langLabelX {
			st.langSel.Open(highlight.LanguageNames())
			return
		}

		ed := st.activeEd()
		if ed == nil {
			return
		}
		ts := st.activeTabState()

		gutterWidth := st.gutterRend.Width(ts.viewport.TotalLines)
		if int(pe.Position.X) < gutterWidth {
			return
		}
		line, col := st.pointerToLineCol(pe.Position)

		ed.Selection.Clear()
		ed.Cursor.SetPosition(ed.Buffer, line, col)
		ed.Selection.Start(ed.Cursor)
		st.dragging = true
		st.cursorRend.ResetBlink()

	case pointer.Drag:
		if !st.dragging {
			return
		}
		ed := st.activeEd()
		if ed == nil {
			return
		}
		line, col := st.pointerToLineCol(pe.Position)
		ed.Cursor.SetPosition(ed.Buffer, line, col)
		ed.Selection.Update(ed.Cursor)
		st.cursorRend.ResetBlink()

	case pointer.Release:
		if st.dragging {
			st.dragging = false
			if ed := st.activeEd(); ed != nil && ed.Selection.IsEmpty() {
				ed.Selection.Clear()
			}
		}

	case pointer.Scroll:
		if ts := st.activeTabState(); ts != nil && st.textRend != nil && st.textRend.LineHeightPx > 0 {
			st.scrollAccum += pe.Scroll.Y
			pixels := int(st.scrollAccum)
			if pixels != 0 {
				ts.viewport.ScrollByPixels(pixels, st.textRend.LineHeightPx)
				st.scrollAccum -= float32(pixels)
			}
		}
	}
}

func (st *appState) pointerToLineCol(pos f32.Point) (line, col int) {
	ts := st.activeTabState()
	if ts == nil {
		return 0, 0
	}
	gutterWidth := st.gutterRend.Width(ts.viewport.TotalLines)
	col = (int(pos.X) - gutterWidth - st.textRend.CharWidth) / st.textRend.CharWidth
	if col < 0 {
		col = 0
	}
	adjustedY := int(pos.Y) - st.tabBarHeight - editorTopPad
	line = ts.viewport.FirstLine + adjustedY/st.textRend.LineHeightPx
	return
}

// ---- Tab management ----

func (st *appState) newTab() {
	ed := editor.NewEmptyEditor()
	st.tabBar.OpenEditor(ed, "Untitled")
	st.activeTabState() // init tab state
	st.updateWindowTitle()
}

// saveAllBeforeQuit prompts the user for each unsaved tab.
// Returns true if the quit should proceed, false if the user cancelled.
func (st *appState) saveAllBeforeQuit() bool {
	for _, tab := range st.tabBar.Tabs {
		if !tab.Editor.Modified {
			continue
		}
		result := st.promptSaveDialog(tab.Title)
		switch result {
		case saveResultCancel:
			return false
		case saveResultDiscard:
			continue
		case saveResultSave:
			if !st.saveTab(tab) {
				return false
			}
		}
	}
	return true
}

// --- Save dialog types ---

type saveResult int

const (
	saveResultCancel  saveResult = iota
	saveResultDiscard
	saveResultSave
)

// promptSaveDialog shows a native "Do you want to save?" dialog.
// Buttons: Cancel (Escape) / Discard / Save
// For untitled files, "Save" triggers a Save As file picker.
func (st *appState) promptSaveDialog(title string) saveResult {
	script := fmt.Sprintf(
		`display dialog "Do you want to save changes to \"%s\"?" buttons {"Discard", "Cancel", "Save"} default button "Save" cancel button "Cancel" with icon caution`,
		title,
	)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return saveResultCancel
	}
	result := strings.TrimSpace(string(out))
	switch {
	case strings.Contains(result, "Save"):
		return saveResultSave
	case strings.Contains(result, "Discard"):
		return saveResultDiscard
	default:
		return saveResultCancel
	}
}

// saveTab saves a tab to its existing path. Returns false if the file is
// untitled (caller should use saveTabAs instead).
func (st *appState) saveTab(tab *ui.Tab) bool {
	if tab.Editor.FilePath == "" {
		return st.saveTabAs(tab)
	}
	if err := tab.Editor.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "save error: %v\n", err)
		return false
	}
	return true
}

// saveTabAs shows a native Save As file dialog. Returns false if the user
// cancelled.
func (st *appState) saveTabAs(tab *ui.Tab) bool {
	defaultName := tab.Title
	if defaultName == "" || defaultName == "Untitled" {
		ts := st.tabStates[tab.Editor]
		if ts != nil && ts.langLabel != "" && ts.langLabel != "Plain Text" {
			defaultName = "Untitled" + langToExtension(ts.langLabel)
		} else {
			defaultName = "Untitled.txt"
		}
	}

	script := fmt.Sprintf(`set filePath to POSIX path of (choose file name with prompt "Save As" default name %q)
return filePath`, defaultName)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return false
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return false
	}
	if err := tab.Editor.SaveAs(path); err != nil {
		fmt.Fprintf(os.Stderr, "save error: %v\n", err)
		return false
	}
	tab.Title = filepath.Base(path)

	// Update highlighter for new extension
	ts := st.tabStates[tab.Editor]
	if ts != nil {
		ts.langLabel = detectLanguage(path)
		h := highlight.NewHighlighter(path)
		if h != nil {
			if ts.highlighter != nil {
				ts.highlighter.Close()
			}
			ts.highlighter = h
			h.Parse([]byte(tab.Editor.Buffer.Text()))
		}
	}
	return true
}

func (st *appState) hasUnsavedChanges() bool {
	for _, tab := range st.tabBar.Tabs {
		if tab.Editor.Modified {
			return true
		}
	}
	return false
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
	switch lang {
	case "Go":
		return ".go"
	case "Python":
		return ".py"
	case "JavaScript":
		return ".js"
	case "TypeScript":
		return ".ts"
	case "Rust":
		return ".rs"
	case "C":
		return ".c"
	case "C++":
		return ".cpp"
	case "Java":
		return ".java"
	case "Ruby":
		return ".rb"
	case "Lua":
		return ".lua"
	case "Markdown":
		return ".md"
	case "JSON":
		return ".json"
	case "YAML":
		return ".yaml"
	case "TOML":
		return ".toml"
	case "HTML":
		return ".html"
	case "CSS":
		return ".css"
	case "Shell":
		return ".sh"
	default:
		return ".txt"
	}
}

// processSaveAs is kept for compatibility but no longer used.
// Save-as is now handled synchronously by saveTabAs.

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

// ---- Drawing ----

func (st *appState) draw(gtx layout.Context, w *app.Window) {
	ed := st.activeEd()
	ts := st.activeTabState()

	// Background
	paint.ColorOp{Color: st.theme.Background}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	// Tab bar
	st.drawTabBar(gtx)

	if ed == nil || ts == nil {
		st.lastMaxY = gtx.Constraints.Max.Y
		st.lastMaxX = gtx.Constraints.Max.X
		gtx.Execute(op.InvalidateCmd{})
		return
	}

	// Update viewport
	statusH := 0
	if st.statusRend != nil {
		statusH = st.statusRend.LineHeightPx + 6
	}
	ts.viewport.TotalLines = ed.Buffer.LineCount()
	if st.textRend.LineHeightPx > 0 {
		ts.viewport.VisibleLines = (gtx.Constraints.Max.Y - statusH - st.tabBarHeight - editorTopPad) / st.textRend.LineHeightPx
	}
	// Only scroll to reveal cursor when it has actually moved (not during
	// trackpad/mouse-wheel scrolling, which should move the viewport freely).
	if ed.Cursor.Line != ts.lastCursorLine || ed.Cursor.Col != ts.lastCursorCol {
		ts.viewport.ScrollToRevealCursor(ed.Cursor.Line)
		ts.lastCursorLine = ed.Cursor.Line
		ts.lastCursorCol = ed.Cursor.Col
	}

	// Offset everything below the tab bar and clip to the editor area.
	editorH := gtx.Constraints.Max.Y - st.tabBarHeight - statusH
	tabOff := op.Offset(image.Pt(0, st.tabBarHeight)).Push(gtx.Ops)
	editorClip := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, editorH)}.Push(gtx.Ops)

	// Gutter
	firstLine, lastLine := ts.viewport.VisibleRange()
	gutterWidth := st.gutterRend.RenderGutter(gtx, gtx.Ops, firstLine, lastLine, ts.viewport.TotalLines, editorTopPad, ts.viewport.PixelOffset)

	// Gutter right separator
	gutterSepColor := color.NRGBA{R: 50, G: 50, B: 50, A: 255}
	sepRect := clip.Rect{
		Min: image.Pt(gutterWidth-1, 0),
		Max: image.Pt(gutterWidth, gtx.Constraints.Max.Y),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: gutterSepColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	sepRect.Pop()

	// Highlight tokens
	var allTokens []highlight.Token
	if ts.highlighter != nil {
		allTokens = ts.highlighter.Tokens()
	}

	// Visible text lines
	byteOffset := 0
	for i := 0; i < firstLine && i < ts.viewport.TotalLines; i++ {
		line, _ := ed.Buffer.Line(i)
		byteOffset += len(line) + 1
	}

	for i := firstLine; i <= lastLine && i < ts.viewport.TotalLines; i++ {
		line, err := ed.Buffer.Line(i)
		if err != nil {
			continue
		}
		y := (i-firstLine)*st.textRend.LineHeightPx + editorTopPad - ts.viewport.PixelOffset

		var spans []render.ColorSpan
		if len(allTokens) > 0 {
			lineStart := byteOffset
			lineEnd := byteOffset + len(line)
			lineTokens := tokensForRange(allTokens, lineStart, lineEnd)
			spans = render.TokensToColorSpans(lineTokens, lineStart, lineEnd, line, st.colorMap, st.theme.Foreground, 4)
		}

		expandedLine := expandTabs(line, 4)
		st.textRend.RenderLine(gtx.Ops, gtx, expandedLine, gutterWidth+st.textRend.CharWidth, y, spans)
		byteOffset += len(line) + 1
	}

	// Offset cursor and selection by top padding minus scroll pixel offset
	padOff := op.Offset(image.Pt(0, editorTopPad-ts.viewport.PixelOffset)).Push(gtx.Ops)

	// Selection
	if ed.Selection.Active && !ed.Selection.IsEmpty() {
		start, end := ed.Selection.Ordered()
		st.cursorRend.RenderSelection(gtx.Ops, st.theme.Selection,
			start.Line, start.Col, end.Line, end.Col,
			firstLine, gutterWidth+st.textRend.CharWidth, gtx.Constraints.Max.Y,
			func(line int) int {
				l, _ := ed.Buffer.Line(line)
				return utf8.RuneCountInString(l)
			})
	}

	// Cursor
	if st.cursorRend.UpdateBlink() {
		w.Invalidate()
	}
	st.cursorRend.RenderCursor(gtx.Ops, ed.Cursor.Line, ed.Cursor.Col, firstLine, gutterWidth+st.textRend.CharWidth)

	padOff.Pop()

	editorClip.Pop()
	tabOff.Pop()

	// Status line
	st.lastMaxY = gtx.Constraints.Max.Y
	st.lastMaxX = gtx.Constraints.Max.X
	st.drawStatusLine(gtx)

	// Language selector dropdown
	if st.langSel.Visible {
		st.drawLangSelector(gtx)
	}

	gtx.Execute(op.InvalidateCmd{})
}

func (st *appState) drawTabBar(gtx layout.Context) {
	tr := st.tabRend
	if tr == nil {
		return
	}

	// Tab bar background
	tabBg := color.NRGBA{R: 46, G: 46, B: 46, A: 255}
	bgRect := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, st.tabBarHeight)}.Push(gtx.Ops)
	paint.ColorOp{Color: tabBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgRect.Pop()

	activeBg := color.NRGBA{R: 30, G: 30, B: 30, A: 255}
	borderColor := color.NRGBA{R: 68, G: 68, B: 68, A: 255}
	hoverFg := st.theme.Foreground
	dimFg := color.NRGBA{R: 140, G: 140, B: 140, A: 255}
	modifiedColor := color.NRGBA{R: 200, G: 180, B: 100, A: 255}
	closeColor := color.NRGBA{R: 150, G: 150, B: 150, A: 255}

	m := st.tabMetrics()

	tabX := st.trafficLightPx
	textY := (st.tabBarHeight - tr.LineHeightPx) / 2

	// Hover detection — is the pointer in the tab bar area?
	inTabBar := st.hoverY >= 0 && st.hoverY < st.tabBarHeight

	closeHoverColor := color.NRGBA{R: 230, G: 60, B: 60, A: 255}   // bright red
	plusHoverColor := color.NRGBA{R: 60, G: 130, B: 230, A: 255}    // blue
	radius := gtx.Dp(6)
	dotR := gtx.Dp(3)

	for i, tab := range st.tabBar.Tabs {
		title := tab.Title
		tabW := st.tabWidth(title)

		// Active tab background with rounded top corners.
		if i == st.tabBar.ActiveIdx {
			activeRect := clip.UniformRRect(image.Rectangle{
				Min: image.Pt(tabX, 0),
				Max: image.Pt(tabX+tabW, st.tabBarHeight+radius),
			}, radius).Push(gtx.Ops)
			paint.ColorOp{Color: activeBg}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			activeRect.Pop()
		}

		// Tab title — layout: [leftPad] title [innerGap] closeBtn [rightPad]
		fg := dimFg
		if i == st.tabBar.ActiveIdx {
			fg = hoverFg
		}
		tr.RenderGlyphs(gtx.Ops, gtx, title, tabX+m.leftPad, textY, fg)

		// Close button / modified indicator — centered in closeW area
		closeX := tabX + m.leftPad + len(title)*tr.CharWidth + m.innerGap
		closeY := st.tabBarHeight / 2
		closeHitLeft := closeX
		closeHitRight := closeX + m.closeW
		closeHovered := inTabBar && st.hoverX >= closeHitLeft && st.hoverX < closeHitRight

		if tab.Editor.Modified {
			dotColor := modifiedColor
			if closeHovered {
				dotColor = closeHoverColor
			}
			dotCx := closeX + m.closeW/2
			dotEllipse := clip.Ellipse{
				Min: image.Pt(dotCx-dotR, closeY-dotR),
				Max: image.Pt(dotCx+dotR, closeY+dotR),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: dotColor}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			dotEllipse.Pop()
		} else {
			xFg := closeColor
			if closeHovered {
				xFg = closeHoverColor
			}
			// Center the "x" glyph within closeW
			xGlyphX := closeX + (m.closeW-tr.CharWidth)/2
			tr.RenderGlyphs(gtx.Ops, gtx, "x", xGlyphX, textY, xFg)
		}

		tabX += tabW

		// Separator between tabs — same color as bottom border
		if i < len(st.tabBar.Tabs)-1 {
			vPad := st.tabBarHeight / 4
			sepRect := clip.Rect{
				Min: image.Pt(tabX-1, vPad),
				Max: image.Pt(tabX, st.tabBarHeight-vPad),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			sepRect.Pop()
		}
	}

	// "+" button
	tabX += m.tabGap
	plusHovered := inTabBar && st.hoverX >= tabX && st.hoverX < tabX+m.plusW
	plusFg := color.NRGBA{R: 170, G: 170, B: 170, A: 255}
	if plusHovered {
		plusFg = plusHoverColor
	}
	plusY := (st.tabBarHeight - st.plusRend.LineHeightPx) / 2
	st.plusRend.RenderGlyphs(gtx.Ops, gtx, "+", tabX+(m.plusW-st.plusRend.CharWidth)/2, plusY, plusFg)
	plusEndX := tabX + m.plusW

	// App title and subtitle (right of "+" if space allows)
	titleX := plusEndX + m.titleGap
	titleText := "Zephyr"
	titleW := len(titleText) * tr.CharWidth
	titleFg := color.NRGBA{R: 160, G: 160, B: 160, A: 255}
	if titleX+titleW < gtx.Constraints.Max.X-20 {
		tr.RenderGlyphs(gtx.Ops, gtx, titleText, titleX, textY, titleFg)

		subtitleText := "The caffeinated editor"
		subtitleX := titleX + titleW + tr.CharWidth
		subtitleW := len(subtitleText) * tr.CharWidth
		subtitleFg := color.NRGBA{R: 90, G: 90, B: 90, A: 255}
		if subtitleX+subtitleW < gtx.Constraints.Max.X-20 {
			tr.RenderGlyphs(gtx.Ops, gtx, subtitleText, subtitleX, textY, subtitleFg)
		}
	}

	// Bottom border — same color as tab separators
	tabBorderRect := clip.Rect{
		Min: image.Pt(0, st.tabBarHeight-1),
		Max: image.Pt(gtx.Constraints.Max.X, st.tabBarHeight),
	}.Push(gtx.Ops)
	paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	tabBorderRect.Pop()
}

func (st *appState) drawStatusLine(gtx layout.Context) {
	sr := st.statusRend
	if sr == nil || sr.LineHeightPx == 0 {
		return
	}
	ed := st.activeEd()
	ts := st.activeTabState()

	statusH := sr.LineHeightPx + 6
	y := gtx.Constraints.Max.Y - statusH

	// Top border
	statusBorderColor := color.NRGBA{R: 45, G: 45, B: 45, A: 255}
	borderOff := op.Offset(image.Pt(0, y-1)).Push(gtx.Ops)
	borderRect := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, 1)}.Push(gtx.Ops)
	paint.ColorOp{Color: statusBorderColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	borderRect.Pop()
	borderOff.Pop()

	// Background
	offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
	rect := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, statusH)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.StatusBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	rect.Pop()
	offset.Pop()

	textY := y + 3

	// line:col on left
	if ed != nil {
		status := fmt.Sprintf("%d:%d", ed.Cursor.Line+1, ed.Cursor.Col+1)
		sr.RenderGlyphs(gtx.Ops, gtx, status, 8, textY, st.theme.StatusFg)
	}

	// Language on right
	lang := ""
	if ts != nil {
		lang = ts.langLabel
	}
	if lang == "" && ed != nil {
		lang = detectLanguage(ed.FilePath)
	}
	if lang == "" {
		lang = "Plain Text"
	}
	langWidth := len(lang) * sr.CharWidth
	st.langLabelX = gtx.Constraints.Max.X - langWidth - 12
	sr.RenderGlyphs(gtx.Ops, gtx, lang, st.langLabelX, textY, st.theme.StatusFg)
}

func (st *appState) langDropdownWidth() int {
	sr := st.statusRend
	if sr == nil {
		return 100
	}
	maxLen := 0
	for _, lang := range st.langSel.Languages {
		if len(lang) > maxLen {
			maxLen = len(lang)
		}
	}
	return (maxLen + 3) * sr.CharWidth
}

func (st *appState) drawLangSelector(gtx layout.Context) {
	sr := st.statusRend
	if sr == nil {
		return
	}
	itemH := sr.LineHeightPx + 4
	count := len(st.langSel.Languages)
	dropdownW := st.langDropdownWidth()
	dropdownH := count * itemH

	statusH := sr.LineHeightPx + 6
	statusY := gtx.Constraints.Max.Y - statusH

	dropdownX := gtx.Constraints.Max.X - dropdownW - 4
	dropdownY := statusY - dropdownH
	if dropdownX < 0 {
		dropdownX = 0
	}

	bgColor := color.NRGBA{R: 37, G: 37, B: 38, A: 245}
	off := op.Offset(image.Pt(dropdownX, dropdownY)).Push(gtx.Ops)
	bgRect := clip.Rect{Max: image.Pt(dropdownW, dropdownH)}.Push(gtx.Ops)
	paint.ColorOp{Color: bgColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgRect.Pop()

	selColor := color.NRGBA{R: 4, G: 57, B: 94, A: 255}
	for i, lang := range st.langSel.Languages {
		iy := i * itemH
		if i == st.langSel.Selected {
			selRect := clip.Rect{
				Min: image.Pt(0, iy),
				Max: image.Pt(dropdownW, iy+itemH),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: selColor}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			selRect.Pop()
		}
		sr.RenderGlyphs(gtx.Ops, gtx, lang, sr.CharWidth, iy+2, st.theme.Foreground)
	}
	off.Pop()
}

func (st *appState) setLanguage(lang string) {
	ts := st.activeTabState()
	if ts == nil {
		return
	}
	ed := st.activeEd()

	if lang == "Plain Text" || lang == "" {
		if ts.highlighter != nil {
			ts.highlighter.Close()
			ts.highlighter = nil
		}
		ts.langLabel = "Plain Text"
		return
	}

	h := highlight.NewHighlighterForLanguage(lang)
	if h == nil {
		ts.langLabel = lang
		return
	}

	if ts.highlighter != nil {
		ts.highlighter.Close()
	}
	ts.highlighter = h
	if ed != nil {
		ts.highlighter.Parse([]byte(ed.Buffer.Text()))
	}
	ts.langLabel = lang
}

func detectLanguage(path string) string {
	if path == "" {
		return "Plain Text"
	}
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".js":
		return "JavaScript"
	case ".ts":
		return "TypeScript"
	case ".rs":
		return "Rust"
	case ".c", ".h":
		return "C"
	case ".cpp", ".hpp", ".cc":
		return "C++"
	case ".java":
		return "Java"
	case ".rb":
		return "Ruby"
	case ".lua":
		return "Lua"
	case ".md":
		return "Markdown"
	case ".json":
		return "JSON"
	case ".yaml", ".yml":
		return "YAML"
	case ".toml":
		return "TOML"
	case ".html", ".htm":
		return "HTML"
	case ".css":
		return "CSS"
	case ".sh", ".bash", ".zsh":
		return "Shell"
	case ".txt":
		return "Plain Text"
	default:
		return "Plain Text"
	}
}

func (st *appState) reparseHighlight() {
	ts := st.activeTabState()
	ed := st.activeEd()
	if ts != nil && ts.highlighter != nil && ed != nil {
		ts.highlighter.Parse([]byte(ed.Buffer.Text()))
	}
}

// ---- Auto-pair, indent, etc. ----

var autoPairs = map[string]string{
	"(": ")", "{": "}", "[": "]", `"`: `"`, "'": "'", "`": "`",
}

var closerSet = map[string]bool{
	")": true, "}": true, "]": true, `"`: true, "'": true, "`": true,
}

func (st *appState) handleTextInput(text string) {
	ed := st.activeEd()
	if ed == nil {
		return
	}

	if closerSet[text] {
		next := ed.RuneAfterCursor()
		if string(next) == text {
			ed.Cursor.MoveRight(ed.Buffer)
			st.reparseHighlight()
			return
		}
	}

	if closer, ok := autoPairs[text]; ok {
		if text == `"` || text == "'" || text == "`" {
			next := ed.RuneAfterCursor()
			if next != 0 && next != ' ' && next != '\t' && next != '\n' &&
				next != ')' && next != ']' && next != '}' && next != ',' && next != ';' {
				ed.InsertText(text)
				st.reparseHighlight()
				return
			}
		}
		ed.InsertText(text + closer)
		ed.Cursor.MoveLeft(ed.Buffer)
		st.reparseHighlight()
		return
	}

	ed.InsertText(text)
	if text == "}" || text == ")" || text == "]" {
		st.autoDedentClosingBracket()
	}
	st.reparseHighlight()
}

func (st *appState) deleteAutoPair() bool {
	ed := st.activeEd()
	if ed == nil || ed.Cursor.Col == 0 {
		return false
	}
	if ed.Selection.Active && !ed.Selection.IsEmpty() {
		return false
	}
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil || ed.Cursor.Col >= len([]rune(line)) {
		return false
	}
	runes := []rune(line)
	col := ed.Cursor.Col
	before := string(runes[col-1])
	after := string(runes[col])
	if closer, ok := autoPairs[before]; ok && closer == after {
		ed.DeleteForward()
		ed.DeleteBackward()
		return true
	}
	return false
}

func (st *appState) softTabBackspace() bool {
	ed := st.activeEd()
	if ed == nil {
		return false
	}
	col := ed.Cursor.Col
	if col == 0 || (ed.Selection.Active && !ed.Selection.IsEmpty()) {
		return false
	}
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return false
	}
	if col > len(line) {
		return false
	}
	prefix := line[:col]
	if strings.TrimLeft(prefix, " ") != "" {
		return false
	}
	remove := len(prefix) % 4
	if remove == 0 {
		remove = 4
	}
	ed.DeleteBackwardN(remove)
	return true
}

func (st *appState) computeAutoIndent() string {
	ed := st.activeEd()
	ts := st.activeTabState()
	if ed == nil {
		return ""
	}
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return ""
	}

	indent := ""
	for _, r := range line {
		if r == ' ' || r == '\t' {
			indent += string(r)
		} else {
			break
		}
	}

	trimmed := strings.TrimRight(line, " \t")
	if len(trimmed) == 0 {
		if len(indent) >= 4 {
			return indent[:len(indent)-4]
		}
		return ""
	}

	last := trimmed[len(trimmed)-1]
	if last == '{' || last == '(' || last == '[' {
		return indent + "    "
	}

	lang := ""
	if ts != nil {
		lang = ts.langLabel
	}
	if lang == "Python" && last == ':' {
		return indent + "    "
	}

	word := lastWord(trimmed)
	switch lang {
	case "Python":
		switch word {
		case "return", "break", "continue", "pass", "raise":
			return dedent(indent)
		}
	case "Go", "Rust", "JavaScript":
		switch word {
		case "return", "break", "continue":
			return dedent(indent)
		}
	}

	return indent
}

func dedent(indent string) string {
	if len(indent) >= 4 {
		return indent[:len(indent)-4]
	}
	return ""
}

func (st *appState) autoDedentClosingBracket() {
	ed := st.activeEd()
	if ed == nil {
		return
	}
	line, err := ed.Buffer.Line(ed.Cursor.Line)
	if err != nil {
		return
	}
	trimmed := strings.TrimLeft(line, " ")
	if len(trimmed) != 1 {
		return
	}
	indent := len(line) - len(trimmed)
	if indent < 4 {
		return
	}
	savedCol := ed.Cursor.Col
	ed.Cursor.Col = 4
	ed.Cursor.PreferredCol = -1
	ed.DeleteBackwardN(4)
	ed.Cursor.Col = savedCol - 4
	ed.Cursor.PreferredCol = -1
}

func lastWord(s string) string {
	s = strings.TrimRight(s, " \t")
	i := strings.LastIndexAny(s, " \t")
	if i >= 0 {
		return s[i+1:]
	}
	return s
}

func expandTabs(s string, tabSize int) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabSize - (col % tabSize)
			for j := 0; j < spaces; j++ {
				b.WriteByte(' ')
			}
			col += spaces
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

func tokensForRange(tokens []highlight.Token, startByte, endByte int) []highlight.Token {
	var result []highlight.Token
	for _, t := range tokens {
		if t.EndByte <= startByte {
			continue
		}
		if t.StartByte >= endByte {
			break
		}
		result = append(result, t)
	}
	return result
}
