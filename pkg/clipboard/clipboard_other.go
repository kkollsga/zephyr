//go:build !darwin && !windows

package clipboard

import (
	"os/exec"
	"strings"
)

// Get returns the current clipboard text content.
// Falls back to xclip on Linux/BSD.
func Get() string {
	out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
	if err != nil {
		// Try xsel as fallback
		out, err = exec.Command("xsel", "--clipboard", "--output").Output()
		if err != nil {
			return ""
		}
	}
	return strings.TrimSuffix(string(out), "\n")
}

// Set sets the clipboard text content.
// Falls back to xclip on Linux/BSD.
func Set(text string) {
	cmd := exec.Command("xclip", "-selection", "clipboard")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		// Try xsel as fallback
		cmd = exec.Command("xsel", "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(text)
		cmd.Run()
	}
}
