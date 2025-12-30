package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/tombee/conductor/pkg/errors"
	"github.com/tombee/conductor/pkg/security"
	"github.com/tombee/conductor/pkg/workflow/expression"
	"github.com/tombee/conductor/pkg/workflow/schema"
)

// StepStatus represents the execution status of a workflow step.
type StepStatus string

const (
	// StepStatusPending indicates the step has not started yet.
	StepStatusPending StepStatus = "pending"
	// StepStatusRunning indicates the step is currently executing.
	StepStatusRunning StepStatus = "running"
	// StepStatusSuccess indicates the step completed successfully.
	StepStatusSuccess StepStatus = "success"
	// StepStatusFailed indicates the step failed.
	StepStatusFailed StepStatus = "failed"
	// StepStatusSkipped indicates the step was skipped due to a condition.
	StepStatusSkipped StepStatus = "skipped"
)

// StepResult represents the result of executing a workflow step.
type StepResult struct {
	// StepID is the ID of the executed step
	StepID string

	// Status is the execution status (pending, running, success, failed, skipped)
	Status StepStatus

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

// DefaultParallelConcurrency is the default maximum number of concurrent parallel steps.
// Set conservatively to avoid overwhelming resources when launching multiple agents.
// Can be overridden via WithParallelConcurrency() or per-step MaxConcurrency field.
const DefaultParallelConcurrency = 3

// Executor executes individual workflow steps.
type Executor struct {
	// toolRegistry provides access to registered tools for action steps
	toolRegistry ToolRegistry

	// llmProvider provides access to LLM for llm steps
	llmProvider LLMProvider

	// operationRegistry provides access to operations for action and integration steps
	operationRegistry OperationRegistry

	// exprEval evaluates condition expressions
	exprEval *expression.Evaluator

	// logger for debug/info logging
	logger *slog.Logger

	// parallelSem limits concurrent parallel step execution
	parallelSem chan struct{}

	// securityProfile defines security restrictions for workflow execution
	securityProfile *security.SecurityProfile
}

// ToolRegistry defines the interface for tool lookup and execution.
type ToolRegistry interface {
	// Get retrieves a tool by name
	Get(name string) (Tool, error)

	// Execute executes a tool with the given inputs
	Execute(ctx context.Context, name string, inputs map[string]interface{}) (map[string]interface{}, error)

	// ListTools returns all registered tools
	ListTools() []Tool
}

// Tool represents an executable tool with a name and schema.
type Tool interface {
	// Name returns the tool identifier
	Name() string

	// Description returns what the tool does
	Description() string

	// Execute runs the tool with the given inputs and returns the output.
	// For now, returns map[string]interface{} for backward compatibility.
	// Tools should include structured metadata (duration, etc.) when possible.
	Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error)
}

// LLMProvider defines the interface for LLM interactions.
type LLMProvider interface {
	// Complete makes a synchronous LLM call
	// tools parameter contains available tools for function calling (optional)
	Complete(ctx context.Context, prompt string, options map[string]interface{}) (string, error)
}

// OperationRegistry defines the interface for operation lookup and execution.
type OperationRegistry interface {
	// Execute runs an operation (action or integration).
	// The reference should be in format "name.operation".
	//
	// Contract: Implementations MUST follow Go conventions:
	//   - On success: return (result, nil) where result is non-nil
	//   - On error: return (nil, error) where error is non-nil
	//   - Returning (nil, nil) is a contract violation and will be treated as an error
	Execute(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error)
}

// OperationResult represents the output of an operation.
type OperationResult interface {
	// GetResponse returns the transformed response data
	GetResponse() interface{}

	// GetRawResponse returns the original response before transformation
	GetRawResponse() interface{}

	// GetStatusCode returns the HTTP status code (for HTTP integrations)
	GetStatusCode() int

	// GetMetadata returns execution metadata
	GetMetadata() map[string]interface{}
}

// NewExecutor creates a new step executor.
func NewExecutor(toolRegistry ToolRegistry, llmProvider LLMProvider) *Executor {
	return &Executor{
		toolRegistry: toolRegistry,
		llmProvider:  llmProvider,
		exprEval:     expression.New(),
		logger:       slog.Default(),
		parallelSem:  make(chan struct{}, DefaultParallelConcurrency),
	}
}

// WithLogger sets a custom logger for the executor.
func (e *Executor) WithLogger(logger *slog.Logger) *Executor {
	e.logger = logger
	return e
}

// WithOperationRegistry sets the operation registry for the executor.
func (e *Executor) WithOperationRegistry(registry OperationRegistry) *Executor {
	e.operationRegistry = registry
	return e
}

// WithParallelConcurrency sets the maximum number of concurrent parallel steps.
func (e *Executor) WithParallelConcurrency(max int) *Executor {
	if max <= 0 {
		max = DefaultParallelConcurrency
	}
	e.parallelSem = make(chan struct{}, max)
	return e
}

