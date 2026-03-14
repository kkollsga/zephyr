package ui

import (
	"fmt"

	"github.com/kristianweb/zephyr/internal/editor"
)

// StatusInfo holds the data to display in the status line.
type StatusInfo struct {
	Line     int
	Col      int
	Language string
	Modified bool
}

// StatusInfoFromEditor extracts status info from an editor.
func StatusInfoFromEditor(ed *editor.Editor, language string) StatusInfo {
	return StatusInfo{
		Line:     ed.Cursor.Line + 1,
		Col:      ed.Cursor.Col + 1,
		Language: language,
		Modified: ed.Modified,
	}
}

// FormatPosition returns the "line:col" string.
func (si StatusInfo) FormatPosition() string {
	return fmt.Sprintf("%d:%d", si.Line, si.Col)
}
