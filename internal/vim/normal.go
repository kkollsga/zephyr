package vim

// handleNormal processes key inputs in Normal mode.
func (s *State) handleNormal(ev KeyInput) Action {
	// Handle waiting-for-char state (f, t, F, T, r)
	if s.WaitingForChar {
		return s.handleCharWait(ev)
	}

	// Handle waiting-for-text-object (i or a after operator)
	if s.WaitingForTextObj {
		return s.handleTextObjDelimiter(ev)
	}

	// Handle operator pending
	if s.Operator != OpNone {
		return s.handleOperatorPending(ev)
	}

	// Handle two-key sequences (g_, z_)
	if len(s.PendingBuf) > 0 && s.PendingBuf[0] == 'g' {
		return s.handleGSequence(ev)
	}
	if len(s.PendingBuf) > 0 && s.PendingBuf[0] == 'z' {
		return s.handleZSequence(ev)
	}

	ch := ev.Char

	// Ctrl key combinations
	if ev.Ctrl {
		return s.handleCtrlKey(ev)
	}

	// Count accumulation — digits build up the count prefix
	if ch >= '1' && ch <= '9' {
		s.Count = s.Count*10 + int(ch-'0')
		s.PendingBuf += string(ch)
		return Action{Kind: ActionNone}
	}
	if ch == '0' && s.Count > 0 {
		s.Count = s.Count * 10
		s.PendingBuf += "0"
		return Action{Kind: ActionNone}
	}

	count := s.Count

	switch ch {
	// Basic motions
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

	// Word motions
	case 'w':
		s.reset()
		return Action{Kind: ActionMoveWordForward, Count: count}
	case 'b':
		s.reset()
		return Action{Kind: ActionMoveWordBackward, Count: count}
	case 'e':
		s.reset()
		return Action{Kind: ActionMoveWordEnd, Count: count}
	case 'W':
		s.reset()
		return Action{Kind: ActionMoveBigWordFwd, Count: count}
	case 'B':
		s.reset()
		return Action{Kind: ActionMoveBigWordBack, Count: count}
	case 'E':
		s.reset()
		return Action{Kind: ActionMoveBigWordEnd, Count: count}

	// Line motions
	case '0':
		s.reset()
		return Action{Kind: ActionMoveLineStart}
	case '$':
		s.reset()
		return Action{Kind: ActionMoveLineEnd, Count: count}
	case '^':
		s.reset()
		return Action{Kind: ActionMoveFirstNonBlank}

	// File motions
	case 'G':
		if count > 0 {
			s.reset()
			return Action{Kind: ActionMoveToLine, Line: count}
		}
		s.reset()
		return Action{Kind: ActionMoveFileEnd}

	// Paragraph motions
	case '{':
		s.reset()
		return Action{Kind: ActionMoveParagraphUp, Count: count}
	case '}':
		s.reset()
		return Action{Kind: ActionMoveParagraphDown, Count: count}

	// Bracket match
	case '%':
		s.reset()
		return Action{Kind: ActionMoveBracketMatch}

	// Operators
	case 'd':
		s.Operator = OpDelete
		s.PendingBuf += "d"
		return Action{Kind: ActionNone}
	case 'c':
		s.Operator = OpChange
		s.PendingBuf += "c"
		return Action{Kind: ActionNone}
	case 'y':
		s.Operator = OpYank
		s.PendingBuf += "y"
		return Action{Kind: ActionNone}
	case '>':
		s.Operator = OpIndent
		s.PendingBuf += ">"
		return Action{Kind: ActionNone}
	case '<':
		s.Operator = OpDedent
		s.PendingBuf += "<"
		return Action{Kind: ActionNone}

	// Shorthand operators
	case 'D': // D = d$
		s.reset()
		return Action{Kind: ActionDelete, Motion: ActionMoveLineEnd, MotionType: MotionCharWise, Count: count}
	case 'C': // C = c$
		s.reset()
		return Action{Kind: ActionChange, Motion: ActionMoveLineEnd, MotionType: MotionCharWise, Count: count}
	case 'Y': // Y = yy
		s.reset()
		return Action{Kind: ActionYank, Motion: ActionNone, MotionType: MotionLineWise, Count: count}

	// Insert mode transitions
	case 'i':
		s.reset()
		return Action{Kind: ActionInsertBefore}
	case 'a':
		s.reset()
		return Action{Kind: ActionInsertAfter}
	case 'I':
		s.reset()
		return Action{Kind: ActionInsertLineStart}
	case 'A':
		s.reset()
		return Action{Kind: ActionInsertLineEnd}
	case 'o':
		s.reset()
		return Action{Kind: ActionOpenBelow}
	case 'O':
		s.reset()
		return Action{Kind: ActionOpenAbove}
	case 's':
		s.reset()
		return Action{Kind: ActionSubstChar, Count: count}
	case 'S':
		s.reset()
		return Action{Kind: ActionSubstLine, Count: count}

	// Delete/put
	case 'x':
		s.reset()
		return Action{Kind: ActionDelete, Motion: ActionMoveRight, MotionType: MotionCharWise, Count: count}
	case 'X':
		s.reset()
		return Action{Kind: ActionDelete, Motion: ActionMoveLeft, MotionType: MotionCharWise, Count: count}
	case 'p':
		s.reset()
		return Action{Kind: ActionPut, Count: count, Register: s.Register}
	case 'P':
		s.reset()
		return Action{Kind: ActionPutBefore, Count: count, Register: s.Register}

	// Undo/redo
	case 'u':
		s.reset()
		return Action{Kind: ActionUndo, Count: count}

	// Join
	case 'J':
		s.reset()
		return Action{Kind: ActionJoinLines, Count: count}

	// Dot repeat
	case '.':
		s.reset()
		return Action{Kind: ActionRepeatLast, Count: count}

	// Replace
	case 'r':
		s.WaitingForChar = true
		s.WaitingForCharType = 'r'
		s.PendingBuf += "r"
		return Action{Kind: ActionNone}

	// Find char
	case 'f':
		s.WaitingForChar = true
		s.WaitingForCharType = 'f'
		s.PendingBuf += "f"
		return Action{Kind: ActionNone}
	case 'F':
		s.WaitingForChar = true
		s.WaitingForCharType = 'F'
		s.PendingBuf += "F"
		return Action{Kind: ActionNone}
	case 't':
		s.WaitingForChar = true
		s.WaitingForCharType = 't'
		s.PendingBuf += "t"
		return Action{Kind: ActionNone}
	case 'T':
		s.WaitingForChar = true
		s.WaitingForCharType = 'T'
		s.PendingBuf += "T"
		return Action{Kind: ActionNone}
	case ';':
		s.reset()
		return s.repeatFindChar(false)
	case ',':
		s.reset()
		return s.repeatFindChar(true)

	// Search
	case '/':
		s.PrevMode = s.Mode
		s.Mode = ModeSearch
		s.SearchDir = 1
		s.CommandLine = ""
		s.CommandCursor = 0
		s.reset()
		return Action{Kind: ActionEnterSearch}
	case '?':
		s.PrevMode = s.Mode
		s.Mode = ModeSearch
		s.SearchDir = -1
		s.CommandLine = ""
		s.CommandCursor = 0
		s.reset()
		return Action{Kind: ActionEnterSearchBack}
	case 'n':
		s.reset()
		return Action{Kind: ActionSearchNext, Count: count}
	case 'N':
		s.reset()
		return Action{Kind: ActionSearchPrev, Count: count}
	case '*':
		s.reset()
		return Action{Kind: ActionSearchWordUnder, Count: count}

	// Command mode
	case ':':
		s.PrevMode = s.Mode
		s.Mode = ModeCommand
		s.CommandLine = ""
		s.CommandCursor = 0
		s.reset()
		return Action{Kind: ActionEnterCommand}

	// Visual mode
	case 'v':
		s.reset()
		return Action{Kind: ActionVisualStart}
	case 'V':
		s.reset()
		return Action{Kind: ActionVisualLineStart}

	// g-prefix sequences
	case 'g':
		s.PendingBuf += "g"
		return Action{Kind: ActionNone}

	// z-prefix sequences
	case 'z':
		s.PendingBuf += "z"
		return Action{Kind: ActionNone}
	}

	// Named keys
	switch ev.Name {
	case NameEscape:
		s.reset()
		return Action{Kind: ActionNone}
	}

	// Unknown key — reset pending state
	s.reset()
	return Action{Kind: ActionNone}
}

