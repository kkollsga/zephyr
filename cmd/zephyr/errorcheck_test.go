package main

import (
	"testing"
	"time"

	"gioui.org/io/key"
)

// TestEnterTriggersErrorDetection drives the real Enter key path and checks that
// error markers are populated for an invalid JSON buffer.
func TestEnterTriggersErrorDetection(t *testing.T) {
	st, ed, ts := testAppWithText(`{"a": 1`, "JSON") // unclosed object
	ed.Cursor.SetPosition(ed.Buffer, 0, len(`{"a": 1`))

	st.handleKey(key.Event{Name: key.NameReturn})

	if len(ts.errorLines) == 0 {
		t.Fatal("expected error markers after Enter on invalid JSON")
	}
	if st.errCheckPending {
		t.Fatal("immediate detection should cancel the pending idle check")
	}
}

// TestIdleFlushRunsDetection exercises the debounced idle path: a scheduled
// check whose deadline has passed must run and clear the pending flag.
func TestIdleFlushRunsDetection(t *testing.T) {
	st, _, ts := testAppWithText("{ broken", "JSON")

	st.scheduleErrCheck()
	if !st.errCheckPending {
		t.Fatal("scheduleErrCheck did not arm the timer")
	}
	// Not yet due: flush must be a no-op.
	st.flushErrCheck()
	if !st.errCheckPending {
		t.Fatal("flush fired before the deadline")
	}
	// Force the deadline into the past and flush.
	st.errCheckDeadline = time.Now().Add(-time.Millisecond)
	st.flushErrCheck()
	if st.errCheckPending {
		t.Fatal("flushErrCheck left the timer armed")
	}
	if len(ts.errorLines) == 0 {
		t.Fatal("idle flush produced no markers for invalid JSON")
	}
}

// TestDetectionClearsStaleMarkers verifies markers vanish once a run finds no
// errors (e.g. after the buffer is fixed).
func TestDetectionClearsStaleMarkers(t *testing.T) {
	st, _, ts := testAppWithText(`{"a": 1}`, "JSON") // valid
	ts.errorLines = map[int]bool{5: true}            // stale marker from earlier

	st.detectErrors()

	if len(ts.errorLines) != 0 {
		t.Fatalf("valid JSON left stale markers: %v", ts.errorLines)
	}
}
