package main

import (
	"os"
	"path/filepath"
	"time"

	"gioui.org/app"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/kristianweb/zephyr/internal/config"
	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/fileio"
	"github.com/kristianweb/zephyr/internal/git"
	"github.com/kristianweb/zephyr/internal/highlight"
	"github.com/kristianweb/zephyr/internal/navigator"
	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/ui"
	"github.com/kristianweb/zephyr/internal/vim"
)

func main() {
	tracePerformanceEvent("process_start")
	defer appPerfTracer.close()
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		printVersion()
		appPerfTracer.close()
		os.Exit(0)
	}
	setupTitlebar()
	go run()
	app.Main()
}

// viewMode tracks whether a tab is in edit or preview mode.
type viewMode int

const (
	viewEdit         viewMode = iota
	viewMarkdownRead          // rendered markdown preview
)

// bufferType distinguishes file, directory, and status buffers.
type bufferType int

const (
	bufFile      bufferType = iota // normal file editing
	bufDirectory                   // oil-style directory listing
	bufStatus                      // git status buffer
	bufOriginal                    // the file's HEAD content, read-only
)

// tabState holds per-tab state that isn't part of the editor itself.
type tabState struct {
	viewport       *render.Viewport
	highlighter    *highlight.Highlighter
	langLabel      string
	lastCursorLine int // tracks cursor to detect movement; -1 = uninitialized
	lastCursorCol  int
	sourceBuf      []byte // reusable buffer for tree-sitter source

	// Markdown preview state
	mode         viewMode
	mdDoc        *render.MarkdownDoc // parsed markdown for read mode
	mdScrollY    float64             // pixel scroll offset for read mode
	mdTotalH     int                 // total rendered height for scroll clamping
	mdCopyBtns   []codeCopyBtn       // code block copy button hit areas
	mdCheckboxes []mdCheckbox        // task list checkbox hit areas
	mdSelActive  bool                // true during a drag-select
	mdSelAnchor  int                 // character offset where selection started
	mdSelCursor  int                 // character offset where selection currently is
	mdSelText    string              // full plain text of the document for selection
	mdSelBlocks  []mdSelBlock        // per-block layout info for selection mapping

	// Word wrap
	wrapMap *wrapMap // visual line mapping (nil when wrap is off)

	// Code folding
	foldState *render.FoldState // fold regions and collapsed state

	// Diagnostics: buffer lines (0-based) flagged with a syntax/format error.
	// Rendered as gutter markers; nil/empty means no errors. Per-tab so switching
	// tabs shows that tab's markers.
	errorLines map[int]bool

	// External-change tracking: the disk state this tab's content was last
	// loaded from or written to, plus any unresolved disagreement with it.
	diskSnap fileio.Snapshot
	conflict conflictState
	// True when a delete forced Modified on so the buffer would not be treated
	// as disposable; cleared if the file comes back (an atomic replace).
	deleteForcedModified bool

	// Navigator mode
	bufType   bufferType              // file, directory, status, or HEAD view
	gitDiff   *git.FileDiff           // diff data for this file (nil if unchanged or not in repo)
	dirBuf    *navigator.DirBuffer    // directory buffer data (non-nil when bufType == bufDirectory)
	statusBuf *navigator.StatusBuffer // status buffer data (non-nil when bufType == bufStatus)
	// The working buffer a HEAD view displaced (non-nil when bufType ==
	// bufOriginal). It holds the piece table itself, not its text, so the undo
	// history's offsets still describe it when it comes back.
	headStash *headStash
}

