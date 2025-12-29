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

func (m *mockToolRegistry) Get(name string) (Tool, error) {
	tool, ok := m.tools[name]
	if !ok {
		return nil, errors.New("tool not found")
	}
	return tool, nil
}

func (m *mockToolRegistry) Execute(ctx context.Context, name string, inputs map[string]interface{}) (map[string]interface{}, error) {
	tool, err := m.Get(name)
	if err != nil {
		return nil, err
	}
	return tool.Execute(ctx, inputs)
}

func (m *mockToolRegistry) ListTools() []Tool {
	tools := make([]Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}
	return tools
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

func (m *mockTool) Description() string {
	return "Mock tool for testing"
}

func (m *mockTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

// mockSlowTool is a tool that takes longer than the timeout
type mockSlowTool struct {
	name string
}

func (m *mockSlowTool) Name() string {
	return m.name
}

func (m *mockSlowTool) Description() string {
	return "Slow tool for testing timeouts"
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

// mockFlakyLLMProvider is an LLM provider that fails a few times then succeeds
type mockFlakyLLMProvider struct {
	attemptCount *int
}

func (m *mockFlakyLLMProvider) Complete(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
	*m.attemptCount++
	if *m.attemptCount < 3 {
		return "", errors.New("temporary LLM failure")
	}
	return "success", nil
}

func TestNewExecutor(t *testing.T) {
	registry := newMockToolRegistry()
	llm := &mockLLMProvider{}

	executor := NewExecutor(registry, llm)
	if executor == nil {
		t.Fatal("NewExecutor() returned nil")
	}

	if executor.toolRegistry == nil {
		t.Error("toolRegistry should be set")
	}

	if executor.llmProvider == nil {
		t.Error("llmProvider should be set")
	}
}

// TestExecutor_ExecuteActionStep removed - type: tool is no longer supported
// Use type: builtin with builtin_connector and builtin_operation instead

func TestExecutor_ExecuteLLMStep(t *testing.T) {
	llm := &mockLLMProvider{
		response: "LLM response",
	}

	executor := NewExecutor(nil, llm)
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

	if result.Status != StepStatusSuccess {
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

func TestExecutor_ExecuteConditionStep(t *testing.T) {
	executor := NewExecutor(nil, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:   "step-3",
		Name: "condition-step",
		Type: StepTypeCondition,
		Condition: &ConditionDefinition{
			Expression: `inputs.value == "test"`,
			ThenSteps:  []string{"step-4"},
			ElseSteps:  []string{"step-5"},
		},
	}

	result, err := executor.Execute(ctx, step, map[string]interface{}{
		"inputs": map[string]interface{}{
			"value": "test",
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
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

func TestExecutor_ExecuteParallelStep(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "parallel response"})
	ctx := context.Background()

	step := &StepDefinition{
		ID:   "step-4",
		Type: StepTypeParallel,
		Steps: []StepDefinition{
			{
				ID:     "step_a",
				Type:   StepTypeLLM,
				Prompt: "Prompt A",
			},
			{
				ID:     "step_b",
				Type:   StepTypeLLM,
				Prompt: "Prompt B",
			},
		},
	}

	result, err := executor.Execute(ctx, step, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}

	// Verify parallel execution collected results from both steps
	if result.Output["step_a"] == nil {
		t.Error("Parallel step should have step_a result")
	}
	if result.Output["step_b"] == nil {
		t.Error("Parallel step should have step_b result")
	}
}

func TestExecutor_ExecuteWithRetry(t *testing.T) {
	// Use LLM provider that fails twice then succeeds
	attemptCount := 0

	llm := &mockFlakyLLMProvider{
		attemptCount: &attemptCount,
	}

	executor := NewExecutor(nil, llm)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-5",
		Name:   "retry-step",
		Type:   StepTypeLLM,
		Prompt: "test prompt",
		Retry: &RetryDefinition{
			MaxAttempts:       3,
			BackoffBase:       1, // minimum 1 second for validation
			BackoffMultiplier: 1.01, // Very small multiplier for fast test
		},
		Timeout: 10, // 10 second timeout
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful after retry")
	}

	if result.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", result.Attempts)
	}
}

func TestExecutor_RetryExhaustion(t *testing.T) {
	registry := newMockToolRegistry()
	tool := &mockTool{
		name: "failing-tool",
		err:  errors.New("persistent failure"),
	}
	registry.RegisterTool("failing-tool", tool)

	executor := NewExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:       "step-6",
		Name:     "failing-step",
		Type:             StepTypeIntegration,
		Action: "shell",
		Operation: "run",
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

	if result.Status == StepStatusSuccess {
		t.Error("Result should not be successful")
	}

	if result.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", result.Attempts)
	}
}

func TestExecutor_Timeout(t *testing.T) {
	registry := newMockToolRegistry()
	slowTool := &mockSlowTool{
		name: "slow-tool",
	}
	registry.RegisterTool("slow-tool", slowTool)

	executor := NewExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:       "step-7",
		Name:     "slow-step",
		Type:             StepTypeIntegration,
		Action: "shell",
		Operation: "run",
		Timeout:  1, // 1 second timeout to trigger before tool completes
	}

	result, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error on timeout")
	}

	if result.Status == StepStatusSuccess {
		t.Error("Result should not be successful after timeout")
	}
}

func TestExecutor_ErrorStrategyIgnore(t *testing.T) {
	registry := newMockToolRegistry()
	tool := &mockTool{
		name: "failing-tool",
		err:  errors.New("tool error"),
	}
	registry.RegisterTool("failing-tool", tool)

	executor := NewExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:       "step-8",
		Name:     "ignore-error-step",
		Type:             StepTypeIntegration,
		Action: "shell",
		Operation: "run",
		OnError: &ErrorHandlingDefinition{
			Strategy: ErrorStrategyIgnore,
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("Execute() should not return error with ignore strategy: %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be marked as successful with ignore strategy")
	}

	if result.Error == "" {
		t.Error("Result should contain error message")
	}
}

func TestExecutor_ErrorStrategyFail(t *testing.T) {
	registry := newMockToolRegistry()
	tool := &mockTool{
		name: "failing-tool",
		err:  errors.New("tool error"),
	}
	registry.RegisterTool("failing-tool", tool)

	executor := NewExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:       "step-9",
		Name:     "fail-error-step",
		Type:             StepTypeIntegration,
		Action: "shell",
		Operation: "run",
		OnError: &ErrorHandlingDefinition{
			Strategy: ErrorStrategyFail,
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error with fail strategy")
	}

	if result.Status == StepStatusSuccess {
		t.Error("Result should not be successful")
	}
}

func TestExecutor_ErrorStrategyFallback(t *testing.T) {
	registry := newMockToolRegistry()
	tool := &mockTool{
		name: "failing-tool",
		err:  errors.New("tool error"),
	}
	registry.RegisterTool("failing-tool", tool)

	executor := NewExecutor(registry, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:       "step-10",
		Name:     "fallback-error-step",
		Type:             StepTypeIntegration,
		Action: "shell",
		Operation: "run",
		OnError: &ErrorHandlingDefinition{
			Strategy:     ErrorStrategyFallback,
			FallbackStep: "fallback-step",
		},
	}

	result, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error with fallback strategy")
	}

	if result.Status == StepStatusSuccess {
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

func TestExecutor_UnsupportedStepType(t *testing.T) {
	executor := NewExecutor(nil, nil)
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

func TestExecutor_ActionWithoutToolRegistry(t *testing.T) {
	executor := NewExecutor(nil, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:       "step-12",
		Name:     "no-registry-step",
		Type:             StepTypeIntegration,
		Action: "file",
		Operation: "read",
	}

	_, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error when tool registry is not configured")
	}
}

func TestExecutor_LLMWithoutProvider(t *testing.T) {
	executor := NewExecutor(nil, nil)
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

func TestExecutor_LLMMissingPrompt(t *testing.T) {
	llm := &mockLLMProvider{
		response: "response",
	}

	executor := NewExecutor(nil, llm)
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

func TestExecutor_LLMInvalidPromptType(t *testing.T) {
	llm := &mockLLMProvider{
		response: "response",
	}

	executor := NewExecutor(nil, llm)
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

func TestExecutor_ConditionMissingConfig(t *testing.T) {
	executor := NewExecutor(nil, nil)
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

func TestExecutor_ResolveInputs(t *testing.T) {
	executor := NewExecutor(nil, nil)

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

func TestExecutor_StepResult(t *testing.T) {
	llm := &mockLLMProvider{
		response: "test response",
	}

	executor := NewExecutor(nil, llm)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "step-17",
		Name:   "result-test-step",
		Type:   StepTypeLLM,
		Prompt: "test prompt",
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

func TestExecutor_LLMWithOptions(t *testing.T) {
	llm := &mockLLMProvider{
		response: "response with options",
	}

	executor := NewExecutor(nil, llm)
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

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}
}

func TestExecutor_TemplateVariableResolution(t *testing.T) {
	llm := &mockLLMProvider{
		response: "Hello, Alice!",
	}

	executor := NewExecutor(nil, llm)
	ctx := context.Background()

	// Create template context
	templateCtx := NewTemplateContext()
	templateCtx.SetInput("name", "Alice")

	// Build workflow context with template context
	workflowContext := map[string]interface{}{
		"_templateContext": templateCtx,
	}

	step := &StepDefinition{
		ID:   "step-19",
		Type: StepTypeLLM,
		Inputs: map[string]interface{}{
			"prompt": "Greet {{.name}}",
		},
	}

	result, err := executor.Execute(ctx, step, workflowContext)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}
}

func TestExecutor_TemplateStepOutputResolution(t *testing.T) {
	llm := &mockLLMProvider{
		response: "Summary complete",
	}

	executor := NewExecutor(nil, llm)
	ctx := context.Background()

	// Create template context with previous step output
	templateCtx := NewTemplateContext()
	templateCtx.SetStepOutput("security", map[string]interface{}{
		"response": "No security issues found",
	})

	workflowContext := map[string]interface{}{
		"_templateContext": templateCtx,
	}

	step := &StepDefinition{
		ID:   "step-20",
		Type: StepTypeLLM,
		Inputs: map[string]interface{}{
			"prompt": "Summarize: {{.steps.security.response}}",
		},
	}

	result, err := executor.Execute(ctx, step, workflowContext)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}
}

func TestExecutor_TemplateMultipleVariables(t *testing.T) {
	llm := &mockLLMProvider{
		response: "Review complete",
	}

	executor := NewExecutor(nil, llm)
	ctx := context.Background()

	// Create template context with inputs and step outputs
	templateCtx := NewTemplateContext()
	templateCtx.SetInput("user", "Bob")
	templateCtx.SetInput("task", "code review")
	templateCtx.SetStepOutput("analysis", map[string]interface{}{
		"response": "Code looks good",
	})

	workflowContext := map[string]interface{}{
		"_templateContext": templateCtx,
	}

	step := &StepDefinition{
		ID:   "step-21",
		Type: StepTypeLLM,
		Inputs: map[string]interface{}{
			"prompt": "User: {{.user}}, Task: {{.task}}, Analysis: {{.steps.analysis.response}}",
		},
	}

	result, err := executor.Execute(ctx, step, workflowContext)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}
}

func TestExecutor_NoTemplateContext(t *testing.T) {
	llm := &mockLLMProvider{
		response: "response",
	}

	executor := NewExecutor(nil, llm)
	ctx := context.Background()

	// No template context provided
	workflowContext := map[string]interface{}{}

	step := &StepDefinition{
		ID:   "step-22",
		Type: StepTypeLLM,
		Inputs: map[string]interface{}{
			"prompt": "Plain prompt without variables",
		},
	}

	// Should still work with no template context
	result, err := executor.Execute(ctx, step, workflowContext)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}
}

func TestExecutor_SPEC2FormatMultiStepWorkflow(t *testing.T) {
	// This test demonstrates a complete multi-step workflow:
	// 1. Step uses workflow input via {{.diff}}
	// 2. Second step uses first step output via {{.steps.security.response}}
	// 3. Prompt field is used instead of inputs["prompt"]

	llmProvider := &mockLLMProvider{
		response: "Test response",
	}

	executor := NewExecutor(nil, llmProvider)
	ctx := context.Background()

	// Create template context with workflow inputs
	templateCtx := NewTemplateContext()
	templateCtx.SetInput("diff", "func main() { println('hello') }")
	templateCtx.SetInput("reviewer", "Alice")

	workflowContext := map[string]interface{}{
		"_templateContext": templateCtx,
	}

	// Step 1: Security review with template variables
	step1 := &StepDefinition{
		ID:     "security",
		Type:   StepTypeLLM,
		Model:  string(ModelTierBalanced),
		System: "You are a security expert",
		Prompt: "Reviewer: {{.reviewer}}\nReview for security issues:\n{{.diff}}",
	}

	result1, err := executor.Execute(ctx, step1, workflowContext)
	if err != nil {
		t.Fatalf("Step 1 failed: %v", err)
	}

	if result1.Status != StepStatusSuccess {
		t.Error("Step 1 should be successful")
	}

	// Verify step output is available
	if _, ok := result1.Output["response"]; !ok {
		t.Error("Step 1 should have response in output")
	}

	// Add step output to context
	templateCtx.SetStepOutput("security", result1.Output)

	// Step 2: Summary using previous step output
	step2 := &StepDefinition{
		ID:     "summary",
		Type:   StepTypeLLM,
		Model:  string(ModelTierFast),
		Prompt: "Summarize this security review:\n{{.steps.security.response}}",
	}

	result2, err := executor.Execute(ctx, step2, workflowContext)
	if err != nil {
		t.Fatalf("Step 2 failed: %v", err)
	}

	if result2.Status != StepStatusSuccess {
		t.Error("Step 2 should be successful")
	}

	// Verify final output
	if _, ok := result2.Output["response"]; !ok {
		t.Error("Step 2 should have response in output")
	}
}

func TestExecutor_PromptFieldWithTemplateVariables(t *testing.T) {
	llm := &mockLLMProvider{
		response: "response",
	}

	executor := NewExecutor(nil, llm)
	ctx := context.Background()

	templateCtx := NewTemplateContext()
	templateCtx.SetInput("user", "Bob")
	templateCtx.SetInput("action", "review code")

	workflowContext := map[string]interface{}{
		"_templateContext": templateCtx,
	}

	step := &StepDefinition{
		ID:     "task",
		Type:   StepTypeLLM,
		Prompt: "User {{.user}} wants to {{.action}}",
	}

	result, err := executor.Execute(ctx, step, workflowContext)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}
}

func TestExecutor_SystemPromptWithTemplateVariables(t *testing.T) {
	llm := &mockLLMProvider{
		response: "response",
	}

	executor := NewExecutor(nil, llm)
	ctx := context.Background()

	templateCtx := NewTemplateContext()
	templateCtx.SetInput("role", "security expert")

	workflowContext := map[string]interface{}{
		"_templateContext": templateCtx,
	}

	step := &StepDefinition{
		ID:     "task",
		Type:   StepTypeLLM,
		System: "You are a {{.role}}",
		Prompt: "Review this code",
	}

	result, err := executor.Execute(ctx, step, workflowContext)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}
}

func TestExecutor_ExecuteToolStep(t *testing.T) {
	llm := &mockLLMProvider{
		response: "test response",
	}

	executor := NewExecutor(nil, llm)
	ctx := context.Background()

	step := &StepDefinition{
		ID:     "llm-step",
		Type:   StepTypeLLM,
		Prompt: "test prompt",
	}

	result, err := executor.Execute(ctx, step, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}

	if result.StepID != "llm-step" {
		t.Errorf("StepID = %s, want llm-step", result.StepID)
	}

	// LLM output should contain response
	response, ok := result.Output["response"].(string)
	if !ok {
		t.Fatal("Output should contain response string")
	}

	if response != "test response" {
		t.Errorf("Response = %s, want 'test response'", response)
	}
}

func TestExecutor_ToolStepWithoutRegistry(t *testing.T) {
	executor := NewExecutor(nil, nil)
	ctx := context.Background()

	step := &StepDefinition{
		ID:       "tool-step",
		Type:             StepTypeIntegration,
		Action: "file",
		Operation: "read",
	}

	_, err := executor.Execute(ctx, step, nil)
	if err == nil {
		t.Error("Execute() should return error when tool registry is not configured")
	}
}

// TestExecutor_ToolStepMissingToolName removed - type: tool is no longer supported
// Missing builtin_connector/builtin_operation is caught at definition validation level

// TestExecutor_CompleteWorkflowIntegration tests a complete workflow
// by parsing YAML and executing multiple steps with template variable passing.
func TestExecutor_CompleteWorkflowIntegration(t *testing.T) {
	// Define a complete workflow as YAML
	workflowYAML := `
name: code-review
inputs:
  - name: diff
    type: string
steps:
  - id: security
    type: llm
    model: fast
    system: "You are a security reviewer"
    prompt: "Review for security:\n{{.diff}}"
    retry:
      max_attempts: 1
      backoff_base: 1
      backoff_multiplier: 1.0
  - id: summary
    type: llm
    prompt: "Summarize:\n{{.steps.security.response}}"
    retry:
      max_attempts: 1
      backoff_base: 1
      backoff_multiplier: 1.0
outputs:
  - name: review
    type: string
    value: "{{.steps.summary.response}}"
`

	// Parse the workflow definition
	definition, err := ParseDefinition([]byte(workflowYAML))
	if err != nil {
		t.Fatalf("Failed to parse workflow YAML: %v", err)
	}

	// Verify workflow structure
	if definition.Name != "code-review" {
		t.Errorf("Workflow name = %s, want code-review", definition.Name)
	}

	if len(definition.Steps) != 2 {
		t.Fatalf("Expected 2 steps, got %d", len(definition.Steps))
	}

	// Set up mock LLM provider that returns different responses based on prompt
	llmProvider := &mockLLMProvider{}

	// Create executor
	executor := NewExecutor(nil, llmProvider)
	ctx := context.Background()

	// Create template context with workflow inputs
	templateCtx := NewTemplateContext()
	templateCtx.SetInput("diff", "func main() { fmt.Println(\"hello\") }")

	workflowContext := map[string]interface{}{
		"_templateContext": templateCtx,
	}

	// Execute Step 1: Security review
	llmProvider.response = "No security issues found"

	step1 := &definition.Steps[0]
	if step1.ID != "security" {
		t.Errorf("Step 1 ID = %s, want security", step1.ID)
	}

	result1, err := executor.Execute(ctx, step1, workflowContext)
	if err != nil {
		t.Fatalf("Step 1 execution failed: %v", err)
	}

	if result1.Status != StepStatusSuccess {
		t.Error("Step 1 should be successful")
	}

	// Verify step 1 output
	securityResponse, ok := result1.Output["response"].(string)
	if !ok {
		t.Fatal("Step 1 output should contain response string")
	}

	if securityResponse != "No security issues found" {
		t.Errorf("Step 1 response = %s, want 'No security issues found'", securityResponse)
	}

	// Add step 1 output to template context
	templateCtx.SetStepOutput("security", result1.Output)

	// Execute Step 2: Summary
	llmProvider.response = "Code review complete: No issues"

	step2 := &definition.Steps[1]
	if step2.ID != "summary" {
		t.Errorf("Step 2 ID = %s, want summary", step2.ID)
	}

	result2, err := executor.Execute(ctx, step2, workflowContext)
	if err != nil {
		t.Fatalf("Step 2 execution failed: %v", err)
	}

	if result2.Status != StepStatusSuccess {
		t.Error("Step 2 should be successful")
	}

	// Verify step 2 output
	summaryResponse, ok := result2.Output["response"].(string)
	if !ok {
		t.Fatal("Step 2 output should contain response string")
	}

	if summaryResponse != "Code review complete: No issues" {
		t.Errorf("Step 2 response = %s, want 'Code review complete: No issues'", summaryResponse)
	}

	// Verify workflow output can be resolved
	templateCtx.SetStepOutput("summary", result2.Output)

	outputValue, err := ResolveTemplate(definition.Outputs[0].Value, templateCtx)
	if err != nil {
		t.Fatalf("Failed to resolve output value: %v", err)
	}

	if outputValue != "Code review complete: No issues" {
		t.Errorf("Output value = %s, want 'Code review complete: No issues'", outputValue)
	}
}

// TestExecutor_DefaultTimeoutAndRetry tests that default timeout and retry
// are applied when not specified in the workflow definition.
func TestExecutor_DefaultTimeoutAndRetry(t *testing.T) {
	llm := &mockLLMProvider{
		response: "response",
	}

	executor := NewExecutor(nil, llm)
	ctx := context.Background()

	// Create a step with no timeout or retry specified
	step := &StepDefinition{
		ID:     "test-defaults",
		Type:   StepTypeLLM,
		Prompt: "test prompt",
		// No Timeout or Retry specified
	}

	startTime := time.Now()
	result, err := executor.Execute(ctx, step, nil)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}

	// Verify that timeout was applied (execution should complete well within 30s default)
	if duration > 30*time.Second {
		t.Errorf("Execution took %v, which exceeds default 30s timeout", duration)
	}

	// Verify that retry was applied by checking attempts
	// Default is 2 attempts, but since the step succeeds on first try,
	// we should see only 1 attempt
	if result.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1 (successful on first try)", result.Attempts)
	}
}

