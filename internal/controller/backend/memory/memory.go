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

// Package memory provides an in-memory backend implementation.
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tombee/conductor/internal/controller/backend"
)

// Compile-time interface assertions.
// Ensures Backend implements all segregated interfaces.
var (
	_ backend.RunStore        = (*Backend)(nil)
	_ backend.RunLister       = (*Backend)(nil)
	_ backend.CheckpointStore = (*Backend)(nil)
	_ backend.Backend         = (*Backend)(nil)
	_ backend.ScheduleBackend = (*Backend)(nil)
)

// Backend is an in-memory storage backend.
type Backend struct {
	mu          sync.RWMutex
	runs        map[string]*backend.Run
	checkpoints map[string]*backend.Checkpoint
	schedules   map[string]*backend.ScheduleState
}

// New creates a new in-memory backend.
func New() *Backend {
	return &Backend{
		runs:        make(map[string]*backend.Run),
		checkpoints: make(map[string]*backend.Checkpoint),
		schedules:   make(map[string]*backend.ScheduleState),
	}
}

// CreateRun creates a new run.
func (b *Backend) CreateRun(ctx context.Context, run *backend.Run) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.runs[run.ID]; exists {
		return fmt.Errorf("run already exists: %s", run.ID)
	}

	run.CreatedAt = time.Now()
	run.UpdatedAt = run.CreatedAt
	b.runs[run.ID] = run
	return nil
}

// GetRun retrieves a run by ID.
func (b *Backend) GetRun(ctx context.Context, id string) (*backend.Run, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	run, exists := b.runs[id]
	if !exists {
		return nil, fmt.Errorf("run not found: %s", id)
	}
	return run, nil
}

// UpdateRun updates an existing run.
func (b *Backend) UpdateRun(ctx context.Context, run *backend.Run) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.runs[run.ID]; !exists {
		return fmt.Errorf("run not found: %s", run.ID)
	}

	run.UpdatedAt = time.Now()
	b.runs[run.ID] = run
	return nil
}

// ListRuns lists runs with optional filtering.
func (b *Backend) ListRuns(ctx context.Context, filter backend.RunFilter) ([]*backend.Run, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []*backend.Run
	for _, run := range b.runs {
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		if filter.Workflow != "" && run.Workflow != filter.Workflow {
			continue
		}
		result = append(result, run)
	}

	// Apply limit
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

// DeleteRun deletes a run.
func (b *Backend) DeleteRun(ctx context.Context, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.runs, id)
	delete(b.checkpoints, id)
	return nil
}

// SaveCheckpoint saves a checkpoint.
func (b *Backend) SaveCheckpoint(ctx context.Context, runID string, checkpoint *backend.Checkpoint) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	checkpoint.RunID = runID
	checkpoint.CreatedAt = time.Now()
	b.checkpoints[runID] = checkpoint
	return nil
}

// GetCheckpoint retrieves a checkpoint.
func (b *Backend) GetCheckpoint(ctx context.Context, runID string) (*backend.Checkpoint, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	checkpoint, exists := b.checkpoints[runID]
	if !exists {
		return nil, fmt.Errorf("checkpoint not found for run: %s", runID)
	}
	return checkpoint, nil
}

// Close closes the backend.
func (b *Backend) Close() error {
	return nil
}

// SaveScheduleState saves or updates a schedule state.
func (b *Backend) SaveScheduleState(ctx context.Context, state *backend.ScheduleState) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	state.UpdatedAt = time.Now()
	b.schedules[state.Name] = state
	return nil
}

// GetScheduleState retrieves a schedule state by name.
func (b *Backend) GetScheduleState(ctx context.Context, name string) (*backend.ScheduleState, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	state, exists := b.schedules[name]
	if !exists {
		return nil, fmt.Errorf("schedule state not found: %s", name)
	}
	return state, nil
}

// ListScheduleStates returns all schedule states.
func (b *Backend) ListScheduleStates(ctx context.Context) ([]*backend.ScheduleState, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*backend.ScheduleState, 0, len(b.schedules))
	for _, state := range b.schedules {
		result = append(result, state)
	}
	return result, nil
}

// DeleteScheduleState deletes a schedule state.
func (b *Backend) DeleteScheduleState(ctx context.Context, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.schedules, name)
	return nil
}
