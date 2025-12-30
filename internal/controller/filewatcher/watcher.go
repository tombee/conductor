// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filewatcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// eventTypeMap maps fsnotify operations to file watcher event types.
var eventTypeMap = map[fsnotify.Op]string{
	fsnotify.Create: "created",
	fsnotify.Write:  "modified",
	fsnotify.Remove: "deleted",
	fsnotify.Rename: "renamed",
}

// Watcher wraps fsnotify.Watcher and handles filesystem events for a single path.
type Watcher struct {
	path      string
	events    map[string]bool // Allowed event types (created, modified, deleted, renamed)
	watcher   *fsnotify.Watcher
	eventChan chan *Context
	logger    *slog.Logger
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewWatcher creates a new file watcher for the specified path.
// events specifies which event types to watch (created, modified, deleted, renamed).
// If events is empty, all event types are watched.
func NewWatcher(path string, events []string) (*Watcher, error) {
	// Create fsnotify watcher
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	// Normalize path
	absPath, err := filepath.Abs(path)
	if err != nil {
		fsw.Close()
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Convert events slice to map for O(1) lookup
	eventMap := make(map[string]bool)
	if len(events) == 0 {
		// Watch all event types if none specified
		eventMap["created"] = true
		eventMap["modified"] = true
		eventMap["deleted"] = true
		eventMap["renamed"] = true
	} else {
		for _, e := range events {
			eventMap[e] = true
		}
	}

	w := &Watcher{
		path:      absPath,
		events:    eventMap,
		watcher:   fsw,
		eventChan: make(chan *Context, 100), // Buffered channel to prevent blocking
		logger:    slog.Default().With(slog.String("component", "filewatcher"), slog.String("path", absPath)),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}

	// Add path to fsnotify watcher
	if err := fsw.Add(absPath); err != nil {
		fsw.Close()
		return nil, fmt.Errorf("failed to watch path: %w", err)
	}

	return w, nil
}

// Start begins watching for file events.
func (w *Watcher) Start(ctx context.Context) error {
	go w.eventLoop(ctx)
	w.logger.Info("file watcher started")
	return nil
}

// Stop stops the watcher and releases resources.
func (w *Watcher) Stop() error {
	close(w.stopCh)
	<-w.doneCh
	return w.watcher.Close()
}

// Events returns a channel that receives file event contexts.
func (w *Watcher) Events() <-chan *Context {
	return w.eventChan
}

// eventLoop processes fsnotify events and converts them to file watcher contexts.
func (w *Watcher) eventLoop(ctx context.Context) {
	defer close(w.doneCh)
	defer close(w.eventChan)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("file watcher stopped (context cancelled)")
			return
		case <-w.stopCh:
			w.logger.Info("file watcher stopped")
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				w.logger.Warn("file watcher event channel closed")
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				w.logger.Warn("file watcher error channel closed")
				return
			}
			w.logger.Error("file watcher error", "error", err)
		}
	}
}

// handleEvent processes a single fsnotify event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Map fsnotify op to event type
	eventType, ok := eventTypeMap[event.Op]
	if !ok {
		// fsnotify.Chmod is not mapped - we ignore it
		w.logger.Debug("ignoring unmapped event", "op", event.Op, "path", event.Name)
		return
	}

	// Check if this event type is enabled
	if !w.events[eventType] {
		w.logger.Debug("event type not enabled", "type", eventType, "path", event.Name)
		return
	}

	// Get file info for size and mtime (except for deleted files)
	var size int64
	var mtime time.Time
	var isDir bool

	if eventType != "deleted" {
		if info, err := os.Stat(event.Name); err == nil {
			size = info.Size()
			mtime = info.ModTime()
			isDir = info.IsDir()
		} else {
			// File may have been deleted between event and stat
			w.logger.Debug("failed to stat file", "path", event.Name, "error", err)
		}
	}

	// Create context
	ctx := NewContext(event.Name, eventType, isDir, size, mtime)

	// Send to event channel (non-blocking)
	select {
	case w.eventChan <- ctx:
		w.logger.Debug("file event", "type", eventType, "path", event.Name)
	default:
		w.logger.Warn("event channel full, dropping event", "type", eventType, "path", event.Name)
	}
}
