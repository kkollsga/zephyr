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

// TestHandleKeyCtrlBindingsSurviveShortcutModifier pins the platform rule:
// Gio reports the shortcut modifier as Cmd on macOS but as Ctrl everywhere
// else, so a Ctrl binding off macOS arrives with both flags set. It must still
// reach the Ctrl handler instead of being discarded as a host accelerator.
func TestHandleKeyCtrlBindingsSurviveShortcutModifier(t *testing.T) {
	s := NewState()
	if got := s.HandleKey(KeyInput{Char: 'd', Ctrl: true, Shortcut: true}); got.Kind != ActionMoveHalfPageDown {
		t.Errorf("Ctrl+d with the shortcut modifier = %v, want ActionMoveHalfPageDown", got.Kind)
	}
	if got := s.HandleKey(KeyInput{Char: 'r', Ctrl: true, Shortcut: true}); got.Kind != ActionRedo {
		t.Errorf("Ctrl+r with the shortcut modifier = %v, want ActionRedo", got.Kind)
	}
	// A Cmd-style accelerator carries no Ctrl and belongs to the host.
	if got := s.HandleKey(KeyInput{Char: 's', Shortcut: true}); got.Kind != ActionNone {
		t.Errorf("Cmd+s = %v, want ActionNone", got.Kind)
	}
	// So does a Ctrl combination vim has no binding for.
	if got := s.HandleKey(KeyInput{Char: 's', Ctrl: true, Shortcut: true}); got.Kind != ActionNone {
		t.Errorf("Ctrl+s = %v, want ActionNone", got.Kind)
	}
}

// TestUnimplementedTextObjectIsInert pins the rule that an accepted delimiter
// must have an executor behind it. The tag object `t` parsed into a complete
// delete action that nothing could carry out, so `dit` consumed the keys and
// reported success while changing nothing.
func TestUnimplementedTextObjectIsInert(t *testing.T) {
	for _, objType := range []rune{'i', 'a'} {
		s := NewState()
		var a Action
		for _, inp := range []KeyInput{charInput('d'), charInput(objType), charInput('t')} {
			a = s.HandleKey(inp)
		}
		if a.Kind != ActionNone {
			t.Errorf("d%ct = %d, want ActionNone", objType, a.Kind)
		}
	}
	// The hunk object is inner-only; `dah` has no reading that excludes context
	// lines, so it must not parse into a delete either.
	s := NewState()
	var a Action
	for _, inp := range []KeyInput{charInput('d'), charInput('a'), charInput('h')} {
		a = s.HandleKey(inp)
	}
	if a.Kind != ActionNone {
		t.Errorf("dah = %d, want ActionNone", a.Kind)
	}

	s = NewState()
	for _, inp := range []KeyInput{charInput('d'), charInput('i'), charInput('h')} {
		a = s.HandleKey(inp)
	}
	if a.Kind != ActionDelete || a.TextObj != 'h' {
		t.Errorf("dih = kind %d obj %c, want ActionDelete obj h", a.Kind, a.TextObj)
	}
}

// A host accelerator is not the character a pending key is waiting for. Off
// macOS the shortcut modifier is Ctrl, so Ctrl+S arrives as a printable 's'
// with Ctrl and Shortcut set: r then Ctrl+S must save, not replace the
// character under the cursor with an "s".
func TestPendingCharStatesReleaseHostAccelerators(t *testing.T) {
	// Ctrl+S as the host sends it on Windows and Linux, and Cmd+S on macOS.
	accels := map[string]KeyInput{
		"ctrl+s":     {Char: 's', Ctrl: true, Shortcut: true},
		"shortcut+s": {Char: 's', Shortcut: true},
	}
	pending := map[string][]KeyInput{
		"r":  {charInput('r')},
		"f":  {charInput('f')},
		"t":  {charInput('t')},
		"F":  {charInput('F')},
		"T":  {charInput('T')},
		"df": {charInput('d'), charInput('f')},
		"ct": {charInput('c'), charInput('t')},
	}
	for keys, prefix := range pending {
		for name, accel := range accels {
			s := NewState()
			for _, k := range prefix {
				s.HandleKey(k)
			}
			if a := s.HandleKey(accel); a.Kind != ActionNone {
				t.Errorf("%s then %s produced action %d (char %q), want ActionNone so the host can act on it",
					keys, name, a.Kind, a.Char)
			}
			if s.WaitingForChar {
				t.Errorf("%s then %s left the pending key waiting; the next typed character would be eaten", keys, name)
			}
			// The state is usable again: a plain x replaces, it is not swallowed.
			s.HandleKey(charInput('r'))
			if a := s.HandleKey(charInput('x')); a.Kind != ActionReplace || a.Char != 'x' {
				t.Errorf("after %s then %s, rx gave action %d char %q, want ActionReplace x", keys, name, a.Kind, a.Char)
			}
		}
	}
}

// The same rule covers the states that wait on something other than a
// character: an operator waiting for a motion, i/a waiting for a delimiter,
// and the two-key sequences. A bare count is not cancelled — it consumes
// nothing on its own.
func TestPendingOperatorsReleaseHostAccelerators(t *testing.T) {
	for _, keys := range []string{"d", "y", "di", "g", "z", "2d"} {
		for _, accel := range []KeyInput{
			{Char: 's', Ctrl: true, Shortcut: true},
			{Char: 's', Shortcut: true},
		} {
			s := NewState()
			for _, c := range keys {
				s.HandleKey(charInput(c))
			}
			if a := s.HandleKey(accel); a.Kind != ActionNone {
				t.Errorf("%s then a host accelerator produced action %d, want ActionNone", keys, a.Kind)
			}
			a := s.HandleKey(charInput('x'))
			if a.Kind != ActionDelete || a.MotionType == MotionLineWise {
				t.Errorf("after %s then a host accelerator, x gave action %d motion %d, want a plain character delete", keys, a.Kind, a.MotionType)
			}
		}
	}

	// A half-typed count survives: nothing is waiting on the next key.
	s := NewState()
	s.HandleKey(charInput('2'))
	s.HandleKey(KeyInput{Char: 's', Shortcut: true})
	if a := s.HandleKey(charInput('x')); a.Count != 2 {
		t.Errorf("a count typed before the accelerator was lost: count %d, want 2", a.Count)
	}
}
