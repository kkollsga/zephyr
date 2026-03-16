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
	viewport    *render.Viewport
	highlighter *highlight.Highlighter
	langLabel   string
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
	tag        *bool
	langSel    *ui.LanguageSelector
	langLabelX int
	lastMaxY   int
	lastMaxX   int
	dragging     bool
	window       *app.Window
	saveAsCh     chan string // result channel from Save As dialog
	tabBarHeight int        // computed from display scale to match native titlebar
	hoverX, hoverY int     // last pointer position for hover effects
}
const editorTopPad = 10 // top margin above first line of text

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
			viewport:  render.NewViewport(),
			langLabel: detectLanguage(ed.FilePath),
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
		saveAsCh:  make(chan string, 1),
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
			st.saveAllBeforeQuit()
			os.Exit(0)

		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			st.initRenderers(gtx)
			st.processSaveAs()
			setUnsavedFlag(st.hasUnsavedChanges())
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

	st.cursorRend = render.NewCursorRenderer(
		st.theme.Cursor,
		st.textRend.CharWidth,
		st.textRend.LineHeightPx,
	)

	// Match the native macOS titlebar height (28dp) plus a little padding.
	// On Retina (2×) this is ~76px; on 1× it's ~38px.
	st.tabBarHeight = gtx.Dp(32)
}

