package vim

import "testing"

func charInput(ch rune) KeyInput {
	return KeyInput{Char: ch}
}

func ctrlInput(ch rune) KeyInput {
	return KeyInput{Char: ch, Ctrl: true}
}

func namedInput(name string) KeyInput {
	return KeyInput{Name: name}
}

func TestNormalMotions(t *testing.T) {
	tests := []struct {
		name   string
		inputs []KeyInput
		want   ActionKind
		count  int
	}{
		{"h", []KeyInput{charInput('h')}, ActionMoveLeft, 0},
		{"j", []KeyInput{charInput('j')}, ActionMoveDown, 0},
		{"k", []KeyInput{charInput('k')}, ActionMoveUp, 0},
		{"l", []KeyInput{charInput('l')}, ActionMoveRight, 0},
		{"w", []KeyInput{charInput('w')}, ActionMoveWordForward, 0},
		{"b", []KeyInput{charInput('b')}, ActionMoveWordBackward, 0},
		{"e", []KeyInput{charInput('e')}, ActionMoveWordEnd, 0},
		{"0", []KeyInput{charInput('0')}, ActionMoveLineStart, 0},
		{"$", []KeyInput{charInput('$')}, ActionMoveLineEnd, 0},
		{"^", []KeyInput{charInput('^')}, ActionMoveFirstNonBlank, 0},
		{"G", []KeyInput{charInput('G')}, ActionMoveFileEnd, 0},
		{"gg", []KeyInput{charInput('g'), charInput('g')}, ActionMoveFileStart, 0},
		{"{", []KeyInput{charInput('{')}, ActionMoveParagraphUp, 0},
		{"}", []KeyInput{charInput('}')}, ActionMoveParagraphDown, 0},
		{"%", []KeyInput{charInput('%')}, ActionMoveBracketMatch, 0},
		{"3j", []KeyInput{charInput('3'), charInput('j')}, ActionMoveDown, 3},
		{"12l", []KeyInput{charInput('1'), charInput('2'), charInput('l')}, ActionMoveRight, 12},
		{"5G", []KeyInput{charInput('5'), charInput('G')}, ActionMoveToLine, 5},
		{"Ctrl+d", []KeyInput{ctrlInput('d')}, ActionMoveHalfPageDown, 0},
		{"Ctrl+u", []KeyInput{ctrlInput('u')}, ActionMoveHalfPageUp, 0},
		{"Ctrl+f", []KeyInput{ctrlInput('f')}, ActionMovePageDown, 0},
		{"Ctrl+b", []KeyInput{ctrlInput('b')}, ActionMovePageUp, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewState()
			var a Action
			for _, inp := range tt.inputs {
				a = s.HandleKey(inp)
			}
			if a.Kind != tt.want {
				t.Errorf("got ActionKind %d, want %d", a.Kind, tt.want)
			}
			if tt.count > 0 {
				// For MoveToLine, count is stored in Line field
				if a.Kind == ActionMoveToLine {
					if a.Line != tt.count {
						t.Errorf("got line %d, want %d", a.Line, tt.count)
					}
				} else if a.Count != tt.count {
					t.Errorf("got count %d, want %d", a.Count, tt.count)
				}
			}
		})
	}
}

func TestInsertTransitions(t *testing.T) {
	tests := []struct {
		name string
		ch   rune
		want ActionKind
	}{
		{"i", 'i', ActionInsertBefore},
		{"a", 'a', ActionInsertAfter},
		{"I", 'I', ActionInsertLineStart},
		{"A", 'A', ActionInsertLineEnd},
		{"o", 'o', ActionOpenBelow},
		{"O", 'O', ActionOpenAbove},
		{"s", 's', ActionSubstChar},
		{"S", 'S', ActionSubstLine},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewState()
			a := s.HandleKey(charInput(tt.ch))
			if a.Kind != tt.want {
				t.Errorf("got ActionKind %d, want %d", a.Kind, tt.want)
			}
		})
	}
}

func TestInsertEscape(t *testing.T) {
	s := NewState()
	s.Mode = ModeInsert

	a := s.HandleKey(namedInput(NameEscape))
	if a.Kind != ActionEnterNormal {
		t.Errorf("got ActionKind %d, want ActionEnterNormal", a.Kind)
	}
	if s.Mode != ModeNormal {
		t.Errorf("got mode %d, want ModeNormal", s.Mode)
	}
}

