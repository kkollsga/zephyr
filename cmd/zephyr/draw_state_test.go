package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/kristianweb/zephyr/internal/render"
)

func headlessContext() layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(900, 600)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
}

func TestHeadlessOverlayDrawingPaths(t *testing.T) {
	gtx := headlessContext()
	st, ed, ts := testAppWithText("alpha beta alpha\nsecond", "Plain Text")
	st.initRenderers(gtx)
	st.lastMaxX = 900
	st.lastMaxY = 600
	st.barTabIdxs = []int{0}
	st.tabBarHeight = 28

	st.findBar.Visible = true
	st.findBar.ShowReplace = true
	st.findBar.Query = "alpha"
	st.findBar.Replacement = "omega"
	st.updateSearchResults()
	st.hoverX = 890
	st.hoverY = 40
	st.drawFindBar(gtx, 550)
	st.drawMatchIndicator(gtx, 550, ed.Buffer.LineCount())

	st.langSel.Open([]string{"Go", "Markdown", "Rust"})
	st.langSel.Selected = 2
	st.drawLangSelector(gtx)

	st.showSaveMenu(0, false, false)
	st.saveMenu.confirmOverwrite = true
	st.drawSaveMenu(gtx)

	st.drawStatusLine(gtx, ed, ts)
	st.drawTabBar(gtx)
	st.drawThemeToggle(gtx, true)
	st.darkMode = false
	st.drawThemeToggle(gtx, false)
}

func TestHeadlessMarkdownPreviewDrawingPaths(t *testing.T) {
	gtx := headlessContext()
	source := "# Heading\n\nParagraph with **bold**, *italic*, and `code`.\n\n" +
		"- [x] done\n- [ ] todo\n\n" +
		"> quoted\n\n```go\nfunc f() {}\n```\n\n---\n\n" +
		"| A | B |\n|---|---|\n| 1 | 2 |\n"
	st, ed, ts := testAppWithText(source, "Markdown")
	st.initRenderers(gtx)
	st.lastMaxX = 900
	st.lastMaxY = 600
	ts.mode = viewMarkdownRead
	ts.mdDoc = render.ParseMarkdown(ed.Buffer.TextBytes(nil))

	st.drawMarkdownPreview(gtx, ts)
	if st.mdRend == nil || ts.mdTotalH <= 0 || ts.mdSelText == "" || len(ts.mdSelBlocks) == 0 {
		t.Fatalf("Markdown draw state renderers=%v total=%d selection=%q blocks=%d", st.mdRend, ts.mdTotalH, ts.mdSelText, len(ts.mdSelBlocks))
	}
	for level := 1; level <= 6; level++ {
		if st.mdRend.heading(level) == nil {
			t.Fatalf("heading renderer %d is nil", level)
		}
	}
}