// handleCtrlKey handles Ctrl+key combinations in Normal mode.
func (s *State) handleCtrlKey(ev KeyInput) Action {
	count := s.Count
	s.reset()
	switch ev.Char {
	case 'd':
		return Action{Kind: ActionMoveHalfPageDown, Count: count}
	case 'u':
		return Action{Kind: ActionMoveHalfPageUp, Count: count}
	case 'f':
		return Action{Kind: ActionMovePageDown, Count: count}
	case 'b':
		return Action{Kind: ActionMovePageUp, Count: count}
	case 'r':
		return Action{Kind: ActionRedo, Count: count}
	case 'v':
		return Action{Kind: ActionVisualBlockStart}
	}
	return Action{Kind: ActionNone}
}

// handleGSequence handles g-prefix two-key sequences.
func (s *State) handleGSequence(ev KeyInput) Action {
	count := s.Count
	s.reset()
	switch ev.Char {
	case 'g': // gg = go to file start (or line N)
		if count > 0 {
			return Action{Kind: ActionMoveToLine, Line: count}
		}
		return Action{Kind: ActionMoveFileStart}
	case 'd': // gd = go to definition
		// Placeholder — requires LSP integration
		return Action{Kind: ActionNone}
	case 'h': // gh = show hover
		// Placeholder — requires LSP integration
		return Action{Kind: ActionNone}
	}
	return Action{Kind: ActionNone}
}