type appState struct {
	tabBar           *ui.TabBar
	tabStates        map[*editor.Editor]*tabState
	theme            config.Theme
	shaper           *text.Shaper
	textRend         *render.TextRenderer
	gutterRend       *render.GutterRenderer
	cursorRend       *render.CursorRenderer
	colorMap         highlight.TokenColorMap
	statusRend       *render.TextRenderer
	tabRend          *render.TextRenderer // font for tab bar
	plusRend         *render.TextRenderer // larger font for "+" button
	tag              *bool
	langSel          *ui.LanguageSelector
	findBar          *ui.FindReplaceBar
	fuzzyFinder      *ui.FuzzyFinder
	scrollbarRend    *render.ScrollbarRenderer
	langLabelX       int
	lastMaxY         int
	lastMaxX         int
	dragging         bool
	activePointer    pointer.ID
	pointerActive    bool
	quitInProgress   bool
	scrollAccum      float32 // accumulated fractional scroll delta
	window           *app.Window
	lastWindowTitle  string             // dedup title updates to avoid Configure() thrash
	darkMode         bool               // true = dark theme, false = light theme
	themeName        string             // active theme bundle name
	themeBundle      config.ThemeBundle // loaded theme bundle
	fontCfg          config.FontConfig  // font configuration from theme
	mdRend           *mdRenderers       // cached markdown preview renderers
	mdToggleX        int                // left edge of Edit/Read toggle button
	mdToggleW        int                // width of the toggle button
	fmtToggleX       int                // left edge of JSON Compact/Expanded toggle
	fmtToggleW       int                // width of the JSON Compact/Expanded toggle
	themeMenuReady   bool               // true once native theme menu has been set up
	wordWrap         bool               // true when word wrap is enabled
	wrapMenuReady    bool               // true once native word wrap menu has been set up
	autoIndent       bool               // true when indentation is fixed on every Enter
	indentMenuReady  bool               // true once native auto indent menu has been set up
	indentWidth      int                // spaces per indentation level (default 2)
	versionMenuReady bool               // true once native app menu version row has been set up

	tabBarHeight   int                 // computed from display scale to match native titlebar
	trafficLightPx int                 // traffic light padding in pixels (scaled from Dp)
	hoverX, hoverY int                 // last pointer position for hover effects
	dp             func(v unit.Dp) int // cached scale function from latest gtx

	// Tab tooltip state
	tooltipTabIdx int       // tab index the pointer is hovering (-1 = none)
	tooltipEnter  time.Time // when the pointer entered the tab
	tooltipX      int       // X position of the hovered tab (for tooltip placement)

	// Tab overflow state
	overflowOpen     bool  // true when the overflow dropdown is visible
	overflowStartIdx int   // first tab index that overflows (== len(Tabs) if none)
	overflowBtnX     int   // left edge X of the ">" button (for click detection)
	overflowBtnW     int   // width of the ">" button
	barTabIdxs       []int // tab indices shown in the bar (computed each frame)
	dropdownTabIdxs  []int // tab indices shown in the dropdown (computed each frame)
	dropdownHeader   int   // tab index shown as first dropdown item for continuity (-1 = none)

	// Debounced reparse state
	reparsePending  bool
	reparseDeadline time.Time

	// Debounced error-check state: runs syntax/format detection 5s after the
	// last buffer-modifying keystroke (any edit resets the deadline).
	errCheckPending  bool
	errCheckDeadline time.Time

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
		forQuit        bool // continue quit flow after action
		closeAfterSave bool // close tab after save (close-tab flow)
		saveAsExpanded bool // true when Save As rows are shown for file-backed tabs

		// Save As fields
		filename         []rune
		cursor           int     // rune position in filename
		selectAll        bool    // entire filename is selected
		dir              string  // directory to save in
		tags             [7]bool // macOS Finder tags: Red, Orange, Yellow, Green, Blue, Purple, Gray
		confirmOverwrite bool    // true when waiting for overwrite confirmation
		confirmClobber   bool    // true when a save is blocked by an external change
	}

	// Folder of the most recent successful Save As, used to preselect the
	// Save As directory for untitled buffers only.
	lastSaveDir string
	// persistConfig applies a mutation to the settings file. It is nil outside
	// the real app so a unit test never writes the user's settings.json.
	persistConfig func(func(*config.Config))

	// Vim mode
	vimEnabled    bool
	vimState      *vim.State
	vimIndicatorX int // X position of "Vim" text in status bar
	vimIndicatorW int // width of "Vim" text

	// Navigator mode
	navigatorActive bool
	navRoot         string // project root path for navigator
	gitRepo         *git.Repo
	gitCache        *git.Cache
	navigator       *navigator.Navigator

	// Navigator root dropdown
	navRootDropdown struct {
		open  bool
		x, w  int      // hit area of the folder name in the header
		items []string // recent root paths to display
	}
	navCachedPath    string // cached display path for breadcrumb
	navCachedPathKey string // key: filePath or dirBufPath that produced the cached path
	navCachedHome    string // cached os.UserHomeDir result
	navPrevTabIdx    int    // tab index before opening directory buffer (for toggle)

	// File watcher for external changes
	watcher        *fileio.Watcher
	watcherPending pendingWatchEvents // filled by the watcher goroutine, drained per frame

	// Test-build performance telemetry.
	perfFrameCount       uint64
	perfPendingEventAt   time.Time
	perfPendingEventKind string

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
			foldState:      render.NewFoldState(),
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
		if ed.FilePath != "" {
			if snap, err := fileio.TakeSnapshot(ed.FilePath); err == nil {
				ts.diskSnap = snap
			}
		}
		// Load git diff data for gutter signs
		if st.gitCache != nil && ed.FilePath != "" {
			if relPath, err := filepath.Rel(st.gitRepo.Root, ed.FilePath); err == nil {
				ts.gitDiff, _ = st.gitCache.FileDiff(relPath)
			}
		}
		// Compute fold regions
		source := ed.Buffer.TextBytes(nil)
		regions := render.ComputeFoldRegions(string(source))
		ts.foldState.SetRegions(regions, ed.Buffer.LineCount())
		// Open markdown files in read mode by default
		if ts.langLabel == "Markdown" {
			ts.mode = viewMarkdownRead
			ts.mdDoc = render.ParseMarkdown(ed.Buffer.TextBytes(nil))
		}
		// Detect syntax/format errors on open. The highlighter (if any) was just
		// parsed above, so its tree is current; call runErrorDetection directly
		// rather than detectErrors, which would re-enter activeTabState.
		st.runErrorDetection(ts, ed)
		st.tabStates[ed] = ts
	}
	return ts
}

