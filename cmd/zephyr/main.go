package main

import (
	"os"
	"path/filepath"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/kristianweb/zephyr/internal/config"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/highlight"
	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/ui"
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
	sourceBuf      []byte // reusable buffer for tree-sitter source
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
	findBar      *ui.FindReplaceBar
	scrollbarRend *render.ScrollbarRenderer
	langLabelX int
	lastMaxY   int
	lastMaxX   int
	dragging        bool
	quitInProgress  bool
	scrollAccum     float32 // accumulated fractional scroll delta
	window          *app.Window
	lastWindowTitle string  // dedup title updates to avoid Configure() thrash
	darkMode        bool   // true = dark theme, false = light theme
	themeName       string              // active theme bundle name
	themeBundle     config.ThemeBundle   // loaded theme bundle
	themeMenuReady  bool                // true once native theme menu has been set up

	tabBarHeight   int      // computed from display scale to match native titlebar
	trafficLightPx int     // traffic light padding in pixels (scaled from Dp)
	hoverX, hoverY int     // last pointer position for hover effects
	dp             func(v unit.Dp) int // cached scale function from latest gtx

	// Tab overflow state
	overflowOpen         bool  // true when the overflow dropdown is visible
	overflowStartIdx     int   // first tab index that overflows (== len(Tabs) if none)
	overflowBtnX         int   // left edge X of the ">" button (for click detection)
	overflowBtnW         int   // width of the ">" button
	barTabIdxs           []int // tab indices shown in the bar (computed each frame)
	dropdownTabIdxs      []int // tab indices shown in the dropdown (computed each frame)
	dropdownHeader       int   // tab index shown as first dropdown item for continuity (-1 = none)

	// Debounced reparse state
	reparsePending  bool
	reparseDeadline time.Time

	// Footer notification (e.g. "Saved to: /path/to/file")
	notification      string
	notificationUntil time.Time

	// Graceful exit state
	exitPending  bool
	exitDeadline time.Time

	// Unified save menu state (in-app dropdown with tag, where, toggle)
	saveMenu struct {
		visible        bool
		tabIdx         int
		forQuit        bool   // continue quit flow after action
		closeAfterSave bool   // close tab after save (close-tab flow)
		saveAsExpanded bool   // true when Save As rows are shown for file-backed tabs

		// Save As fields
		filename  []rune
		cursor    int    // rune position in filename
		selectAll bool   // entire filename is selected
		dir              string // directory to save in
		tags             [7]bool // macOS Finder tags: Red, Orange, Yellow, Green, Blue, Purple, Gray
		confirmOverwrite bool   // true when waiting for overwrite confirmation
	}

	// Tab drag state
	tabDrag struct {
		active        bool // a tab press is in progress
		tabIdx        int  // index of the tab being dragged
		startX        int  // pointer X at press
		startY        int  // pointer Y at press
		currentX      int  // current pointer X
		currentY      int  // current pointer Y
		started       bool // true once drag threshold (5px) exceeded
		dropTargetIdx int  // flat MoveTab target index
		fromDropdown  bool // drag started from overflow dropdown
		dropInBar     bool // true if gap renders in bar, false if in dropdown
		dropSlot      int  // gap position within bar or dropdown section
	}
}
// macOS Finder tag names (indexed 0–6).
var finderTagNames = [7]string{"Red", "Orange", "Yellow", "Green", "Blue", "Purple", "Gray"}

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
				ts.sourceBuf = ed.Buffer.TextBytes(ts.sourceBuf)
				ts.highlighter.Parse(ts.sourceBuf)
				ts.langLabel = ts.highlighter.Language()
			}
		}
		st.tabStates[ed] = ts
	}
	return ts
}

