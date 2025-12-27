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

// Run state management - snapshot creation, persistence, and logging.
// Handles deep copying of mutable state to prevent aliasing, conversion to backend format,
// and log entry management with subscriber notifications.
package runner

import (
	"time"

	"github.com/tombee/conductor/internal/daemon/backend"
	"github.com/tombee/conductor/pkg/workflow"
)

// snapshotRun creates an immutable deep copy of a Run (must hold lock).
// Returns a snapshot with NO aliasing to internal mutable state.
func (r *Runner) snapshotRun(run *Run) *RunSnapshot {
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
	}
}

// toBackendRun converts a Run to a backend.Run.
func (r *Runner) toBackendRun(run *Run) *backend.Run {
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

// addLog adds a log entry and notifies subscribers.
func (r *Runner) addLog(run *Run, level, message, stepID string) {
	entry := LogEntry{
		Timestamp:     time.Now(),
		Level:         level,
		Message:       message,
		StepID:        stepID,
		CorrelationID: run.CorrelationID,
	}

	r.mu.Lock()
	run.Logs = append(run.Logs, entry)
	r.mu.Unlock()

	// Notify subscribers
	r.subMu.RLock()
	subs := r.subscribers[run.ID]
	r.subMu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- entry:
		default:
			// Channel full, skip
		}
	}
}
