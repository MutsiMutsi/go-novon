package core

import (
	"strings"
	"sync"
)

// A LogHandler checks a line and returns true if it's done (so it can be unregistered).
type LogHandler func(line string) (done bool)

type LogWatcher struct {
	mu       sync.Mutex
	handlers map[string]LogHandler // keyed by marker string
}

func NewLogWatcher() *LogWatcher {
	return &LogWatcher{
		handlers: make(map[string]LogHandler),
	}
}

// Register a new handler for a substring marker.
func (w *LogWatcher) Register(marker string, handler LogHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[marker] = handler
}

// Unregister a handler by marker.
func (w *LogWatcher) Unregister(marker string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.handlers, marker)
}

// Dispatch checks each line against registered markers.
func (w *LogWatcher) Dispatch(line string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for marker, handler := range w.handlers {
		if strings.Contains(line, marker) {
			if done := handler(line); done {
				delete(w.handlers, marker)
			}
		}
	}
}
