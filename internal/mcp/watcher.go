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

package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors MCP server source files for changes and triggers restarts.
type Watcher struct {
	// fsWatcher is the underlying filesystem watcher
	fsWatcher *fsnotify.Watcher

	// manager is the MCP server manager to notify of changes
	manager *Manager

	// logger is used for structured logging
	logger *slog.Logger

	// debounceDelay is the delay before triggering a restart after file changes
	debounceDelay time.Duration

	// watchedServers maps server names to their watched paths
	watchedServers map[string][]string

	// pendingRestarts tracks servers with pending debounced restarts
	pendingRestarts map[string]*time.Timer

	// mu protects watchedServers and pendingRestarts
	mu sync.RWMutex

	// ctx is the watcher's lifecycle context
	ctx context.Context

	// cancel stops the watcher
	cancel context.CancelFunc

	// wg tracks active goroutines
	wg sync.WaitGroup
}

// WatcherConfig configures the file watcher.
type WatcherConfig struct {
	// Manager is the MCP server manager to notify of changes
	Manager *Manager

	// Logger is used for structured logging (optional)
	Logger *slog.Logger

	// DebounceDelay is the delay before triggering a restart after file changes (defaults to 200ms)
	DebounceDelay time.Duration
}

// NewWatcher creates a new file watcher for MCP servers.
func NewWatcher(cfg WatcherConfig) (*Watcher, error) {
	if cfg.Manager == nil {
		return nil, fmt.Errorf("manager is required")
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	debounceDelay := cfg.DebounceDelay
	if debounceDelay == 0 {
		debounceDelay = 200 * time.Millisecond
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &Watcher{
		fsWatcher:       fsWatcher,
		manager:         cfg.Manager,
		logger:          logger,
		debounceDelay:   debounceDelay,
		watchedServers:  make(map[string][]string),
		pendingRestarts: make(map[string]*time.Timer),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Start event processing loop
	w.wg.Add(1)
	go w.processEvents()

	return w, nil
}

// Watch adds file paths to watch for a specific MCP server.
// When files change, the server will be automatically restarted.
func (w *Watcher) Watch(serverName string, paths []string) error {
	if serverName == "" {
		return fmt.Errorf("server name is required")
	}
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Add paths to fsnotify watcher
	for _, path := range paths {
		// Resolve to absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to resolve path %s: %w", path, err)
		}

		// Add to watcher
		if err := w.fsWatcher.Add(absPath); err != nil {
			return fmt.Errorf("failed to watch path %s: %w", absPath, err)
		}

		w.logger.Debug("watching path for mcp server",
			"server", serverName,
			"path", absPath,
		)
	}

	// Store watched paths for this server
	w.watchedServers[serverName] = paths

	return nil
}

// Unwatch removes file path watches for a specific MCP server.
func (w *Watcher) Unwatch(serverName string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	paths, exists := w.watchedServers[serverName]
	if !exists {
		return nil // Already unwatched
	}

	// Remove paths from fsnotify watcher
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}

		// Check if any other server is watching this path
		inUse := false
		for otherServer, otherPaths := range w.watchedServers {
			if otherServer == serverName {
				continue
			}
			for _, otherPath := range otherPaths {
				otherAbs, _ := filepath.Abs(otherPath)
				if otherAbs == absPath {
					inUse = true
					break
				}
			}
			if inUse {
				break
			}
		}

		// Only remove if no other server is watching
		if !inUse {
			_ = w.fsWatcher.Remove(absPath)
		}
	}

	delete(w.watchedServers, serverName)

	// Cancel pending restart timer
	if timer, exists := w.pendingRestarts[serverName]; exists {
		timer.Stop()
		delete(w.pendingRestarts, serverName)
	}

	return nil
}

// processEvents processes filesystem events and triggers server restarts.
func (w *Watcher) processEvents() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// Handle file modifications and writes
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				w.handleFileChange(event.Name)
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.logger.Error("file watcher error", "error", err)

		case <-w.ctx.Done():
			return
		}
	}
}

// handleFileChange handles a file change event by scheduling a debounced restart.
func (w *Watcher) handleFileChange(changedPath string) {
	absPath, err := filepath.Abs(changedPath)
	if err != nil {
		return
	}

	// Find which server(s) are watching this path and collect them
	var serversToRestart []string

	w.mu.Lock()
	for serverName, watchedPaths := range w.watchedServers {
		isWatched := false
		for _, watchedPath := range watchedPaths {
			watchedAbs, _ := filepath.Abs(watchedPath)
			if watchedAbs == absPath {
				isWatched = true
				break
			}
		}

		if isWatched {
			serversToRestart = append(serversToRestart, serverName)
		}
	}
	w.mu.Unlock()

	// Schedule restarts outside the lock to avoid holding lock across blocking operation
	for _, serverName := range serversToRestart {
		w.logger.Info("mcp server source file changed",
			"server", serverName,
			"file", absPath,
		)

		w.scheduleRestart(serverName)
	}
}

// scheduleRestart schedules a debounced restart for a server.
// This is extracted to keep lock scopes minimal and avoid holding locks across timer operations.
func (w *Watcher) scheduleRestart(serverName string) {
	w.mu.Lock()
	// Cancel existing timer if any
	if timer, exists := w.pendingRestarts[serverName]; exists {
		timer.Stop()
		delete(w.pendingRestarts, serverName)
	}
	w.mu.Unlock()

	// Create a copy of serverName for the closure to avoid capture issues
	name := serverName

	// Schedule debounced restart (outside lock)
	timer := time.AfterFunc(w.debounceDelay, func() {
		w.triggerRestart(name)
	})

	// Store the timer
	w.mu.Lock()
	w.pendingRestarts[serverName] = timer
	w.mu.Unlock()
}

// triggerRestart triggers an immediate restart of a server.
func (w *Watcher) triggerRestart(serverName string) {
	w.mu.Lock()
	delete(w.pendingRestarts, serverName)
	w.mu.Unlock()

	w.logger.Info("restarting mcp server after file changes", "server", serverName)

	if err := w.manager.Restart(serverName); err != nil {
		w.logger.Error("failed to restart mcp server",
			"server", serverName,
			"error", err,
		)
	}
}

// Close shuts down the watcher.
func (w *Watcher) Close() error {
	w.cancel()

	// Cancel all pending restart timers
	w.mu.Lock()
	for _, timer := range w.pendingRestarts {
		timer.Stop()
	}
	w.mu.Unlock()

	// Wait for event processing to stop
	w.wg.Wait()

	return w.fsWatcher.Close()
}
