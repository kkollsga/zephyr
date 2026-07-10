package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

var processStartedAt = time.Now()

type perfTraceRecord struct {
	Event              string `json:"event"`
	SinceStartUS       int64  `json:"sinceStartUs"`
	FrameUS            int64  `json:"frameUs,omitempty"`
	EventUS            int64  `json:"eventUs,omitempty"`
	DrawUS             int64  `json:"drawUs,omitempty"`
	GioEventToSubmitUS int64  `json:"gioEventToSubmitUs,omitempty"`
	InputKind          string `json:"inputKind,omitempty"`
	Frame              uint64 `json:"frame,omitempty"`
	First              bool   `json:"first,omitempty"`
}

type perfTracer struct {
	mu      sync.Mutex
	output  io.Writer
	closer  io.Closer
	enabled bool
}

var appPerfTracer = newPerfTracer(os.Getenv("ZEPHYR_PERF_TRACE"))

func newPerfTracer(destination string) *perfTracer {
	t := &perfTracer{}
	if destination == "" {
		return t
	}
	t.enabled = true
	if destination == "stderr" || destination == "1" {
		t.output = os.Stderr
		return t
	}
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "performance trace: %v\n", err)
		t.enabled = false
		return t
	}
	t.output = f
	t.closer = f
	return t
}

func (t *perfTracer) record(record perfTraceRecord) {
	if t == nil {
		return
	}
	record.SinceStartUS = time.Since(processStartedAt).Microseconds()
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.enabled || t.output == nil {
		return
	}
	_, _ = fmt.Fprintf(t.output, "%s\n", data)
}

func (t *perfTracer) close() {
	if t == nil {
		return
	}
	t.mu.Lock()
	closer := t.closer
	t.enabled = false
	t.output = nil
	t.closer = nil
	t.mu.Unlock()
	if closer != nil {
		_ = closer.Close()
	}
}

func (t *perfTracer) isEnabled() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	enabled := t.enabled
	t.mu.Unlock()
	return enabled
}

func tracePerformanceEvent(name string) {
	appPerfTracer.record(perfTraceRecord{Event: name})
}

func (st *appState) notePerformanceInput(event any) {
	if !appPerfTracer.isEnabled() {
		return
	}
	kind := ""
	switch e := event.(type) {
	case key.Event:
		if e.State == key.Press {
			kind = "key"
		}
	case key.EditEvent:
		if e.Text != "" {
			kind = "text"
		}
	case pointer.Event:
		switch e.Kind {
		case pointer.Press:
			kind = "pointer_press"
		case pointer.Drag:
			kind = "pointer_drag"
		case pointer.Release:
			kind = "pointer_release"
		case pointer.Scroll:
			kind = "pointer_scroll"
		}
	}
	if kind != "" && st.perfPendingEventAt.IsZero() {
		st.perfPendingEventAt = time.Now()
		st.perfPendingEventKind = kind
	}
}

// recordPerformanceFrame records CPU-side frame work. GioEventToSubmitUS starts
// when Gio dequeues the earliest pending input event and ends after e.Frame
// returns; it is intentionally not presented as device-to-display latency.
func (st *appState) recordPerformanceFrame(frameStart time.Time, eventDuration, drawDuration time.Duration) {
	if !appPerfTracer.isEnabled() {
		return
	}
	st.perfFrameCount++
	record := perfTraceRecord{
		Event:   "frame",
		Frame:   st.perfFrameCount,
		First:   st.perfFrameCount == 1,
		FrameUS: time.Since(frameStart).Microseconds(),
		EventUS: eventDuration.Microseconds(),
		DrawUS:  drawDuration.Microseconds(),
	}
	if !st.perfPendingEventAt.IsZero() {
		record.GioEventToSubmitUS = time.Since(st.perfPendingEventAt).Microseconds()
		record.InputKind = st.perfPendingEventKind
		st.perfPendingEventAt = time.Time{}
		st.perfPendingEventKind = ""
	}
	appPerfTracer.record(record)
}
