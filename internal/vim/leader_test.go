package vim

import (
	"fmt"
	"testing"
)

// TestLeaderKeyDetailed traces every step of <Space>e processing.
func TestLeaderKeyDetailed(t *testing.T) {
	s := NewState()
	s.NavigatorEnabled = true

	// Step 1: Press space
	a1 := s.HandleKey(KeyInput{Char: ' '})
	t.Logf("After space: PendingBuf=%q, Action=%d, Mode=%d", s.PendingBuf, a1.Kind, s.Mode)

	if s.PendingBuf != " " {
		t.Fatalf("PendingBuf after space = %q, want %q", s.PendingBuf, " ")
	}
	if a1.Kind != ActionNone {
		t.Fatalf("space should return ActionNone, got %d", a1.Kind)
	}

	// Step 2: Press e
	a2 := s.HandleKey(KeyInput{Char: 'e'})
	t.Logf("After e: PendingBuf=%q, Action=%d", s.PendingBuf, a2.Kind)

	if a2.Kind != ActionNavOpenRoot {
		t.Fatalf("space+e should return ActionNavOpenRoot (%d), got %d", ActionNavOpenRoot, a2.Kind)
	}
	if s.PendingBuf != "" {
		t.Fatalf("PendingBuf after space+e = %q, want empty", s.PendingBuf)
	}
}

// TestAllLeaderSequencesDetailed tests every leader key combination.
func TestAllLeaderSequencesDetailed(t *testing.T) {
	tests := []struct {
		second rune
		want   ActionKind
		name   string
	}{
		{'g', ActionNavOpenStatus, "status"},
		{'f', ActionNavFindFiles, "find files"},
		{'b', ActionNavFindChanged, "find changed"},
		{'e', ActionNavOpenRoot, "open root"},
		{'c', ActionNavNextHunk, "next hunk"},
		{'C', ActionNavPrevHunk, "prev hunk"},
		{'n', ActionNavNextFile, "next file"},
		{'N', ActionNavPrevFile, "prev file"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Space+%c=%s", tt.second, tt.name), func(t *testing.T) {
			s := NewState()
			s.NavigatorEnabled = true

			a1 := s.HandleKey(KeyInput{Char: ' '})
			if a1.Kind != ActionNone {
				t.Fatalf("space returned %d, want ActionNone", a1.Kind)
			}
			if s.PendingBuf != " " {
				t.Fatalf("PendingBuf = %q after space, want %q", s.PendingBuf, " ")
			}

			a2 := s.HandleKey(KeyInput{Char: tt.second})
			if a2.Kind != tt.want {
				t.Errorf("Space+%c: got action %d, want %d", tt.second, a2.Kind, tt.want)
			}
		})
	}
}

// TestLeaderDisabledDoesNothing verifies space is ignored when navigator is off.
func TestLeaderDisabledDoesNothing(t *testing.T) {
	s := NewState()
	s.NavigatorEnabled = false

	a := s.HandleKey(KeyInput{Char: ' '})
	if s.PendingBuf != "" {
		t.Errorf("PendingBuf = %q, want empty when navigator disabled", s.PendingBuf)
	}
	if a.Kind != ActionNone {
		t.Errorf("space without navigator: got %d, want ActionNone", a.Kind)
	}

	// Next key should work normally, not as leader
	a2 := s.HandleKey(KeyInput{Char: 'e'})
	if a2.Kind != ActionMoveWordEnd {
		t.Errorf("e after disabled space: got %d, want ActionMoveWordEnd (%d)", a2.Kind, ActionMoveWordEnd)
	}
}

// TestLeaderAfterCount verifies count + space + key works.
func TestLeaderAfterCount(t *testing.T) {
	s := NewState()
	s.NavigatorEnabled = true

	s.HandleKey(KeyInput{Char: '3'})
	s.HandleKey(KeyInput{Char: ' '})
	// Space should reset PendingBuf to just " " (dropping the count display)
	// but Count should still be 3
	a := s.HandleKey(KeyInput{Char: 'c'})
	t.Logf("3<Space>c: action=%d, count=%d", a.Kind, a.Count)
	if a.Kind != ActionNavNextHunk {
		t.Errorf("3<Space>c: got %d, want ActionNavNextHunk", a.Kind)
	}
	if a.Count != 3 {
		t.Errorf("3<Space>c: count=%d, want 3", a.Count)
	}
}

// TestNormalKeysStillWork verifies basic vim keys are not broken.
func TestNormalKeysStillWork(t *testing.T) {
	tests := []struct {
		char rune
		want ActionKind
	}{
		{'h', ActionMoveLeft},
		{'j', ActionMoveDown},
		{'k', ActionMoveUp},
		{'l', ActionMoveRight},
		{'w', ActionMoveWordForward},
		{'b', ActionMoveWordBackward},
		{'e', ActionMoveWordEnd},
		{'0', ActionMoveLineStart},
		{'$', ActionMoveLineEnd},
		{'^', ActionMoveFirstNonBlank},
		{'G', ActionMoveFileEnd},
		{'x', ActionDelete},
		{'p', ActionPut},
		{'u', ActionUndo},
		{'-', ActionNavOpenParent},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%c", tt.char), func(t *testing.T) {
			s := NewState()
			s.NavigatorEnabled = true // even with navigator on
			a := s.HandleKey(KeyInput{Char: tt.char})
			if a.Kind != tt.want {
				t.Errorf("%c: got %d, want %d", tt.char, a.Kind, tt.want)
			}
		})
	}
}
