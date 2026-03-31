package vim

// handleOperatorPending processes key inputs when an operator (d, c, y, >, <) is pending.
func (s *State) handleOperatorPending(ev KeyInput) Action {
	// Handle waiting for char in operator context (e.g., df{char})
	if s.WaitingForChar {
		return s.handleCharWaitOperator(ev)
	}

	// Handle text object delimiter (e.g., diw, ci")
	if s.WaitingForTextObj {
		return s.handleTextObjDelimiter(ev)
	}

	ch := ev.Char

	// Escape cancels
	if ev.Name == NameEscape {
		s.reset()
		return Action{Kind: ActionNone}
	}

	// Count accumulation within operator pending (e.g., d2w)
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

	op := s.Operator
	count := s.Count

	// Double operator = line-wise (dd, yy, cc, >>, <<)
	if s.isOperatorChar(ch) {
		s.reset()
		return Action{
			Kind:       opToAction(op),
			Motion:     ActionNone,
			MotionType: MotionLineWise,
			Count:      count,
		}
	}

	// Text objects: i or a followed by delimiter
	if ch == 'i' || ch == 'a' {
		s.WaitingForTextObj = true
		s.WaitingForTextObjType = ch
		s.PendingBuf += string(ch)
		return Action{Kind: ActionNone}
	}

	// f/t/F/T in operator context
	if ch == 'f' || ch == 't' || ch == 'F' || ch == 'T' {
		s.WaitingForChar = true
		s.WaitingForCharType = ch
		s.PendingBuf += string(ch)
		return Action{Kind: ActionNone}
	}

	// Motion keys complete the operator
	motion, motionType := s.charToMotion(ev)
	if motion != ActionNone {
		s.reset()
		return Action{
			Kind:       opToAction(op),
			Motion:     motion,
			MotionType: motionType,
			Count:      count,
			Char:       ev.Char,
		}
	}

	// Unknown — cancel
	s.reset()
	return Action{Kind: ActionNone}
}

// handleCharWaitOperator processes f/t/F/T target char in operator-pending mode.
func (s *State) handleCharWaitOperator(ev KeyInput) Action {
	if ev.Name == NameEscape {
		s.reset()
		return Action{Kind: ActionNone}
	}
	ch := ev.Char
	if ch == 0 {
		return Action{Kind: ActionNone}
	}

	charType := s.WaitingForCharType
	op := s.Operator
	count := s.Count
	s.WaitingForChar = false

	var motion ActionKind
	switch charType {
	case 'f':
		motion = ActionMoveFindChar
		s.FindChar = ch
		s.FindCharForward = true
		s.FindCharTill = false
	case 'F':
		motion = ActionMoveFindCharBack
		s.FindChar = ch
		s.FindCharForward = false
		s.FindCharTill = false
	case 't':
		motion = ActionMoveTillChar
		s.FindChar = ch
		s.FindCharForward = true
		s.FindCharTill = true
	case 'T':
		motion = ActionMoveTillCharBack
		s.FindChar = ch
		s.FindCharForward = false
		s.FindCharTill = true
	default:
		s.reset()
		return Action{Kind: ActionNone}
	}

	s.reset()
	return Action{
		Kind:       opToAction(op),
		Motion:     motion,
		MotionType: MotionCharWise,
		Count:      count,
		Char:       ch,
	}
}

// handleTextObjDelimiter processes the delimiter after i/a in operator-pending mode.
func (s *State) handleTextObjDelimiter(ev KeyInput) Action {
	if ev.Name == NameEscape {
		s.reset()
		return Action{Kind: ActionNone}
	}
	ch := ev.Char
	if ch == 0 {
		return Action{Kind: ActionNone}
	}

	op := s.Operator
	count := s.Count
	objType := s.WaitingForTextObjType
	s.reset()

	// Valid text object delimiters
	switch ch {
	case 'w', 'W', '"', '\'', '`', '(', ')', '[', ']', '{', '}', '<', '>', 'b', 'B', 't', 'h':
		return Action{
			Kind:        opToAction(op),
			MotionType:  MotionCharWise,
			Count:       count,
			TextObj:     ch,
			TextObjType: objType,
		}
	}

	return Action{Kind: ActionNone}
}

// isOperatorChar checks if the char matches the current pending operator.
func (s *State) isOperatorChar(ch rune) bool {
	switch s.Operator {
	case OpDelete:
		return ch == 'd'
	case OpChange:
		return ch == 'c'
	case OpYank:
		return ch == 'y'
	case OpIndent:
		return ch == '>'
	case OpDedent:
		return ch == '<'
	}
	return false
}

// charToMotion maps a character to a motion action.
func (s *State) charToMotion(ev KeyInput) (ActionKind, MotionType) {
	ch := ev.Char

	// Ctrl combinations
	if ev.Ctrl {
		switch ch {
		case 'd':
			return ActionMoveHalfPageDown, MotionLineWise
		case 'u':
			return ActionMoveHalfPageUp, MotionLineWise
		case 'f':
			return ActionMovePageDown, MotionLineWise
		case 'b':
			return ActionMovePageUp, MotionLineWise
		}
		return ActionNone, MotionCharWise
	}

	switch ch {
	case 'h':
		return ActionMoveLeft, MotionCharWise
	case 'l':
		return ActionMoveRight, MotionCharWise
	case 'j':
		return ActionMoveDown, MotionLineWise
	case 'k':
		return ActionMoveUp, MotionLineWise
	case 'w':
		return ActionMoveWordForward, MotionCharWise
	case 'b':
		return ActionMoveWordBackward, MotionCharWise
	case 'e':
		return ActionMoveWordEnd, MotionCharWise
	case 'W':
		return ActionMoveBigWordFwd, MotionCharWise
	case 'B':
		return ActionMoveBigWordBack, MotionCharWise
	case 'E':
		return ActionMoveBigWordEnd, MotionCharWise
	case '0':
		return ActionMoveLineStart, MotionCharWise
	case '$':
		return ActionMoveLineEnd, MotionCharWise
	case '^':
		return ActionMoveFirstNonBlank, MotionCharWise
	case 'G':
		if s.Count > 0 {
			return ActionMoveToLine, MotionLineWise
		}
		return ActionMoveFileEnd, MotionLineWise
	case '{':
		return ActionMoveParagraphUp, MotionLineWise
	case '}':
		return ActionMoveParagraphDown, MotionLineWise
	case '%':
		return ActionMoveBracketMatch, MotionCharWise
	}

	// g-prefix in operator context
	if ch == 'g' {
		s.PendingBuf += "g"
		return ActionNone, MotionCharWise
	}

	return ActionNone, MotionCharWise
}

// opToAction maps an Operator to the corresponding ActionKind.
func opToAction(op Operator) ActionKind {
	switch op {
	case OpDelete:
		return ActionDelete
	case OpChange:
		return ActionChange
	case OpYank:
		return ActionYank
	case OpIndent:
		return ActionIndent
	case OpDedent:
		return ActionDedent
	}
	return ActionNone
}
