package vim

import "testing"

// visualState returns a state in Visual mode, the way the app layer leaves it
// after executing ActionVisualStart.
func visualState() *State {
	s := NewState()
	s.HandleKey(charInput('v'))
	s.Mode = ModeVisual
	return s
}

func TestVisualGGGoesToFileStart(t *testing.T) {
	s := visualState()
	if a := s.HandleKey(charInput('g')); a.Kind != ActionNone {
		t.Fatalf("first g = %d, want ActionNone", a.Kind)
	}
	a := s.HandleKey(charInput('g'))
	if a.Kind != ActionMoveFileStart {
		t.Fatalf("gg in visual = %d, want ActionMoveFileStart (%d)", a.Kind, ActionMoveFileStart)
	}
	if s.Mode != ModeVisual {
		t.Fatalf("gg left visual mode: %v", s.Mode)
	}
}

func TestVisualGGWithCountGoesToLine(t *testing.T) {
	s := visualState()
	s.HandleKey(charInput('7'))
	s.HandleKey(charInput('g'))
	a := s.HandleKey(charInput('g'))
	if a.Kind != ActionMoveToLine || a.Line != 7 {
		t.Fatalf("7gg in visual = kind %d line %d, want ActionMoveToLine line 7", a.Kind, a.Line)
	}
}

func TestVisualTextObjectParses(t *testing.T) {
	tests := []struct {
		name    string
		keys    string
		wantObj rune
		wantTyp rune
	}{
		{"viw", "iw", 'w', 'i'},
		{`va"`, `a"`, '"', 'a'},
		{"vi(", "i(", '(', 'i'},
		{"vih", "ih", 'h', 'i'},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := visualState()
			keys := []rune(tt.keys)
			if a := s.HandleKey(charInput(keys[0])); a.Kind != ActionNone {
				t.Fatalf("%c = %d, want ActionNone while waiting for the delimiter", keys[0], a.Kind)
			}
			if !s.WaitingForTextObj {
				t.Fatalf("%c did not arm WaitingForTextObj", keys[0])
			}
			a := s.HandleKey(charInput(keys[1]))
			if a.Kind != ActionSelectTextObject {
				t.Fatalf("%s = %d, want ActionSelectTextObject (%d)", tt.name, a.Kind, ActionSelectTextObject)
			}
			if a.TextObj != tt.wantObj || a.TextObjType != tt.wantTyp {
				t.Fatalf("%s = obj %q type %q, want %q/%q", tt.name, a.TextObj, a.TextObjType, tt.wantObj, tt.wantTyp)
			}
			if s.Mode != ModeVisual {
				t.Fatalf("%s left visual mode: %v", tt.name, s.Mode)
			}
			if s.WaitingForTextObj || s.PendingBuf != "" {
				t.Fatalf("%s left pending state: waiting=%v buf=%q", tt.name, s.WaitingForTextObj, s.PendingBuf)
			}
		})
	}
}

func TestVisualTextObjectRejectsUnsupportedDelimiters(t *testing.T) {
	// 't' and 'p' have no executor: accepting them would parse into a complete
	// action that silently does nothing.
	for _, keys := range []string{"ix", "ip", "it", "ah"} {
		t.Run(keys, func(t *testing.T) {
			s := visualState()
			r := []rune(keys)
			s.HandleKey(charInput(r[0]))
			a := s.HandleKey(charInput(r[1]))
			if a.Kind != ActionNone {
				t.Fatalf("v%s = %d, want ActionNone", keys, a.Kind)
			}
			if s.WaitingForTextObj || s.PendingBuf != "" {
				t.Fatalf("v%s left pending state: waiting=%v buf=%q", keys, s.WaitingForTextObj, s.PendingBuf)
			}
			if s.Mode != ModeVisual {
				t.Fatalf("v%s left visual mode: %v", keys, s.Mode)
			}
		})
	}
}

// A host accelerator arriving while i/a waits for its delimiter must cancel the
// pending key rather than feed it (the Ctrl/Shortcut cancel in HandleKey).
func TestVisualTextObjectCancelledByShortcut(t *testing.T) {
	s := visualState()
	s.HandleKey(charInput('i'))
	a := s.HandleKey(KeyInput{Char: 's', Shortcut: true})
	if a.Kind != ActionNone {
		t.Fatalf("Cmd+S while waiting = %d, want ActionNone", a.Kind)
	}
	if s.WaitingForTextObj || s.PendingBuf != "" {
		t.Fatalf("Cmd+S left pending state: waiting=%v buf=%q", s.WaitingForTextObj, s.PendingBuf)
	}
}

func TestVisualTextObjectEscapeClearsPendingOnly(t *testing.T) {
	s := visualState()
	s.HandleKey(charInput('i'))
	a := s.HandleKey(namedInput(NameEscape))
	if a.Kind != ActionNone {
		t.Fatalf("Escape while waiting = %d, want ActionNone", a.Kind)
	}
	if s.Mode != ModeVisual {
		t.Fatalf("Escape while waiting left visual mode: %v", s.Mode)
	}
	if s.WaitingForTextObj {
		t.Fatal("Escape left WaitingForTextObj set")
	}
}
