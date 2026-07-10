package main

import (
	"time"

	"github.com/kristianweb/zephyr/internal/editor"
	"github.com/kristianweb/zephyr/internal/format"
)

// maxErrorMarkers bounds how many error lines are tracked (and drawn) per tab,
// keeping the gutter cheap on pathological files with thousands of errors.
const maxErrorMarkers = 50

// errCheckDelay is the idle debounce before a syntax/format check runs.
const errCheckDelay = 5 * time.Second

// scheduleErrCheck (re)arms the idle error-check timer. Called from afterEdit so
// any buffer-modifying keystroke pushes the deadline out; detection fires once
// the buffer has been quiet for errCheckDelay.
func (st *appState) scheduleErrCheck() {
	st.errCheckPending = true
	st.errCheckDeadline = time.Now().Add(errCheckDelay)
}

// flushErrCheck runs the pending error check if the idle deadline has passed.
// Called once per frame (after flushReparse) so the reparsed tree is current.
func (st *appState) flushErrCheck() {
	if st.errCheckPending && !time.Now().Before(st.errCheckDeadline) {
		st.detectErrors()
	}
}

// detectErrors runs detection on the active tab, first bringing the highlight
// tree in sync with the buffer so tree-sitter sees the latest edits (the 50ms
// debounced reparse may not have fired yet after an Enter keypress). It also
// cancels any pending idle check, since results are now fresh.
func (st *appState) detectErrors() {
	st.errCheckPending = false
	ts := st.activeTabState()
	ed := st.activeEd()
	if ts == nil || ed == nil {
		return
	}
	if st.reparsePending {
		st.reparseHighlight() // drains edits, updates the tree incrementally
	}
	st.runErrorDetection(ts, ed)
}

// runErrorDetection computes and stores the error lines for ts. It assumes the
// highlighter tree (if any) already reflects the current buffer, so callers that
// have just edited without reparsing must sync the tree first (see detectErrors).
//
// JSON is validated with encoding/json regardless of the highlighter (its
// highlighter is a simple tokenizer with no parse tree). Tree-sitter languages
// walk the parse tree for ERROR/MISSING nodes. Languages with neither get no
// markers.
func (st *appState) runErrorDetection(ts *tabState, ed *editor.Editor) {
	switch ts.langLabel {
	case "JSON":
		if line, ok := format.JSONErrorLine(ed.Buffer.TextBytes(nil)); ok {
			setErrorLines(ts, []int{line})
		} else {
			setErrorLines(ts, nil)
		}
	default:
		if ts.highlighter == nil {
			setErrorLines(ts, nil)
			return
		}
		setErrorLines(ts, ts.highlighter.ErrorLines(maxErrorMarkers))
	}
}

// setErrorLines replaces the tab's error-line set. An empty result clears the
// map so the renderer draws nothing (markers vanish once a run finds no errors).
func setErrorLines(ts *tabState, lines []int) {
	if len(lines) == 0 {
		ts.errorLines = nil
		return
	}
	m := make(map[int]bool, len(lines))
	for _, l := range lines {
		m[l] = true
	}
	ts.errorLines = m
}
