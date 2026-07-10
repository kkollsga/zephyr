package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/kristianweb/zephyr/internal/config"
	"github.com/kristianweb/zephyr/internal/render"
)

func TestResetRenderersClearsDerivedState(t *testing.T) {
	st := &appState{
		shaper:        text.NewShaper(),
		textRend:      &render.TextRenderer{},
		gutterRend:    &render.GutterRenderer{},
		cursorRend:    &render.CursorRenderer{},
		statusRend:    &render.TextRenderer{},
		tabRend:       &render.TextRenderer{},
		plusRend:      &render.TextRenderer{},
		scrollbarRend: &render.ScrollbarRenderer{},
		mdRend:        &mdRenderers{},
	}

	st.resetRenderers()

	if st.shaper != nil || st.textRend != nil || st.gutterRend != nil ||
		st.cursorRend != nil || st.statusRend != nil || st.tabRend != nil ||
		st.plusRend != nil || st.scrollbarRend != nil || st.mdRend != nil {
		t.Fatal("resetRenderers left theme-derived renderer state cached")
	}
}

func TestInitRenderersRebuildsResetCache(t *testing.T) {
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Constraints: layout.Exact(image.Pt(900, 600)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	st := &appState{
		theme:   config.DarkTheme(),
		fontCfg: config.DefaultFontConfig(),
	}

	st.initRenderers(gtx)
	oldShaper := st.shaper
	if oldShaper == nil || st.textRend == nil || st.gutterRend == nil ||
		st.cursorRend == nil || st.statusRend == nil || st.tabRend == nil ||
		st.plusRend == nil || st.scrollbarRend == nil {
		t.Fatal("initRenderers did not populate the renderer cache")
	}

	light := config.LightTheme()
	st.theme = light
	st.resetRenderers()
	st.initRenderers(gtx)

	if st.shaper == nil || st.shaper == oldShaper {
		t.Fatal("initRenderers did not create a new shaper after reset")
	}
	if got := st.textRend.Style.Foreground; got != light.Foreground {
		t.Fatalf("rebuilt renderer uses foreground %v, want %v", got, light.Foreground)
	}
}