func run() {
	// Load config and theme bundle early (before tab creation)
	config.EnsureDefaultThemes()
	cfg := config.LoadConfig()

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

	bundle, err := config.LoadBundleByName(cfg.Theme)
	if err != nil {
		bundle = config.DefaultBundle()
		cfg.Theme = "default"
	}
	theme := bundle.Theme(cfg.DarkMode)
	if cfg.VimMode {
		theme.TabAccent = vimGreen
	}

	w := &app.Window{}

	st := &appState{
		tabBar:        tabBar,
		tabStates:     make(map[*editor.Editor]*tabState),
		theme:         theme,
		colorMap:      render.TokenColorMap(theme),
		tag:           new(bool),
		langSel:       ui.NewLanguageSelector(),
		findBar:       ui.NewFindReplaceBar(),
		fuzzyFinder:   ui.NewFuzzyFinder(),
		window:        w,
		darkMode:      cfg.DarkMode,
		themeName:     cfg.Theme,
		themeBundle:   bundle,
		fontCfg:       bundle.Fonts,
		wordWrap:      cfg.WordWrap,
		autoIndent:    cfg.AutoIndent,
		indentWidth:   cfg.IndentWidth,
		vimEnabled:    cfg.VimMode,
		tooltipTabIdx: -1,
		lastSaveDir:   cfg.LastSaveDir,
		persistConfig: applyToConfigFile,
	}

	if st.vimEnabled {
		st.vimState = vim.NewState()
	}

	// The finder's directory scan runs off the UI goroutine and repaints when
	// it lands. The window is captured rather than reached through st, which
	// the scan goroutine must not touch.
	st.fuzzyFinder.OnResults = w.Invalidate

	// Start file watcher
	if fw, err := fileio.NewWatcher(); err == nil {
		st.watcher = fw
		// Watch initial file if present
		if tab := tabBar.ActiveTab(); tab != nil && tab.Editor.FilePath != "" {
			fw.Watch(tab.Editor.FilePath)
		}
		startWatcherPump(fw.Events, &st.watcherPending, st.invalidate)
	}

	// Init tab state for first tab
	st.activeTabState()

	st.updateWindowTitle()
	w.Option(
		app.Decorated(platformDecorated()),
		app.Size(unit.Dp(900), unit.Dp(600)),
		app.MinSize(unit.Dp(400), unit.Dp(300)),
	)
	tracePerformanceEvent("window_requested")

	var ops op.Ops

	for {
		evt := w.Event()
		switch e := evt.(type) {
		case app.DestroyEvent:
			if !st.hasUnsavedChanges() {
				appPerfTracer.close()
				os.Exit(0)
			}
			// Has unsaved changes — re-create window and show save prompt.
			w = new(app.Window)
			w.Option(
				app.Decorated(platformDecorated()),
				app.Title("Zephyr"),
				app.Size(unit.Dp(1024), unit.Dp(768)),
				app.MinSize(unit.Dp(400), unit.Dp(300)),
			)
			st.window = w
			setupTitlebar()
			st.startQuitFlow()

		case app.FrameEvent:
			frameStart := time.Now()
			gtx := app.NewContext(&ops, e)
			st.initRenderers(gtx)
			setUnsavedFlag(st.hasUnsavedChanges())

			// Defer menu setup until the titlebar/window is ready
			if !st.themeMenuReady && titlebarReady() {
				st.initThemeMenu()
				st.themeMenuReady = true
			}
			if !st.wrapMenuReady && titlebarReady() {
				setupWordWrapMenu(st.wordWrap)
				st.wrapMenuReady = true
			}
			if !st.indentMenuReady && titlebarReady() {
				setupAutoIndentMenu(st.autoIndent)
				setupIndentWidthMenu(st.indentWidth)
				st.indentMenuReady = true
			}
			if !st.versionMenuReady && titlebarReady() {
				setupAppMenuVersionItem(versionMenuTitle())
				st.versionMenuReady = true
			}

			// Check if graceful exit delay has elapsed
			if st.exitPending && !time.Now().Before(st.exitDeadline) {
				appPerfTracer.close()
				os.Exit(0)
			}

			if closeRequested() && !st.quitInProgress && !st.exitPending {
				st.startQuitFlow()
			}
			if sel := checkThemeSelection(); sel != "" && sel != st.themeName {
				st.selectThemeBundle(sel)
			}
			if wordWrapToggled() {
				st.toggleWordWrap()
			}
			if autoIndentToggled() {
				st.toggleAutoIndent()
			}
			if w := checkIndentWidthSelection(); w > 0 && w != st.indentWidth {
				st.selectIndentWidth(w)
			}
			if openPath := checkOpenFile(); openPath != "" {
				// If the only tab is an empty untitled one, replace it
				if len(st.tabBar.Tabs) == 1 && st.tabBar.Tabs[0].IsUntitled &&
					!st.tabBar.Tabs[0].Editor.Modified {
					// Remove the empty tab's state
					delete(st.tabStates, st.tabBar.Tabs[0].Editor)
					st.tabBar.ForceCloseTab(0)
				}
				st.openFileInTab(openPath)
				st.activeTabState() // init tab state (incl. markdown read mode)
				st.updateWindowTitle()
				w.Invalidate()
			}
			eventStart := time.Now()
			if !st.exitPending {
				st.handleEvents(gtx, w)
				st.flushReparse()
				st.flushErrCheck()
				st.pollFileWatcher()
			}
			eventDuration := time.Since(eventStart)
			// Theme-dependent state may be invalidated by this frame's events.
			// Rebuild it before drawing so the frame uses one renderer generation.
			drawStart := time.Now()
			st.initRenderers(gtx)
			st.draw(gtx, w)
			e.Frame(gtx.Ops)
			st.recordPerformanceFrame(frameStart, eventDuration, time.Since(drawStart))
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
		if st.window != nil {
			st.window.Option(app.Title(title))
		}
	}
}

func (st *appState) invalidate() {
	if st.window != nil {
		st.window.Invalidate()
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
	st.invalidate()
}

func (st *appState) initRenderers(gtx layout.Context) {
	if st.shaper != nil {
		return
	}
	st.shaper = text.NewShaper()
	mono := st.fontCfg.Monospace
	st.textRend = render.NewTextRenderer(st.shaper, render.TextStyle{
		FontSize:   13,
		LineHeight: 1.4,
		Foreground: st.theme.Foreground,
		Typeface:   mono,
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
		Typeface:   mono,
	})
	st.statusRend.ComputeMetrics(gtx)

	st.tabRend = render.NewTextRenderer(st.shaper, render.TextStyle{
		FontSize:   11,
		LineHeight: 1.3,
		Foreground: st.theme.Foreground,
		Typeface:   mono,
	})
	st.tabRend.ComputeMetrics(gtx)

	st.plusRend = render.NewTextRenderer(st.shaper, render.TextStyle{
		FontSize:   18,
		LineHeight: 1.0,
		Foreground: st.theme.Foreground,
		Typeface:   mono,
	})
	st.plusRend.ComputeMetrics(gtx)

	st.cursorRend = render.NewCursorRenderer(
		st.theme.Cursor,
		st.textRend.CharWidth,
		st.textRend.CharAdvance,
		st.textRend.LineHeightPx,
	)

	st.scrollbarRend = render.NewScrollbarRenderer(st.theme.ScrollbarThumb)

	// Match the native macOS titlebar height (~28pt).
	st.dp = gtx.Dp
	st.tabBarHeight = gtx.Dp(28)
	st.trafficLightPx = gtx.Dp(trafficLightPaddingDp)

	// On platforms with the theme toggle on the left, reserve space for it.
	if platformThemeToggleLeft() {
		_, hitW := st.themeToggleSize()
		if hitW > st.trafficLightPx {
			st.trafficLightPx = hitW
		}
	}
}

// resetRenderers discards state derived from the current theme and fonts.
// initRenderers must be called before the next draw.
func (st *appState) resetRenderers() {
	st.shaper = nil
	st.textRend = nil
	st.gutterRend = nil
	st.cursorRend = nil
	st.statusRend = nil
	st.tabRend = nil
	st.plusRend = nil
	st.scrollbarRend = nil
	st.mdRend = nil
}

// applyTheme switches to a new theme at runtime, rebuilding derived state.
func (st *appState) applyTheme(theme config.Theme) {
	st.theme = theme
	if st.vimEnabled {
		st.theme.TabAccent = vimGreen
	}
	st.colorMap = render.TokenColorMap(theme)
	st.resetRenderers()
	st.invalidate()
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
	st.fontCfg = bundle.Fonts
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

// applyToConfigFile reads the settings file, applies mutate, and writes it
// back.
func applyToConfigFile(mutate func(*config.Config)) {
	cfg := config.LoadConfig()
	mutate(&cfg)
	config.SaveConfig(cfg)
}

// persistThemeConfig saves the current theme name and dark mode preference.
func (st *appState) persistThemeConfig() {
	cfg := config.LoadConfig()
	cfg.Theme = st.themeName
	cfg.DarkMode = st.darkMode
	config.SaveConfig(cfg)
}

// toggleVimMode toggles vim mode on/off and persists the setting.
func (st *appState) toggleVimMode() {
	st.vimEnabled = !st.vimEnabled
	if st.vimEnabled {
		st.vimState = vim.NewState()
	} else {
		st.vimState = nil
	}
	// Re-apply theme so accent color is updated
	st.applyTheme(st.themeBundle.Theme(st.darkMode))
	cfg := config.LoadConfig()
	cfg.VimMode = st.vimEnabled
	config.SaveConfig(cfg)
}

// toggleNavigatorMode toggles Navigator Mode on/off.
func (st *appState) toggleNavigatorMode() {
	st.navigatorActive = !st.navigatorActive
	if !st.navigatorActive {
		st.navRootDropdown.open = false
		return
	}

	// Navigator requires vim mode — activate it if not already on
	if !st.vimEnabled {
		st.vimEnabled = true
		st.vimState = vim.NewState()
		st.applyTheme(st.themeBundle.Theme(st.darkMode))
	}
	st.vimState.NavigatorEnabled = true

	// Init navigator
	if st.navigator == nil {
		st.navigator = navigator.New()
	}

	// Auto-detect root if not already set
	if st.navRoot == "" {
		st.detectNavRoot()
	}

	// If still no root, open the dropdown so the user can pick one
	if st.navRoot == "" {
		st.openNavRootDropdown()
	}
}

// detectNavRoot attempts to discover the project root automatically.
// Priority: git repo root > open file's directory > CWD.
func (st *appState) detectNavRoot() {
	// Try git repo discovery
	var searchPath string
	if ed := st.activeEd(); ed != nil && ed.FilePath != "" {
		searchPath = filepath.Dir(ed.FilePath)
	} else if wd, err := os.Getwd(); err == nil {
		searchPath = wd
	}

	if searchPath != "" {
		if repo, err := git.Discover(searchPath); err == nil && repo != nil {
			st.setNavRoot(repo.Root)
			return
		}
	}

	// Fallback: directory of open file
	if ed := st.activeEd(); ed != nil && ed.FilePath != "" {
		st.setNavRoot(filepath.Dir(ed.FilePath))
		return
	}

	// Fallback: CWD
	if wd, err := os.Getwd(); err == nil {
		st.setNavRoot(wd)
	}
}

// setNavRoot sets the navigator root and initializes git if available.
func (st *appState) setNavRoot(root string) {
	if root == "" || root == "/" || root == "." {
		return
	}
	st.navRoot = root

	// Try git discovery from the root
	if repo, err := git.Discover(root); err == nil && repo != nil {
		st.gitRepo = repo
		st.gitCache = git.NewCache(repo)
	} else {
		st.gitRepo = nil
		st.gitCache = nil
	}

	// Persist to recent roots (async to avoid blocking UI)
	go func() {
		cfg := config.LoadConfig()
		cfg.AddRecentRoot(root)
		config.SaveConfig(cfg)
	}()

	// Update cached dropdown items
	st.navRootDropdown.items = nil // force reload on next open
	st.navRootDropdown.open = false

	// Invalidate breadcrumb cache
	st.navCachedPathKey = ""
}

// openNavRootDropdown populates and opens the root folder dropdown.
func (st *appState) openNavRootDropdown() {
	// Only reload from disk if items are empty (first open or after clear)
	if len(st.navRootDropdown.items) == 0 {
		cfg := config.LoadConfig()
		st.navRootDropdown.items = cfg.RecentRoots
	}
	st.navRootDropdown.open = true
}

// toggleWordWrap toggles word wrap on/off and persists the setting.
func (st *appState) toggleWordWrap() {
	st.wordWrap = !st.wordWrap
	cfg := config.LoadConfig()
	cfg.WordWrap = st.wordWrap
	config.SaveConfig(cfg)
	updateWordWrapMenuCheck(st.wordWrap)
}

// toggleAutoIndent toggles auto indent on/off and persists the setting.
func (st *appState) toggleAutoIndent() {
	st.autoIndent = !st.autoIndent
	cfg := config.LoadConfig()
	cfg.AutoIndent = st.autoIndent
	config.SaveConfig(cfg)
	updateAutoIndentMenuCheck(st.autoIndent)
}

// selectIndentWidth sets the indentation width and persists the setting.
func (st *appState) selectIndentWidth(width int) {
	st.indentWidth = width
	cfg := config.LoadConfig()
	cfg.IndentWidth = st.indentWidth
	config.SaveConfig(cfg)
	updateIndentWidthMenuCheck(st.indentWidth)
}
