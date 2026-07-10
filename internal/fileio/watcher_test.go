package fileio

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/kristianweb/zephyr/internal/buffer"
)

const watcherTimeout = 3 * time.Second

func TestWatcher_ReportsExternalWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	fw, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()
	if err := fw.Watch(path); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("after"), 0644); err != nil {
		t.Fatal(err)
	}
	event := awaitFileEvent(t, fw.Events)
	if event.Path != path || !event.Op.Has(fsnotify.Write) {
		t.Fatalf("event = {%q %v}, want write for %q", event.Path, event.Op, path)
	}
}

func TestWatcher_OwnAtomicSaveIsSuppressedThenExternalWriteReports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	fw, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()
	if err := fw.Watch(path); err != nil {
		t.Fatal(err)
	}

	fw.MarkOwnWrite(path)
	if err := SaveFile(buffer.NewFromString("own save"), path); err != nil {
		t.Fatal(err)
	}
	if err := fw.Rewatch(path); err != nil {
		t.Fatal(err)
	}
	assertNoFileEvent(t, fw.Events, 250*time.Millisecond)

	if err := os.WriteFile(path, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	event := awaitFileEvent(t, fw.Events)
	if event.Path != path {
		t.Fatalf("event path = %q, want %q", event.Path, path)
	}
}

func TestWatcher_RewatchAfterAtomicSaveStress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	fw, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()
	if err := fw.Watch(path); err != nil {
		t.Fatal(err)
	}

	// Exercise repeated human-scale saves. A small interval also lets kqueue's
	// directory backend finish reconciling replacement child inodes.
	for i := 0; i < 50; i++ {
		fw.MarkOwnWrite(path)
		if err := SaveFile(buffer.NewFromString("own save"), path); err != nil {
			fw.CancelOwnWrite(path)
			t.Fatal(err)
		}
		if err := fw.Rewatch(path); err != nil {
			t.Fatalf("rewatch %d: %v", i, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	// A deliberately unrealistic burst can outlive the short self-event
	// suppression window. Drain those delayed notifications; the product layer
	// also ignores events whose disk content already matches the buffer. The
	// important invariant here is that the directory-backed watch remains live.
	drainFileEventsUntilQuiet(fw.Events, 100*time.Millisecond)

	if err := os.WriteFile(path, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	awaitFileEvent(t, fw.Events)
}

func TestWatcher_CancelOwnWriteRestoresExternalEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	fw, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()
	if err := fw.Watch(path); err != nil {
		t.Fatal(err)
	}

	fw.MarkOwnWrite(path)
	fw.CancelOwnWrite(path)
	if err := os.WriteFile(path, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	awaitFileEvent(t, fw.Events)
}

func TestWatcher_RewatchFailureClearsOwnWrite(t *testing.T) {
	fw, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()
	path := filepath.Join(t.TempDir(), "missing", "file.txt")
	fw.MarkOwnWrite(path)
	if err := fw.Rewatch(path); err == nil {
		t.Fatal("expected rewatch with a missing parent directory to fail")
	}
	fw.mu.Lock()
	_, suppressed := fw.ownWrites[filepath.Clean(path)]
	fw.mu.Unlock()
	if suppressed {
		t.Fatal("failed rewatch left own-write suppression installed")
	}
}

func TestWatcher_CloseWithPendingEventAndFullConsumerQueue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watched.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	fw, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	if err := fw.Watch(path); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < cap(fw.Events); i++ {
		fw.Events <- FileEvent{Path: "queue-saturation"}
	}
	if err := os.WriteFile(path, []byte("pending"), 0644); err != nil {
		t.Fatal(err)
	}
	// Give the forwarding goroutine time to block on the saturated queue.
	time.Sleep(100 * time.Millisecond)

	closed := make(chan error, 1)
	go func() { closed <- fw.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(watcherTimeout):
		t.Fatal("Watcher.Close blocked with a pending event")
	}

	for range fw.Events {
	}
}

func TestWatcher_CloseIsIdempotentAndClosesEvents(t *testing.T) {
	fw, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-fw.Events:
		if ok {
			t.Fatal("event channel remained open after Close")
		}
	case <-time.After(watcherTimeout):
		t.Fatal("event channel remained open after Close")
	}
}

func awaitFileEvent(t *testing.T, events <-chan FileEvent) FileEvent {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("watcher event channel closed")
		}
		return event
	case <-time.After(watcherTimeout):
		t.Fatal("timed out waiting for file event")
		return FileEvent{}
	}
}

func assertNoFileEvent(t *testing.T, events <-chan FileEvent, duration time.Duration) {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("watcher event channel closed")
		}
		t.Fatalf("unexpected file event: {%q %v}", event.Path, event.Op)
	case <-time.After(duration):
	}
}

func drainFileEventsUntilQuiet(events <-chan FileEvent, quiet time.Duration) {
	timer := time.NewTimer(quiet)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(quiet)
		case <-timer.C:
			return
		}
	}
}
