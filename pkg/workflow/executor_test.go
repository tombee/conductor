package workflow

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockToolRegistry is a mock tool registry for testing
type mockToolRegistry struct {
	tools map[string]Tool
}

func newMockToolRegistry() *mockToolRegistry {
	return &mockToolRegistry{
		tools: make(map[string]Tool),
	}
}

func (m *mockToolRegistry) RegisterTool(name string, tool Tool) {
	m.tools[name] = tool
}

func (m *mockToolRegistry) GetTool(name string) (Tool, error) {
	tool, ok := m.tools[name]
	if !ok {
		return nil, errors.New("tool not found")
	}
	return tool, nil
}

func (m *mockToolRegistry) ExecuteTool(ctx context.Context, name string, inputs map[string]interface{}) (map[string]interface{}, error) {
	tool, err := m.GetTool(name)
	if err != nil {
		return nil, err
	}
	return tool.Execute(ctx, inputs)
}

// mockTool is a mock tool for testing
type mockTool struct {
	name      string
	output    map[string]interface{}
	err       error
	callCount int
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

// mockFlakyTool is a tool that fails a few times then succeeds
type mockFlakyTool struct {
	name         string
	attemptCount *int
}

func (m *mockFlakyTool) Name() string {
	return m.name
}

func (m *mockFlakyTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	*m.attemptCount++
	if *m.attemptCount < 3 {
		return nil, errors.New("temporary failure")
	}
	return map[string]interface{}{"result": "success"}, nil
}

// mockSlowTool is a tool that takes longer than the timeout
type mockSlowTool struct {
	name string
}

func (m *mockSlowTool) Name() string {
	return m.name
}

func (m *mockSlowTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Sleep longer than timeout
	select {
	case <-time.After(2 * time.Second):
		return map[string]interface{}{"result": "success"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// mockLLMProvider is a mock LLM provider for testing
type mockLLMProvider struct {
	response string
	err      error
}

func (m *mockLLMProvider) Complete(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func TestNewStepExecutor(t *testing.T) {
	registry := newMockToolRegistry()
	llm := &mockLLMProvider{}

	executor := NewStepExecutor(registry, llm)
	if executor == nil {
		t.Fatal("NewStepExecutor() returned nil")
	}

	if executor.toolRegistry == nil {
		t.Error("toolRegistry should be set")
	}

	if executor.llmProvider == nil {
		t.Error("llmProvider should be set")
	}
}

func TestStepExecutor_ExecuteActionStep(t *testing.T) {
	registry := newMockToolRegistry()
	tool := &mockTool{
		name:   "test-tool",
		output: map[string]interface{}{"result": "success"},
	}
	registry.RegisterTool("test-tool", tool)

	executor := NewStepExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-1",
		Type:   StepTypeAction,
		Action: "test-tool",
		Inputs: map[string]interface{}{
			"input": "test",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.Success {
		t.Error("Result should be successful")
	}

	if result.StepID != "step-1" {
		t.Errorf("StepID = %s, want step-1", result.StepID)
	}

	if tool.callCount != 1 {
		t.Errorf("Tool call count = %d, want 1", tool.callCount)
	}
}

func TestStepExecutor_ExecuteLLMStep(t *testing.T) {
	llm := &mockLLMProvider{
		response: "LLM response",
	}

	executor := NewStepExecutor(nil, llm)
	ctx := context.Background()

	step := &StepDefinition{
		ID:   "step-2",
		Type: StepTypeLLM,
		Inputs: map[string]interface{}{
			"prompt": "What is 2+2?",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.Success {
		t.Error("Result should be successful")
	}

	response, ok := result.Output["response"].(string)
	if !ok {
		t.Fatal("Output should contain response string")
	}

	if response != "LLM response" {
		t.Errorf("Response = %s, want 'LLM response'", response)
	}
}

func TestStepExecutor_ExecuteConditionStep(t *testing.T) {
	executor := NewStepExecutor(nil, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:   "step-3",
		Name: "condition-step",
		Type: StepTypeCondition,
		Condition: &ConditionDefinition{
			Expression: "$.value == 'test'",
			ThenSteps:  []string{"step-4"},
			ElseSteps:  []string{"step-5"},
		},
	}

	result, err := executor.Execute(ctx, step, map[string]interface{}{
		"value": "test",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.Success {
		t.Error("Result should be successful")
	}

	conditionMet, ok := result.Output["condition_met"].(bool)
	if !ok {
		t.Fatal("Output should contain condition_met boolean")
	}

	if !conditionMet {
		t.Error("Condition should be met")
	}
}

func TestStepExecutor_ExecuteParallelStep(t *testing.T) {
	executor := NewStepExecutor(nil, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:   "step-4",
		Type: StepTypeParallel,
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.Success {
		t.Error("Result should be successful")
	}

	// Phase 1: parallel is not implemented, just returns placeholder
	if result.Output["parallel"] != "not yet implemented" {
		t.Error("Parallel step should return placeholder")
	}
}

func TestStepExecutor_ExecuteWithRetry(t *testing.T) {
	registry := newMockToolRegistry()

	// Tool that fails twice then succeeds
	attemptCount := 0

	// Create a custom mock tool that changes behavior
	flakyTool := &mockFlakyTool{
		name:         "flaky-tool",
		attemptCount: &attemptCount,
	}
	registry.RegisterTool("flaky-tool", flakyTool)
	executor := NewStepExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-5",
		Name:   "retry-step",
		Type:   StepTypeAction,
		Action: "flaky-tool",
		Retry: &RetryDefinition{
			MaxAttempts:        3,
			BackoffBase:        1, // minimum 1 second for validation
			BackoffMultiplier:  1.01, // Very small multiplier for fast test
		},
		Timeout: 10, // 10 second timeout
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.Success {
		t.Error("Result should be successful after retry")
	}

	if result.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", result.Attempts)
	}
}

func TestStepExecutor_RetryExhaustion(t *testing.T) {
	registry := newMockToolRegistry()
	tool := &mockTool{
		name: "failing-tool",
		err:  errors.New("persistent failure"),
	}
	registry.RegisterTool("failing-tool", tool)

	executor := NewStepExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-6",
		Name:   "failing-step",
		Type:   StepTypeAction,
		Action: "failing-tool",
		Retry: &RetryDefinition{
			MaxAttempts:        2,
			BackoffBase:        1,
			BackoffMultiplier:  1.0,
		},
		Timeout: 10,
	}

	result, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error after retry exhaustion")
	}

	if result.Success {
		t.Error("Result should not be successful")
	}

	if result.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", result.Attempts)
	}
}

func TestStepExecutor_Timeout(t *testing.T) {
	registry := newMockToolRegistry()
	slowTool := &mockSlowTool{
		name: "slow-tool",
	}
	registry.RegisterTool("slow-tool", slowTool)

	executor := NewStepExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:      "step-7",
		Name:    "slow-step",
		Type:    StepTypeAction,
		Action:  "slow-tool",
		Timeout: 1, // 1 second timeout to trigger before tool completes
	}

	result, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error on timeout")
	}

	if result.Success {
		t.Error("Result should not be successful after timeout")
	}
}

func TestStepExecutor_ErrorStrategyIgnore(t *testing.T) {
	registry := newMockToolRegistry()
	tool := &mockTool{
		name: "failing-tool",
		err:  errors.New("tool error"),
	}
	registry.RegisterTool("failing-tool", tool)

	executor := NewStepExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-8",
		Name:   "ignore-error-step",
		Type:   StepTypeAction,
		Action: "failing-tool",
		OnError: &ErrorHandlingDefinition{
			Strategy: ErrorStrategyIgnore,
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("Execute() should not return error with ignore strategy: %v", err)
	}

	if !result.Success {
		t.Error("Result should be marked as successful with ignore strategy")
	}

	if result.Error == "" {
		t.Error("Result should contain error message")
	}
}

func TestStepExecutor_ErrorStrategyFail(t *testing.T) {
	registry := newMockToolRegistry()
	tool := &mockTool{
		name: "failing-tool",
		err:  errors.New("tool error"),
	}
	registry.RegisterTool("failing-tool", tool)

	executor := NewStepExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-9",
		Name:   "fail-error-step",
		Type:   StepTypeAction,
		Action: "failing-tool",
		OnError: &ErrorHandlingDefinition{
			Strategy: ErrorStrategyFail,
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error with fail strategy")
	}

	if result.Success {
		t.Error("Result should not be successful")
	}
}

func TestStepExecutor_ErrorStrategyFallback(t *testing.T) {
	registry := newMockToolRegistry()
	tool := &mockTool{
		name: "failing-tool",
		err:  errors.New("tool error"),
	}
	registry.RegisterTool("failing-tool", tool)

	executor := NewStepExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-10",
		Name:   "fallback-error-step",
		Type:   StepTypeAction,
		Action: "failing-tool",
		OnError: &ErrorHandlingDefinition{
			Strategy:     ErrorStrategyFallback,
			FallbackStep: "fallback-step",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error with fallback strategy")
	}

	if result.Success {
		t.Error("Result should not be successful")
	}

	fallbackStep, ok := result.Output["fallback_step"].(string)
	if !ok {
		t.Fatal("Output should contain fallback_step")
	}

	if fallbackStep != "fallback-step" {
		t.Errorf("Fallback step = %s, want fallback-step", fallbackStep)
	}
}

func TestStepExecutor_UnsupportedStepType(t *testing.T) {
	executor := NewStepExecutor(nil, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:   "step-11",
		Name: "unsupported-step",
		Type: "unsupported",
	}

	_, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error for unsupported step type")
	}
}

func TestStepExecutor_ActionWithoutToolRegistry(t *testing.T) {
	executor := NewStepExecutor(nil, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-12",
		Name:   "no-registry-step",
		Type:   StepTypeAction,
		Action: "test-tool",
	}

	_, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error when tool registry is not configured")
	}
}

func TestStepExecutor_LLMWithoutProvider(t *testing.T) {
	executor := NewStepExecutor(nil, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:   "step-13",
		Name: "no-llm-step",
		Type: StepTypeLLM,
		Inputs: map[string]interface{}{
			"prompt": "test",
		},
	}

	_, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error when LLM provider is not configured")
	}
}

func TestStepExecutor_LLMMissingPrompt(t *testing.T) {
	llm := &mockLLMProvider{
		response: "response",
	}

	executor := NewStepExecutor(nil, llm)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-14",
		Name:   "missing-prompt-step",
		Type:   StepTypeLLM,
		Inputs: map[string]interface{}{},
	}

	_, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error when prompt is missing")
	}
}

