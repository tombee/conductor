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

// Package filewatcher provides filesystem event monitoring for triggering workflows.
package filewatcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tombee/conductor/internal/controller/runner"
	"golang.org/x/time/rate"
)

// WatchConfig defines configuration for a single file watcher.
type WatchConfig struct {
	// Name is a unique identifier for this watcher
	Name string

	// Workflow is the workflow file to run when events occur
	Workflow string

	// Paths are the filesystem paths to watch
	Paths []string

	// Events are the event types to watch (created, modified, deleted, renamed)
	// If empty, defaults to all event types
	Events []string

	// IncludePatterns are glob patterns for files to include
	// If empty, all files are included
	IncludePatterns []string

	// ExcludePatterns are glob patterns for files to exclude
	// Applied after include patterns
	ExcludePatterns []string

	// DebounceWindow is the duration to wait for additional events before triggering
	// Zero disables debouncing
	DebounceWindow time.Duration

	// BatchMode determines if events during debounce window are batched together
	// If false, only the last event is delivered
	BatchMode bool

	// MaxTriggersPerMinute limits the rate of workflow triggers
	// Zero means no limit
	MaxTriggersPerMinute int

	// Recursive enables recursive directory watching
	Recursive bool

	// MaxDepth limits recursive watching depth (0 = default 10 levels)
	MaxDepth int

	// Inputs are passed to the workflow along with the file context
	Inputs map[string]any
}

// Service manages all file watchers for the controller.
type Service struct {
	mu           sync.RWMutex
	watchers     map[string]*watcherEntry // keyed by watcher name
	runner       *runner.Runner
	workflowsDir string
	ctx          context.Context
	cancel       context.CancelFunc
	logger       *slog.Logger
}

// watcherEntry tracks a watcher and its configuration.
type watcherEntry struct {
	config         WatchConfig
	watcher        *Watcher
	patternMatcher *PatternMatcher
	debouncer      *Debouncer
	rateLimiter    *rate.Limiter
	stopCh         chan struct{}
}

// NewService creates a new file watcher service.
func NewService(workflowsDir string, r *runner.Runner) *Service {
	return &Service{
		watchers:     make(map[string]*watcherEntry),
		runner:       r,
		workflowsDir: workflowsDir,
		logger:       slog.Default().With(slog.String("component", "filewatcher-service")),
	}
}

// Start starts the file watcher service.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.logger.Info("file watcher service started")
	return nil
}

// Stop stops the file watcher service and all active watchers.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	// Stop all watchers
	for name, entry := range s.watchers {
		// Stop debouncer first to flush pending events
		if entry.debouncer != nil {
			entry.debouncer.Stop()
		}

		if err := entry.watcher.Stop(); err != nil {
			s.logger.Error("failed to stop watcher", "name", name, "error", err)
		}
	}

	s.watchers = make(map[string]*watcherEntry)
	s.logger.Info("file watcher service stopped")
	return nil
}