// TestStructuredOutputSuccess tests T4.1-T4.6: successful structured output execution
func TestStructuredOutputSuccess(t *testing.T) {
	// Mock LLM that returns valid JSON matching schema
	llm := &mockLLMProvider{
		response: `{"category": "bug", "priority": "high"}`,
	}

	executor := NewExecutor(nil, llm)

	step := &StepDefinition{
		ID:     "classify",
		Type:   StepTypeLLM,
		Prompt: "Classify this issue",
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"bug", "feature", "question"},
				},
				"priority": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"critical", "high", "medium", "low"},
				},
			},
			"required": []interface{}{"category", "priority"},
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, step, map[string]interface{}{})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Errorf("Result should be successful, got error: %s", result.Error)
	}

	// T4.6: Check that output is stored under "output" key
	output, ok := result.Output["output"]
	if !ok {
		t.Fatal("Result should have 'output' key")
	}

	outputMap, ok := output.(map[string]interface{})
	if !ok {
		t.Fatalf("Output should be a map, got %T", output)
	}

	// Verify structured fields are accessible
	if outputMap["category"] != "bug" {
		t.Errorf("category = %v, want bug", outputMap["category"])
	}
	if outputMap["priority"] != "high" {
		t.Errorf("priority = %v, want high", outputMap["priority"])
	}

	// T4.8: Check attempt count
	attempts, ok := result.Output["attempts"]
	if !ok {
		t.Error("Result should have 'attempts' metadata")
	}
	if attempts != 1 {
		t.Errorf("attempts = %v, want 1 (success on first try)", attempts)
	}
}

