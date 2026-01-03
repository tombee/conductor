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
	"log/slog"
	"time"

	"github.com/tombee/conductor/internal/controller/backend"
	"github.com/tombee/conductor/internal/debug"
	"github.com/tombee/conductor/pkg/workflow"
)

// execute runs the workflow.
func (r *Runner) execute(run *Run) {
	// Track this goroutine for clean shutdown
	r.wg.Add(1)
	defer r.wg.Done()

	// Check if cancelled before even starting
	select {
	case <-run.stopped:
		run.mu.Lock()
		run.Status = RunStatusCancelled
		now := time.Now()
		run.CompletedAt = &now
		run.mu.Unlock()
		r.addLog(run, "info", "Run cancelled before execution started", "")
		return
	default:
	}

	// Acquire semaphore
	select {
	case r.semaphore <- struct{}{}:
		defer func() { <-r.semaphore }()
	case <-run.stopped:
		run.mu.Lock()
		run.Status = RunStatusCancelled
		now := time.Now()
		run.CompletedAt = &now
		run.mu.Unlock()
		r.addLog(run, "info", "Run cancelled while waiting for semaphore", "")
		return
	}

	// Update status to running
	run.mu.Lock()
	run.Status = RunStatusRunning
	now := time.Now()
	run.StartedAt = &now
	run.mu.Unlock()

	// Record run start for metrics and decrement queue depth (no longer pending)
	r.mu.RLock()
	metrics := r.metrics
	r.mu.RUnlock()
	if metrics != nil {
		metrics.RecordRunStart(run.ctx, run.ID, run.WorkflowID)
		metrics.DecrementQueueDepth()
	}

	// Update backend with run context (respects cancellation during execution)
	if be := r.getBackend(); be != nil {
		beRun := r.toBackendRun(run)
		_ = be.UpdateRun(run.ctx, beRun)
	}

	// Log run start with profile context
	startMsg := fmt.Sprintf("Starting workflow: %s", run.Workflow)
	if run.Workspace != "" || run.Profile != "" {
		profileInfo := ""
		if run.Workspace != "" {
			profileInfo = fmt.Sprintf("workspace=%s", run.Workspace)
		}
		if run.Profile != "" {
			if profileInfo != "" {
				profileInfo += ", "
			}
			profileInfo += fmt.Sprintf("profile=%s", run.Profile)
		}
		startMsg = fmt.Sprintf("%s (%s)", startMsg, profileInfo)
	}
	r.addLog(run, "info", startMsg, "")

	// Create log function for lifecycle manager
	logFn := func(level, message, stepID string) {
		r.addLog(run, level, message, stepID)
	}

	// Start MCP servers using LifecycleManager
	mcpServerNames, err := r.lifecycle.StartMCPServers(run.ctx, run.definition, logFn)
	if err != nil {
		run.mu.Lock()
		run.Status = RunStatusFailed
		run.Error = fmt.Sprintf("Failed to start MCP servers: %v", err)
		completedAt := time.Now()
		run.CompletedAt = &completedAt
		run.mu.Unlock()
		r.addLog(run, "error", fmt.Sprintf("Failed to start MCP servers: %v", err), "")
		return
	}

	// Ensure MCP servers are stopped when execution completes
	defer r.lifecycle.StopMCPServers(mcpServerNames, logFn)

	// Execute workflow using the execution adapter
	r.mu.RLock()
	adapter := r.adapter
	r.mu.RUnlock()

	if adapter == nil {
		// No adapter configured - workflow execution will fail
		run.mu.Lock()
		run.Status = RunStatusFailed
		run.Error = "no execution adapter configured - check controller initialization"
		completedAt := time.Now()
		run.CompletedAt = &completedAt
		run.mu.Unlock()
		r.addLog(run, "error", "No execution adapter configured for step execution", "")

		// Update backend with run context (respects cancellation during execution)
		if be := r.getBackend(); be != nil {
			beRun := r.toBackendRun(run)
			_ = be.UpdateRun(run.ctx, beRun)
		}
		return
	}

	// Use adapter for workflow execution
	r.executeWithAdapter(run, adapter)
}