// handleZSequence handles z-prefix two-key sequences.
func (s *State) handleZSequence(ev KeyInput) Action {
	s.reset()
	switch ev.Char {
	case 'z':
		return Action{Kind: ActionScrollCenter}
	case 't':
		return Action{Kind: ActionScrollTop}
	case 'b':
		return Action{Kind: ActionScrollBottom}
	}
	return Action{Kind: ActionNone}
}

// handleCharWait processes the character after f/t/F/T/r.
func (s *State) handleCharWait(ev KeyInput) Action {
	if ev.Name == NameEscape {
		s.WaitingForChar = false
		s.reset()
		return Action{Kind: ActionNone}
	}

	ch := ev.Char
	if ch == 0 {
		return Action{Kind: ActionNone}
	}

	charType := s.WaitingForCharType
	count := s.Count
	s.WaitingForChar = false

	switch charType {
	case 'r':
		s.reset()
		return Action{Kind: ActionReplace, Char: ch, Count: count}
	case 'f':
		s.FindChar = ch
		s.FindCharForward = true
		s.FindCharTill = false
		s.reset()
		return Action{Kind: ActionMoveFindChar, Char: ch, Count: count}
	case 'F':
		s.FindChar = ch
		s.FindCharForward = false
		s.FindCharTill = false
		s.reset()
		return Action{Kind: ActionMoveFindCharBack, Char: ch, Count: count}
	case 't':
		s.FindChar = ch
		s.FindCharForward = true
		s.FindCharTill = true
		s.reset()
		return Action{Kind: ActionMoveTillChar, Char: ch, Count: count}
	case 'T':
		s.FindChar = ch
		s.FindCharForward = false
		s.FindCharTill = true
		s.reset()
		return Action{Kind: ActionMoveTillCharBack, Char: ch, Count: count}
	}

	s.reset()
	return Action{Kind: ActionNone}
}

// repeatFindChar generates the action for ; or , (repeat last f/t/F/T).
func (s *State) repeatFindChar(reverse bool) Action {
	if s.FindChar == 0 {
		return Action{Kind: ActionNone}
	}
	fwd := s.FindCharForward
	if reverse {
		fwd = !fwd
	}
	till := s.FindCharTill
	if fwd && !till {
		return Action{Kind: ActionMoveFindChar, Char: s.FindChar}
	}
	if fwd && till {
		return Action{Kind: ActionMoveTillChar, Char: s.FindChar}
	}
	if !fwd && !till {
		return Action{Kind: ActionMoveFindCharBack, Char: s.FindChar}
	}
	return Action{Kind: ActionMoveTillCharBack, Char: s.FindChar}
}
