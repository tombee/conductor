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

package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tombee/conductor/internal/binding"
	"github.com/tombee/conductor/internal/controller/backend"
	"github.com/tombee/conductor/internal/tracing"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/cost"
	"github.com/tombee/conductor/pkg/workflow"
)

// StateManager handles run state management with thread-safe snapshots.
// In-memory map is the source of truth; backend persistence is best-effort.
type StateManager struct {
	mu        sync.RWMutex
	runs      map[string]*Run
	backend   backend.Backend
	costStore cost.CostStore
}

// NewStateManager creates a new StateManager.
func NewStateManager(be backend.Backend) *StateManager {
	return &StateManager{
		runs:    make(map[string]*Run),
		backend: be,
	}
}

// SetCostStore sets the cost store for aggregating run costs.
func (s *StateManager) SetCostStore(store cost.CostStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.costStore = store
}

// CreateRun creates a new run and persists to backend (best-effort).
// Returns the created Run (internal) for further processing.
func (s *StateManager) CreateRun(ctx context.Context, def *workflow.Definition, inputs map[string]any, sourceURL, workspace, profile string, bindings *binding.ResolvedBinding, overrides *RunOverrides) (*Run, error) {
	runID := uuid.New().String()[:8]
	runCtx, cancel := context.WithCancel(ctx)

	// Extract correlation ID from context (set by HTTP middleware)
	correlationID := string(tracing.FromContextOrEmpty(ctx))

	run := &Run{
		ID:            runID,
		WorkflowID:    def.Name,
		Workflow:      def.Name,
		Status:        RunStatusPending,
		CorrelationID: correlationID,
		Inputs:        inputs,
		CreatedAt:     time.Now(),
		SourceURL:     sourceURL,
		Workspace:     workspace,
		Profile:       profile,
		Progress: &Progress{
			Total: len(def.Steps),
		},
		ctx:        runCtx,
		cancel:     cancel,
		definition: def,
		bindings:   bindings,
		stopped:    make(chan struct{}),
	}

	// Apply runtime overrides if provided
	if overrides != nil {
		run.Provider = overrides.Provider
		run.Model = overrides.Model
		run.Timeout = overrides.Timeout
		run.Security = overrides.Security
		run.AllowHosts = overrides.AllowHosts
		run.AllowPaths = overrides.AllowPaths
		run.MCPDev = overrides.MCPDev
	}

	s.mu.Lock()
	s.runs[runID] = run
	s.mu.Unlock()

	// Persist to backend (best-effort)
	if s.backend != nil {
		beRun := s.toBackendRun(run)
		if err := s.backend.CreateRun(ctx, beRun); err != nil {
			// Log error but continue - in-memory state is the source of truth
			// Caller can add log via LogAggregator if needed
			_ = err
		}
	}

	return run, nil
}

// GetRun returns an immutable snapshot of a run by ID.
func (s *StateManager) GetRun(id string) (*RunSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, exists := s.runs[id]
	if !exists {
		return nil, fmt.Errorf("run not found: %s", id)
	}
	return s.snapshotRun(run), nil
}

// GetRunInternal returns the internal Run by ID (for execution).
// Use with caution - caller must handle thread-safety.
func (s *StateManager) GetRunInternal(id string) (*Run, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, exists := s.runs[id]
	return run, exists
}

// UpdateRun updates run state and persists to backend (best-effort).
func (s *StateManager) UpdateRun(ctx context.Context, run *Run) error {
	if s.backend != nil {
		beRun := s.toBackendRun(run)
		if err := s.backend.UpdateRun(ctx, beRun); err != nil {
			// Best-effort - log but don't fail
			_ = err
		}
	}
	return nil
}

// ListRuns returns immutable snapshots of all runs, optionally filtered.
func (s *StateManager) ListRuns(filter ListFilter) []*RunSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*RunSnapshot
	for _, run := range s.runs {
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		if filter.Workflow != "" && run.Workflow != filter.Workflow {
			continue
		}
		result = append(result, s.snapshotRun(run))
	}
	return result
}

// ActiveRunCount returns the number of currently active (running or pending) runs.
func (s *StateManager) ActiveRunCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, run := range s.runs {
		if run.Status == RunStatusRunning || run.Status == RunStatusPending {
			count++
		}
	}
	return count
}

// CancelAll cancels all active runs.
func (s *StateManager) CancelAll() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, run := range s.runs {
		if run.Status == RunStatusRunning || run.Status == RunStatusPending {
			run.cancel()
		}
	}
}

