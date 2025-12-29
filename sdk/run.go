package sdk

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	pkgAgent "github.com/tombee/conductor/pkg/agent"
	pkgWorkflow "github.com/tombee/conductor/pkg/workflow"
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

	// Execute the workflow with cost limit enforcement
	err := s.executeWorkflow(ctx, wf, inputs, cfg, result, costLimit)

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

	// Import agent package dynamically
	agentPkg, err := s.createAgent()
	if err != nil {
		return nil, err
	}

	// Run the agent loop
	agentResult, err := agentPkg.Run(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Convert pkg/agent result to SDK AgentResult
	result := &AgentResult{
		Success:       agentResult.Success,
		FinalResponse: agentResult.FinalResponse,
		ToolCalls:     make([]ToolExecution, len(agentResult.ToolExecutions)),
		Iterations:    agentResult.Iterations,
		Duration:      agentResult.Duration,
	}

	// Convert tool executions
	for i, exec := range agentResult.ToolExecutions {
		result.ToolCalls[i] = ToolExecution{
			ToolName: exec.ToolName,
			Inputs:   exec.Inputs,
			Outputs:  exec.Outputs,
			Success:  exec.Success,
			Error:    exec.Error,
			Duration: exec.Duration,
		}
	}

	// Convert tokens
	result.Tokens = TokenUsage{
		PromptTokens:     agentResult.TokensUsed.InputTokens,
		CompletionTokens: agentResult.TokensUsed.OutputTokens,
		TotalTokens:      agentResult.TokensUsed.TotalTokens,
	}

	if agentResult.Error != "" {
		result.Error = fmt.Errorf("%s", agentResult.Error)
	}

	return result, nil
}

// executeWorkflow executes the workflow steps with cost tracking and enforcement
func (s *SDK) executeWorkflow(ctx context.Context, wf *Workflow, inputs map[string]any, cfg *runConfig, result *Result, costLimit float64) error {
	// Build workflow context with inputs and step outputs
	workflowContext := make(map[string]any)
	workflowContext["inputs"] = inputs
	workflowContext["steps"] = make(map[string]any)

	// Execute steps in order (respecting dependencies)
	// For now, we'll do a simple sequential execution
	for _, stepDef := range wf.steps {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check cost limit before executing next step
		if costLimit > 0 && result.Cost.EstimatedCost > costLimit {
			return &CostLimitExceededError{
				Limit:  costLimit,
				Actual: result.Cost.EstimatedCost,
			}
		}

		// Convert SDK stepDef to pkg/workflow StepDefinition
		pkgStep, err := s.convertStepDef(stepDef)
		if err != nil {
			return fmt.Errorf("convert step %s: %w", stepDef.id, err)
		}

		// Create executor for this step
		executor, err := s.createExecutor()
		if err != nil {
			return fmt.Errorf("create executor: %w", err)
		}

		// Emit step started event
		s.emitEvent(ctx, &Event{
			Type:       EventStepStarted,
			Timestamp:  time.Now(),
			WorkflowID: wf.name,
			StepID:     stepDef.id,
		})

		// Execute the step
		stepResult, err := executor.Execute(ctx, pkgStep, workflowContext)
		if err != nil {
			// Emit step failed event
			s.emitEvent(ctx, &Event{
				Type:       EventStepFailed,
				Timestamp:  time.Now(),
				WorkflowID: wf.name,
				StepID:     stepDef.id,
				Data:       err,
			})

			return &StepExecutionError{
				StepID: stepDef.id,
				Cause:  err,
			}
		}

		// Convert pkg/workflow StepResult to SDK StepResult
		sdkStepResult := &StepResult{
			StepID:   stepResult.StepID,
			Status:   StepStatus(stepResult.Status),
			Output:   stepResult.Output,
			Duration: stepResult.Duration,
			Error:    stepResult.Error,
		}

		// Store result
		result.Steps[stepDef.id] = sdkStepResult

		// Update workflow context with step output
		stepsMap := workflowContext["steps"].(map[string]any)
		stepsMap[stepDef.id] = map[string]any{
			"output": stepResult.Output,
		}

		// Track costs from step execution (when available)
		// Note: Token tracking will be added to pkg/workflow.StepResult in a future update
		// For now, emit cost update event with zero cost for non-LLM steps
		stepCost := 0.0
		if stepDef.stepType == "llm" || stepDef.stepType == "agent" {
			// Cost tracking will be fully implemented when pkg/workflow.StepResult
			// includes token usage information
			stepCost = 0.0 // Placeholder for now
		}

		result.Cost.ByStep[stepDef.id] = stepCost
		result.Cost.EstimatedCost += stepCost

		// Emit cost update event
		s.emitEvent(ctx, &Event{
			Type:       EventCostUpdate,
			Timestamp:  time.Now(),
			WorkflowID: wf.name,
			StepID:     stepDef.id,
			Data: map[string]any{
				"step_cost":      stepCost,
				"total_cost":     result.Cost.EstimatedCost,
				"cost_remaining": costLimit - result.Cost.EstimatedCost,
			},
		})

		// Emit step completed event
		s.emitEvent(ctx, &Event{
			Type:       EventStepCompleted,
			Timestamp:  time.Now(),
			WorkflowID: wf.name,
			StepID:     stepDef.id,
			Data: &StepCompletedEvent{
				Output:   stepResult.Output,
				Duration: stepResult.Duration,
				Tokens:   sdkStepResult.Tokens,
			},
		})
	}

	// Set final output (for now, use last step's output)
	if len(wf.steps) > 0 {
		lastStepID := wf.steps[len(wf.steps)-1].id
		if lastResult, ok := result.Steps[lastStepID]; ok {
			result.Output = lastResult.Output
		}
	}

	return nil
}