// AddWatcher adds a new file watcher.
func (s *Service) AddWatcher(config WatchConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate config
	if config.Name == "" {
		return fmt.Errorf("watcher name is required")
	}
	if config.Workflow == "" {
		return fmt.Errorf("workflow is required")
	}
	if len(config.Paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}

	// Check if watcher already exists
	if _, exists := s.watchers[config.Name]; exists {
		return fmt.Errorf("watcher %s already exists", config.Name)
	}

	// For MVP, support single path only
	if len(config.Paths) > 1 {
		return fmt.Errorf("multiple paths not yet supported (use separate watchers)")
	}

	path := config.Paths[0]

	// Normalize and validate path (expands ~, env vars, resolves symlinks, validates security)
	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Create watcher for the root path
	w, err := NewWatcher(normalizedPath, config.Events)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Handle recursive watching by adding subdirectories
	if config.Recursive {
		maxDepth := config.MaxDepth
		if maxDepth == 0 {
			maxDepth = 10 // Default max depth to prevent runaway recursion
		}
		pathsToWatch, err := WalkDirectory(normalizedPath, maxDepth)
		if err != nil {
			w.Stop()
			return fmt.Errorf("failed to walk directory: %w", err)
		}
		// Add all subdirectories to the watcher (skip first which is root, already added)
		for i := 1; i < len(pathsToWatch); i++ {
			if err := w.AddPath(pathsToWatch[i]); err != nil {
				s.logger.Warn("failed to add subdirectory to watcher",
					"path", pathsToWatch[i],
					"error", err)
			}
		}
	}

	// Start watcher
	if err := w.Start(s.ctx); err != nil {
		w.Stop()
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	// Create pattern matcher if patterns specified
	var pm *PatternMatcher
	if len(config.IncludePatterns) > 0 || len(config.ExcludePatterns) > 0 {
		pm, err = NewPatternMatcher(config.IncludePatterns, config.ExcludePatterns)
		if err != nil {
			w.Stop()
			return fmt.Errorf("failed to create pattern matcher: %w", err)
		}
	}

	// Create entry
	entry := &watcherEntry{
		config:         config,
		watcher:        w,
		patternMatcher: pm,
		stopCh:         make(chan struct{}),
	}

	// Setup rate limiter if configured
	if config.MaxTriggersPerMinute > 0 {
		// Convert triggers per minute to tokens per second
		tokensPerSecond := float64(config.MaxTriggersPerMinute) / 60.0
		entry.rateLimiter = rate.NewLimiter(rate.Limit(tokensPerSecond), 1)
	}

	s.watchers[config.Name] = entry

	// Update active watchers metric
	fileWatcherActive.Set(float64(len(s.watchers)))

	// Start event handler
	go s.handleEvents(entry)

	s.logger.Info("file watcher added",
		"name", config.Name,
		"path", normalizedPath,
		"recursive", config.Recursive,
		"max_depth", config.MaxDepth,
		"debounce", config.DebounceWindow,
		"batch_mode", config.BatchMode,
		"rate_limit", config.MaxTriggersPerMinute)
	return nil
}

// RemoveWatcher removes a file watcher.
func (s *Service) RemoveWatcher(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.watchers[name]
	if !exists {
		return fmt.Errorf("watcher %s not found", name)
	}

	// Stop debouncer if present
	if entry.debouncer != nil {
		entry.debouncer.Stop()
	}

	// Stop watcher
	if err := entry.watcher.Stop(); err != nil {
		return fmt.Errorf("failed to stop watcher: %w", err)
	}

	delete(s.watchers, name)

	// Update active watchers metric
	fileWatcherActive.Set(float64(len(s.watchers)))

	s.logger.Info("file watcher removed", "name", name)
	return nil
}

// ListWatchers returns all configured watchers.
func (s *Service) ListWatchers() []WatchConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configs := make([]WatchConfig, 0, len(s.watchers))
	for _, entry := range s.watchers {
		configs = append(configs, entry.config)
	}
	return configs
}

// WatcherStatus represents the status of a file watcher.
type WatcherStatus struct {
	Name      string   `json:"name"`
	Paths     []string `json:"paths"`
	Events    []string `json:"events"`
	Recursive bool     `json:"recursive"`
	MaxDepth  int      `json:"max_depth,omitempty"`
	Workflow  string   `json:"workflow"`
}

// Status returns the status of all file watchers.
func (s *Service) Status() []WatcherStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := make([]WatcherStatus, 0, len(s.watchers))
	for _, entry := range s.watchers {
		status = append(status, WatcherStatus{
			Name:      entry.config.Name,
			Paths:     entry.config.Paths,
			Events:    entry.config.Events,
			Recursive: entry.config.Recursive,
			MaxDepth:  entry.config.MaxDepth,
			Workflow:  entry.config.Workflow,
		})
	}
	return status
}

