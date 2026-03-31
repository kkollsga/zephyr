package vim

import "testing"

func TestBracketSequences(t *testing.T) {
	tests := []struct {
		name   string
		inputs []KeyInput
		want   ActionKind
		count  int
	}{
		{"]c", []KeyInput{charInput(']'), charInput('c')}, ActionNavNextHunk, 0},
		{"[c", []KeyInput{charInput('['), charInput('c')}, ActionNavPrevHunk, 0},
		{"]C", []KeyInput{charInput(']'), charInput('C')}, ActionNavNextFile, 0},
		{"[C", []KeyInput{charInput('['), charInput('C')}, ActionNavPrevFile, 0},
		{"3]c", []KeyInput{charInput('3'), charInput(']'), charInput('c')}, ActionNavNextHunk, 3},
		{"2[c", []KeyInput{charInput('2'), charInput('['), charInput('c')}, ActionNavPrevHunk, 2},
		{"]x unknown", []KeyInput{charInput(']'), charInput('x')}, ActionNone, 0},
		{"[x unknown", []KeyInput{charInput('['), charInput('x')}, ActionNone, 0},
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
			if tt.count > 0 && a.Count != tt.count {
				t.Errorf("got count %d, want %d", a.Count, tt.count)
			}
		})
	}
}

func TestLeaderSequences_Enabled(t *testing.T) {
	tests := []struct {
		name   string
		inputs []KeyInput
		want   ActionKind
	}{
		{"<Space>g", []KeyInput{charInput(' '), charInput('g')}, ActionNavOpenStatus},
		{"<Space>f", []KeyInput{charInput(' '), charInput('f')}, ActionNavFindFiles},
		{"<Space>b", []KeyInput{charInput(' '), charInput('b')}, ActionNavFindChanged},
		{"<Space>e", []KeyInput{charInput(' '), charInput('e')}, ActionNavOpenRoot},
		{"<Space>c next hunk", []KeyInput{charInput(' '), charInput('c')}, ActionNavNextHunk},
		{"<Space>C prev hunk", []KeyInput{charInput(' '), charInput('C')}, ActionNavPrevHunk},
		{"<Space>n next file", []KeyInput{charInput(' '), charInput('n')}, ActionNavNextFile},
		{"<Space>N prev file", []KeyInput{charInput(' '), charInput('N')}, ActionNavPrevFile},
		{"<Space>x unknown", []KeyInput{charInput(' '), charInput('x')}, ActionNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewState()
			s.NavigatorEnabled = true
			var a Action
			for _, inp := range tt.inputs {
				a = s.HandleKey(inp)
			}
			if a.Kind != tt.want {
				t.Errorf("got ActionKind %d, want %d", a.Kind, tt.want)
			}
		})
	}
}

func TestLeaderSequences_Disabled(t *testing.T) {
	s := NewState()
	s.NavigatorEnabled = false

	// Space should not start a leader sequence
	a := s.HandleKey(charInput(' '))
	if a.Kind != ActionNone {
		t.Errorf("space without navigator: got %d, want ActionNone", a.Kind)
	}
	// PendingBuf should be empty
	if s.PendingBuf != "" {
		t.Errorf("PendingBuf = %q, want empty", s.PendingBuf)
	}
}

func TestGSequenceExtensions(t *testing.T) {
	tests := []struct {
		name   string
		inputs []KeyInput
		want   ActionKind
	}{
		{"go", []KeyInput{charInput('g'), charInput('o')}, ActionNavToggleOriginal},
		{"gf", []KeyInput{charInput('g'), charInput('f')}, ActionNavGoFile},
		{"gi", []KeyInput{charInput('g'), charInput('i')}, ActionNavGoImports},
		{"ga", []KeyInput{charInput('g'), charInput('a')}, ActionNavGoAlternate},
		{"g?", []KeyInput{charInput('g'), charInput('?')}, ActionNavHelp},
		// Existing keys still work
		{"gg", []KeyInput{charInput('g'), charInput('g')}, ActionMoveFileStart},
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
		})
	}
}

func TestDashKey(t *testing.T) {
	s := NewState()
	a := s.HandleKey(charInput('-'))
	if a.Kind != ActionNavOpenParent {
		t.Errorf("got %d, want ActionNavOpenParent", a.Kind)
	}
}

func TestEnterKey(t *testing.T) {
	s := NewState()
	a := s.HandleKey(namedInput(NameReturn))
	if a.Kind != ActionEnterKey {
		t.Errorf("got %d, want ActionEnterKey", a.Kind)
	}
}

func TestTabKey(t *testing.T) {
	s := NewState()
	a := s.HandleKey(namedInput(NameTab))
	if a.Kind != ActionTabKey {
		t.Errorf("got %d, want ActionTabKey", a.Kind)
	}
}

func TestHunkTextObject(t *testing.T) {
	tests := []struct {
		name    string
		inputs  []KeyInput
		wantKind ActionKind
		wantObj  rune
		wantType rune
	}{
		{
			"dih",
			[]KeyInput{charInput('d'), charInput('i'), charInput('h')},
			ActionDelete, 'h', 'i',
		},
		{
			"yih",
			[]KeyInput{charInput('y'), charInput('i'), charInput('h')},
			ActionYank, 'h', 'i',
		},
		{
			"cih",
			[]KeyInput{charInput('c'), charInput('i'), charInput('h')},
			ActionChange, 'h', 'i',
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewState()
			var a Action
			for _, inp := range tt.inputs {
				a = s.HandleKey(inp)
			}
			if a.Kind != tt.wantKind {
				t.Errorf("Kind = %d, want %d", a.Kind, tt.wantKind)
			}
			if a.TextObj != tt.wantObj {
				t.Errorf("TextObj = %c, want %c", a.TextObj, tt.wantObj)
			}
			if a.TextObjType != tt.wantType {
				t.Errorf("TextObjType = %c, want %c", a.TextObjType, tt.wantType)
			}
		})
	}
}

func TestBracketDoesNotBreakExistingKeys(t *testing.T) {
	// Verify that adding ]/[ doesn't break other normal mode keys
	s := NewState()
	a := s.HandleKey(charInput('j'))
	if a.Kind != ActionMoveDown {
		t.Errorf("j: got %d, want ActionMoveDown", a.Kind)
	}

	s = NewState()
	a = s.HandleKey(charInput('d'))
	// d enters operator pending, next key determines action
	if s.Operator != OpDelete {
		t.Errorf("d: operator = %d, want OpDelete", s.Operator)
	}
}

func TestPendingBufCleared(t *testing.T) {
	// After a bracket sequence completes, PendingBuf should be empty
	s := NewState()
	s.HandleKey(charInput(']'))
	if s.PendingBuf != "]" {
		t.Errorf("after ]: PendingBuf = %q, want ]", s.PendingBuf)
	}
	s.HandleKey(charInput('c'))
	if s.PendingBuf != "" {
		t.Errorf("after ]c: PendingBuf = %q, want empty", s.PendingBuf)
	}
}
