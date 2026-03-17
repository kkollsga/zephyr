package main

import (
	"os"
	"path/filepath"

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
	byteOffsets    []int // byte offset at the start of each line; rebuilt on reparse
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
		findBar:   ui.NewFindReplaceBar(),
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