func (st *appState) handleEvents(gtx layout.Context, w *app.Window) {
	areaStack := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, st.tag)
	key.InputHintOp{Tag: st.tag, Hint: key.HintAny}.Add(gtx.Ops)
	areaStack.Pop()
	gtx.Source.Execute(key.FocusCmd{Tag: st.tag})

	for {
		ev, ok := gtx.Source.Event(
			key.FocusFilter{Target: st.tag},
			key.Filter{Focus: st.tag, Optional: key.ModShortcut | key.ModShift},
			key.Filter{Focus: st.tag, Name: key.NameTab},
			key.Filter{Focus: st.tag, Name: key.NameTab, Optional: key.ModShift},
			pointer.Filter{Target: st.tag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Scroll},
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
		if ed.FilePath == "" {
			go st.saveAsDialog()
		} else {
			if err := ed.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "save error: %v\n", err)
			}
			st.updateWindowTitle()
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
		os.Exit(0)
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
		if ts := st.activeTabState(); ts != nil {
			ts.viewport.ScrollBy(int(pe.Scroll.Y / 3))
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

// saveAllBeforeQuit saves each modified tab. Files with a path are saved
// directly; untitled files get a Save As dialog. If the user cancels any
// Save As dialog that file is skipped.
func (st *appState) saveAllBeforeQuit() {
	for _, tab := range st.tabBar.Tabs {
		if !tab.Editor.Modified {
			continue
		}
		if tab.Editor.FilePath != "" {
			_ = tab.Editor.Save()
		} else {
			// Show a synchronous Save As dialog for this tab.
			defaultName := tab.Title
			if defaultName == "" || defaultName == "Untitled" {
				defaultName = "Untitled.txt"
			}
			script := fmt.Sprintf(
				`set filePath to POSIX path of (choose file name with prompt "Save \"%s\" as" default name %q)`+"\n"+`return filePath`,
				tab.Title, defaultName,
			)
			out, err := exec.Command("osascript", "-e", script).Output()
			if err != nil {
				continue // user cancelled — skip this file
			}
			path := strings.TrimSpace(string(out))
			if path != "" {
				_ = tab.Editor.SaveAs(path)
			}
		}
	}
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
// shows a Save/Discard/Cancel prompt via osascript.
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

// promptAndCloseTab shows a native Save/Discard/Cancel dialog.  Runs in a
// goroutine because osascript blocks.
func (st *appState) promptAndCloseTab(idx int) {
	title := "Untitled"
	if idx < len(st.tabBar.Tabs) {
		title = st.tabBar.Tabs[idx].Title
	}
	script := fmt.Sprintf(
		`display dialog "Do you want to save changes to \"%s\"?" buttons {"Cancel", "Discard", "Save"} default button "Save" with icon caution`,
		title,
	)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		// User pressed Cancel or closed the dialog.
		return
	}
	result := strings.TrimSpace(string(out))
	switch {
	case strings.Contains(result, "Save"):
		if idx < len(st.tabBar.Tabs) {
			tab := st.tabBar.Tabs[idx]
			if tab.Editor.FilePath != "" {
				_ = tab.Editor.Save()
			} else {
				// No path yet — trigger Save As, then close when done.
				go st.saveAsDialog()
				return
			}
		}
		st.forceCloseTab(idx)
		st.window.Invalidate()
	case strings.Contains(result, "Discard"):
		st.forceCloseTab(idx)
		st.window.Invalidate()
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

func (st *appState) saveAsDialog() {
	// Suggest filename based on active language
	defaultName := "Untitled"
	ts := st.activeTabState()
	if ts != nil && ts.langLabel != "" && ts.langLabel != "Plain Text" {
		defaultName += langToExtension(ts.langLabel)
	} else {
		defaultName += ".txt"
	}

	script := fmt.Sprintf(`set filePath to POSIX path of (choose file name with prompt "Save As" default name %q)
return filePath`, defaultName)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return
	}
	path := strings.TrimSpace(string(out))
	if path != "" {
		st.saveAsCh <- path
		st.window.Invalidate()
	}
}

func (st *appState) processSaveAs() {
	select {
	case path := <-st.saveAsCh:
		ed := st.activeEd()
		if ed == nil {
			return
		}
		if err := ed.SaveAs(path); err != nil {
			fmt.Fprintf(os.Stderr, "save error: %v\n", err)
			return
		}

		// Update tab title
		tab := st.tabBar.ActiveTab()
		if tab != nil {
			tab.Title = filepath.Base(path)
		}

		// Init highlighter for the new file extension
		ts := st.activeTabState()
		if ts != nil {
			ts.langLabel = detectLanguage(path)
			h := highlight.NewHighlighter(path)
			if h != nil {
				if ts.highlighter != nil {
					ts.highlighter.Close()
				}
				ts.highlighter = h
				ts.highlighter.Parse([]byte(ed.Buffer.Text()))
				ts.langLabel = ts.highlighter.Language()
			}
		}

		st.updateWindowTitle()
	default:
	}
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

	// Calculate tab positions
	tabX := trafficLightPadding
	for i, tab := range st.tabBar.Tabs {
		title := tab.Title
		tabW := (len(title)+4)*tr.CharWidth + 20 + 12 // title + padding + close button + right pad
		if x >= tabX && x < tabX+tabW {
			// Check if click is on the close button area
			closeX := tabX + tabW - 34
			if x >= closeX {
				st.closeTabAt(i)
			} else {
				st.switchTab(i)
			}
			return
		}
		tabX += tabW
	}

	// Check if click is on the "+" button
	plusX := tabX + 10
	plusW := 36
	if x >= plusX && x < plusX+plusW {
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
	ts.viewport.TotalLines = ed.Buffer.LineCount()
	if st.textRend.LineHeightPx > 0 {
		statusH := 0
		if st.statusRend != nil {
			statusH = st.statusRend.LineHeightPx + 6
		}
		ts.viewport.VisibleLines = (gtx.Constraints.Max.Y - statusH - st.tabBarHeight - editorTopPad) / st.textRend.LineHeightPx
	}
	ts.viewport.ScrollToRevealCursor(ed.Cursor.Line)

	// Offset everything below the tab bar
	tabOff := op.Offset(image.Pt(0, st.tabBarHeight)).Push(gtx.Ops)

	// Gutter
	firstLine, lastLine := ts.viewport.VisibleRange()
	gutterWidth := st.gutterRend.RenderGutter(gtx, gtx.Ops, firstLine, lastLine, ts.viewport.TotalLines, editorTopPad)

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
		y := (i-firstLine)*st.textRend.LineHeightPx + editorTopPad

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

	// Offset cursor and selection by top padding
	padOff := op.Offset(image.Pt(0, editorTopPad)).Push(gtx.Ops)

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

	tabX := trafficLightPadding
	textY := (st.tabBarHeight - tr.LineHeightPx) / 2

	tabPadLeft := tr.CharWidth + 4 // left padding inside each tab

	// Hover detection — is the pointer in the tab bar area?
	inTabBar := st.hoverY >= 0 && st.hoverY < st.tabBarHeight

	closeHoverColor := color.NRGBA{R: 230, G: 60, B: 60, A: 255}   // bright red
	plusHoverColor := color.NRGBA{R: 60, G: 130, B: 230, A: 255}    // blue

	for i, tab := range st.tabBar.Tabs {
		title := tab.Title
		tabW := (len(title)+4)*tr.CharWidth + 20 + 12 // extra right padding after close btn

		// Active tab background with rounded top corners.
		if i == st.tabBar.ActiveIdx {
			radius := 6
			activeRect := clip.UniformRRect(image.Rectangle{
				Min: image.Pt(tabX, 0),
				Max: image.Pt(tabX+tabW, st.tabBarHeight+radius),
			}, radius).Push(gtx.Ops)
			paint.ColorOp{Color: activeBg}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			activeRect.Pop()
		}

		// Tab title
		fg := dimFg
		if i == st.tabBar.ActiveIdx {
			fg = hoverFg
		}
		tr.RenderGlyphs(gtx.Ops, gtx, title, tabX+tabPadLeft, textY, fg)

		// Close button / modified indicator — inset from tab right edge
		closeX := tabX + tabW - 28
		closeY := st.tabBarHeight / 2
		closeHitLeft := tabX + tabW - 34
		closeHovered := inTabBar && st.hoverX >= closeHitLeft && st.hoverX < tabX+tabW-8

		if tab.Editor.Modified {
			dotColor := modifiedColor
			if closeHovered {
				dotColor = closeHoverColor
			}
			r := 4
			dotRect := clip.Rect{
				Min: image.Pt(closeX-r+4, closeY-r),
				Max: image.Pt(closeX+r+4, closeY+r),
			}.Push(gtx.Ops)
			paint.ColorOp{Color: dotColor}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			dotRect.Pop()
		} else {
			xFg := closeColor
			if closeHovered {
				xFg = closeHoverColor
			}
			tr.RenderGlyphs(gtx.Ops, gtx, "x", closeX, textY, xFg)
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

	// "+" button — slightly larger, with spacing from last tab
	tabX += 10
	plusW := 36
	plusHovered := inTabBar && st.hoverX >= tabX && st.hoverX < tabX+plusW
	plusFg := color.NRGBA{R: 170, G: 170, B: 170, A: 255}
	if plusHovered {
		plusFg = plusHoverColor
	}
	plusY := (st.tabBarHeight - st.textRend.LineHeightPx) / 2
	st.textRend.RenderGlyphs(gtx.Ops, gtx, "+", tabX+(plusW-st.textRend.CharWidth)/2, plusY, plusFg)
	plusEndX := tabX + plusW

	// App title and subtitle (right of "+" if space allows)
	titleX := plusEndX + 20
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
