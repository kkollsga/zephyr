//go:build !darwin

package main

func setupTitlebar() {}

func titlebarReady() bool { return true }

func setUnsavedFlag(unsaved bool) {}

func closeRequested() bool { return false }

const trafficLightPaddingDp = 0
