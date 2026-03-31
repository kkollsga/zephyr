package navigator

// Navigator holds the state for Navigator Mode.
type Navigator struct {
	DirCursors map[string]int // remembered cursor line per directory path
}

// New creates a new Navigator.
func New() *Navigator {
	return &Navigator{
		DirCursors: make(map[string]int),
	}
}

// RememberCursor stores the cursor position for a directory.
func (n *Navigator) RememberCursor(dirPath string, line int) {
	n.DirCursors[dirPath] = line
}

// RecallCursor returns the remembered cursor line for a directory (0 if unvisited).
func (n *Navigator) RecallCursor(dirPath string) int {
	return n.DirCursors[dirPath]
}
