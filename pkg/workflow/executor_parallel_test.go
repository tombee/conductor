package workflow

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: mockLLMProvider is defined in executor_test.go

func TestExecuteParallel_AllSuccess(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	step := &StepDefinition{
		ID:   "parallel_step",
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
			{
				ID:     "step_c",
				Type:   StepTypeLLM,
				Prompt: "Prompt C",
			},
		},
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)
	assert.True(t, result.Status == StepStatusSuccess)

	// Verify all step results are aggregated
	output := result.Output
	assert.NotNil(t, output["step_a"])
	assert.NotNil(t, output["step_b"])
	assert.NotNil(t, output["step_c"])
}

func TestExecuteParallel_WithConditions(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	step := &StepDefinition{
		ID:   "parallel_step",
		Type: StepTypeParallel,
		Steps: []StepDefinition{
			{
				ID:   "security",
				Type: StepTypeLLM,
				Condition: &ConditionDefinition{
					Expression: `"security" in inputs.personas`,
				},
				Prompt: "Security review",
			},
			{
				ID:   "performance",
				Type: StepTypeLLM,
				Condition: &ConditionDefinition{
					Expression: `"performance" in inputs.personas`,
				},
				Prompt: "Performance review",
			},
			{
				ID:   "style",
				Type: StepTypeLLM,
				Condition: &ConditionDefinition{
					Expression: `"style" in inputs.personas`, // Not in list
				},
				Prompt: "Style review",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"inputs": map[string]interface{}{
			"personas": []interface{}{"security", "performance"},
		},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)

	output := result.Output

	// Security and performance should have run
	securityOutput := output["security"].(map[string]interface{})
	assert.NotNil(t, securityOutput["response"])

	perfOutput := output["performance"].(map[string]interface{})
	assert.NotNil(t, perfOutput["response"])

	// Style should be skipped
	styleOutput := output["style"].(map[string]interface{})
	assert.Equal(t, true, styleOutput["skipped"])
}

func TestExecuteParallel_OneFailsFailFast(t *testing.T) {
	// Create a provider that fails for step_b
	provider := &conditionalMockProvider{
		responses: map[string]string{
			"Prompt A": "Response A",
			"Prompt C": "Response C",
		},
		errors: map[string]error{
			"Prompt B": errors.New("step_b failed"),
		},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:   "parallel_step",
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
			{
				ID:     "step_c",
				Type:   StepTypeLLM,
				Prompt: "Prompt C",
			},
		},
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step_b")

	// Still returns partial results
	assert.NotNil(t, result.Output)
}

func TestExecuteParallel_OneFailsContinue(t *testing.T) {
	// Create a provider that fails for step_b
	provider := &conditionalMockProvider{
		responses: map[string]string{
			"Prompt A": "Response A",
			"Prompt C": "Response C",
		},
		errors: map[string]error{
			"Prompt B": errors.New("step_b failed"),
		},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:   "parallel_step",
		Type: StepTypeParallel,
		OnError: &ErrorHandlingDefinition{
			Strategy: ErrorStrategyIgnore, // Continue on error
		},
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
			{
				ID:     "step_c",
				Type:   StepTypeLLM,
				Prompt: "Prompt C",
			},
		},
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	// With ErrorStrategyIgnore, error is suppressed but result captures it
	require.NoError(t, err)
	// With ignore strategy, failures are captured but don't fail the parallel step
	assert.NotEqual(t, StepStatusFailed, result.Status)

	// All successful steps should have results
	output := result.Output
	assert.NotNil(t, output["step_a"])
	assert.NotNil(t, output["step_c"])
}

func TestExecuteParallel_Timeout(t *testing.T) {
	// Create a provider that delays response
	provider := &delayedMockProvider{
		delay:    2 * time.Second,
		response: "delayed response",
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:      "parallel_step",
		Type:    StepTypeParallel,
		Timeout: 1, // 1 second timeout
		Steps: []StepDefinition{
			{
				ID:     "slow_step",
				Type:   StepTypeLLM,
				Prompt: "Slow prompt",
			},
		},
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
	assert.Equal(t, StepStatusFailed, result.Status)
}

func TestExecuteParallel_ConcurrencyLimit(t *testing.T) {
	var concurrent int32
	var maxConcurrent int32

	// Create a provider that tracks concurrency
	provider := &trackingMockProvider{
		response: "response",
		onCall: func() {
			current := atomic.AddInt32(&concurrent, 1)
			// Track max concurrent
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if current <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond) // Small delay to allow overlap
			atomic.AddInt32(&concurrent, -1)
		},
	}

	// Set concurrency limit to 2 via executor config
	executor := NewExecutor(nil, provider).WithParallelConcurrency(2)

	// Create 5 parallel steps
	step := &StepDefinition{
		ID:   "parallel_step",
		Type: StepTypeParallel,
		Steps: []StepDefinition{
			{ID: "step_1", Type: StepTypeLLM, Prompt: "P1"},
			{ID: "step_2", Type: StepTypeLLM, Prompt: "P2"},
			{ID: "step_3", Type: StepTypeLLM, Prompt: "P3"},
			{ID: "step_4", Type: StepTypeLLM, Prompt: "P4"},
			{ID: "step_5", Type: StepTypeLLM, Prompt: "P5"},
		},
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)
	assert.Equal(t, StepStatusSuccess, result.Status)

	// Max concurrent should not exceed 2
	assert.LessOrEqual(t, atomic.LoadInt32(&maxConcurrent), int32(2))
}

func TestExecuteParallel_StepLevelConcurrency(t *testing.T) {
	var concurrent int32
	var maxConcurrent int32

	// Create a provider that tracks concurrency
	provider := &trackingMockProvider{
		response: "response",
		onCall: func() {
			current := atomic.AddInt32(&concurrent, 1)
			// Track max concurrent
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if current <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond) // Small delay to allow overlap
			atomic.AddInt32(&concurrent, -1)
		},
	}

	// Executor has default concurrency (3), but step overrides to 1
	executor := NewExecutor(nil, provider)

	// Create 5 parallel steps with step-level max_concurrency of 1
	step := &StepDefinition{
		ID:             "parallel_step",
		Type:           StepTypeParallel,
		MaxConcurrency: 1, // Override to run only 1 at a time
		Steps: []StepDefinition{
			{ID: "step_1", Type: StepTypeLLM, Prompt: "P1"},
			{ID: "step_2", Type: StepTypeLLM, Prompt: "P2"},
			{ID: "step_3", Type: StepTypeLLM, Prompt: "P3"},
		},
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)
	assert.Equal(t, StepStatusSuccess, result.Status)

	// Max concurrent should not exceed 1 (step-level override)
	assert.LessOrEqual(t, atomic.LoadInt32(&maxConcurrent), int32(1))
}

func TestExecuteParallel_EmptySteps(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test"})

	step := &StepDefinition{
		ID:    "parallel_step",
		Type:  StepTypeParallel,
		Steps: []StepDefinition{}, // Empty steps
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no nested steps")
	assert.Equal(t, StepStatusFailed, result.Status)
}

func TestExecuteParallel_NestedParallel(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	step := &StepDefinition{
		ID:   "outer_parallel",
		Type: StepTypeParallel,
		Steps: []StepDefinition{
			{
				ID:   "inner_parallel",
				Type: StepTypeParallel,
				Steps: []StepDefinition{
					{ID: "inner_a", Type: StepTypeLLM, Prompt: "A"},
					{ID: "inner_b", Type: StepTypeLLM, Prompt: "B"},
				},
			},
			{
				ID:     "sibling",
				Type:   StepTypeLLM,
				Prompt: "Sibling",
			},
		},
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)
	assert.Equal(t, StepStatusSuccess, result.Status)

	// Verify nested structure
	output := result.Output
	assert.NotNil(t, output["inner_parallel"])
	assert.NotNil(t, output["sibling"])
}

// Helper mock providers for testing

type conditionalMockProvider struct {
	responses map[string]string
	errors    map[string]error
}

func (m *conditionalMockProvider) Complete(ctx context.Context, prompt string, options map[string]interface{}) (*CompletionResult, error) {
	if err, ok := m.errors[prompt]; ok {
		return nil, err
	}
	content := "default response"
	if resp, ok := m.responses[prompt]; ok {
		content = resp
	}
	return &CompletionResult{Content: content, Model: "mock"}, nil
}

type delayedMockProvider struct {
	delay    time.Duration
	response string
}

func (m *delayedMockProvider) Complete(ctx context.Context, prompt string, options map[string]interface{}) (*CompletionResult, error) {
	select {
	case <-time.After(m.delay):
		return &CompletionResult{Content: m.response, Model: "mock"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type trackingMockProvider struct {
	response string
	onCall   func()
}

func (m *trackingMockProvider) Complete(ctx context.Context, prompt string, options map[string]interface{}) (*CompletionResult, error) {
	if m.onCall != nil {
		m.onCall()
	}
	return &CompletionResult{Content: m.response, Model: "mock"}, nil
}

func TestExecuteForeach_ArraySizeLimit(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	// Create an array with 10,001 items (exceeds limit of 10,000)
	largeArray := make([]interface{}, 10001)
	for i := range largeArray {
		largeArray[i] = i
	}

	step := &StepDefinition{
		ID:      "foreach_step",
		Type:    StepTypeParallel,
		Foreach: "{{.inputs.items}}", // Template expression referencing inputs
		Steps: []StepDefinition{
			{
				ID:     "process",
				Type:   StepTypeLLM,
				Prompt: "Process item",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"_templateContext": &TemplateContext{
			Inputs: map[string]interface{}{
				"items": largeArray,
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "foreach array size")
	assert.Contains(t, err.Error(), "10001")
	assert.Contains(t, err.Error(), "exceeds maximum of 10000")
	assert.Equal(t, StepStatusFailed, result.Status)
}
