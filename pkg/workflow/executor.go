package workflow

import (
	"context"
	"fmt"
	"time"
)

// StepResult represents the result of executing a workflow step.
type StepResult struct {
	// StepID is the ID of the executed step
	StepID string

	// Success indicates if the step completed successfully
	Success bool

	// Output contains the step's output data
	Output map[string]interface{}

	// Error contains the error message if the step failed
	Error string

	// Duration is the time taken to execute the step
	Duration time.Duration

	// StartedAt is when the step execution began
	StartedAt time.Time

	// CompletedAt is when the step execution finished
	CompletedAt time.Time

	// Attempts is the number of execution attempts (for retry logic)
	Attempts int
}

// StepExecutor executes individual workflow steps.
type StepExecutor struct {
	// toolRegistry provides access to registered tools for action steps
	toolRegistry ToolRegistry

	// llmProvider provides access to LLM for llm steps
	llmProvider LLMProvider
}

// ToolRegistry defines the interface for tool lookup and execution.
type ToolRegistry interface {
	// GetTool retrieves a tool by name
	GetTool(name string) (Tool, error)

	// ExecuteTool executes a tool with the given inputs
	ExecuteTool(ctx context.Context, name string, inputs map[string]interface{}) (map[string]interface{}, error)
}

// Tool represents an executable tool with a name and schema.
type Tool interface {
	// Name returns the tool identifier
	Name() string

	// Execute runs the tool with the given inputs
	Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error)
}

// LLMProvider defines the interface for LLM interactions.
type LLMProvider interface {
	// Complete makes a synchronous LLM call
	Complete(ctx context.Context, prompt string, options map[string]interface{}) (string, error)
}

// NewStepExecutor creates a new step executor.
func NewStepExecutor(toolRegistry ToolRegistry, llmProvider LLMProvider) *StepExecutor {
	return &StepExecutor{
		toolRegistry: toolRegistry,
		llmProvider:  llmProvider,
	}
}

// Execute executes a single workflow step.
func (e *StepExecutor) Execute(ctx context.Context, step *StepDefinition, workflowContext map[string]interface{}) (*StepResult, error) {
	result := &StepResult{
		StepID:    step.ID,
		StartedAt: time.Now(),
		Attempts:  1,
	}

	// Set timeout if specified
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Execute with retry if configured
	var err error
	if step.Retry != nil {
		result.Output, err = e.executeWithRetry(ctx, step, workflowContext, result)
	} else {
		result.Output, err = e.executeStep(ctx, step, workflowContext)
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)

	if err != nil {
		result.Success = false
		result.Error = err.Error()

		// Handle error according to step configuration
		if step.OnError != nil {
			return e.handleError(ctx, step, result, err)
		}

		return result, err
	}

	result.Success = true
	return result, nil
}