// ActionRegistryFactory is a function that creates an OperationRegistry from a workflow directory.
// This allows the executor to be independent of the internal/operation package.
type ActionRegistryFactory func(workflowDir string) (OperationRegistry, error)

var (
	// defaultActionRegistryFactory is set by the operation package during init.
	defaultActionRegistryFactory ActionRegistryFactory
	// factoryOnce ensures the factory is set exactly once for thread-safe initialization.
	factoryOnce sync.Once
)

// SetDefaultActionRegistryFactory sets the factory used by WithWorkflowDir.
// This is called by the operation package during initialization.
// The factory can only be set once; subsequent calls are ignored.
func SetDefaultActionRegistryFactory(factory ActionRegistryFactory) {
	factoryOnce.Do(func() {
		defaultActionRegistryFactory = factory
	})
}

// WithWorkflowDir sets the workflow directory and initializes the builtin action registry.
// Actions like file.read and shell.run will resolve paths relative to this directory.
func (e *Executor) WithWorkflowDir(workflowDir string) *Executor {
	if defaultActionRegistryFactory == nil {
		e.logger.Warn("action registry factory not configured, actions will not be available")
		return e
	}

	registry, err := defaultActionRegistryFactory(workflowDir)
	if err != nil {
		e.logger.Error("failed to create action registry", "error", err)
		return e
	}

	e.operationRegistry = registry
	return e
}

// WithSecurity sets the security profile for workflow execution.
// Security checks will be applied to action executions when a profile is set.
func (e *Executor) WithSecurity(profile *security.SecurityProfile) *Executor {
	e.securityProfile = profile
	return e
}

// Execute executes a single workflow step.
func (e *Executor) Execute(ctx context.Context, step *StepDefinition, workflowContext map[string]interface{}) (*StepResult, error) {
	result := &StepResult{
		StepID:    step.ID,
		Status:    StepStatusRunning,
		StartedAt: time.Now(),
		Attempts:  1,
	}

	// Check condition before executing (skip if condition is false)
	if step.Condition != nil && step.Condition.Expression != "" {
		shouldRun, err := e.evaluateCondition(step.Condition.Expression, workflowContext)
		if err != nil {
			result.Status = StepStatusFailed
			result.Error = fmt.Sprintf("condition evaluation failed: %s", err.Error())
			result.CompletedAt = time.Now()
			result.Duration = result.CompletedAt.Sub(result.StartedAt)
			return result, fmt.Errorf("evaluate condition for step %s: %w", step.ID, err)
		}

		if !shouldRun {
			e.logger.Debug("step skipped due to condition",
				"step_id", step.ID,
				"expression", step.Condition.Expression,
			)
			result.Status = StepStatusSkipped
			result.Output = map[string]interface{}{
				"content": "",
				"skipped": true,
				"reason":  "condition evaluated to false",
			}
			result.CompletedAt = time.Now()
			result.Duration = result.CompletedAt.Sub(result.StartedAt)
			return result, nil
		}
	}

	// Apply default timeout (30s) when step.Timeout is 0
	timeout := step.Timeout
	if timeout == 0 {
		timeout = 30 // Default 30 seconds
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	// Apply default retry config when step.Retry is nil
	retry := step.Retry
	if retry == nil {
		retry = &RetryDefinition{
			MaxAttempts:       2,
			BackoffBase:       1,
			BackoffMultiplier: 2.0,
		}
	}

	// Execute with retry
	var err error
	result.Output, err = e.executeWithRetry(ctx, step, workflowContext, result, retry)

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)

	if err != nil {
		result.Status = StepStatusFailed
		result.Error = err.Error()

		// Handle error according to step configuration
		if step.OnError != nil {
			return e.handleError(ctx, step, result, err)
		}

		return result, err
	}

	result.Status = StepStatusSuccess
	return result, nil
}

// executeStep executes a step once without retry logic.
func (e *Executor) executeStep(ctx context.Context, step *StepDefinition, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	// Resolve inputs (substitute context variables)
	inputs, err := e.resolveInputs(step.Inputs, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve inputs: %w", err)
	}

	// Resolve step fields (prompt, system) for template variables
	resolvedStep := *step
	if err := e.resolveStepFields(&resolvedStep, workflowContext); err != nil {
		return nil, fmt.Errorf("failed to resolve step fields: %w", err)
	}

	// Execute based on step type
	switch step.Type {
	case StepTypeLLM:
		return e.executeLLM(ctx, &resolvedStep, inputs)
	case StepTypeCondition:
		return e.executeCondition(ctx, &resolvedStep, inputs, workflowContext)
	case StepTypeParallel:
		return e.executeParallel(ctx, &resolvedStep, inputs, workflowContext)
	case StepTypeIntegration:
		return e.executeIntegration(ctx, &resolvedStep, inputs)
	case StepTypeLoop:
		return e.executeLoop(ctx, &resolvedStep, inputs, workflowContext)
	default:
		return nil, &errors.ValidationError{
			Field:      "type",
			Message:    fmt.Sprintf("unsupported step type: %s", step.Type),
			Suggestion: "use one of: llm, condition, parallel, integration, loop",
		}
	}
}

