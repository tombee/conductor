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
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

// ExecutionAdapter bridges the daemon Runner with workflow execution.
// It provides a clean interface for executing workflows, allowing the Runner
// to focus on orchestration while delegating step execution to the workflow package.
type ExecutionAdapter interface {
	// ExecuteWorkflow runs a complete workflow and returns the aggregated results.
	// The adapter handles step sequencing, context management, and result aggregation.
	// Progress callbacks are invoked for each step to allow real-time updates.
	ExecuteWorkflow(ctx context.Context, def *workflow.Definition, inputs map[string]any, opts ExecutionOptions) (*ExecutionResult, error)
}

// ExecutionOptions configures workflow execution behavior and progress callbacks.
type ExecutionOptions struct {
	// RunID is the unique identifier for this run (for logging and tracking)
	RunID string

	// OnStepStart is called when a step begins execution.
	// stepID is the step identifier, stepIndex is 0-based, total is the total step count.
	OnStepStart func(stepID string, stepIndex int, total int)

	// OnStepEnd is called when a step completes (successfully or with error).
	// result contains the step output, err is non-nil if the step failed.
	OnStepEnd func(stepID string, result *workflow.StepResult, err error)

	// OnLog is called for log messages during execution.
	// level is one of "debug", "info", "warn", "error".
	OnLog func(level, message, stepID string)
}

// ExecutionResult contains the aggregated results of a workflow execution.
type ExecutionResult struct {
	// StepOutput contains the typed final workflow output
	StepOutput *workflow.StepOutput

	// Duration is the total wall-clock time for the workflow execution
	Duration time.Duration

	// Steps contains the result of each step execution in order
	Steps []workflow.StepResult

	// StepOutputs maps step IDs to their outputs (for output template resolution)
	StepOutputs map[string]any

	// FinalError is the error that caused execution to stop (if any)
	FinalError error
}

// ExecutorAdapter implements ExecutionAdapter using the workflow.Executor.
// It bridges the daemon Runner with the workflow execution layer.
type ExecutorAdapter struct {
	// executor is the underlying step executor from pkg/workflow
	executor *workflow.Executor
}

// NewExecutorAdapter creates a new adapter wrapping the given Executor.
func NewExecutorAdapter(executor *workflow.Executor) *ExecutorAdapter {
	return &ExecutorAdapter{
		executor: executor,
	}
}