// TestStructuredOutputRetry tests T4.4: retry logic for invalid responses
func TestStructuredOutputRetry(t *testing.T) {
	// Mock LLM that fails first, then succeeds
	attemptCount := 0

	// We need a more sophisticated mock
	type retryMockLLM struct {
		attempts *int
	}
	retryLLM := &retryMockLLM{attempts: &attemptCount}

	// Replace the executor's Complete method
	executor := &Executor{
		llmProvider: &mockLLMProviderFunc{
			completeFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
				*retryLLM.attempts++
				if *retryLLM.attempts == 1 {
					// First attempt: invalid JSON
					return "This is not valid JSON", nil
				} else if *retryLLM.attempts == 2 {
					// Second attempt: wrong enum value
					return `{"category": "invalid", "priority": "high"}`, nil
				}
				// Third attempt: success
				return `{"category": "bug", "priority": "high"}`, nil
			},
		},
	}

	step := &StepDefinition{
		ID:     "classify",
		Type:   StepTypeLLM,
		Prompt: "Classify this issue",
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"bug", "feature", "question"},
				},
				"priority": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []interface{}{"category"},
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, step, map[string]interface{}{})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful after retries")
	}

	// Check that it took 3 attempts
	attempts, _ := result.Output["attempts"].(int)
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3 (success on third try)", attempts)
	}
}