// executeWithRetry executes a step with retry logic.
func (e *Executor) executeWithRetry(ctx context.Context, step *StepDefinition, workflowContext map[string]interface{}, result *StepResult, retry *RetryDefinition) (map[string]interface{}, error) {
	// Don't retry parallel steps - they have their own internal error handling
	// Retrying a parallel step would re-execute all nested steps unnecessarily
	if step.Type == StepTypeParallel {
		return e.executeStep(ctx, step, workflowContext)
	}

	// Don't retry loop steps - they have their own internal iteration logic
	// Retrying a loop step would restart from iteration 0, losing history
	if step.Type == StepTypeLoop {
		return e.executeStep(ctx, step, workflowContext)
	}

	// If only one attempt, execute directly without retry logic
	if retry.MaxAttempts == 1 {
		return e.executeStep(ctx, step, workflowContext)
	}

	var lastErr error
	var lastOutput map[string]interface{}
	backoffDuration := time.Duration(retry.BackoffBase) * time.Second

	for attempt := 1; attempt <= retry.MaxAttempts; attempt++ {
		result.Attempts = attempt

		output, err := e.executeStep(ctx, step, workflowContext)
		if err == nil {
			return output, nil
		}

		lastErr = err
		lastOutput = output // Preserve partial output for error cases

		// Don't retry if this was the last attempt
		if attempt == retry.MaxAttempts {
			break
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return lastOutput, ctx.Err()
		case <-time.After(backoffDuration):
			// Calculate next backoff duration
			backoffDuration = time.Duration(float64(backoffDuration) * retry.BackoffMultiplier)
		}
	}

	return lastOutput, fmt.Errorf("step failed after %d attempts: %w", retry.MaxAttempts, lastErr)
}

// executeIntegration executes an integration step by invoking an operation.
// The operation reference can be either step.Integration (for integrations) or
// step.Action + step.Operation (for builtin actions).
// Inputs are passed through to operation registry without type assertions.
func (e *Executor) executeIntegration(ctx context.Context, step *StepDefinition, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Check if operation registry is configured
	if e.operationRegistry == nil {
		return nil, &errors.ConfigError{
			Key:    "operation_registry",
			Reason: "operation registry not configured for workflow executor",
		}
	}

	// Determine the operation reference
	var operationRef string
	if step.Integration != "" {
		operationRef = step.Integration
	} else if step.Action != "" && step.Operation != "" {
		operationRef = step.Action + "." + step.Operation
	} else {
		return nil, &errors.ValidationError{
			Field:      "integration/action",
			Message:    "integration step requires either 'integration' or 'action'+'operation' fields",
			Suggestion: "specify integration in format 'name.operation' or use action and operation fields",
		}
	}

	// Log operation execution start with structured logging
	e.logger.Debug("executing operation",
		"step_id", step.ID,
		"operation", operationRef,
		"inputs", maskSensitiveInputs(inputs),
	)

	// Execute the operation
	result, err := e.operationRegistry.Execute(ctx, operationRef, inputs)
	if err != nil {
		e.logger.Error("operation execution failed",
			"step_id", step.ID,
			"operation", operationRef,
			"error", err,
		)
		return nil, fmt.Errorf("operation execution failed: %w", err)
	}

	// Check for nil result (contract violation: Execute returned nil without error)
	if result == nil {
		e.logger.Error("operation returned nil result without error",
			"step_id", step.ID,
			"operation", operationRef,
		)
		return nil, fmt.Errorf("operation %q returned nil result without error", operationRef)
	}

	// Log successful execution with metadata
	e.logger.Debug("operation completed",
		"step_id", step.ID,
		"operation", operationRef,
		"status_code", result.GetStatusCode(),
		"metadata", result.GetMetadata(),
	)

	// Map operation result to step output format
	// The response is the primary output, but we also include metadata for debugging
	output := map[string]interface{}{
		"response": result.GetResponse(),
	}

	// Include metadata if present
	if metadata := result.GetMetadata(); len(metadata) > 0 {
		output["metadata"] = metadata
	}

	// Include status code for HTTP operations
	if statusCode := result.GetStatusCode(); statusCode > 0 {
		output["status_code"] = statusCode
	}

	return output, nil
}

