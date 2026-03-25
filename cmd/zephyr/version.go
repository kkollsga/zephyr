package main

import (
	"fmt"
	"runtime"
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
