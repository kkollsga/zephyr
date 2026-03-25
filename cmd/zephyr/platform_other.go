//go:build !darwin && !windows

package main

import (
	"image/color"

	"gioui.org/layout"

	"github.com/kristianweb/zephyr/internal/render"
	"github.com/kristianweb/zephyr/internal/ui"
)

// platformDecorated returns true — use native window decorations.
func platformDecorated() bool { return true }

// platformThemeToggleLeft returns true — on non-macOS the toggle is on the left.
func platformThemeToggleLeft() bool { return true }

// platformHasFinderTags returns false — Finder tags are macOS-only.
func platformHasFinderTags() bool { return false }

// warningColor returns the orange color used for overwrite warnings.
func warningColor() color.NRGBA {
	return color.NRGBA{R: 0xFF, G: 0x9F, B: 0x0A, A: 0xFF}
}

func (st *appState) pickSaveDir() {}

func (st *appState) saveTabAs(tab *ui.Tab) bool { return false }

func (st *appState) applyFinderTags(path string) {}

func (st *appState) drawFinderTagRow(gtx layout.Context, tr *render.TextRenderer, dx, dw, fieldX, curY, itemH int) {
}
