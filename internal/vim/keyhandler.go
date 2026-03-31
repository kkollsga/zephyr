package vim

// KeyInput represents a key press from the host editor.
// This is a Gio-independent representation.
type KeyInput struct {
	Char     rune   // printable character (from EditEvent or derived)
	Name     string // key name for named keys (e.g., "Escape", "Return")
	Ctrl     bool
	Shift    bool
	Alt      bool
	Shortcut bool // Cmd on macOS, Ctrl on other platforms
}

// Named key constants matching Gio's key.Name values.
const (
	NameEscape          = "Escape"
	NameReturn          = "⏎"
	NameDeleteBackward  = "⌫"
	NameDeleteForward   = "⌦"
	NameUpArrow         = "↑"
	NameDownArrow       = "↓"
	NameLeftArrow       = "←"
	NameRightArrow      = "→"
	NameHome            = "⇱"
	NameEnd             = "⇲"
	NamePageUp          = "⇞"
	NamePageDown        = "⇟"
	NameTab             = "Tab"
)

// HandleKey processes a key input and returns an action.
// The caller is responsible for executing the action on the editor.
func (s *State) HandleKey(ev KeyInput) Action {
	// Shortcut keys (Cmd+S, Cmd+C, etc.) always pass through to the host
	if ev.Shortcut {
		return Action{Kind: ActionNone}
	}

	switch s.Mode {
	case ModeNormal:
		return s.handleNormal(ev)
	case ModeInsert:
		return s.handleInsert(ev)
	case ModeVisual, ModeVisualLine, ModeVisualBlock:
		return s.handleVisual(ev)
	case ModeCommand, ModeSearch:
		return s.handleCommandLine(ev)
	case ModeReplace:
		return s.handleReplace(ev)
	}
	return Action{Kind: ActionNone}
}

// handleInsert handles keys in Insert mode.
// Most keys pass through to the host editor; only Escape and Ctrl+C are intercepted.
func (s *State) handleInsert(ev KeyInput) Action {
	switch {
	case ev.Name == NameEscape:
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionEnterNormal}
	case ev.Char == 'c' && ev.Ctrl:
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionEnterNormal}
	}
	// Everything else passes through to normal text editing
	return Action{Kind: ActionNone}
}

// handleReplace handles the single-character replace mode (r{char}).
func (s *State) handleReplace(ev KeyInput) Action {
	if ev.Name == NameEscape {
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionNone}
	}
	ch := ev.Char
	if ch == 0 {
		return Action{Kind: ActionNone}
	}
	s.Mode = ModeNormal
	count := s.Count
	s.reset()
	return Action{Kind: ActionReplace, Char: ch, Count: count}
}