// executeWithAdapter executes the workflow using the ExecutionAdapter.
func (r *Runner) executeWithAdapter(run *Run, adapter ExecutionAdapter) {
	// Set up debug adapter if breakpoints are configured
	var debugAdapter *debug.Adapter
	var debugShell *debug.Shell

	if len(run.DebugBreakpoints) > 0 {
		// Create debug configuration
		debugConfig := debug.New(run.DebugBreakpoints, run.LogLevel)

		// Validate debug configuration against workflow
		if err := debugConfig.Validate(run.definition); err != nil {
			r.addLog(run, "error", fmt.Sprintf("Invalid debug configuration: %v", err), "")
			run.mu.Lock()
			run.Status = RunStatusFailed
			run.Error = fmt.Sprintf("Invalid debug configuration: %v", err)
			completedAt := time.Now()
			run.CompletedAt = &completedAt
			run.mu.Unlock()
			return
		}

		// Create logger for debug adapter
		logger := slog.Default()
		if run.LogLevel != "" {
			// Parse log level (debug adapter will use this)
			var level slog.Level
			switch run.LogLevel {
			case "trace", "debug":
				level = slog.LevelDebug
			case "info":
				level = slog.LevelInfo
			case "warn":
				level = slog.LevelWarn
			case "error":
				level = slog.LevelError
			default:
				level = slog.LevelInfo
			}
			handler := slog.NewTextHandler(nil, &slog.HandlerOptions{Level: level})
			logger = slog.New(handler)
		}

		// Create debug adapter and shell
		debugAdapter = debug.NewAdapter(debugConfig, logger)
		debugShell = debug.NewShell(debugAdapter)

		// Start debug shell in background
		go func() {
			if err := debugShell.Run(run.ctx); err != nil && err != context.Canceled {
				r.addLog(run, "warn", fmt.Sprintf("Debug shell error: %v", err), "")
			}
		}()

		r.addLog(run, "info", fmt.Sprintf("Debug mode enabled with %d breakpoint(s)", len(run.DebugBreakpoints)), "")
	}

	opts := ExecutionOptions{
		RunID:       run.ID,
		WorkflowDir: run.WorkflowDir,
		OnStepStart: func(stepID string, stepIndex int, total int) {
			run.mu.Lock()
			run.Progress.CurrentStep = stepID
			run.Progress.Completed = stepIndex
			run.Progress.Total = total
			run.mu.Unlock()

			// Get step name for display (use Name if set, otherwise use ID)
			stepName := stepID
			if stepIndex < len(run.definition.Steps) {
				step := run.definition.Steps[stepIndex]
				if step.Name != "" {
					stepName = step.Name
				}
			}

			// Send step_start event for CLI progress display
			r.addStepStart(run, stepID, stepName, stepIndex, total)

			// Notify debug adapter if active
			if debugAdapter != nil {
				// Create inputs map from workflow context
				inputs := make(map[string]interface{})
				inputs["workflow_inputs"] = run.Inputs
				if err := debugAdapter.OnStepStart(run.ctx, stepID, stepIndex, inputs); err != nil {
					r.addLog(run, "warn", fmt.Sprintf("Debug adapter error: %v", err), stepID)
				}
			}

			// Save checkpoint before step using LifecycleManager.
			// Use context.Background() to ensure checkpoint persists even if run is cancelled.
			workflowCtx := make(map[string]any)
			workflowCtx["inputs"] = run.Inputs
			if err := r.lifecycle.SaveCheckpoint(context.Background(), run, stepIndex, workflowCtx); err != nil {
				r.addLog(run, "warn", fmt.Sprintf("Failed to save checkpoint: %v", err), "")
			}
		},
		OnStepEnd: func(stepID string, result *workflow.StepResult, err error) {
			// Notify debug adapter if active
			if debugAdapter != nil {
				if debugErr := debugAdapter.OnStepEnd(run.ctx, stepID, result, err); debugErr != nil {
					r.addLog(run, "warn", fmt.Sprintf("Debug adapter error: %v", debugErr), stepID)
				}
			}

			// Get step name and compute status for events
			stepName := stepID
			stepIndex := -1
			for i, step := range run.definition.Steps {
				if step.ID == stepID {
					stepIndex = i
					if step.Name != "" {
						stepName = step.Name
					}
					break
				}
			}

			// Determine step status
			status := "success"
			errMsg := ""
			if err != nil {
				status = "error"
				errMsg = err.Error()
			} else if result != nil {
				if result.Status == workflow.StepStatusSkipped {
					status = "skipped"
				} else if result.Status == workflow.StepStatusFailed {
					status = "failed"
					if result.Error != "" {
						errMsg = result.Error
					}
				}
			}

			// Calculate tokens, duration, and output
			var durationMs int64
			var costUSD float64
			var tokensIn, tokensOut int
			var output map[string]any
			if result != nil {
				durationMs = result.Duration.Milliseconds()
				costUSD = result.CostUSD
				output = result.Output
				if result.TokenUsage != nil {
					tokensIn = result.TokenUsage.InputTokens
					tokensOut = result.TokenUsage.OutputTokens
					fmt.Printf("DEBUG OnStepEnd: result.TokenUsage set, in=%d out=%d\n", tokensIn, tokensOut)
				} else {
					fmt.Printf("DEBUG OnStepEnd: result.TokenUsage is nil\n")
				}
			} else {
				fmt.Printf("DEBUG OnStepEnd: result is nil\n")
			}

			// Send step_complete event for CLI progress display
			fmt.Printf("DEBUG OnStepEnd: calling addStepComplete with tokensIn=%d, tokensOut=%d\n", tokensIn, tokensOut)
			r.addStepComplete(run, stepID, stepName, status, output, durationMs, costUSD, tokensIn, tokensOut, errMsg)

			// Save step result to backend if available
			if result != nil {
				if be := r.getBackend(); be != nil {
					// Check if backend supports step result storage
					if stepStore, ok := be.(backend.StepResultStore); ok {
						backendResult := &backend.StepResult{
							RunID:     run.ID,
							StepID:    stepID,
							StepIndex: stepIndex,
							Inputs:    nil, // Step inputs not available at this level
							Outputs:   result.Output,
							Duration:  result.Duration,
							Status:    string(result.Status),
							Error:     errorToString(err),
							CostUSD:   result.CostUSD,
							CreatedAt: time.Now(),
						}
						// Use context.Background() to ensure step result persists
						if saveErr := stepStore.SaveStepResult(context.Background(), backendResult); saveErr != nil {
							r.addLog(run, "warn", fmt.Sprintf("Failed to save step result: %v", saveErr), stepID)
						}
					}
				}
			}

			// Record step metrics
			r.mu.RLock()
			metricsCollector := r.metrics
			r.mu.RUnlock()

			if metricsCollector != nil && result != nil {
				metricsCollector.RecordStepComplete(run.ctx, run.WorkflowID, stepID, status, result.Duration)
			}
		},
		OnLog: func(level, message, stepID string) {
			r.addLog(run, level, message, stepID)
		},
		// Apply runtime overrides from run
		Provider:   run.Provider,
		Model:      run.Model,
		Timeout:    run.Timeout,
		Security:   run.Security,
		AllowHosts: run.AllowHosts,
		AllowPaths: run.AllowPaths,
		MCPDev:     run.MCPDev,
	}

	result, err := adapter.ExecuteWorkflow(run.ctx, run.definition, run.Inputs, opts)

	// Close debug resources if active
	if debugAdapter != nil {
		debugAdapter.Close()
	}
	if debugShell != nil {
		debugShell.Close()
	}

	// Update final status
	run.mu.Lock()
	completedAt := time.Now()
	run.CompletedAt = &completedAt
	run.Progress.Completed = len(run.definition.Steps)

	var duration time.Duration
	if run.StartedAt != nil {
		duration = completedAt.Sub(*run.StartedAt)
	}

	if err != nil {
		// Check if the error is due to cancellation or timeout
		if err == context.Canceled {
			run.Status = RunStatusCancelled
			run.Error = "cancelled by user"
		} else if err == context.DeadlineExceeded {
			run.Status = RunStatusFailed
			currentStep := run.Progress.CurrentStep
			if currentStep != "" {
				run.Error = fmt.Sprintf("step '%s' timed out", currentStep)
			} else {
				run.Error = "step timed out"
			}
		} else {
			run.Status = RunStatusFailed
			run.Error = err.Error()
		}
	} else {
		run.Status = RunStatusCompleted
		if result != nil {
			// Try to resolve workflow-defined outputs first
			if workflowOutputs := resolveWorkflowOutputs(run.definition, result.StepOutputs); workflowOutputs != nil {
				run.Output = workflowOutputs
			} else if result.StepOutput != nil {
				// Fall back to last step output if no workflow outputs defined
				run.Output = stepOutputToMap(*result.StepOutput)
			}
		}
	}

	status := string(run.Status)
	run.mu.Unlock()

	// Record run completion for metrics
	r.mu.RLock()
	metrics := r.metrics
	r.mu.RUnlock()
	if metrics != nil {
		metrics.RecordRunComplete(run.ctx, run.ID, run.WorkflowID, status, "api", duration)
	}

	// Update backend with final status.
	// Use context.Background() to ensure final state persists even if run was cancelled.
	if be := r.getBackend(); be != nil {
		beRun := r.toBackendRun(run)
		_ = be.UpdateRun(context.Background(), beRun)
	}

	// Clean up checkpoint on successful completion.
	// Use context.Background() to ensure cleanup completes.
	if run.Status == RunStatusCompleted {
		_ = r.lifecycle.CleanupCheckpoint(context.Background(), run.ID)
	}

	// Send status event for CLI to display final state
	r.addStatus(run, status, run.Error)
}