// maskSensitiveInputs masks sensitive values in inputs for logging.
// This prevents credentials from appearing in logs.
func maskSensitiveInputs(inputs map[string]interface{}) map[string]interface{} {
	masked := make(map[string]interface{})
	sensitiveKeys := map[string]bool{
		"token":      true,
		"password":   true,
		"secret":     true,
		"api_key":    true,
		"apikey":     true,
		"auth":       true,
		"credential": true,
	}

	for key, value := range inputs {
		// Check if key contains sensitive terms (case-insensitive)
		lowerKey := ""
		for i := 0; i < len(key); i++ {
			ch := key[i]
			if ch >= 'A' && ch <= 'Z' {
				lowerKey += string(ch + 32) // Convert to lowercase
			} else {
				lowerKey += string(ch)
			}
		}

		isSensitive := false
		for sensitiveKey := range sensitiveKeys {
			// Check if the key contains the sensitive term
			if stringContains(lowerKey, sensitiveKey) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			masked[key] = "***MASKED***"
		} else {
			masked[key] = value
		}
	}

	return masked
}

// stringContains checks if a string contains a substring.
func stringContains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		found := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}

// executeLLM executes an LLM step by making an LLM API call.
// Supports structured output validation via OutputSchema.
func (e *Executor) executeLLM(ctx context.Context, step *StepDefinition, inputs map[string]interface{}) (map[string]interface{}, error) {
	if e.llmProvider == nil {
		return nil, &errors.ConfigError{
			Key:    "llm_provider",
			Reason: "LLM provider not configured for workflow executor",
		}
	}

	// Wrap inputs in type-safe context for safer access
	inputCtx := NewWorkflowContext(inputs)

	// Primary format: Use Prompt field from step definition
	// Alternative: prompt from inputs (for dynamic prompts)
	prompt := step.Prompt
	if prompt == "" {
		// Alternative format: prompt from inputs (type-safe accessor)
		promptInput, err := inputCtx.GetString("prompt")
		if err != nil {
			return nil, &errors.ValidationError{
				Field:      "prompt",
				Message:    "prompt is required for LLM step and must be a string",
				Suggestion: "add 'prompt' field to step definition or inputs",
			}
		}
		prompt = promptInput
	}

	// Get options (if any) - type-safe accessor with default
	options, err := inputCtx.GetMap("options")
	if err != nil {
		// If options is missing or not a map, use empty map
		options = make(map[string]interface{})
	}

	// Add system prompt if specified
	if step.System != "" {
		options["system"] = step.System
	}

	// Add model if specified
	if step.Model != "" {
		options["model"] = step.Model
	}

	// Filter and include tools based on step's Tools field
	if e.toolRegistry != nil && len(step.Tools) > 0 {
		filteredTools := e.filterTools(step.Tools)
		if len(filteredTools) > 0 {
			options["tools"] = filteredTools
		}
	}

	// Inject cost tracking context from workflow context
	if runID, ok := inputs["run_id"].(string); ok {
		options["run_id"] = runID
	}
	if workflowID, ok := inputs["workflow_id"].(string); ok {
		options["workflow_id"] = workflowID
	}
	// Add step name for cost tracking
	options["step_name"] = step.Name

	// Check if structured output is required
	if step.OutputSchema != nil {
		return e.executeLLMWithSchema(ctx, prompt, options, step.OutputSchema)
	}

	// Log prompt at trace level
	if e.logger != nil && e.logger.Enabled(nil, -8) {
		e.logger.Log(nil, -8, "LLM prompt",
			"prompt", prompt,
			"options", options,
		)
	}

	// Standard unstructured LLM call
	response, err := e.llmProvider.Complete(ctx, prompt, options)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Log response at trace level
	if e.logger != nil && e.logger.Enabled(nil, -8) {
		e.logger.Log(nil, -8, "LLM response",
			"response", response,
		)
	}

	return map[string]interface{}{
		"response": response,
	}, nil
}

// executeLLMWithSchema executes an LLM call with structured output validation and retry.
// Implements T4.1-T4.8: schema-aware execution with validation and retry logic.
func (e *Executor) executeLLMWithSchema(ctx context.Context, basePrompt string, options map[string]interface{}, outputSchema map[string]interface{}) (map[string]interface{}, error) {
	const maxAttempts = 3
	validator := schema.NewValidator()

	var lastResponse string
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// T4.2: Inject schema requirements into prompt
		prompt := schema.BuildPromptWithSchema(basePrompt, outputSchema, attempt)

		// Log prompt at trace level
		if e.logger != nil && e.logger.Enabled(nil, -8) {
			e.logger.Log(nil, -8, "LLM prompt with schema",
				"prompt", prompt,
				"options", options,
				"attempt", attempt+1,
				"schema", outputSchema,
			)
		}

		// Make LLM call
		response, err := e.llmProvider.Complete(ctx, prompt, options)
		if err != nil {
			return nil, fmt.Errorf("LLM call failed on attempt %d: %w", attempt+1, err)
		}

		lastResponse = response

		// Log response at trace level
		if e.logger != nil && e.logger.Enabled(nil, -8) {
			e.logger.Log(nil, -8, "LLM response with schema",
				"response", response,
				"attempt", attempt+1,
			)
		}

		// T3.2 & T4.3: Extract and parse JSON from response
		data, err := schema.ExtractJSON(response)
		if err != nil {
			lastErr = fmt.Errorf("failed to extract JSON: %w", err)
			continue // Retry with clearer instructions
		}

		// T4.3: Validate against schema
		if err := validator.Validate(outputSchema, data); err != nil {
			lastErr = fmt.Errorf("schema validation failed: %w", err)
			continue // Retry with clearer instructions
		}

		// T4.5: Strip extra fields (validator already does this implicitly)
		// T4.6: Store validated output under "output" key
		return map[string]interface{}{
			"output":   data,                  // Structured output accessible as {{.steps.id.output.field}}
			"response": response,              // Original response for debugging
			"attempts": attempt + 1,           // T4.8: Track retry attempts
		}, nil
	}

	// T4.7: All retries exhausted, return structured error
	return nil, &SchemaValidationError{
		ErrorCode:        "SCHEMA_VALIDATION_FAILED",
		ExpectedSchema:   outputSchema,
		ActualResponse:   truncateString(lastResponse, 500),
		ValidationErrors: []string{lastErr.Error()},
		Attempts:         maxAttempts,
	}
}

