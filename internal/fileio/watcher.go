package fileio

import (
	"sync"

	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a file system change event.
type FileEvent struct {
	Path string
	Op   fsnotify.Op
}

// Watcher monitors files for external changes.
type Watcher struct {
	watcher    *fsnotify.Watcher
	Events     chan FileEvent
	mu         sync.Mutex
	ownWrites  map[string]bool // paths we recently wrote (to ignore our own saves)
}

// NewWatcher creates a new file watcher.
func NewWatcher() (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &Watcher{
		watcher:   w,
		Events:    make(chan FileEvent, 16),
		ownWrites: make(map[string]bool),
	}

	go fw.run()
	return fw, nil
}

func (fw *Watcher) run() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				fw.mu.Lock()
				isOwn := fw.ownWrites[event.Name]
				delete(fw.ownWrites, event.Name)
				fw.mu.Unlock()

				if !isOwn {
					fw.Events <- FileEvent{Path: event.Name, Op: event.Op}
				}
			}
		case _, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// Watch adds a file to the watch list.
func (fw *Watcher) Watch(path string) error {
	return fw.watcher.Add(path)
}

// Unwatch removes a file from the watch list.
func (fw *Watcher) Unwatch(path string) error {
	return fw.watcher.Remove(path)
}

// MarkOwnWrite marks a path as recently written by us (to ignore the next change event).
func (fw *Watcher) MarkOwnWrite(path string) {
	fw.mu.Lock()
	fw.ownWrites[path] = true
	fw.mu.Unlock()
}

// Close stops the watcher.
func (fw *Watcher) Close() error {
	close(fw.Events)
	return fw.watcher.Close()
}
