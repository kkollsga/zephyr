package vim

import (
	"strconv"
	"strings"
)

// ParseCommand parses a command-line string (after the `:`) and returns an Action.
func ParseCommand(cmdLine string) Action {
	cmd := strings.TrimSpace(cmdLine)
	if cmd == "" {
		return Action{Kind: ActionNone}
	}

	// Check for line number
	if n, err := strconv.Atoi(cmd); err == nil && n > 0 {
		return Action{Kind: ActionMoveToLine, Line: n}
	}

	// Parse command
	switch {
	case cmd == "w":
		return Action{Kind: ActionWrite}
	case cmd == "q":
		return Action{Kind: ActionQuit}
	case cmd == "wq" || cmd == "x":
		return Action{Kind: ActionWriteQuit}
	case cmd == "q!":
		return Action{Kind: ActionForceQuit}
	case cmd == "Tutor" || cmd == "tutor":
		return Action{Kind: ActionTutor}
	case strings.HasPrefix(cmd, "w "):
		// :w filename — save as
		return Action{Kind: ActionWrite, Text: strings.TrimSpace(cmd[2:])}
	case strings.HasPrefix(cmd, "commit "):
		// :commit message — git commit
		return Action{Kind: ActionNavCommit, Text: strings.TrimSpace(cmd[7:])}
	case cmd == "commit":
		return Action{Kind: ActionNavCommit}
	}

	return Action{Kind: ActionNone}
}
