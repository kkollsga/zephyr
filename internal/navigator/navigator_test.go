package navigator

import "testing"

func TestCursorMemory(t *testing.T) {
	n := New()

	// Default is 0
	if got := n.RecallCursor("/some/dir"); got != 0 {
		t.Errorf("RecallCursor(unvisited) = %d, want 0", got)
	}

	n.RememberCursor("/some/dir", 5)
	if got := n.RecallCursor("/some/dir"); got != 5 {
		t.Errorf("RecallCursor = %d, want 5", got)
	}

	// Different directory
	n.RememberCursor("/other/dir", 12)
	if got := n.RecallCursor("/other/dir"); got != 12 {
		t.Errorf("RecallCursor(/other/dir) = %d, want 12", got)
	}
	// First dir unchanged
	if got := n.RecallCursor("/some/dir"); got != 5 {
		t.Errorf("RecallCursor(/some/dir) = %d, want 5", got)
	}

	// Overwrite
	n.RememberCursor("/some/dir", 8)
	if got := n.RecallCursor("/some/dir"); got != 8 {
		t.Errorf("RecallCursor after overwrite = %d, want 8", got)
	}
}
