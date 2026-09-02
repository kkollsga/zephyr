//go:build !darwin && !windows

package main

import "image/color"

func setupTitlebar() {}

func titlebarReady() bool { return true }

func setUnsavedFlag(unsaved bool) {}

func ensureTrafficLights() {}

func closeRequested() bool { return false }

func pointerOutsideWindow() bool { return false }

func startWindowDrag() {}

const trafficLightPaddingDp = 0

func updateWindowBackground(c color.NRGBA) {}

func setupAppMenuVersionItem(title string) {}

func setupThemeMenu(themeNames []string, activeTheme string) {}

func checkThemeSelection() string { return "" }

func updateThemeMenuCheck(activeTheme string) {}

func setupWordWrapMenu(checked bool) {}

func wordWrapToggled() bool { return false }

func updateWordWrapMenuCheck(checked bool) {}

func setupAutoIndentMenu(checked bool) {}

func autoIndentToggled() bool { return false }

func updateAutoIndentMenuCheck(checked bool) {}

func setupIndentWidthMenu(activeWidth int) {}

func checkIndentWidthSelection() int { return 0 }

func updateIndentWidthMenuCheck(activeWidth int) {}

func checkOpenFile() string { return "" }
