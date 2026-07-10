package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

func TestPerfTracerWritesStructuredRecord(t *testing.T) {
	var output bytes.Buffer
	tracer := &perfTracer{enabled: true, output: &output}
	tracer.record(perfTraceRecord{Event: "frame", Frame: 3, DrawUS: 42, GioEventToSubmitUS: 17})
	got := output.String()
	for _, want := range []string{`"event":"frame"`, `"frame":3`, `"drawUs":42`, `"gioEventToSubmitUs":17`} {
		if !strings.Contains(got, want) {
			t.Fatalf("trace %q does not contain %q", got, want)
		}
	}
}

func TestNewPerfTracerTruncatesPreviousRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	if err := os.WriteFile(path, []byte("stale run\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tracer := newPerfTracer(path)
	tracer.record(perfTraceRecord{Event: "current"})
	tracer.close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "stale") || !strings.Contains(string(data), `"event":"current"`) {
		t.Fatalf("trace was not replaced: %q", data)
	}
}

func TestNewPerfTracerInvalidDestinationStaysDisabled(t *testing.T) {
	tracer := newPerfTracer(t.TempDir()) // opening a directory as a file must fail
	if tracer.isEnabled() || tracer.output != nil || tracer.closer != nil {
		t.Fatalf("failed tracer remained active: %+v", tracer)
	}
	tracer.record(perfTraceRecord{Event: "ignored"})
	tracer.close()
}

type trackingWriteCloser struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	closed bool
	closes int
}

func (w *trackingWriteCloser) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, errors.New("write after close")
	}
	return w.buffer.Write(data)
}

func (w *trackingWriteCloser) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	w.closes++
	return nil
}

func TestPerfTracerCloseIsIdempotentAndStopsRecords(t *testing.T) {
	output := &trackingWriteCloser{}
	tracer := &perfTracer{enabled: true, output: output, closer: output}
	tracer.record(perfTraceRecord{Event: "before"})
	tracer.close()
	tracer.close()
	tracer.record(perfTraceRecord{Event: "after"})

	output.mu.Lock()
	defer output.mu.Unlock()
	if output.closes != 1 || strings.Contains(output.buffer.String(), "after") {
		t.Fatalf("close count=%d output=%q", output.closes, output.buffer.String())
	}
}

func TestPerformanceInputPreservesEarliestEventUntilSubmit(t *testing.T) {
	original := appPerfTracer
	var output bytes.Buffer
	appPerfTracer = &perfTracer{enabled: true, output: &output}
	t.Cleanup(func() { appPerfTracer = original })

	st := &appState{}
	st.notePerformanceInput(key.Event{Name: "A", State: key.Press})
	firstAt := st.perfPendingEventAt
	time.Sleep(time.Millisecond)
	st.notePerformanceInput(key.EditEvent{Text: "a"})
	if st.perfPendingEventKind != "key" || !st.perfPendingEventAt.Equal(firstAt) {
		t.Fatalf("later event replaced earliest: kind=%q at=%v first=%v", st.perfPendingEventKind, st.perfPendingEventAt, firstAt)
	}

	st.recordPerformanceFrame(time.Now().Add(-time.Millisecond), 100*time.Microsecond, 200*time.Microsecond)
	if st.perfFrameCount != 1 || !st.perfPendingEventAt.IsZero() || st.perfPendingEventKind != "" {
		t.Fatalf("frame lifecycle count=%d input=%v/%q", st.perfFrameCount, st.perfPendingEventAt, st.perfPendingEventKind)
	}
	got := output.String()
	if !strings.Contains(got, `"inputKind":"key"`) || !strings.Contains(got, `"gioEventToSubmitUs":`) {
		t.Fatalf("frame record does not describe Gio timing: %q", got)
	}

	st.notePerformanceInput(pointer.Event{Kind: pointer.Move})
	if !st.perfPendingEventAt.IsZero() {
		t.Fatal("pointer move should not start a Gio event timing sample")
	}
	st.notePerformanceInput(pointer.Event{Kind: pointer.Scroll})
	if st.perfPendingEventKind != "pointer_scroll" {
		t.Fatalf("scroll event kind = %q", st.perfPendingEventKind)
	}
}

func TestDisabledPerfTracerDoesNotTrackInputOrWrite(t *testing.T) {
	original := appPerfTracer
	appPerfTracer = &perfTracer{}
	t.Cleanup(func() { appPerfTracer = original })

	st := &appState{}
	st.notePerformanceInput(key.Event{Name: "A", State: key.Press})
	st.recordPerformanceFrame(time.Now(), 0, 0)
	if !st.perfPendingEventAt.IsZero() || st.perfFrameCount != 0 {
		t.Fatalf("disabled tracer changed state: %+v", st)
	}
}