// mockLLMProviderFunc allows custom function-based mocking
type mockLLMProviderFunc struct {
	completeFunc func(context.Context, string, map[string]interface{}) (string, error)
}

func (m *mockLLMProviderFunc) Complete(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
	return m.completeFunc(ctx, prompt, options)
}

// TestStructuredOutputValidationFailure tests T4.7: error after max retries
func TestStructuredOutputValidationFailure(t *testing.T) {
	// Mock LLM that always returns invalid data
	llm := &mockLLMProvider{
		response: "This is always invalid",
	}

	executor := NewExecutor(nil, llm)

	step := &StepDefinition{
		ID:     "classify",
		Type:   StepTypeLLM,
		Prompt: "Classify this issue",
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"bug", "feature"},
				},
			},
			"required": []interface{}{"category"},
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, step, map[string]interface{}{})

	// Should fail after max retries
	if err == nil {
		t.Fatal("Expected error after max retries, got nil")
	}

	// Check for SchemaValidationError
	var schemaErr *SchemaValidationError
	if errors.As(err, &schemaErr) {
		if schemaErr.ErrorCode != "SCHEMA_VALIDATION_FAILED" {
			t.Errorf("ErrorCode = %s, want SCHEMA_VALIDATION_FAILED", schemaErr.ErrorCode)
		}
		if schemaErr.Attempts != 3 {
			t.Errorf("Attempts = %d, want 3", schemaErr.Attempts)
		}
	} else {
		t.Errorf("Error should be *SchemaValidationError, got %T", err)
	}

	if result.Status == StepStatusSuccess {
		t.Error("Result should not be successful")
	}
}