func TestOperatorMotion(t *testing.T) {
	tests := []struct {
		name   string
		inputs []KeyInput
		action ActionKind
		motion ActionKind
		mtype  MotionType
	}{
		{"dw", []KeyInput{charInput('d'), charInput('w')}, ActionDelete, ActionMoveWordForward, MotionCharWise},
		{"dd", []KeyInput{charInput('d'), charInput('d')}, ActionDelete, ActionNone, MotionLineWise},
		{"yy", []KeyInput{charInput('y'), charInput('y')}, ActionYank, ActionNone, MotionLineWise},
		{"cc", []KeyInput{charInput('c'), charInput('c')}, ActionChange, ActionNone, MotionLineWise},
		{"d$", []KeyInput{charInput('d'), charInput('$')}, ActionDelete, ActionMoveLineEnd, MotionCharWise},
		{"dj", []KeyInput{charInput('d'), charInput('j')}, ActionDelete, ActionMoveDown, MotionLineWise},
		{"yw", []KeyInput{charInput('y'), charInput('w')}, ActionYank, ActionMoveWordForward, MotionCharWise},
		{">>", []KeyInput{charInput('>'), charInput('>')}, ActionIndent, ActionNone, MotionLineWise},
		{"<<", []KeyInput{charInput('<'), charInput('<')}, ActionDedent, ActionNone, MotionLineWise},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewState()
			var a Action
			for _, inp := range tt.inputs {
				a = s.HandleKey(inp)
			}
			if a.Kind != tt.action {
				t.Errorf("got action %d, want %d", a.Kind, tt.action)
			}
			if a.Motion != tt.motion {
				t.Errorf("got motion %d, want %d", a.Motion, tt.motion)
			}
			if a.MotionType != tt.mtype {
				t.Errorf("got motion type %d, want %d", a.MotionType, tt.mtype)
			}
		})
	}
}

func TestCountedOperator(t *testing.T) {
	s := NewState()
	// 3dd = delete 3 lines
	s.HandleKey(charInput('3'))
	s.HandleKey(charInput('d'))
	a := s.HandleKey(charInput('d'))
	if a.Kind != ActionDelete {
		t.Fatalf("got action %d, want ActionDelete", a.Kind)
	}
	if a.MotionType != MotionLineWise {
		t.Errorf("got motion type %d, want MotionLineWise", a.MotionType)
	}
	if a.Count != 3 {
		t.Errorf("got count %d, want 3", a.Count)
	}
}

func TestTextObject(t *testing.T) {
	tests := []struct {
		name    string
		inputs  []KeyInput
		action  ActionKind
		objType rune
		obj     rune
	}{
		{"ciw", []KeyInput{charInput('c'), charInput('i'), charInput('w')}, ActionChange, 'i', 'w'},
		{"di\"", []KeyInput{charInput('d'), charInput('i'), charInput('"')}, ActionDelete, 'i', '"'},
		{"ya(", []KeyInput{charInput('y'), charInput('a'), charInput('(')}, ActionYank, 'a', '('},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewState()
			var a Action
			for _, inp := range tt.inputs {
				a = s.HandleKey(inp)
			}
			if a.Kind != tt.action {
				t.Errorf("got action %d, want %d", a.Kind, tt.action)
			}
			if a.TextObjType != tt.objType {
				t.Errorf("got text obj type %c, want %c", a.TextObjType, tt.objType)
			}
			if a.TextObj != tt.obj {
				t.Errorf("got text obj %c, want %c", a.TextObj, tt.obj)
			}
		})
	}
}

func TestCommandParsing(t *testing.T) {
	tests := []struct {
		cmd  string
		want ActionKind
		line int
	}{
		{"w", ActionWrite, 0},
		{"q", ActionQuit, 0},
		{"wq", ActionWriteQuit, 0},
		{"q!", ActionForceQuit, 0},
		{"Tutor", ActionTutor, 0},
		{"tutor", ActionTutor, 0},
		{"42", ActionMoveToLine, 42},
		{"1", ActionMoveToLine, 1},
		{"x", ActionWriteQuit, 0},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			a := ParseCommand(tt.cmd)
			if a.Kind != tt.want {
				t.Errorf("got action %d, want %d", a.Kind, tt.want)
			}
			if tt.line > 0 && a.Line != tt.line {
				t.Errorf("got line %d, want %d", a.Line, tt.line)
			}
		})
	}
}

func TestVisualOperators(t *testing.T) {
	s := NewState()
	// v enters visual mode
	a := s.HandleKey(charInput('v'))
	if a.Kind != ActionVisualStart {
		t.Fatalf("got action %d, want ActionVisualStart", a.Kind)
	}
	if s.Mode != ModeNormal {
		// Mode transition happens in the app layer
	}

	// Simulate being in visual mode
	s.Mode = ModeVisual
	a = s.HandleKey(charInput('d'))
	if a.Kind != ActionDelete {
		t.Errorf("got action %d, want ActionDelete", a.Kind)
	}
	if a.Text != "visual" {
		t.Errorf("got text %q, want 'visual'", a.Text)
	}
	if s.Mode != ModeNormal {
		t.Errorf("got mode %d, want ModeNormal", s.Mode)
	}
}

func TestShortcutPassthrough(t *testing.T) {
	s := NewState()
	a := s.HandleKey(KeyInput{Char: 's', Shortcut: true})
	if a.Kind != ActionNone {
		t.Errorf("shortcut keys should return ActionNone, got %d", a.Kind)
	}
}