// handleEvents processes file events and triggers workflows.
func (s *Service) handleEvents(entry *watcherEntry) {
	config := entry.config

	// Setup debouncer if configured
	if config.DebounceWindow > 0 {
		entry.debouncer = NewDebouncer(config.DebounceWindow, config.BatchMode, func(events []*Context) {
			s.triggerWorkflows(entry, events)
		})
	}

	// Process events from watcher
	for ctx := range entry.watcher.Events() {
		// Record event
		recordEvent(config.Name, ctx.Event)

		// Re-resolve symlinks for TOCTOU safety (prevent symlink attack between watch and trigger)
		resolvedPath, err := ResolveSymlink(ctx.Path)
		if err != nil {
			recordError(config.Name, "symlink_resolution")
			s.logger.Warn("failed to resolve symlink, skipping event",
				"path", ctx.Path,
				"watcher", config.Name,
				"error", err)
			continue
		}
		// Update context with resolved path
		ctx.Path = resolvedPath

		// Apply pattern matching if configured
		if entry.patternMatcher != nil {
			if !entry.patternMatcher.Match(ctx.Path) {
				recordPatternExcluded(config.Name)
				s.logger.Debug("file excluded by pattern",
					"path", ctx.Path,
					"watcher", config.Name)
				continue
			}
		}

		// Route through debouncer or trigger directly
		if entry.debouncer != nil {
			entry.debouncer.Add(ctx)
		} else {
			s.triggerWorkflows(entry, []*Context{ctx})
		}
	}
}

// triggerWorkflows triggers workflow runs for the given file events.
// It handles rate limiting and batch mode.
func (s *Service) triggerWorkflows(entry *watcherEntry, events []*Context) {
	if len(events) == 0 {
		return
	}

	config := entry.config

	// Apply rate limiting if configured
	if entry.rateLimiter != nil {
		if !entry.rateLimiter.Allow() {
			recordRateLimited(config.Name)
			s.logger.Warn("rate limit exceeded, dropping events",
				"watcher", config.Name,
				"count", len(events))
			return
		}
	}

	// Record trigger
	recordTrigger(config.Name)

	// Resolve workflow path
	workflowPath := config.Workflow
	if !filepath.IsAbs(workflowPath) {
		workflowPath = filepath.Join(s.workflowsDir, workflowPath)
	}

	// Read workflow file
	workflowYAML, err := os.ReadFile(workflowPath)
	if err != nil {
		recordError(config.Name, "workflow_read")
		s.logger.Error("failed to read workflow file",
			"workflow", config.Workflow,
			"path", workflowPath,
			"error", err)
		return
	}

	// Build workflow inputs
	inputs := make(map[string]any)

	// Copy configured inputs
	for k, v := range config.Inputs {
		inputs[k] = v
	}

	// Add trigger context
	// In batch mode with multiple events, provide both single file context and array
	if config.BatchMode && len(events) > 1 {
		// Provide array of all file events
		fileEvents := make([]any, len(events))
		for i, evt := range events {
			fileEvents[i] = evt
		}
		inputs["trigger"] = map[string]any{
			"file":  events[0], // First event for backward compatibility
			"files": fileEvents,
			"count": len(events),
		}
	} else {
		// Single event (either non-batch or batch with 1 event)
		inputs["trigger"] = map[string]any{
			"file": events[0],
		}
	}

	s.logger.Info("file event triggered workflow",
		"workflow", config.Name,
		"event_count", len(events),
		"paths", func() []string {
			paths := make([]string, len(events))
			for i, evt := range events {
				paths[i] = evt.Path
			}
			return paths
		}())

	// Submit workflow run
	_, err = s.runner.Submit(s.ctx, runner.SubmitRequest{
		WorkflowYAML: workflowYAML,
		Inputs:       inputs,
	})
	if err != nil {
		recordError(config.Name, "workflow_submit")
		s.logger.Error("failed to submit workflow run",
			"workflow", config.Workflow,
			"error", err)
	}
}