// errorToString converts an error to a string, returning empty string if nil.
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// resolveWorkflowOutputs resolves workflow output definitions using step outputs.
// If the workflow has outputs defined, resolve each output template and return a map.
// If no outputs are defined, returns nil to indicate fallback to step output.
func resolveWorkflowOutputs(def *workflow.Definition, stepOutputs map[string]any) map[string]any {
	if len(def.Outputs) == 0 {
		return nil
	}

	// Build template context with step outputs
	ctx := workflow.NewTemplateContext()
	for stepID, output := range stepOutputs {
		// SetStepOutput expects map[string]interface{}
		if outputMap, ok := output.(map[string]interface{}); ok {
			ctx.SetStepOutput(stepID, outputMap)
		} else if outputMap, ok := output.(map[string]any); ok {
			// Convert map[string]any to map[string]interface{}
			converted := make(map[string]interface{}, len(outputMap))
			for k, v := range outputMap {
				converted[k] = v
			}
			ctx.SetStepOutput(stepID, converted)
		}
	}

	outputs := make(map[string]any)
	for _, outputDef := range def.Outputs {
		// Resolve the output value template
		value, err := workflow.ResolveTemplate(outputDef.Value, ctx)
		if err != nil {
			// On error, use the raw template as value
			outputs[outputDef.Name] = outputDef.Value
			continue
		}
		outputs[outputDef.Name] = value
	}

	return outputs
}
