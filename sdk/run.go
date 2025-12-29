package sdk

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Run executes a workflow with the given inputs and options.
// Returns a Result containing execution state, outputs, costs, and any errors.
//
// The workflow must have been created via NewWorkflow().Build() or loaded via LoadWorkflow().
//
// Example:
//
//	result, err := s.Run(ctx, wf, map[string]any{
//		"code": "func main() {}",
//		"language": "Go",
//	})
//	if err != nil {
//		return err
//	}
//	fmt.Printf("Result: %v\nCost: $%.4f\n", result.Output, result.Cost.EstimatedCost)
func (s *SDK) Run(ctx context.Context, wf *Workflow, inputs map[string]any, opts ...RunOption) (*Result, error) {
	if s.closed {
		return nil, fmt.Errorf("SDK is closed")
	}

	if wf == nil {
		return nil, &ValidationError{
			Field:   "workflow",
			Message: "workflow cannot be nil",
		}
	}

	// Apply run options
	cfg := &runConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Determine cost limit for this run
	costLimit := s.costLimit
	if cfg.hasCostLimit {
		costLimit = cfg.costLimit
	}

	// Validate inputs
	if err := s.ValidateInputs(ctx, wf, inputs); err != nil {
		return nil, err
	}

	// Generate run ID
	runID := uuid.New().String()
	workflowID := wf.name

	// Create run record
	run := &Run{
		ID:         runID,
		WorkflowID: workflowID,
		Status:     RunStatusRunning,
		Steps:      make(map[string]*StepResult),
		StartedAt:  time.Now(),
	}

	// Save initial run state
	if err := s.store.SaveRun(ctx, run); err != nil {
		s.logger.Warn("failed to save run state", "error", err)
	}

	// Emit workflow started event
	s.emitEvent(ctx, &Event{
		Type:       EventWorkflowStarted,
		Timestamp:  time.Now(),
		WorkflowID: workflowID,
	})

	// Create result structure
	result := &Result{
		WorkflowID: workflowID,
		Success:    false,
		Steps:      make(map[string]*StepResult),
		Cost: CostSummary{
			ByStep: make(map[string]float64),
		},
	}

	startTime := time.Now()

	// TODO: Implement actual workflow execution
	// This will require:
	// 1. Converting SDK workflow definition to pkg/workflow format
	// 2. Creating workflow executor with our registries
	// 3. Executing steps with dependency resolution
	// 4. Tracking costs and emitting events
	// 5. Handling errors and cost limits
	//
	// For now, return a placeholder error
	err := fmt.Errorf("workflow execution not yet implemented (Phase 1 in progress)")

	result.Duration = time.Since(startTime)

	if err != nil {
		result.Error = err
		run.Status = RunStatusFailed
		now := time.Now()
		run.CompletedAt = &now

		s.emitEvent(ctx, &Event{
			Type:       EventWorkflowFailed,
			Timestamp:  time.Now(),
			WorkflowID: workflowID,
			Data:       err,
		})
	} else {
		result.Success = true
		run.Status = RunStatusCompleted
		now := time.Now()
		run.CompletedAt = &now

		s.emitEvent(ctx, &Event{
			Type:       EventWorkflowCompleted,
			Timestamp:  time.Now(),
			WorkflowID: workflowID,
		})
	}

	// Update run record
	run.Steps = result.Steps
	if err := s.store.SaveRun(ctx, run); err != nil {
		s.logger.Warn("failed to save final run state", "error", err)
	}

	// Check cost limit
	if costLimit > 0 && result.Cost.EstimatedCost > costLimit {
		return result, &CostLimitExceededError{
			Limit:  costLimit,
			Actual: result.Cost.EstimatedCost,
		}
	}

	return result, err
}

// RunAgent executes an agent loop with the given prompts.
// This is a convenience method for simple agent execution without defining a full workflow.
//
// Example:
//
//	result, err := s.RunAgent(ctx,
//		"You are a helpful research assistant.",
//		"Research the history of Go programming language and summarize key points.",
//	)
func (s *SDK) RunAgent(ctx context.Context, systemPrompt, userPrompt string) (*AgentResult, error) {
	if s.closed {
		return nil, fmt.Errorf("SDK is closed")
	}

	// TODO: Implement using pkg/agent
	// This will require:
	// 1. Creating an agent with our tool registry and LLM provider
	// 2. Running the agent loop
	// 3. Tracking tool calls and costs
	// 4. Emitting events
	//
	// For now, return a placeholder error
	return nil, fmt.Errorf("RunAgent not yet implemented (Phase 1 in progress)")
}
