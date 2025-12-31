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

// Package backend provides storage backends for the controller.
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

// StepResultStore is an optional interface for step result storage.
// Backends can implement this to support step-level debugging and inspection.
// Use type assertion to detect if a backend supports this capability:
//
//	if stepStore, ok := store.(StepResultStore); ok {
//	    err := stepStore.SaveStepResult(ctx, result)
//	}
type StepResultStore interface {
	// SaveStepResult saves a step execution result.
	SaveStepResult(ctx context.Context, result *StepResult) error

	// GetStepResult retrieves a step result by run ID and step ID.
	GetStepResult(ctx context.Context, runID, stepID string) (*StepResult, error)

	// ListStepResults retrieves all step results for a run.
	ListStepResults(ctx context.Context, runID string) ([]*StepResult, error)
}

// Backend defines the full interface for controller storage.
// This is a composite interface that embeds all segregated interfaces
// plus io.Closer for lifecycle management.
//
// Existing backends (memory, postgres) implement all methods and satisfy
// this interface. New minimal backends can implement just RunStore.
type Backend interface {
	RunStore
	RunLister
	CheckpointStore
	StepResultStore
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
	ParentRunID   string         `json:"parent_run_id,omitempty"`   // ID of the parent run for replay runs
	ReplayConfig  *ReplayConfig  `json:"replay_config,omitempty"`   // Configuration for replay execution
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

// StepResult represents the result of a single step execution.
type StepResult struct {
	RunID     string         `json:"run_id"`
	StepID    string         `json:"step_id"`
	StepIndex int            `json:"step_index"`
	Inputs    map[string]any `json:"inputs,omitempty"`
	Outputs   map[string]any `json:"outputs,omitempty"`
	Duration  time.Duration  `json:"duration"`
	Status    string         `json:"status"`
	Error     string         `json:"error,omitempty"`
	CostUSD   float64        `json:"cost_usd,omitempty"` // Cost of this step in USD
	CreatedAt time.Time      `json:"created_at"`
}

// ReplayConfig represents the configuration for a replay execution.
type ReplayConfig struct {
	ParentRunID    string            `json:"parent_run_id"`              // Original run to replay from
	FromStepID     string            `json:"from_step_id,omitempty"`     // Step to resume from (empty = start)
	OverrideInputs map[string]any    `json:"override_inputs,omitempty"`  // Input overrides
	OverrideSteps  map[string]any    `json:"override_steps,omitempty"`   // Step output overrides
	MaxCost        float64           `json:"max_cost,omitempty"`         // Cost limit in USD (0 = no limit)
	ValidateSchema bool              `json:"validate_schema"`            // Validate cached outputs
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
