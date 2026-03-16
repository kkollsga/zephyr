//go:build !darwin

package main

func setupTitlebar() {}

func titlebarReady() bool { return true }

func setUnsavedFlag(unsaved bool) {}

const trafficLightPadding = 0
