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
	// A host accelerator arriving while a key is waiting for its argument
	// cancels that key instead of feeding it. Off macOS the shortcut modifier
	// is Ctrl, so Ctrl+S reaches vim as a printable 's': fed to a pending r it
	// replaced the character under the cursor and the file was never saved.
	// Cancelling matters as much on macOS, where Cmd+S was passed through but
	// left the pending r up, and it then swallowed the next real keystroke.
	if (ev.Ctrl || ev.Shortcut) && s.pendingInput() {
		s.reset()
		return Action{Kind: ActionNone}
	}

	// Host accelerators (Cmd+S, Cmd+C, …) are not vim keys. Off macOS the
	// shortcut modifier *is* Ctrl, so a bare Shortcut test would also discard
	// every Ctrl binding (Ctrl+d, Ctrl+r, Ctrl+v); Ctrl keeps them here and the
	// host still gets the ones vim declines with ActionNone.
	if ev.Shortcut && !ev.Ctrl {
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