// ExecuteWorkflow implements ExecutionAdapter by executing each step in sequence.
func (a *ExecutorAdapter) ExecuteWorkflow(ctx context.Context, def *workflow.Definition, inputs map[string]any, opts ExecutionOptions) (*ExecutionResult, error) {
	startTime := time.Now()

	// Build workflow context (same pattern as CLI in internal/cli/run.go)
	templateCtx := workflow.NewTemplateContext()
	for k, v := range inputs {
		templateCtx.SetInput(k, v)
	}

	workflowContext := map[string]interface{}{
		"inputs":           inputs,
		"steps":            make(map[string]interface{}),
		"_templateContext": templateCtx,
	}

	result := &ExecutionResult{
		Steps:       make([]workflow.StepResult, 0, len(def.Steps)),
		StepOutputs: make(map[string]any),
	}

	var lastStepOutput workflow.StepOutput
	totalSteps := len(def.Steps)

	// Execute each step in sequence
	for i, step := range def.Steps {
		// Check for cancellation
		select {
		case <-ctx.Done():
			result.FinalError = ctx.Err()
			result.Duration = time.Since(startTime)
			return result, ctx.Err()
		default:
		}

		// Notify step start
		if opts.OnStepStart != nil {
			opts.OnStepStart(step.ID, i, totalSteps)
		}

		if opts.OnLog != nil {
			opts.OnLog("info", fmt.Sprintf("Executing step: %s (%s)", step.Name, step.Type), step.ID)
		}

		// Execute the step
		stepResult, err := a.executor.Execute(ctx, &step, workflowContext)

		// Notify step end
		if opts.OnStepEnd != nil {
			opts.OnStepEnd(step.ID, stepResult, err)
		}

		// Store step result
		if stepResult != nil {
			result.Steps = append(result.Steps, *stepResult)
		}

		if err != nil {
			if opts.OnLog != nil {
				opts.OnLog("error", fmt.Sprintf("Step failed: %v", err), step.ID)
			}

			// Check error handling strategy
			if step.OnError != nil && step.OnError.Strategy == workflow.ErrorStrategyIgnore {
				if opts.OnLog != nil {
					opts.OnLog("info", "Error ignored per step configuration", step.ID)
				}
				continue
			}

			// Stop execution on error
			result.FinalError = err
			result.Duration = time.Since(startTime)
			return result, err
		}

		// Update workflow context with step results
		if stepResult != nil && stepResult.Output != nil {
			workflowContext["steps"].(map[string]interface{})[step.ID] = stepResult.Output
			templateCtx.SetStepOutput(step.ID, stepResult.Output)
			result.StepOutputs[step.ID] = stepResult.Output

			// Convert to typed StepOutput
			lastStepOutput = stepResultToOutput(stepResult)
		}

		if opts.OnLog != nil {
			if stepResult != nil && stepResult.Status == workflow.StepStatusSkipped {
				opts.OnLog("info", fmt.Sprintf("Step skipped: %s", step.ID), step.ID)
			} else {
				opts.OnLog("info", fmt.Sprintf("Step completed: %s", step.ID), step.ID)
			}
		}
	}

	// Set final output
	result.StepOutput = &lastStepOutput
	result.Duration = time.Since(startTime)

	return result, nil
}

// stepResultToOutput converts a workflow.StepResult to a typed workflow.StepOutput.
// This helper bridges the old map-based result format with the new typed format.
func stepResultToOutput(result *workflow.StepResult) workflow.StepOutput {
	if result == nil {
		return workflow.StepOutput{}
	}

	output := workflow.StepOutput{
		Error: result.Error,
		Metadata: workflow.OutputMetadata{
			Duration: result.Duration,
		},
	}

	// Extract text from output map if present
	// LLM steps return {"response": "..."} while other steps may use "text"
	if result.Output != nil {
		if text, ok := result.Output["text"].(string); ok {
			output.Text = text
		} else if response, ok := result.Output["response"].(string); ok {
			output.Text = response
		}
		// Store the entire output as Data for now
		output.Data = result.Output
	}

	return output
}


// MockExecutionAdapter is a test double for ExecutionAdapter.
type MockExecutionAdapter struct {
	// ExecuteWorkflowFunc is called when ExecuteWorkflow is invoked.
	// Set this to control the mock's behavior in tests.
	ExecuteWorkflowFunc func(ctx context.Context, def *workflow.Definition, inputs map[string]any, opts ExecutionOptions) (*ExecutionResult, error)

	// Calls records all calls to ExecuteWorkflow for verification.
	Calls []ExecuteWorkflowCall
}

// ExecuteWorkflowCall records a call to ExecuteWorkflow.
type ExecuteWorkflowCall struct {
	Def    *workflow.Definition
	Inputs map[string]any
	Opts   ExecutionOptions
}

// ExecuteWorkflow implements ExecutionAdapter for testing.
func (m *MockExecutionAdapter) ExecuteWorkflow(ctx context.Context, def *workflow.Definition, inputs map[string]any, opts ExecutionOptions) (*ExecutionResult, error) {
	m.Calls = append(m.Calls, ExecuteWorkflowCall{
		Def:    def,
		Inputs: inputs,
		Opts:   opts,
	})

	if m.ExecuteWorkflowFunc != nil {
		return m.ExecuteWorkflowFunc(ctx, def, inputs, opts)
	}

	// Default: return empty success result
	return &ExecutionResult{
		Duration:    time.Millisecond,
		Steps:       []workflow.StepResult{},
		StepOutputs: make(map[string]any),
	}, nil
}
