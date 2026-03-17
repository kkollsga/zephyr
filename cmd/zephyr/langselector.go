package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/kristianweb/zephyr/internal/highlight"
)

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

	off := op.Offset(image.Pt(dropdownX, dropdownY)).Push(gtx.Ops)
	bgRect := clip.Rect{Max: image.Pt(dropdownW, dropdownH)}.Push(gtx.Ops)
	paint.ColorOp{Color: st.theme.DropdownBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgRect.Pop()

	selColor := st.theme.DropdownSel
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
	return highlight.DetectLanguage(path)
}

func (st *appState) reparseHighlight() {
	ts := st.activeTabState()
	ed := st.activeEd()
	if ts == nil || ed == nil {
		return
	}
	if ts.highlighter != nil {
		ts.highlighter.Parse([]byte(ed.Buffer.Text()))
	}
	// Rebuild byte offset cache
	n := ed.Buffer.LineCount()
	if cap(ts.byteOffsets) >= n {
		ts.byteOffsets = ts.byteOffsets[:n]
	} else {
		ts.byteOffsets = make([]int, n)
	}
	offset := 0
	for i := 0; i < n; i++ {
		ts.byteOffsets[i] = offset
		line, _ := ed.Buffer.Line(i)
		offset += len(line) + 1
	}
	if st.findBar.Visible {
		st.updateSearchResults()
	}
}
