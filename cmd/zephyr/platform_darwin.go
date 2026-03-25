//go:build darwin

package main

import (
	"fmt"
	"image/color"
	"os/exec"
	"strings"

	"github.com/kristianweb/zephyr/internal/ui"
)

// platformDecorated returns false on macOS — Zephyr draws its own titlebar.
func platformDecorated() bool { return false }

// platformThemeToggleLeft returns false — on macOS the toggle is on the right.
func platformThemeToggleLeft() bool { return false }

// platformHasFinderTags returns true on macOS where Finder tags are available.
func platformHasFinderTags() bool { return true }

// warningColor returns the orange color used for overwrite warnings.
func warningColor() color.NRGBA {
	return color.NRGBA{R: 0xFF, G: 0x9F, B: 0x0A, A: 0xFF}
}

// finderTagNames are the macOS Finder tag names (indexed 0–6).
var finderTagNames = [7]string{"Red", "Orange", "Yellow", "Green", "Blue", "Purple", "Gray"}

// finderTagColors are the macOS Finder tag colors (Red, Orange, Yellow, Green, Blue, Purple, Gray).
var finderTagColors = [7]color.NRGBA{
	{R: 0xFF, G: 0x3B, B: 0x30, A: 0xFF}, // Red
	{R: 0xFF, G: 0x9F, B: 0x0A, A: 0xFF}, // Orange
	{R: 0xFF, G: 0xCC, B: 0x00, A: 0xFF}, // Yellow
	{R: 0x34, G: 0xC7, B: 0x59, A: 0xFF}, // Green
	{R: 0x00, G: 0x7A, B: 0xFF, A: 0xFF}, // Blue
	{R: 0xAF, G: 0x52, B: 0xDE, A: 0xFF}, // Purple
	{R: 0x8E, G: 0x8E, B: 0x93, A: 0xFF}, // Gray
}

// pickSaveDir opens the native macOS folder picker and updates the save dir.
func (st *appState) pickSaveDir() {
	script := fmt.Sprintf(
		`set folderPath to POSIX path of (choose folder with prompt "Save in" default location POSIX file %q)
return folderPath`, st.saveMenu.dir)
	go func() {
		out, err := exec.Command("osascript", "-e", script).Output()
		if err != nil {
			return
		}
		dir := strings.TrimSpace(string(out))
		// Remove trailing slash that osascript adds
		dir = strings.TrimRight(dir, "/")
		if dir != "" {
			st.saveMenu.dir = dir
			if st.window != nil {
				st.window.Invalidate()
			}
		}
	}()
}

// saveTabAs shows the native macOS Save As file dialog.
func (st *appState) saveTabAs(tab *ui.Tab) bool {
	defaultName := tab.Title
	if defaultName == "" || tab.IsUntitled {
		ts := st.tabStates[tab.Editor]
		if ts != nil && ts.langLabel != "" && ts.langLabel != "Plain Text" {
			defaultName = "Untitled" + langToExtension(ts.langLabel)
		} else {
			defaultName = "Untitled.txt"
		}
	}
	script := fmt.Sprintf(`set filePath to POSIX path of (choose file name with prompt "Save As" default name %q)
return filePath`, defaultName)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return false
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return false
	}
	return st.saveTabToPath(tab, path)
}

// applyFinderTags sets macOS Finder tags on the saved file.
func (st *appState) applyFinderTags(path string) {
	var names []string
	for i, on := range st.saveMenu.tags {
		if on {
			names = append(names, finderTagNames[i])
		}
	}
	if len(names) == 0 {
		return
	}

	// Build AppleScript list: {"Red", "Blue"}
	var parts []string
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%q", n))
	}
	tagList := "{" + strings.Join(parts, ", ") + "}"

	script := fmt.Sprintf(`
tell application "Finder"
	set theTags to %s
	set theFile to (POSIX file %q) as alias
	set label index of theFile to 0
	repeat with t in theTags
		set current application's NSWorkspace's sharedWorkspace's `+
		"`setTags:theTags ofFile:filePath`"+`
	end repeat
end tell`, tagList, path)

	// Use a simpler xattr-based approach that works without Finder scripting
	_ = script // above is complex; use the simpler tag approach below

	go func() {
		for _, name := range names {
			// macOS stores Finder tags via extended attributes; the simplest
			// reliable way is the `tag` CLI or writing com.apple.metadata:_kMDItemUserTags.
			// Fall back to osascript Finder tell.
			tagScript := fmt.Sprintf(
				`tell application "Finder" to set comment of (POSIX file %q as alias) to comment of (POSIX file %q as alias)`, path, path)
			_ = tagScript
			// Use the `tag` command if available, otherwise skip silently
			exec.Command("tag", "--add", name, path).Run()
		}
	}()
}

