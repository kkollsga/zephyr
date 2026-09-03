package vim

// handleVisual processes key inputs in Visual, Visual Line, or Visual Block mode.
func (s *State) handleVisual(ev KeyInput) Action {
	ch := ev.Char

	// i/a is waiting for its delimiter (viw, va"). Ahead of the Escape branch:
	// Escape cancels the pending key and leaves the selection up, as in vim.
	if s.WaitingForTextObj {
		return s.handleVisualTextObj(ev)
	}

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

	// A pending g consumes this key. This has to precede visualMotionKey, whose
	// `case 'g'` would otherwise re-arm the prefix on the second g of gg.
	if len(s.PendingBuf) > 0 && s.PendingBuf[0] == 'g' {
		s.reset()
		if ch == 'g' {
			if count > 0 {
				return Action{Kind: ActionMoveToLine, Line: count}
			}
			return Action{Kind: ActionMoveFileStart}
		}
		return Action{Kind: ActionNone}
	}

	if a, ok := s.visualMotionKey(ch, count); ok {
		return a
	}
	if a, ok := s.visualOperatorKey(ch); ok {
		return a
	}
	if a, ok := s.visualModeKey(ch, count); ok {
		return a
	}

	s.reset()
	return Action{Kind: ActionNone}
}

// visualMotionKey handles the keys that move the cursor, and so extend the
// selection, plus the prefixes (g, i, a) that wait for a second key.
func (s *State) visualMotionKey(ch rune, count int) (Action, bool) {
	switch ch {
	case 'h':
		s.reset()
		return Action{Kind: ActionMoveLeft, Count: count}, true
	case 'j':
		s.reset()
		return Action{Kind: ActionMoveDown, Count: count}, true
	case 'k':
		s.reset()
		return Action{Kind: ActionMoveUp, Count: count}, true
	case 'l':
		s.reset()
		return Action{Kind: ActionMoveRight, Count: count}, true
	case 'w':
		s.reset()
		return Action{Kind: ActionMoveWordForward, Count: count}, true
	case 'b':
		s.reset()
		return Action{Kind: ActionMoveWordBackward, Count: count}, true
	case 'e':
		s.reset()
		return Action{Kind: ActionMoveWordEnd, Count: count}, true
	case '0':
		s.reset()
		return Action{Kind: ActionMoveLineStart}, true
	case '$':
		s.reset()
		return Action{Kind: ActionMoveLineEnd}, true
	case '^':
		s.reset()
		return Action{Kind: ActionMoveFirstNonBlank}, true
	case 'G':
		s.reset()
		if count > 0 {
			return Action{Kind: ActionMoveToLine, Line: count}, true
		}
		return Action{Kind: ActionMoveFileEnd}, true
	case '{':
		s.reset()
		return Action{Kind: ActionMoveParagraphUp, Count: count}, true
	case '}':
		s.reset()
		return Action{Kind: ActionMoveParagraphDown, Count: count}, true
	case '%':
		s.reset()
		return Action{Kind: ActionMoveBracketMatch}, true
	case 'g':
		s.PendingBuf = "g"
		return Action{Kind: ActionNone}, true
	case 'i', 'a':
		s.WaitingForTextObj = true
		s.WaitingForTextObjType = ch
		s.PendingBuf += string(ch)
		return Action{Kind: ActionNone}, true
	}
	return Action{}, false
}

// visualOperatorKey handles the operators that act on the current selection and
// leave visual mode.
func (s *State) visualOperatorKey(ch rune) (Action, bool) {
	switch ch {
	case 'd', 'x':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionDelete, MotionType: MotionCharWise, Text: "visual"}, true
	case 'c', 's':
		s.Mode = ModeInsert
		s.reset()
		return Action{Kind: ActionChange, MotionType: MotionCharWise, Text: "visual"}, true
	case 'y':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionYank, MotionType: MotionCharWise, Text: "visual"}, true
	case 'D':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionDelete, MotionType: MotionLineWise, Text: "visual"}, true
	case 'C', 'S':
		s.Mode = ModeInsert
		s.reset()
		return Action{Kind: ActionChange, MotionType: MotionLineWise, Text: "visual"}, true
	case 'Y':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionYank, MotionType: MotionLineWise, Text: "visual"}, true
	case 'p':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionPut, Text: "visual", Register: s.Register}, true
	case 'J':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionJoinLines, Text: "visual"}, true
	case '>':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionIndent, Text: "visual"}, true
	case '<':
		s.Mode = ModeNormal
		s.reset()
		return Action{Kind: ActionDedent, Text: "visual"}, true
	}
	return Action{}, false
}

// visualModeKey handles the sub-mode switches, the anchor swap and the
// search/command-line entries.
func (s *State) visualModeKey(ch rune, count int) (Action, bool) {
	switch ch {
	case 'v':
		if s.Mode == ModeVisual {
			s.Mode = ModeNormal
			s.reset()
			return Action{Kind: ActionVisualEscape}, true
		}
		s.Mode = ModeVisual
		return Action{Kind: ActionVisualStart}, true
	case 'V':
		if s.Mode == ModeVisualLine {
			s.Mode = ModeNormal
			s.reset()
			return Action{Kind: ActionVisualEscape}, true
		}
		s.Mode = ModeVisualLine
		return Action{Kind: ActionVisualLineStart}, true

	// o swaps anchor and cursor
	case 'o':
		s.reset()
		return Action{Kind: ActionNone, Text: "swap_anchor"}, true

	case '/':
		s.PrevMode = s.Mode
		s.Mode = ModeSearch
		s.SearchDir = 1
		s.CommandLine = ""
		s.CommandCursor = 0
		return Action{Kind: ActionEnterSearch}, true
	case '?':
		s.PrevMode = s.Mode
		s.Mode = ModeSearch
		s.SearchDir = -1
		s.CommandLine = ""
		s.CommandCursor = 0
		return Action{Kind: ActionEnterSearchBack}, true
	case 'n':
		s.reset()
		return Action{Kind: ActionSearchNext, Count: count}, true
	case 'N':
		s.reset()
		return Action{Kind: ActionSearchPrev, Count: count}, true
	case ':':
		s.PrevMode = s.Mode
		s.Mode = ModeCommand
		s.CommandLine = ""
		s.CommandCursor = 0
		return Action{Kind: ActionEnterCommand}, true
	}
	return Action{}, false
}

// handleVisualTextObj processes the delimiter after i/a in visual mode. The
// object replaces the selection instead of being operated on, so the mode stays
// visual and the executor gets ActionSelectTextObject.
func (s *State) handleVisualTextObj(ev KeyInput) Action {
	if ev.Name == NameEscape {
		s.reset()
		return Action{Kind: ActionNone}
	}
	ch := ev.Char
	if ch == 0 {
		return Action{Kind: ActionNone}
	}
	objType := s.WaitingForTextObjType
	s.reset()
	if !acceptedTextObj(ch, objType) {
		return Action{Kind: ActionNone}
	}
	return Action{Kind: ActionSelectTextObject, TextObj: ch, TextObjType: objType}
}
