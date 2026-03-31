package vim

// handleVisual processes key inputs in Visual, Visual Line, or Visual Block mode.
func (s *State) handleVisual(ev KeyInput) Action {
	ch := ev.Char

	// Escape returns to Normal
	if ev.Name == NameEscape || (ch == 'c' && ev.Ctrl) {
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionVisualEscape}
	}

	// Ctrl combos
	if ev.Ctrl {
		count := s.Count
		s.reset()
		switch ch {
		case 'd':
			return Action{Kind: ActionMoveHalfPageDown, Count: count}
		case 'u':
			return Action{Kind: ActionMoveHalfPageUp, Count: count}
		case 'f':
			return Action{Kind: ActionMovePageDown, Count: count}
		case 'b':
			return Action{Kind: ActionMovePageUp, Count: count}
		}
		return Action{Kind: ActionNone}
	}

	// Count accumulation
	if ch >= '1' && ch <= '9' {
		s.Count = s.Count*10 + int(ch-'0')
		return Action{Kind: ActionNone}
	}
	if ch == '0' && s.Count > 0 {
		s.Count = s.Count * 10
		return Action{Kind: ActionNone}
	}

	count := s.Count

	// Motion keys extend the selection
	switch ch {
	case 'h':
		s.reset()
		return Action{Kind: ActionMoveLeft, Count: count}
	case 'j':
		s.reset()
		return Action{Kind: ActionMoveDown, Count: count}
	case 'k':
		s.reset()
		return Action{Kind: ActionMoveUp, Count: count}
	case 'l':
		s.reset()
		return Action{Kind: ActionMoveRight, Count: count}
	case 'w':
		s.reset()
		return Action{Kind: ActionMoveWordForward, Count: count}
	case 'b':
		s.reset()
		return Action{Kind: ActionMoveWordBackward, Count: count}
	case 'e':
		s.reset()
		return Action{Kind: ActionMoveWordEnd, Count: count}
	case '0':
		s.reset()
		return Action{Kind: ActionMoveLineStart}
	case '$':
		s.reset()
		return Action{Kind: ActionMoveLineEnd}
	case '^':
		s.reset()
		return Action{Kind: ActionMoveFirstNonBlank}
	case 'G':
		if count > 0 {
			s.reset()
			return Action{Kind: ActionMoveToLine, Line: count}
		}
		s.reset()
		return Action{Kind: ActionMoveFileEnd}
	case '{':
		s.reset()
		return Action{Kind: ActionMoveParagraphUp, Count: count}
	case '}':
		s.reset()
		return Action{Kind: ActionMoveParagraphDown, Count: count}
	case '%':
		s.reset()
		return Action{Kind: ActionMoveBracketMatch}
	case 'g':
		s.PendingBuf = "g"
		return Action{Kind: ActionNone}
	}

	// Handle g-prefix in visual mode
	if len(s.PendingBuf) > 0 && s.PendingBuf[0] == 'g' {
		s.reset()
		switch ch {
		case 'g':
			if count > 0 {
				return Action{Kind: ActionMoveToLine, Line: count}
			}
			return Action{Kind: ActionMoveFileStart}
		}
		return Action{Kind: ActionNone}
	}

	// Operators act on the visual selection
	switch ch {
	case 'd', 'x':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionDelete, MotionType: MotionCharWise, Text: "visual"}
	case 'c', 's':
		s.Mode = ModeInsert
		s.reset()
		return Action{Kind: ActionChange, MotionType: MotionCharWise, Text: "visual"}
	case 'y':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionYank, MotionType: MotionCharWise, Text: "visual"}
	case 'D':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionDelete, MotionType: MotionLineWise, Text: "visual"}
	case 'C', 'S':
		s.Mode = ModeInsert
		s.reset()
		return Action{Kind: ActionChange, MotionType: MotionLineWise, Text: "visual"}
	case 'Y':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionYank, MotionType: MotionLineWise, Text: "visual"}
	case 'p':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionPut, Text: "visual", Register: s.Register}
	case 'J':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionJoinLines, Text: "visual"}
	case '>':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionIndent, Text: "visual"}
	case '<':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionDedent, Text: "visual"}

	// Switch visual sub-mode
	case 'v':
		if s.Mode == ModeVisual {
			s.Mode = ModeNormal
			s.reset()
			return Action{Kind: ActionVisualEscape}
		}
		s.Mode = ModeVisual
		return Action{Kind: ActionVisualStart}
	case 'V':
		if s.Mode == ModeVisualLine {
			s.Mode = ModeNormal
			s.reset()
			return Action{Kind: ActionVisualEscape}
		}
		s.Mode = ModeVisualLine
		return Action{Kind: ActionVisualLineStart}

	// o swaps anchor and cursor
	case 'o':
		s.reset()
		return Action{Kind: ActionNone, Text: "swap_anchor"}

	// Search
	case '/':
		s.PrevMode = s.Mode
		s.Mode = ModeSearch
		s.SearchDir = 1
		s.CommandLine = ""
		s.CommandCursor = 0
		return Action{Kind: ActionEnterSearch}
	case '?':
		s.PrevMode = s.Mode
		s.Mode = ModeSearch
		s.SearchDir = -1
		s.CommandLine = ""
		s.CommandCursor = 0
		return Action{Kind: ActionEnterSearchBack}
	case 'n':
		s.reset()
		return Action{Kind: ActionSearchNext, Count: count}
	case 'N':
		s.reset()
		return Action{Kind: ActionSearchPrev, Count: count}
	case ':':
		s.PrevMode = s.Mode
		s.Mode = ModeCommand
		s.CommandLine = ""
		s.CommandCursor = 0
		return Action{Kind: ActionEnterCommand}
	}

	s.reset()
	return Action{Kind: ActionNone}
}
