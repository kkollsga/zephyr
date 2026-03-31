package vim

// handleBracketNext handles ]-prefix sequences for forward navigation.
func (s *State) handleBracketNext(ev KeyInput) Action {
	count := s.Count
	s.reset()
	switch ev.Char {
	case 'c':
		return Action{Kind: ActionNavNextHunk, Count: count}
	case 'C':
		return Action{Kind: ActionNavNextFile, Count: count}
	}
	return Action{Kind: ActionNone}
}

// handleBracketPrev handles [-prefix sequences for backward navigation.
func (s *State) handleBracketPrev(ev KeyInput) Action {
	count := s.Count
	s.reset()
	switch ev.Char {
	case 'c':
		return Action{Kind: ActionNavPrevHunk, Count: count}
	case 'C':
		return Action{Kind: ActionNavPrevFile, Count: count}
	}
	return Action{Kind: ActionNone}
}

// handleLeaderSequence handles <Space>-prefix sequences (navigator leader key).
func (s *State) handleLeaderSequence(ev KeyInput) Action {
	count := s.Count
	s.reset()
	switch ev.Char {
	case 'g':
		return Action{Kind: ActionNavOpenStatus}
	case 'f':
		return Action{Kind: ActionNavFindFiles}
	case 'b':
		return Action{Kind: ActionNavFindChanged}
	case 'e':
		return Action{Kind: ActionNavOpenRoot}
	// Hunk/file navigation — ergonomic alternative to ]c/[c/]C/[C
	case 'c':
		return Action{Kind: ActionNavNextHunk, Count: count}
	case 'C':
		return Action{Kind: ActionNavPrevHunk, Count: count}
	case 'n':
		return Action{Kind: ActionNavNextFile, Count: count}
	case 'N':
		return Action{Kind: ActionNavPrevFile, Count: count}
	}
	return Action{Kind: ActionNone}
}