// SchemaValidationError represents a structured output validation failure.
// Implements T4.7: structured error for SCHEMA_VALIDATION_FAILED.
type SchemaValidationError struct {
	ErrorCode        string
	ExpectedSchema   map[string]interface{}
	ActualResponse   string
	ValidationErrors []string
	Attempts         int
}

// Error implements the error interface.
func (e *SchemaValidationError) Error() string {
	return fmt.Sprintf("%s: validation failed after %d attempts. Last error: %s. Response (truncated): %q",
		e.ErrorCode, e.Attempts, e.ValidationErrors[len(e.ValidationErrors)-1], e.ActualResponse)
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// filterTools returns tools filtered by the given tool names.
// Returns tool descriptors suitable for LLM function calling.
func (e *Executor) filterTools(toolNames []string) []map[string]interface{} {
	if e.toolRegistry == nil {
		return nil
	}

	allTools := e.toolRegistry.ListTools()
	filtered := []map[string]interface{}{}

	// Create a set of allowed tool names for quick lookup
	allowedNames := make(map[string]bool)
	for _, name := range toolNames {
		allowedNames[name] = true
	}

	// Filter tools and build descriptors
	for _, tool := range allTools {
		if allowedNames[tool.Name()] {
			filtered = append(filtered, map[string]interface{}{
				"name":        tool.Name(),
				"description": tool.Description(),
			})
		}
	}

	return filtered
}

// executeCondition executes a condition step by evaluating an expression.
// Inputs are passed through without type assertions.
func (e *Executor) executeCondition(ctx context.Context, step *StepDefinition, inputs map[string]interface{}, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	if step.Condition == nil {
		return nil, &errors.ValidationError{
			Field:      "condition",
			Message:    "condition is required for condition step",
			Suggestion: "add 'condition' field with expression and then/else steps",
		}
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

// executeParallel executes nested steps concurrently and aggregates results.
func (e *Executor) executeParallel(ctx context.Context, step *StepDefinition, inputs map[string]interface{}, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	if len(step.Steps) == 0 {
		return nil, &errors.ValidationError{
			Field:      "steps",
			Message:    "parallel step has no nested steps",
			Suggestion: "add at least one nested step to execute in parallel",
		}
	}

	// Handle foreach iteration if specified
	if step.Foreach != "" {
		return e.executeForeach(ctx, step, inputs, workflowContext)
	}

	// Apply parent timeout if specified
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Determine error strategy
	failFast := true
	if step.OnError != nil && step.OnError.Strategy == ErrorStrategyIgnore {
		failFast = false
	}

	// Use step-specific concurrency limit if set, otherwise use executor's default
	var sem chan struct{}
	if step.MaxConcurrency > 0 {
		sem = make(chan struct{}, step.MaxConcurrency)
		e.logger.Debug("using step-specific concurrency limit",
			"step_id", step.ID,
			"max_concurrency", step.MaxConcurrency,
		)
	} else {
		sem = e.parallelSem
	}

	type stepResult struct {
		id     string
		result *StepResult
		err    error
	}

	results := make(chan stepResult, len(step.Steps))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()

	e.logger.Debug("starting parallel execution",
		"step_id", step.ID,
		"nested_count", len(step.Steps),
		"max_concurrency", step.MaxConcurrency,
		"fail_fast", failFast,
	)

	// Launch goroutines for each nested step with concurrency limiting
	for _, nested := range step.Steps {
		go func(s StepDefinition) {
			stepStart := time.Now()

			// Acquire semaphore (blocks if at capacity)
			select {
			case sem <- struct{}{}:
				// Got slot
			case <-ctx.Done():
				e.logger.Debug("nested step cancelled while waiting for slot",
					"parent_step_id", step.ID,
					"step_id", s.ID,
				)
				results <- stepResult{
					id:  s.ID,
					err: ctx.Err(),
				}
				return
			}
			defer func() { <-sem }() // Release slot

			// Check if context is already cancelled
			select {
			case <-ctx.Done():
				e.logger.Debug("nested step cancelled before execution",
					"parent_step_id", step.ID,
					"step_id", s.ID,
				)
				results <- stepResult{
					id:  s.ID,
					err: ctx.Err(),
				}
				return
			default:
			}

			// Create a copy of workflow context for this step
			nestedContext := copyWorkflowContext(workflowContext)

			// Execute the step
			result, err := e.Execute(ctx, &s, nestedContext)

			// Log completion with nil-safe result access
			if result != nil {
				e.logger.Debug("nested step completed",
					"parent_step_id", step.ID,
					"step_id", s.ID,
					"status", result.Status,
					"duration", time.Since(stepStart),
					"error", err,
				)
			} else {
				e.logger.Debug("nested step failed without result",
					"parent_step_id", step.ID,
					"step_id", s.ID,
					"duration", time.Since(stepStart),
					"error", err,
				)
			}

			if err != nil && failFast {
				cancel() // Cancel other steps on first error
			}
			results <- stepResult{
				id:     s.ID,
				result: result,
				err:    err,
			}
		}(nested)
	}

	// Collect results
	output := make(map[string]interface{})
	var errors []error
	for i := 0; i < len(step.Steps); i++ {
		r := <-results
		if r.err != nil {
			errors = append(errors, fmt.Errorf("step %s: %w", r.id, r.err))
			// Include partial result even on error
			if r.result != nil {
				output[r.id] = r.result.Output
			}
		} else if r.result != nil {
			output[r.id] = r.result.Output
		}
	}

	e.logger.Debug("parallel execution complete",
		"step_id", step.ID,
		"duration", time.Since(start),
		"nested_count", len(step.Steps),
		"error_count", len(errors),
	)

	// Return error if any step failed and fail_fast is enabled
	if len(errors) > 0 && failFast {
		return output, errors[0]
	}

	// Return combined error if any step failed
	if len(errors) > 0 {
		return output, fmt.Errorf("parallel execution had %d errors: %v", len(errors), errors)
	}

	return output, nil
}

// executeForeach executes steps for each element in an array with context injection.
// Implements fail-last error semantics: all iterations run to completion,
// then the step fails if any iteration failed. Results are ordered by index.
func (e *Executor) executeForeach(ctx context.Context, step *StepDefinition, inputs map[string]interface{}, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	// Resolve the foreach expression to get the array value
	arrayValue, err := e.resolveForeachValue(step.Foreach, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve foreach expression: %w", err)
	}

	// Validate that the resolved value is an array
	array, ok := arrayValue.([]interface{})
	if !ok {
		// Check if it's a different type and provide helpful error
		typeName := "unknown"
		if arrayValue == nil {
			typeName = "null"
		} else {
			switch arrayValue.(type) {
			case map[string]interface{}:
				typeName = "object"
			case string:
				typeName = "string"
			case int, int64, float64:
				typeName = "number"
			case bool:
				typeName = "boolean"
			}
		}
		return nil, &errors.ValidationError{
			Field:      "foreach",
			Message:    fmt.Sprintf("foreach requires array input, got %s", typeName),
			Suggestion: "ensure the foreach expression resolves to an array value",
		}
	}

	// Handle empty array case - return empty results
	if len(array) == 0 {
		e.logger.Debug("foreach with empty array, returning empty results",
			"step_id", step.ID,
		)
		return map[string]interface{}{
			"results": []interface{}{},
		}, nil
	}

	// Apply parent timeout if specified
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Use step-specific concurrency limit if set, otherwise use executor's default
	var sem chan struct{}
	if step.MaxConcurrency > 0 {
		sem = make(chan struct{}, step.MaxConcurrency)
		e.logger.Debug("using step-specific concurrency limit for foreach",
			"step_id", step.ID,
			"max_concurrency", step.MaxConcurrency,
		)
	} else {
		sem = e.parallelSem
	}

	type iterationResult struct {
		index  int
		result interface{}
		err    error
	}

	total := len(array)
	results := make(chan iterationResult, total)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()

	e.logger.Debug("starting foreach execution",
		"step_id", step.ID,
		"array_length", total,
		"max_concurrency", step.MaxConcurrency,
	)

	// Launch goroutines for each array element
	for idx, item := range array {
		go func(index int, element interface{}) {
			iterStart := time.Now()

			// Acquire semaphore (blocks if at capacity)
			select {
			case sem <- struct{}{}:
				// Got slot
			case <-ctx.Done():
				e.logger.Debug("foreach iteration cancelled while waiting for slot",
					"parent_step_id", step.ID,
					"index", index,
				)
				results <- iterationResult{
					index: index,
					err:   ctx.Err(),
				}
				return
			}
			defer func() { <-sem }() // Release slot

			// Check if context is already cancelled
			select {
			case <-ctx.Done():
				e.logger.Debug("foreach iteration cancelled before execution",
					"parent_step_id", step.ID,
					"index", index,
				)
				results <- iterationResult{
					index: index,
					err:   ctx.Err(),
				}
				return
			default:
			}

			// Create a copy of workflow context for this iteration
			iterContext := copyWorkflowContext(workflowContext)

			// Inject foreach context variables into template context
			if templateCtx, ok := iterContext["_templateContext"].(*TemplateContext); ok {
				// Create a new template context with foreach variables
				newTemplateCtx := &TemplateContext{
					Inputs: make(map[string]interface{}),
					Steps:  templateCtx.Steps,
					Env:    templateCtx.Env,
					Tools:  templateCtx.Tools,
				}
				// Copy existing inputs
				if templateCtx.Inputs != nil {
					for k, v := range templateCtx.Inputs {
						newTemplateCtx.Inputs[k] = v
					}
				}
				// Add foreach-specific variables
				newTemplateCtx.Inputs["item"] = element
				newTemplateCtx.Inputs["index"] = index
				newTemplateCtx.Inputs["total"] = total
				iterContext["_templateContext"] = newTemplateCtx
			}

			// Execute all nested steps for this iteration
			// We treat the nested steps as a mini workflow
			iterOutput := make(map[string]interface{})
			var iterErr error
			for _, nested := range step.Steps {
				result, err := e.Execute(ctx, &nested, iterContext)
				if err != nil {
					iterErr = err
					e.logger.Debug("foreach iteration step failed",
						"parent_step_id", step.ID,
						"index", index,
						"step_id", nested.ID,
						"error", err,
					)
					break
				}
				if result != nil && nested.ID != "" {
					iterOutput[nested.ID] = result.Output
					// Update context for subsequent steps in this iteration
					if tc, ok := iterContext["_templateContext"].(*TemplateContext); ok {
						if tc.Steps == nil {
							tc.Steps = make(map[string]map[string]interface{})
						}
						tc.Steps[nested.ID] = map[string]interface{}{
							"response": result.Output,
							"status":   result.Status,
						}
					}
				}
			}

			e.logger.Debug("foreach iteration completed",
				"parent_step_id", step.ID,
				"index", index,
				"duration", time.Since(iterStart),
				"error", iterErr,
			)

			results <- iterationResult{
				index:  index,
				result: iterOutput,
				err:    iterErr,
			}
		}(idx, item)
	}

	// Collect all results (fail-last semantics: let all iterations complete)
	iterResults := make([]iterationResult, total)
	for i := 0; i < total; i++ {
		r := <-results
		iterResults[i] = r
	}

	// Sort results by index to maintain original order
	sortedResults := make([]iterationResult, total)
	for _, r := range iterResults {
		sortedResults[r.index] = r
	}

	// Build output array and collect errors
	outputArray := make([]interface{}, total)
	var errors []error
	for i, r := range sortedResults {
		if r.err != nil {
			errors = append(errors, fmt.Errorf("iteration %d: %w", i, r.err))
		}
		outputArray[i] = r.result
	}

	e.logger.Debug("foreach execution complete",
		"step_id", step.ID,
		"duration", time.Since(start),
		"total_iterations", total,
		"error_count", len(errors),
	)

	// Fail-last: if any iteration failed, fail the entire foreach
	if len(errors) > 0 {
		return nil, fmt.Errorf("foreach had %d failed iterations (first error: %v)", len(errors), errors[0])
	}

	return map[string]interface{}{
		"results": outputArray,
	}, nil
}

// copyWorkflowContext creates a shallow copy of the workflow context.
// This ensures each parallel step has its own context without interfering with others.
func copyWorkflowContext(ctx map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{})
	for k, v := range ctx {
		copy[k] = v
	}
	return copy
}

// evaluateCondition evaluates a condition expression using the expression evaluator.
// Supports expressions like:
//   - "security" in inputs.personas
//   - has(inputs.personas, "security")
//   - inputs.mode == "strict"
//   - inputs.count > 5 && inputs.enabled
func (e *Executor) evaluateCondition(expr string, workflowContext map[string]interface{}) (bool, error) {
	// Build expression context from workflow context
	ctx := expression.BuildContext(workflowContext)

	// Evaluate the expression
	result, err := e.exprEval.Evaluate(expr, ctx)
	if err != nil {
		return false, err
	}

	e.logger.Debug("condition evaluated",
		"expression", expr,
		"result", result,
	)

	return result, nil
}

// resolveInputs resolves input values by substituting context variables.
// Uses Go template syntax ({{.variable}}) for variable resolution.
func (e *Executor) resolveInputs(inputs map[string]interface{}, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	// Extract template context from workflow context
	ctx, ok := workflowContext["_templateContext"].(*TemplateContext)
	if !ok || ctx == nil {
		// No template context available, return inputs as-is
		return inputs, nil
	}

	// Use ResolveInputs to resolve all string values
	resolved, err := ResolveInputs(inputs, ctx)
	if err != nil {
		return nil, err
	}

	e.logger.Debug("resolved template inputs",
		"original", inputs,
		"resolved", resolved,
	)

	return resolved, nil
}

// resolveStepFields resolves template variables in step definition fields (prompt, system).
func (e *Executor) resolveStepFields(step *StepDefinition, workflowContext map[string]interface{}) error {
	// Extract template context from workflow context
	ctx, ok := workflowContext["_templateContext"].(*TemplateContext)
	if !ok || ctx == nil {
		// No template context available, nothing to resolve
		return nil
	}

	// Resolve prompt field
	if step.Prompt != "" {
		original := step.Prompt
		resolved, err := ResolveTemplate(step.Prompt, ctx)
		if err != nil {
			return fmt.Errorf("failed to resolve prompt: %w", err)
		}
		step.Prompt = resolved
		e.logger.Debug("resolved prompt template",
			"original", original,
			"resolved", resolved,
		)
	}

	// Resolve system field
	if step.System != "" {
		original := step.System
		resolved, err := ResolveTemplate(step.System, ctx)
		if err != nil {
			return fmt.Errorf("failed to resolve system prompt: %w", err)
		}
		step.System = resolved
		e.logger.Debug("resolved system prompt template",
			"original", original,
			"resolved", resolved,
		)
	}

	return nil
}

// handleError handles step execution errors according to the step's error handling configuration.
func (e *Executor) handleError(ctx context.Context, step *StepDefinition, result *StepResult, err error) (*StepResult, error) {
	switch step.OnError.Strategy {
	case ErrorStrategyFail:
		// Default behavior: propagate error
		return result, err

	case ErrorStrategyIgnore:
		// Mark as success but include error info
		result.Status = StepStatusSuccess
		result.Error = fmt.Sprintf("ignored error: %s", err.Error())
		return result, nil

	case ErrorStrategyRetry:
		// Retry logic is handled by executeWithRetry
		return result, err

	case ErrorStrategyFallback:
		// Execute fallback step
		// Phase 1: Return error with fallback step ID
		// Future implementation would actually execute the fallback step
		result.Error = fmt.Sprintf("error (fallback to %s): %s", step.OnError.FallbackStep, err.Error())
		result.Output = map[string]interface{}{
			"fallback_step": step.OnError.FallbackStep,
		}
		return result, fmt.Errorf("step failed, fallback required: %w", err)

	default:
		return result, err
	}
}

// resolveForeachValue resolves a template expression to get the actual value (not string).
// This is needed for foreach to access array values from the context.
func (e *Executor) resolveForeachValue(expr string, workflowContext map[string]interface{}) (interface{}, error) {
	templateCtx, ok := workflowContext["_templateContext"].(*TemplateContext)
	if !ok {
		return nil, &errors.ConfigError{
			Key:    "_templateContext",
			Reason: "template context not available in workflow context",
		}
	}

	// Directly look up the value from the context by parsing the expression path
	// For example: "{{.steps.step_id}}" or "{{.steps.step_id.field}}"

	// Remove {{ and }} if present
	path := expr
	if strings.HasPrefix(path, "{{") && strings.HasSuffix(path, "}}") {
		path = strings.TrimSpace(path[2 : len(path)-2])
	}

	// Remove leading dot
	if strings.HasPrefix(path, ".") {
		path = path[1:]
	}

	// Split path by dots
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, &errors.ValidationError{
			Field:      "foreach",
			Message:    fmt.Sprintf("invalid expression: %s", expr),
			Suggestion: "use template syntax like {{.steps.step_id.field}}",
		}
	}

	// Navigate through the context
	var current interface{}
	contextMap := templateCtx.ToMap()
	current = contextMap

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, &errors.ValidationError{
					Field:      "foreach",
					Message:    fmt.Sprintf("field %s not found in context", part),
					Suggestion: fmt.Sprintf("check that the path %s exists in workflow context", expr),
				}
			}
			current = val
		case map[string]map[string]interface{}:
			// Handle the Steps field which is map[string]map[string]interface{}
			val, ok := v[part]
			if !ok {
				return nil, &errors.ValidationError{
					Field:      "foreach",
					Message:    fmt.Sprintf("field %s not found in context", part),
					Suggestion: fmt.Sprintf("check that the path %s exists in workflow context", expr),
				}
			}
			current = val
		default:
			return nil, &errors.ValidationError{
				Field:      "foreach",
				Message:    fmt.Sprintf("cannot access field %s on non-object (type %T)", part, current),
				Suggestion: "ensure the expression path references object fields only",
			}
		}
	}

	return current, nil
}
