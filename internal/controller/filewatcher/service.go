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

	"github.com/tombee/conductor/internal/controller/runner"
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
	config  WatchConfig
	watcher *Watcher
	stopCh  chan struct{}
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

	// Expand path (handle ~ and env vars would go here in future phases)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Create watcher
	w, err := NewWatcher(absPath, config.Events)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Start watcher
	if err := w.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	// Create entry
	entry := &watcherEntry{
		config:  config,
		watcher: w,
		stopCh:  make(chan struct{}),
	}

	s.watchers[config.Name] = entry

	// Start event handler
	go s.handleEvents(config, w)

	s.logger.Info("file watcher added", "name", config.Name, "path", absPath)
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

	// Stop watcher
	if err := entry.watcher.Stop(); err != nil {
		return fmt.Errorf("failed to stop watcher: %w", err)
	}

	delete(s.watchers, name)
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

// handleEvents processes file events and triggers workflows.
func (s *Service) handleEvents(config WatchConfig, w *Watcher) {
	for ctx := range w.Events() {
		s.logger.Info("file event triggered workflow",
			"workflow", config.Name,
			"event", ctx.Event,
			"path", ctx.Path)

		// Resolve workflow path
		workflowPath := config.Workflow
		if !filepath.IsAbs(workflowPath) {
			workflowPath = filepath.Join(s.workflowsDir, workflowPath)
		}

		// Read workflow file
		workflowYAML, err := os.ReadFile(workflowPath)
		if err != nil {
			s.logger.Error("failed to read workflow file",
				"workflow", config.Workflow,
				"path", workflowPath,
				"error", err)
			continue
		}

		// Build workflow inputs
		inputs := make(map[string]any)

		// Copy configured inputs
		for k, v := range config.Inputs {
			inputs[k] = v
		}

		// Add trigger context
		inputs["trigger"] = map[string]any{
			"file": ctx,
		}

		// Submit workflow run
		_, err = s.runner.Submit(s.ctx, runner.SubmitRequest{
			WorkflowYAML: workflowYAML,
			Inputs:       inputs,
		})
		if err != nil {
			s.logger.Error("failed to submit workflow run",
				"workflow", config.Workflow,
				"error", err)
		}
	}
}
