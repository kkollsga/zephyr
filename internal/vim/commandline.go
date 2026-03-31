package vim

import "unicode/utf8"

// handleCommandLine processes key inputs in Command (:) or Search (/, ?) mode.
func (s *State) handleCommandLine(ev KeyInput) Action {
	switch ev.Name {
	case NameEscape:
		s.Mode = ModeNormal
		s.CommandLine = ""
		s.CommandCursor = 0
		return Action{Kind: ActionCancelCommand}

	case NameReturn:
		line := s.CommandLine
		s.CommandLine = ""
		s.CommandCursor = 0
		if s.Mode == ModeCommand {
			s.Mode = ModeNormal
			return Action{Kind: ActionExecCommand, Text: line}
		}
		// Search mode
		if line != "" {
			s.SearchPattern = line
			s.Registers.Search = line
		}
		s.Mode = ModeNormal
		if s.SearchDir > 0 {
			return Action{Kind: ActionSearchNext, Text: s.SearchPattern}
		}
		return Action{Kind: ActionSearchPrev, Text: s.SearchPattern}

	case NameDeleteBackward:
		if len(s.CommandLine) > 0 {
			runes := []rune(s.CommandLine)
			if s.CommandCursor > 0 {
				s.CommandCursor--
				runes = append(runes[:s.CommandCursor], runes[s.CommandCursor+1:]...)
				s.CommandLine = string(runes)
			}
		} else {
			// Empty command line + backspace = cancel
			s.Mode = ModeNormal
			return Action{Kind: ActionCancelCommand}
		}
		return Action{Kind: ActionNone}

	case NameLeftArrow:
		if s.CommandCursor > 0 {
			s.CommandCursor--
		}
		return Action{Kind: ActionNone}

	case NameRightArrow:
		runes := []rune(s.CommandLine)
		if s.CommandCursor < len(runes) {
			s.CommandCursor++
		}
		return Action{Kind: ActionNone}

	case NameHome:
		s.CommandCursor = 0
		return Action{Kind: ActionNone}

	case NameEnd:
		s.CommandCursor = utf8.RuneCountInString(s.CommandLine)
		return Action{Kind: ActionNone}
	}

	// Regular character input
	if ev.Char != 0 && !ev.Ctrl && !ev.Alt {
		runes := []rune(s.CommandLine)
		// Insert at cursor position
		newRunes := make([]rune, 0, len(runes)+1)
		newRunes = append(newRunes, runes[:s.CommandCursor]...)
		newRunes = append(newRunes, ev.Char)
		newRunes = append(newRunes, runes[s.CommandCursor:]...)
		s.CommandLine = string(newRunes)
		s.CommandCursor++
		return Action{Kind: ActionNone}
	}

	return Action{Kind: ActionNone}
}