func TestStepExecutor_LLMInvalidPromptType(t *testing.T) {
	llm := &mockLLMProvider{
		response: "response",
	}

	executor := NewStepExecutor(nil, llm)
	ctx := context.Background()

	step := &StepDefinition{
		ID:   "step-15",
		Name: "invalid-prompt-step",
		Type: StepTypeLLM,
		Inputs: map[string]interface{}{
			"prompt": 123, // Invalid type
		},
	}

	_, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error when prompt is not a string")
	}
}

func TestStepExecutor_ConditionMissingConfig(t *testing.T) {
	executor := NewStepExecutor(nil, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:        "step-16",
		Name:      "missing-condition-step",
		Type:      StepTypeCondition,
		Condition: nil, // Missing condition config
	}

	_, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error when condition config is missing")
	}
}

func TestStepExecutor_ResolveInputs(t *testing.T) {
	executor := NewStepExecutor(nil, nil)

	inputs := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	workflowContext := map[string]interface{}{
		"contextKey": "contextValue",
	}

	resolved, err := executor.resolveInputs(inputs, workflowContext)
	if err != nil {
		t.Fatalf("resolveInputs() error = %v", err)
	}

	if len(resolved) != len(inputs) {
		t.Errorf("Resolved inputs length = %d, want %d", len(resolved), len(inputs))
	}

	// Phase 1: Simple copy without variable substitution
	for key, value := range inputs {
		if resolved[key] != value {
			t.Errorf("Resolved[%s] = %v, want %v", key, resolved[key], value)
		}
	}
}

func TestStepExecutor_StepResult(t *testing.T) {
	registry := newMockToolRegistry()
	tool := &mockTool{
		name:   "test-tool",
		output: map[string]interface{}{"result": "success"},
	}
	registry.RegisterTool("test-tool", tool)

	executor := NewStepExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-17",
		Name:   "result-test-step",
		Type:   StepTypeAction,
		Action: "test-tool",
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify result fields
	if result.StepID == "" {
		t.Error("StepID should be set")
	}

	if result.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}

	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}

	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}

	if result.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", result.Attempts)
	}
}

func TestStepExecutor_LLMWithOptions(t *testing.T) {
	llm := &mockLLMProvider{
		response: "response with options",
	}

	executor := NewStepExecutor(nil, llm)
	ctx := context.Background()

	step := &StepDefinition{
		ID:   "step-18",
		Name: "llm-with-options-step",
		Type: StepTypeLLM,
		Inputs: map[string]interface{}{
			"prompt": "test prompt",
			"options": map[string]interface{}{
				"temperature": 0.7,
				"max_tokens":  100,
			},
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.Success {
		t.Error("Result should be successful")
	}
}
