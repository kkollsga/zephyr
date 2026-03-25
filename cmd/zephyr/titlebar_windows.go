//go:build windows

package main

import "image/color"

// Windows uses native window decorations via Gio's Decorated(true),
// so most titlebar functions are no-ops.

func setupTitlebar() {}

func titlebarReady() bool { return true }

func setUnsavedFlag(unsaved bool) {}

func ensureTrafficLights() {}

func closeRequested() bool { return false }

func pointerOutsideWindow() bool { return false }

func startWindowDrag() {}

func globalMousePosition() (x, y float64) { return 0, 0 }

func windowFrame() (x, y, w, h float64) { return 0, 0, 0, 0 }

const trafficLightPaddingDp = 0

func updateWindowBackground(c color.NRGBA) {}

func setupThemeMenu(themeNames []string, activeTheme string) {}

func checkThemeSelection() string { return "" }

func updateThemeMenuCheck(activeTheme string) {}

func setupWordWrapMenu(checked bool) {}

func wordWrapToggled() bool { return false }

func updateWordWrapMenuCheck(checked bool) {}

func checkOpenFile() string { return "" }
