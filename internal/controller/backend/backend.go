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

// Package backend provides storage backends for the daemon.
//
// # Interface Hierarchy
//
// The backend package uses interface segregation to allow minimal implementations:
//
//   - RunStore (core, required): CreateRun, GetRun, UpdateRun
//   - RunLister (optional): ListRuns, DeleteRun
//   - CheckpointStore (optional): SaveCheckpoint, GetCheckpoint
//   - io.Closer (optional): Close
//
// The Backend interface composes all of these for full-featured implementations.
// Components can accept RunStore for minimal requirements and use type assertions
// to detect optional capabilities at runtime.
package backend

import (
	"context"
	"io"
	"time"
)

// RunStore is the core interface for run storage operations.
// This is the minimal interface that backends must implement for basic operation.
// Components that only need create/get/update operations should accept this interface.
type RunStore interface {
	// CreateRun creates a new run in storage.
	CreateRun(ctx context.Context, run *Run) error

	// GetRun retrieves a run by ID.
	GetRun(ctx context.Context, id string) (*Run, error)

	// UpdateRun updates an existing run.
	UpdateRun(ctx context.Context, run *Run) error
}

// RunLister is an optional interface for listing and deleting runs.
// Backends can implement this to support run listing and deletion.
// Use type assertion to detect if a backend supports this capability:
//
//	if lister, ok := store.(RunLister); ok {
//	    runs, err := lister.ListRuns(ctx, filter)
//	}
type RunLister interface {
	// ListRuns lists runs with optional filtering.
	ListRuns(ctx context.Context, filter RunFilter) ([]*Run, error)

	// DeleteRun deletes a run by ID.
	DeleteRun(ctx context.Context, id string) error
}

// CheckpointStore is an optional interface for checkpoint storage.
// Backends can implement this to support checkpoint persistence.
// Use type assertion to detect if a backend supports this capability:
//
//	if checkpointer, ok := store.(CheckpointStore); ok {
//	    err := checkpointer.SaveCheckpoint(ctx, runID, checkpoint)
//	}
type CheckpointStore interface {
	// SaveCheckpoint saves a checkpoint for a run.
	SaveCheckpoint(ctx context.Context, runID string, checkpoint *Checkpoint) error

	// GetCheckpoint retrieves a checkpoint for a run.
	GetCheckpoint(ctx context.Context, runID string) (*Checkpoint, error)
}

// Backend defines the full interface for daemon storage.
// This is a composite interface that embeds all segregated interfaces
// plus io.Closer for lifecycle management.
//
// Existing backends (memory, postgres) implement all methods and satisfy
// this interface. New minimal backends can implement just RunStore.
type Backend interface {
	RunStore
	RunLister
	CheckpointStore
	io.Closer
}

// Run represents a workflow run in storage.
type Run struct {
	ID            string         `json:"id"`
	WorkflowID    string         `json:"workflow_id"`
	Workflow      string         `json:"workflow"`
	Status        string         `json:"status"`
	CorrelationID string         `json:"correlation_id,omitempty"` // Correlation ID for request tracing
	Inputs        map[string]any `json:"inputs,omitempty"`
	Output        map[string]any `json:"output,omitempty"`
	Error         string         `json:"error,omitempty"`
	CurrentStep   string         `json:"current_step,omitempty"`
	Completed     int            `json:"completed"`
	Total         int            `json:"total"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// RunFilter contains filtering options for listing runs.
type RunFilter struct {
	Status   string
	Workflow string
	Limit    int
	Offset   int
}

// Checkpoint represents a workflow execution checkpoint.
type Checkpoint struct {
	RunID     string         `json:"run_id"`
	StepID    string         `json:"step_id"`
	StepIndex int            `json:"step_index"`
	Context   map[string]any `json:"context"`
	CreatedAt time.Time      `json:"created_at"`
}

// ScheduleState represents the persistent state of a schedule.
type ScheduleState struct {
	Name       string     `json:"name"`
	LastRun    *time.Time `json:"last_run,omitempty"`
	NextRun    *time.Time `json:"next_run,omitempty"`
	RunCount   int64      `json:"run_count"`
	ErrorCount int64      `json:"error_count"`
	Enabled    bool       `json:"enabled"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// ScheduleBackend extends Backend with schedule persistence.
// This is an optional interface that backends can implement.
type ScheduleBackend interface {
	Backend

	// Schedule state operations
	SaveScheduleState(ctx context.Context, state *ScheduleState) error
	GetScheduleState(ctx context.Context, name string) (*ScheduleState, error)
	ListScheduleStates(ctx context.Context) ([]*ScheduleState, error)
	DeleteScheduleState(ctx context.Context, name string) error
}