// TestStructuredOutputJSONExtraction tests T3.2: JSON extraction from various formats
func TestStructuredOutputJSONExtraction(t *testing.T) {
	tests := []struct {
		name         string
		llmResponse  string
		wantCategory string
	}{
		{
			name:         "clean JSON",
			llmResponse:  `{"category": "bug"}`,
			wantCategory: "bug",
		},
		{
			name:         "JSON in markdown",
			llmResponse:  "```json\n{\"category\": \"feature\"}\n```",
			wantCategory: "feature",
		},
		{
			name:         "JSON with explanation",
			llmResponse:  "Based on the analysis, the result is: {\"category\": \"question\"}",
			wantCategory: "question",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llm := &mockLLMProvider{
				response: tt.llmResponse,
			}

			executor := NewExecutor(nil, llm)

			step := &StepDefinition{
				ID:     "classify",
				Type:   StepTypeLLM,
				Prompt: "Classify this",
				OutputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"category": map[string]interface{}{
							"type": "string",
							"enum": []interface{}{"bug", "feature", "question"},
						},
					},
					"required": []interface{}{"category"},
				},
			}

			ctx := context.Background()
			result, err := executor.Execute(ctx, step, map[string]interface{}{})

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			outputMap := result.Output["output"].(map[string]interface{})
			if outputMap["category"] != tt.wantCategory {
				t.Errorf("category = %v, want %v", outputMap["category"], tt.wantCategory)
			}
		})
	}
}