func TestSearchMode(t *testing.T) {
	s := NewState()
	// / enters search mode
	a := s.HandleKey(charInput('/'))
	if a.Kind != ActionEnterSearch {
		t.Fatalf("got action %d, want ActionEnterSearch", a.Kind)
	}
	if s.Mode != ModeSearch {
		t.Fatalf("got mode %d, want ModeSearch", s.Mode)
	}
	if s.SearchDir != 1 {
		t.Errorf("got search dir %d, want 1", s.SearchDir)
	}

	// Type search text
	s.HandleKey(charInput('f'))
	s.HandleKey(charInput('o'))
	s.HandleKey(charInput('o'))
	if s.CommandLine != "foo" {
		t.Errorf("got command line %q, want 'foo'", s.CommandLine)
	}

	// Enter executes search
	a = s.HandleKey(namedInput(NameReturn))
	if a.Kind != ActionSearchNext {
		t.Errorf("got action %d, want ActionSearchNext", a.Kind)
	}
	if s.SearchPattern != "foo" {
		t.Errorf("got pattern %q, want 'foo'", s.SearchPattern)
	}
	if s.Mode != ModeNormal {
		t.Errorf("got mode %d, want ModeNormal", s.Mode)
	}
}

func TestFindChar(t *testing.T) {
	s := NewState()
	// fa
	s.HandleKey(charInput('f'))
	a := s.HandleKey(charInput('a'))
	if a.Kind != ActionMoveFindChar {
		t.Errorf("got action %d, want ActionMoveFindChar", a.Kind)
	}
	if a.Char != 'a' {
		t.Errorf("got char %c, want 'a'", a.Char)
	}

	// ; repeats
	a = s.HandleKey(charInput(';'))
	if a.Kind != ActionMoveFindChar {
		t.Errorf("got action %d, want ActionMoveFindChar", a.Kind)
	}
	if a.Char != 'a' {
		t.Errorf("repeat should use same char 'a', got %c", a.Char)
	}

	// , reverses
	a = s.HandleKey(charInput(','))
	if a.Kind != ActionMoveFindCharBack {
		t.Errorf("got action %d, want ActionMoveFindCharBack", a.Kind)
	}
}

func TestReplaceChar(t *testing.T) {
	s := NewState()
	s.HandleKey(charInput('r'))
	a := s.HandleKey(charInput('x'))
	if a.Kind != ActionReplace {
		t.Errorf("got action %d, want ActionReplace", a.Kind)
	}
	if a.Char != 'x' {
		t.Errorf("got char %c, want 'x'", a.Char)
	}
}

func TestRegisterFile(t *testing.T) {
	rf := NewRegisterFile()

	// Yank stores in unnamed and yank registers
	rf.RecordYank("hello", '"')
	if rf.Unnamed != "hello" {
		t.Errorf("unnamed = %q, want 'hello'", rf.Unnamed)
	}
	if rf.Yank != "hello" {
		t.Errorf("yank = %q, want 'hello'", rf.Yank)
	}

	// Delete shifts numbered registers
	rf.RecordDelete("line1\n", true, '"')
	if rf.Delete[0] != "line1\n" {
		t.Errorf("delete[0] = %q, want 'line1\\n'", rf.Delete[0])
	}

	rf.RecordDelete("line2\n", true, '"')
	if rf.Delete[0] != "line2\n" {
		t.Errorf("delete[0] = %q, want 'line2\\n'", rf.Delete[0])
	}
	if rf.Delete[1] != "line1\n" {
		t.Errorf("delete[1] = %q, want 'line1\\n'", rf.Delete[1])
	}

	// Small delete (no newline)
	rf.RecordDelete("word", false, '"')
	if rf.Small != "word" {
		t.Errorf("small = %q, want 'word'", rf.Small)
	}

	// Named register
	rf.Set('a', "test")
	if rf.Get('a') != "test" {
		t.Errorf("register a = %q, want 'test'", rf.Get('a'))
	}

	// Append with uppercase
	rf.Set('A', " more")
	if rf.Get('a') != "test more" {
		t.Errorf("register a = %q, want 'test more'", rf.Get('a'))
	}
}

func TestDotRepeat(t *testing.T) {
	s := NewState()
	a := s.HandleKey(charInput('.'))
	if a.Kind != ActionRepeatLast {
		t.Errorf("got action %d, want ActionRepeatLast", a.Kind)
	}
}

func TestScrollCommands(t *testing.T) {
	s := NewState()
	s.HandleKey(charInput('z'))
	a := s.HandleKey(charInput('z'))
	if a.Kind != ActionScrollCenter {
		t.Errorf("zz: got action %d, want ActionScrollCenter", a.Kind)
	}

	s = NewState()
	s.HandleKey(charInput('z'))
	a = s.HandleKey(charInput('t'))
	if a.Kind != ActionScrollTop {
		t.Errorf("zt: got action %d, want ActionScrollTop", a.Kind)
	}
}
