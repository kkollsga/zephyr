package main

import (
	"fmt"
	"runtime"
	"strings"
)

// Set via ldflags at build time:
//
//	-X main.version=v1.0.0 -X main.commit=abc1234 -X main.date=2026-01-01T00:00:00Z
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func printVersion() {
	fmt.Printf("Zephyr %s (%s, built %s, %s/%s)\n", version, commit, date, runtime.GOOS, runtime.GOARCH)
}

// versionMenuTitle returns the short version label shown as the first row of
// the native macOS application menu, e.g. "Zephyr 0.1.0" or "Zephyr dev".
func versionMenuTitle() string {
	return "Zephyr " + strings.TrimPrefix(version, "v")
}