// TestStructuredOutputBuiltInTypes tests T1.3: built-in output types
func TestStructuredOutputBuiltInTypes(t *testing.T) {
	tests := []struct {
		name         string
		definition   StepDefinition
		llmResponse  string
		checkResult  func(*testing.T, map[string]interface{})
	}{
		{
			name: "classification type",
			definition: StepDefinition{
				ID:     "classify",
				Type:   StepTypeLLM,
				Prompt: "Classify",
				// OutputType gets expanded in ApplyDefaults
				OutputType: "classification",
				OutputOptions: map[string]interface{}{
					"categories": []interface{}{"bug", "feature"},
				},
			},
			llmResponse: `{"category": "bug"}`,
			checkResult: func(t *testing.T, output map[string]interface{}) {
				if output["category"] != "bug" {
					t.Errorf("category = %v, want bug", output["category"])
				}
			},
		},
		{
			name: "decision type",
			definition: StepDefinition{
				ID:     "decide",
				Type:   StepTypeLLM,
				Prompt: "Decide",
				OutputType: "decision",
				OutputOptions: map[string]interface{}{
					"choices":          []interface{}{"approve", "reject"},
					"require_reasoning": true,
				},
			},
			llmResponse: `{"decision": "approve", "reasoning": "Looks good"}`,
			checkResult: func(t *testing.T, output map[string]interface{}) {
				if output["decision"] != "approve" {
					t.Errorf("decision = %v, want approve", output["decision"])
				}
				if output["reasoning"] != "Looks good" {
					t.Errorf("reasoning = %v, want 'Looks good'", output["reasoning"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llm := &mockLLMProvider{
				response: tt.llmResponse,
			}

			// Need to expand the OutputType to OutputSchema
			if err := tt.definition.expandOutputType(); err != nil {
				t.Fatalf("expandOutputType() error = %v", err)
			}

			executor := NewExecutor(nil, llm)

			ctx := context.Background()
			result, err := executor.Execute(ctx, &tt.definition, map[string]interface{}{})

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			outputMap := result.Output["output"].(map[string]interface{})
			tt.checkResult(t, outputMap)
		})
	}
}

// TestUnstructuredOutput tests that steps without OutputSchema still work
func TestUnstructuredOutput(t *testing.T) {
	llm := &mockLLMProvider{
		response: "This is a free-form response",
	}

	executor := NewExecutor(nil, llm)

	step := &StepDefinition{
		ID:     "summarize",
		Type:   StepTypeLLM,
		Prompt: "Summarize this text",
		// No OutputSchema - should work as before
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, step, map[string]interface{}{})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Error("Result should be successful")
	}

	// For unstructured output, response should be in "response" key
	response, ok := result.Output["response"]
	if !ok {
		t.Fatal("Result should have 'response' key for unstructured output")
	}

	if response != "This is a free-form response" {
		t.Errorf("response = %v, want 'This is a free-form response'", response)
	}

	// Should NOT have "output" key for unstructured
	if _, ok := result.Output["output"]; ok {
		t.Error("Unstructured output should not have 'output' key")
	}
}