// Snapshot creates an immutable snapshot of a run (must hold lock or call externally).
func (s *StateManager) Snapshot(run *Run) *RunSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotRun(run)
}

// snapshotRun creates an immutable deep copy of a Run (must hold StateManager lock).
// Returns a snapshot with NO aliasing to internal mutable state.
func (s *StateManager) snapshotRun(run *Run) *RunSnapshot {
	run.mu.RLock()
	defer run.mu.RUnlock()

	// Deep copy logs slice
	logs := make([]LogEntry, len(run.Logs))
	copy(logs, run.Logs)

	// Deep copy Progress struct if it exists
	var progress *Progress
	if run.Progress != nil {
		progressCopy := *run.Progress
		progress = &progressCopy
	}

	// Deep copy Inputs map to prevent aliasing
	var inputs map[string]any
	if run.Inputs != nil {
		inputs = make(map[string]any, len(run.Inputs))
		for k, v := range run.Inputs {
			inputs[k] = v
		}
	}

	// Deep copy Output map to prevent aliasing
	var output map[string]any
	if run.Output != nil {
		output = make(map[string]any, len(run.Output))
		for k, v := range run.Output {
			output[k] = v
		}
	}

	// Deep copy AllowHosts and AllowPaths slices to prevent aliasing
	var allowHosts []string
	if run.AllowHosts != nil {
		allowHosts = make([]string, len(run.AllowHosts))
		copy(allowHosts, run.AllowHosts)
	}
	var allowPaths []string
	if run.AllowPaths != nil {
		allowPaths = make([]string, len(run.AllowPaths))
		copy(allowPaths, run.AllowPaths)
	}

	// Query and aggregate costs for this run
	var costAggregate *llm.CostAggregate
	if s.costStore != nil {
		// Query costs by RunID
		costs, err := s.costStore.GetByRunID(context.Background(), run.ID)
		if err == nil && len(costs) > 0 {
			// Aggregate the costs
			aggregate, err := s.costStore.Aggregate(context.Background(), cost.AggregateOptions{
				RunID: run.ID,
			})
			if err == nil {
				costAggregate = aggregate
			}
		}
	}

	return &RunSnapshot{
		ID:            run.ID,
		WorkflowID:    run.WorkflowID,
		Workflow:      run.Workflow,
		Status:        run.Status,
		CorrelationID: run.CorrelationID,
		Inputs:        inputs,
		Output:        output,
		Error:         run.Error,
		Progress:      progress,
		StartedAt:     run.StartedAt,
		CompletedAt:   run.CompletedAt,
		CreatedAt:     run.CreatedAt,
		Logs:          logs,
		SourceURL:     run.SourceURL,
		Workspace:     run.Workspace,
		Profile:       run.Profile,
		Provider:      run.Provider,
		Model:         run.Model,
		Timeout:       run.Timeout,
		Security:      run.Security,
		AllowHosts:    allowHosts,
		AllowPaths:    allowPaths,
		MCPDev:        run.MCPDev,
		Cost:          costAggregate,
	}
}

// toBackendRun converts a Run to a backend.Run.
// Must acquire run.mu.RLock to safely read mutable fields.
func (s *StateManager) toBackendRun(run *Run) *backend.Run {
	run.mu.RLock()
	defer run.mu.RUnlock()

	beRun := &backend.Run{
		ID:            run.ID,
		WorkflowID:    run.WorkflowID,
		Workflow:      run.Workflow,
		Status:        string(run.Status),
		CorrelationID: run.CorrelationID,
		Inputs:        run.Inputs,
		Output:        run.Output,
		Error:         run.Error,
		CreatedAt:     run.CreatedAt,
	}
	if run.Progress != nil {
		beRun.CurrentStep = run.Progress.CurrentStep
		beRun.Completed = run.Progress.Completed
		beRun.Total = run.Progress.Total
	}
	if run.StartedAt != nil {
		beRun.StartedAt = run.StartedAt
	}
	if run.CompletedAt != nil {
		beRun.CompletedAt = run.CompletedAt
	}
	return beRun
}

// stepOutputToMap converts a workflow.StepOutput to a map for run.Output.
func stepOutputToMap(output workflow.StepOutput) map[string]any {
	result := make(map[string]any)

	if output.Text != "" {
		result["response"] = output.Text
	}

	if output.Data != nil {
		// If Data is already a map, merge it
		if dataMap, ok := output.Data.(map[string]any); ok {
			for k, v := range dataMap {
				result[k] = v
			}
		} else {
			result["data"] = output.Data
		}
	}

	if output.Error != "" {
		result["error"] = output.Error
	}

	return result
}