func run() {
	tabBar := ui.NewTabBar()

	if len(os.Args) > 2 && os.Args[1] == "--temp" {
		// Load content from temp file but treat as untitled
		path, _ := filepath.Abs(os.Args[2])
		ed, err := editor.NewEditorFromFile(path)
		if err != nil {
			ed = editor.NewEmptyEditor()
		} else {
			ed.FilePath = "" // mark as untitled
			os.Remove(path)  // clean up temp file
		}
		title := "Untitled"
		for i := 3; i < len(os.Args)-1; i++ {
			if os.Args[i] == "--title" {
				title = os.Args[i+1]
				break
			}
		}
		tabBar.OpenEditor(ed, title)
	} else if len(os.Args) > 1 {
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

	// Load config and theme bundle
	config.EnsureDefaultThemes()
	cfg := config.LoadConfig()

	bundle, err := config.LoadBundleByName(cfg.Theme)
	if err != nil {
		bundle = config.DefaultBundle()
		cfg.Theme = "default"
	}
	theme := bundle.Theme(cfg.DarkMode)

	w := &app.Window{}

	st := &appState{
		tabBar:      tabBar,
		tabStates:   make(map[*editor.Editor]*tabState),
		theme:       theme,
		colorMap:    render.TokenColorMap(theme),
		tag:         new(bool),
		langSel:     ui.NewLanguageSelector(),
		findBar:     ui.NewFindReplaceBar(),
		window:      w,
		darkMode:    cfg.DarkMode,
		themeName:   cfg.Theme,
		themeBundle: bundle,
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
			if !st.hasUnsavedChanges() {
				os.Exit(0)
			}
			// Has unsaved changes — re-create window and show save prompt.
			w = new(app.Window)
			w.Option(
				app.Decorated(false),
				app.Title("Zephyr"),
				app.Size(unit.Dp(1024), unit.Dp(768)),
				app.MinSize(unit.Dp(400), unit.Dp(300)),
			)
			st.window = w
			setupTitlebar()
			st.startQuitFlow()

		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			st.initRenderers(gtx)
			setUnsavedFlag(st.hasUnsavedChanges())

			// Defer theme menu setup until the titlebar/window is ready
			if !st.themeMenuReady && titlebarReady() {
				st.initThemeMenu()
				st.themeMenuReady = true
			}

			// Check if graceful exit delay has elapsed
			if st.exitPending && !time.Now().Before(st.exitDeadline) {
				os.Exit(0)
			}

			if closeRequested() && !st.quitInProgress && !st.exitPending {
				st.startQuitFlow()
			}
			if sel := checkThemeSelection(); sel != "" && sel != st.themeName {
				st.selectThemeBundle(sel)
			}
			if !st.exitPending {
				st.handleEvents(gtx, w)
				st.flushReparse()
			}
			st.draw(gtx, w)
			e.Frame(gtx.Ops)
			ensureTrafficLights()

			// Keep requesting frames during exit countdown
			if st.exitPending {
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
}

func (st *appState) updateWindowTitle() {
	var title string
	tab := st.tabBar.ActiveTab()
	if tab != nil {
		title = tab.Title
		if tab.Editor.Modified {
			title += " •"
		}
		title += " — Zephyr"
	} else {
		title = "Zephyr — The caffeinated editor"
	}
	if title != st.lastWindowTitle {
		st.lastWindowTitle = title
		st.window.Option(app.Title(title))
	}
}

// gracefulExit shows a "Closing…" notification and exits after a short delay
// so the user sees the message and the app doesn't feel like it crashed.
func (st *appState) gracefulExit() {
	if st.exitPending {
		return
	}
	st.exitPending = true
	st.exitDeadline = time.Now().Add(500 * time.Millisecond)
	st.notification = "Closing\u2026"
	st.notificationUntil = st.exitDeadline.Add(time.Second) // keep visible until exit
	st.window.Invalidate()
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
		st.textRend.CharAdvance,
		st.textRend.LineHeightPx,
	)

	st.scrollbarRend = render.NewScrollbarRenderer(st.theme.ScrollbarThumb)

	// Match the native macOS titlebar height (28dp) plus a little padding.
	// On Retina (2×) this is ~76px; on 1× it's ~38px.
	st.dp = gtx.Dp
	st.tabBarHeight = gtx.Dp(32)
	st.trafficLightPx = gtx.Dp(trafficLightPaddingDp)
}

// applyTheme switches to a new theme at runtime, rebuilding derived state.
func (st *appState) applyTheme(theme config.Theme) {
	st.theme = theme
	st.colorMap = render.TokenColorMap(theme)
	st.shaper = nil // forces initRenderers to re-run next frame
	st.window.Invalidate()
}

// toggleTheme switches between dark and light mode within the current bundle.
func (st *appState) toggleTheme() {
	st.darkMode = !st.darkMode
	st.applyTheme(st.themeBundle.Theme(st.darkMode))
	updateWindowBackground(st.theme.TabBarBg)
	st.persistThemeConfig()
}

// selectThemeBundle switches to a different theme bundle by name.
func (st *appState) selectThemeBundle(name string) {
	bundle, err := config.LoadBundleByName(name)
	if err != nil {
		return
	}
	st.themeName = name
	st.themeBundle = bundle
	st.applyTheme(bundle.Theme(st.darkMode))
	updateWindowBackground(st.theme.TabBarBg)
	updateThemeMenuCheck(name)
	st.persistThemeConfig()
}

// initThemeMenu sets up the native macOS View > Theme menu.
func (st *appState) initThemeMenu() {
	metas := config.ListThemes()
	names := make([]string, len(metas))
	for i, m := range metas {
		names[i] = m.Name
	}
	if len(names) == 0 {
		names = []string{"default"}
	}
	setupThemeMenu(names, st.themeName)
}

// persistThemeConfig saves the current theme name and dark mode preference.
func (st *appState) persistThemeConfig() {
	cfg := config.LoadConfig()
	cfg.Theme = st.themeName
	cfg.DarkMode = st.darkMode
	config.SaveConfig(cfg)
}