// convertStepDef converts SDK stepDef to pkg/workflow StepDefinition
func (s *SDK) convertStepDef(stepDef *stepDef) (*pkgWorkflow.StepDefinition, error) {
	pkgStep := &pkgWorkflow.StepDefinition{
		ID: stepDef.id,
		// Note: Dependencies are handled at the workflow level, not in StepDefinition
	}

	switch stepDef.stepType {
	case "llm":
		pkgStep.Type = pkgWorkflow.StepTypeLLM
		pkgStep.Model = stepDef.model
		pkgStep.System = stepDef.system
		pkgStep.Prompt = stepDef.prompt
		pkgStep.OutputSchema = stepDef.outputSchema
		pkgStep.Tools = stepDef.tools
		if stepDef.temperature != nil {
			// TODO: Add temperature to StepDefinition
		}
		if stepDef.maxTokens != nil {
			pkgStep.MaxTokens = stepDef.maxTokens
		}

	case "action":
		pkgStep.Type = pkgWorkflow.StepTypeIntegration
		pkgStep.Action = stepDef.actionName
		pkgStep.Inputs = stepDef.actionInputs

	case "agent":
		// Agent steps are implemented as LLM steps with tool use
		pkgStep.Type = pkgWorkflow.StepTypeLLM
		pkgStep.Prompt = stepDef.agentPrompt
		// TODO: Configure agent-specific settings

	case "parallel":
		return nil, fmt.Errorf("parallel steps are not yet implemented - this is a planned feature for a future release")

	case "condition":
		return nil, fmt.Errorf("conditional steps are not yet implemented - this is a planned feature for a future release")

	default:
		return nil, &ValidationError{
			Field:   "type",
			Message: fmt.Sprintf("unknown step type: %s", stepDef.stepType),
		}
	}

	return pkgStep, nil
}

// createExecutor creates a pkg/workflow Executor with SDK registries
func (s *SDK) createExecutor() (*pkgWorkflow.Executor, error) {
	// Create LLM provider adapter
	llmAdapter := &sdkLLMProviderAdapter{
		sdk: s,
	}

	// Create tool registry adapter
	toolAdapter := &sdkToolRegistryAdapter{
		sdk: s,
	}

	// Create executor
	executor := pkgWorkflow.NewExecutor(toolAdapter, llmAdapter)
	executor.WithLogger(s.logger)

	return executor, nil
}

// createAgent creates a pkg/agent Agent with SDK registries
func (s *SDK) createAgent() (*pkgAgent.Agent, error) {
	// Create agent LLM provider adapter
	agentLLMAdapter := &sdkAgentLLMProviderAdapter{
		sdk: s,
	}

	// Create agent
	agent := pkgAgent.NewAgent(agentLLMAdapter, s.toolRegistry)

	return agent, nil
}