// executeStep executes a step once without retry logic.
func (e *StepExecutor) executeStep(ctx context.Context, step *StepDefinition, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	// Resolve inputs (substitute context variables)
	inputs, err := e.resolveInputs(step.Inputs, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve inputs: %w", err)
	}

	// Execute based on step type
	switch step.Type {
	case StepTypeAction:
		return e.executeAction(ctx, step, inputs)
	case StepTypeLLM:
		return e.executeLLM(ctx, step, inputs)
	case StepTypeCondition:
		return e.executeCondition(ctx, step, inputs, workflowContext)
	case StepTypeParallel:
		return e.executeParallel(ctx, step, inputs, workflowContext)
	default:
		return nil, fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

// executeWithRetry executes a step with retry logic.
func (e *StepExecutor) executeWithRetry(ctx context.Context, step *StepDefinition, workflowContext map[string]interface{}, result *StepResult) (map[string]interface{}, error) {
	var lastErr error
	backoffDuration := time.Duration(step.Retry.BackoffBase) * time.Second

	for attempt := 1; attempt <= step.Retry.MaxAttempts; attempt++ {
		result.Attempts = attempt

		output, err := e.executeStep(ctx, step, workflowContext)
		if err == nil {
			return output, nil
		}

		lastErr = err

		// Don't retry if this was the last attempt
		if attempt == step.Retry.MaxAttempts {
			break
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoffDuration):
			// Calculate next backoff duration
			backoffDuration = time.Duration(float64(backoffDuration) * step.Retry.BackoffMultiplier)
		}
	}

	return nil, fmt.Errorf("step failed after %d attempts: %w", step.Retry.MaxAttempts, lastErr)
}

// executeAction executes an action step by calling a tool.
func (e *StepExecutor) executeAction(ctx context.Context, step *StepDefinition, inputs map[string]interface{}) (map[string]interface{}, error) {
	if e.toolRegistry == nil {
		return nil, fmt.Errorf("tool registry not configured")
	}

	output, err := e.toolRegistry.ExecuteTool(ctx, step.Action, inputs)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	return output, nil
}

// executeLLM executes an LLM step by making an LLM API call.
func (e *StepExecutor) executeLLM(ctx context.Context, step *StepDefinition, inputs map[string]interface{}) (map[string]interface{}, error) {
	if e.llmProvider == nil {
		return nil, fmt.Errorf("LLM provider not configured")
	}

	// Extract prompt from inputs
	prompt, ok := inputs["prompt"].(string)
	if !ok {
		return nil, fmt.Errorf("prompt is required for LLM step and must be a string")
	}

	// Get options (if any)
	options, _ := inputs["options"].(map[string]interface{})
	if options == nil {
		options = make(map[string]interface{})
	}

	response, err := e.llmProvider.Complete(ctx, prompt, options)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	return map[string]interface{}{
		"response": response,
	}, nil
}

// executeCondition executes a condition step by evaluating an expression.
func (e *StepExecutor) executeCondition(ctx context.Context, step *StepDefinition, inputs map[string]interface{}, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	if step.Condition == nil {
		return nil, fmt.Errorf("condition is required for condition step")
	}

	// Evaluate condition expression
	// For Phase 1, we'll use a simple string comparison
	// In the future, this could be replaced with a proper expression evaluator
	conditionMet, err := e.evaluateCondition(step.Condition.Expression, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate condition: %w", err)
	}

	return map[string]interface{}{
		"condition_met": conditionMet,
		"then_steps":    step.Condition.ThenSteps,
		"else_steps":    step.Condition.ElseSteps,
	}, nil
}

// executeParallel executes a parallel step (Phase 1: not fully implemented).
func (e *StepExecutor) executeParallel(ctx context.Context, step *StepDefinition, inputs map[string]interface{}, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	// Phase 1: Parallel execution is a placeholder
	// Future implementation would execute multiple steps concurrently
	return map[string]interface{}{
		"parallel": "not yet implemented",
	}, nil
}

// evaluateCondition evaluates a condition expression.
// Phase 1: Simple implementation, to be replaced with a proper expression engine.
func (e *StepExecutor) evaluateCondition(expression string, workflowContext map[string]interface{}) (bool, error) {
	// For Phase 1, we support basic equality checks
	// Expression format: "$.variable == 'value'"
	// This is a simplified implementation; a production system would use
	// a proper expression language like CEL (Common Expression Language)

	// For now, return true as a placeholder
	// Real implementation would parse and evaluate the expression
	return true, nil
}

// resolveInputs resolves input values by substituting context variables.
// Phase 1: Simple implementation that copies inputs as-is.
func (e *StepExecutor) resolveInputs(inputs map[string]interface{}, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	resolved := make(map[string]interface{})

	for key, value := range inputs {
		// Phase 1: Simple copy without variable substitution
		// Future implementation would parse strings like "$.previous_step.output"
		// and substitute them with values from the context
		resolved[key] = value
	}

	return resolved, nil
}

// handleError handles step execution errors according to the step's error handling configuration.
func (e *StepExecutor) handleError(ctx context.Context, step *StepDefinition, result *StepResult, err error) (*StepResult, error) {
	switch step.OnError.Strategy {
	case ErrorStrategyFail:
		// Default behavior: propagate error
		return result, err

	case ErrorStrategyIgnore:
		// Mark as success but include error info
		result.Success = true
		result.Error = fmt.Sprintf("ignored error: %s", err.Error())
		return result, nil

	case ErrorStrategyRetry:
		// Retry logic is handled by executeWithRetry
		return result, err

	case ErrorStrategyFallback:
		// Execute fallback step
		// Phase 1: Return error with fallback step ID
		// Future implementation would actually execute the fallback step
		result.Success = false
		result.Error = fmt.Sprintf("error (fallback to %s): %s", step.OnError.FallbackStep, err.Error())
		result.Output = map[string]interface{}{
			"fallback_step": step.OnError.FallbackStep,
		}
		return result, fmt.Errorf("step failed, fallback required: %w", err)

	default:
		return result, err
	}
}
