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

// Step execution logic for workflow runs.
// Contains the core execution loop that orchestrates workflow execution,
// manages MCP server lifecycle, and tracks execution state.
package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

// execute runs the workflow.
func (r *Runner) execute(run *Run) {
	// Check if cancelled before even starting
	select {
	case <-run.stopped:
		r.mu.Lock()
		run.Status = RunStatusCancelled
		now := time.Now()
		run.CompletedAt = &now
		r.mu.Unlock()
		r.addLog(run, "info", "Run cancelled before execution started", "")
		return
	default:
	}

	// Acquire semaphore
	select {
	case r.semaphore <- struct{}{}:
		defer func() { <-r.semaphore }()
	case <-run.stopped:
		r.mu.Lock()
		run.Status = RunStatusCancelled
		now := time.Now()
		run.CompletedAt = &now
		r.mu.Unlock()
		r.addLog(run, "info", "Run cancelled while waiting for semaphore", "")
		return
	}

	// Update status to running
	r.mu.Lock()
	run.Status = RunStatusRunning
	now := time.Now()
	run.StartedAt = &now
	metrics := r.metrics
	r.mu.Unlock()

	// Record run start for metrics and decrement queue depth (no longer pending)
	if metrics != nil {
		metrics.RecordRunStart(run.ctx, run.ID, run.WorkflowID)
		metrics.DecrementQueueDepth()
	}

	// Update backend
	if r.backend != nil {
		beRun := r.toBackendRun(run)
		_ = r.backend.UpdateRun(context.Background(), beRun)
	}

	r.addLog(run, "info", fmt.Sprintf("Starting workflow: %s", run.Workflow), "")

	// Start MCP servers if defined in workflow
	mcpServerNames := []string{}
	if err := r.startMCPServers(run); err != nil {
		r.mu.Lock()
		run.Status = RunStatusFailed
		run.Error = fmt.Sprintf("Failed to start MCP servers: %v", err)
		completedAt := time.Now()
		run.CompletedAt = &completedAt
		r.mu.Unlock()
		r.addLog(run, "error", fmt.Sprintf("Failed to start MCP servers: %v", err), "")
		return
	}
	// Track MCP server names for cleanup
	for _, mcpServer := range run.definition.MCPServers {
		mcpServerNames = append(mcpServerNames, mcpServer.Name)
	}
	// Ensure MCP servers are stopped when execution completes
	defer func() {
		for _, serverName := range mcpServerNames {
			if err := r.mcpManager.Stop(serverName); err != nil {
				r.addLog(run, "warn", fmt.Sprintf("Failed to stop MCP server %s: %v", serverName, err), "")
			}
		}
	}()

	// Execute workflow using the execution adapter
	r.mu.RLock()
	adapter := r.adapter
	r.mu.RUnlock()

	if adapter == nil {
		// No adapter configured - workflow execution will fail
		r.mu.Lock()
		run.Status = RunStatusFailed
		run.Error = "no execution adapter configured - check daemon initialization"
		completedAt := time.Now()
		run.CompletedAt = &completedAt
		r.mu.Unlock()
		r.addLog(run, "error", "No execution adapter configured for step execution", "")

		// Update backend
		if r.backend != nil {
			beRun := r.toBackendRun(run)
			_ = r.backend.UpdateRun(context.Background(), beRun)
		}
		return
	}

	// Use adapter for workflow execution
	r.executeWithAdapter(run, adapter)
}

// executeWithAdapter executes the workflow using the ExecutionAdapter.
func (r *Runner) executeWithAdapter(run *Run, adapter ExecutionAdapter) {
	opts := ExecutionOptions{
		RunID: run.ID,
		OnStepStart: func(stepID string, stepIndex int, total int) {
			r.mu.Lock()
			run.Progress.CurrentStep = stepID
			run.Progress.Completed = stepIndex
			r.mu.Unlock()

			// Save checkpoint before step
			workflowCtx := make(map[string]any)
			workflowCtx["inputs"] = run.Inputs
			r.saveCheckpoint(run, stepIndex, workflowCtx)
		},
		OnStepEnd: func(stepID string, result *workflow.StepResult, err error) {
			// Record step metrics
			r.mu.RLock()
			metricsCollector := r.metrics
			r.mu.RUnlock()

			if metricsCollector != nil && result != nil {
				status := "success"
				if err != nil {
					status = "error"
				} else if result.Status == workflow.StepStatusSkipped {
					status = "skipped"
				} else if result.Status == workflow.StepStatusFailed {
					status = "failed"
				}
				metricsCollector.RecordStepComplete(run.ctx, run.WorkflowID, stepID, status, result.Duration)
			}
		},
		OnLog: func(level, message, stepID string) {
			r.addLog(run, level, message, stepID)
		},
	}

	result, err := adapter.ExecuteWorkflow(run.ctx, run.definition, run.Inputs, opts)

	// Update final status
	r.mu.Lock()
	completedAt := time.Now()
	run.CompletedAt = &completedAt
	run.Progress.Completed = len(run.definition.Steps)

	var duration time.Duration
	if run.StartedAt != nil {
		duration = completedAt.Sub(*run.StartedAt)
	}

	if err != nil {
		// Check if the error is due to cancellation
		if err == context.Canceled || err == context.DeadlineExceeded {
			run.Status = RunStatusCancelled
			run.Error = "cancelled by user"
		} else {
			run.Status = RunStatusFailed
			run.Error = err.Error()
		}
	} else {
		run.Status = RunStatusCompleted
		if result != nil {
			// Use typed StepOutput if available, fall back to legacy Output
			if result.StepOutput != nil {
				run.Output = outputToMap(*result.StepOutput)
			} else {
				run.Output = result.Output
			}
		}
	}

	status := string(run.Status)
	metrics := r.metrics
	r.mu.Unlock()

	// Record run completion for metrics
	if metrics != nil {
		metrics.RecordRunComplete(run.ctx, run.ID, run.WorkflowID, status, "api", duration)
	}

	// Update backend
	if r.backend != nil {
		beRun := r.toBackendRun(run)
		_ = r.backend.UpdateRun(context.Background(), beRun)
	}

	// Clean up checkpoint on successful completion
	if r.checkpoints != nil && run.Status == RunStatusCompleted {
		_ = r.checkpoints.Delete(context.Background(), run.ID)
	}

	r.addLog(run, "info", fmt.Sprintf("Workflow %s: %s", run.Status, run.Workflow), "")
}
