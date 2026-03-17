//go:build !darwin

package main

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
