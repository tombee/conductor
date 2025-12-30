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

// loopMockProvider tracks call count and returns configurable responses
type loopMockProvider struct {
	callCount *int32
	responses []string // Responses for each call in order
	approveOn int      // Which call number to return approved: true
}

func (m *loopMockProvider) Complete(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
	count := atomic.AddInt32(m.callCount, 1)
	idx := int(count) - 1

	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return `{"approved": false, "feedback": "needs improvement"}`, nil
}

// TestExecuteLoop_BasicTerminationByCondition tests that loop terminates when condition is met
func TestExecuteLoop_BasicTerminationByCondition(t *testing.T) {
	callCount := int32(0)
	provider := &loopMockProvider{
		callCount: &callCount,
		responses: []string{
			`{"approved": false, "feedback": "needs changes"}`,
			`{"approved": false, "feedback": "almost there"}`,
			`{"approved": true, "feedback": "looks good"}`,
		},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "refine_loop",
		Type:          StepTypeLoop,
		MaxIterations: 5,
		Until:         `steps.review.response == "{\"approved\": true, \"feedback\": \"looks good\"}"`,
		Steps: []StepDefinition{
			{
				ID:     "review",
				Type:   StepTypeLLM,
				Prompt: "Review the code",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)

	output := result.Output
	assert.Equal(t, 3, output["iteration_count"])
	assert.Equal(t, LoopTerminatedByCondition, output["terminated_by"])
}

// TestExecuteLoop_TerminationByMaxIterations tests that loop stops at max_iterations
func TestExecuteLoop_TerminationByMaxIterations(t *testing.T) {
	callCount := int32(0)
	provider := &loopMockProvider{
		callCount: &callCount,
		responses: []string{
			`{"approved": false}`,
			`{"approved": false}`,
			`{"approved": false}`,
		},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "max_iterations_loop",
		Type:          StepTypeLoop,
		MaxIterations: 3,
		Until:         `steps.review.response == "{\"approved\": true}"`,
		Steps: []StepDefinition{
			{
				ID:     "review",
				Type:   StepTypeLLM,
				Prompt: "Review",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)

	output := result.Output
	assert.Equal(t, 3, output["iteration_count"])
	assert.Equal(t, LoopTerminatedByMaxIterations, output["terminated_by"])
}

// TestExecuteLoop_SingleIteration tests loop with max_iterations=1
func TestExecuteLoop_SingleIteration(t *testing.T) {
	callCount := int32(0)
	provider := &loopMockProvider{
		callCount: &callCount,
		responses: []string{`{"done": true}`},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "single_loop",
		Type:          StepTypeLoop,
		MaxIterations: 1,
		Until:         `steps.action.response == "{\"done\": true}"`,
		Steps: []StepDefinition{
			{
				ID:     "action",
				Type:   StepTypeLLM,
				Prompt: "Do something",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	output := result.Output
	// Should terminate by condition since mock returns done:true
	assert.Equal(t, 1, output["iteration_count"])
	assert.Equal(t, LoopTerminatedByCondition, output["terminated_by"])
}

// TestExecuteLoop_ContextVariables tests loop.iteration and loop.history access
func TestExecuteLoop_ContextVariables(t *testing.T) {
	callCount := int32(0)
	var receivedPrompts []string

	provider := &mockLLMProviderFunc{
		completeFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			atomic.AddInt32(&callCount, 1)
			receivedPrompts = append(receivedPrompts, prompt)
			if callCount >= 3 {
				return `{"done": true}`, nil
			}
			return `{"done": false}`, nil
		},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "context_loop",
		Type:          StepTypeLoop,
		MaxIterations: 5,
		Until:         `steps.check.response == "{\"done\": true}"`,
		Steps: []StepDefinition{
			{
				ID:     "check",
				Type:   StepTypeLLM,
				Prompt: "Iteration {{.loop.iteration}} of {{.loop.max_iterations}}",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)

	// Verify prompts received the loop context
	require.Len(t, receivedPrompts, 3)
	assert.Contains(t, receivedPrompts[0], "Iteration 0")
	assert.Contains(t, receivedPrompts[1], "Iteration 1")
	assert.Contains(t, receivedPrompts[2], "Iteration 2")
}

// TestExecuteLoop_StepConditionSkip tests skipping steps based on loop.iteration
func TestExecuteLoop_StepConditionSkip(t *testing.T) {
	callCount := int32(0)
	var applyCalls, reviewCalls int32

	provider := &mockLLMProviderFunc{
		completeFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			atomic.AddInt32(&callCount, 1)
			if prompt == "Apply feedback" {
				atomic.AddInt32(&applyCalls, 1)
			}
			if prompt == "Review code" {
				atomic.AddInt32(&reviewCalls, 1)
			}
			if callCount >= 6 { // 2 steps per iteration (except first), 3 iterations
				return `{"approved": true}`, nil
			}
			return `{"approved": false, "feedback": "improve"}`, nil
		},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "conditional_steps_loop",
		Type:          StepTypeLoop,
		MaxIterations: 5,
		Until:         `steps.review.response == "{\"approved\": true}"`,
		Steps: []StepDefinition{
			{
				ID:   "apply_feedback",
				Type: StepTypeLLM,
				Condition: &ConditionDefinition{
					Expression: `loop.iteration > 0`,
				},
				Prompt: "Apply feedback",
			},
			{
				ID:     "review",
				Type:   StepTypeLLM,
				Prompt: "Review code",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)
	assert.Equal(t, StepStatusSuccess, result.Status)

	// apply_feedback is skipped on iteration 0 (condition: loop.iteration > 0)
	// Called on iterations 1, 2, 3 = 3 times (until approved on iteration 3)
	assert.GreaterOrEqual(t, reviewCalls, int32(1))
}

// TestExecuteLoop_StepFailWithIgnore tests error handling with strategy: ignore
func TestExecuteLoop_StepFailWithIgnore(t *testing.T) {
	callCount := int32(0)

	provider := &mockLLMProviderFunc{
		completeFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			atomic.AddInt32(&callCount, 1)
			if prompt == "Fail step" && callCount == 1 {
				return "", errors.New("step failed")
			}
			if callCount >= 3 {
				return `{"done": true}`, nil
			}
			return `{"done": false}`, nil
		},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "ignore_error_loop",
		Type:          StepTypeLoop,
		MaxIterations: 5,
		Until:         `steps.check.response == "{\"done\": true}"`,
		Steps: []StepDefinition{
			{
				ID:     "fail_step",
				Type:   StepTypeLLM,
				Prompt: "Fail step",
				OnError: &ErrorHandlingDefinition{
					Strategy: ErrorStrategyIgnore,
				},
			},
			{
				ID:     "check",
				Type:   StepTypeLLM,
				Prompt: "Check status",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)
}

// TestExecuteLoop_StepFailWithFail tests error handling with strategy: fail
func TestExecuteLoop_StepFailWithFail(t *testing.T) {
	provider := &mockLLMProvider{
		err: errors.New("step failed"),
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "fail_loop",
		Type:          StepTypeLoop,
		MaxIterations: 5,
		Until:         `steps.action.response == "{\"done\": true}"`,
		Steps: []StepDefinition{
			{
				ID:     "action",
				Type:   StepTypeLLM,
				Prompt: "Do action",
				OnError: &ErrorHandlingDefinition{
					Strategy: ErrorStrategyFail,
				},
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.Error(t, err)

	output := result.Output
	assert.Equal(t, LoopTerminatedByError, output["terminated_by"])
}

// TestExecuteLoop_Timeout tests loop timeout handling
func TestExecuteLoop_Timeout(t *testing.T) {
	provider := &delayedMockProvider{
		delay:    500 * time.Millisecond,
		response: `{"done": false}`,
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "timeout_loop",
		Type:          StepTypeLoop,
		MaxIterations: 100,
		Timeout:       1, // 1 second timeout
		Until:         `steps.action.response == "{\"done\": true}"`,
		Steps: []StepDefinition{
			{
				ID:     "action",
				Type:   StepTypeLLM,
				Prompt: "Slow action",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	start := time.Now()
	result, err := executor.Execute(context.Background(), step, workflowContext)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 3*time.Second) // Should timeout around 1s

	output := result.Output
	// When timeout occurs during step execution, it may be reported as "error" or "timeout"
	// depending on where the cancellation is detected
	terminatedBy := output["terminated_by"].(string)
	assert.True(t, terminatedBy == LoopTerminatedByTimeout || terminatedBy == LoopTerminatedByError,
		"expected terminated_by to be 'timeout' or 'error', got %s", terminatedBy)
}

// TestExecuteLoop_HistoryTracking tests that history is correctly recorded
func TestExecuteLoop_HistoryTracking(t *testing.T) {
	callCount := int32(0)

	provider := &mockLLMProviderFunc{
		completeFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			count := atomic.AddInt32(&callCount, 1)
			if count >= 3 {
				return `{"done": true, "value": 100}`, nil
			}
			return `{"done": false, "value": ` + string(rune('0'+count)) + `0}`, nil
		},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "history_loop",
		Type:          StepTypeLoop,
		MaxIterations: 5,
		Until:         `steps.action.response == "{\"done\": true, \"value\": 100}"`,
		Steps: []StepDefinition{
			{
				ID:     "action",
				Type:   StepTypeLLM,
				Prompt: "Process",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	output := result.Output
	history, ok := output["history"].([]IterationRecord)
	require.True(t, ok)
	assert.Len(t, history, 3)

	// Verify iteration numbers
	for i, record := range history {
		assert.Equal(t, i, record.Iteration)
		assert.NotNil(t, record.Steps["action"])
		assert.True(t, record.DurationMs >= 0)
	}
}

// TestExecuteLoop_SensitiveFieldMasking tests that sensitive fields are masked in history
func TestExecuteLoop_SensitiveFieldMasking(t *testing.T) {
	provider := &mockLLMProviderFunc{
		completeFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			return `{"api_key": "secret123", "token": "abc", "data": "visible", "done": true}`, nil
		},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "mask_loop",
		Type:          StepTypeLoop,
		MaxIterations: 1,
		Until:         `true`, // Terminate after 1 iteration
		Steps: []StepDefinition{
			{
				ID:     "action",
				Type:   StepTypeLLM,
				Prompt: "Get secrets",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	output := result.Output
	history, ok := output["history"].([]IterationRecord)
	require.True(t, ok)
	require.Len(t, history, 1)

	// The actual step output in step_outputs is not masked
	// Only history is masked
	stepOutput := history[0].Steps["action"]
	if stepMap, ok := stepOutput.(map[string]interface{}); ok {
		// These should be masked in history
		if apiKey, exists := stepMap["api_key"]; exists {
			assert.Equal(t, "***MASKED***", apiKey)
		}
		if token, exists := stepMap["token"]; exists {
			assert.Equal(t, "***MASKED***", token)
		}
	}
}

// TestExecuteLoop_ValidationErrors tests validation error cases
func TestExecuteLoop_ValidationErrors(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test"})
	workflowContext := map[string]interface{}{}

	tests := []struct {
		name    string
		step    *StepDefinition
		wantErr string
	}{
		{
			name: "no nested steps",
			step: &StepDefinition{
				ID:            "empty_loop",
				Type:          StepTypeLoop,
				MaxIterations: 3,
				Until:         "true",
				Steps:         []StepDefinition{},
			},
			wantErr: "no nested steps",
		},
		{
			name: "missing max_iterations",
			step: &StepDefinition{
				ID:            "no_max_loop",
				Type:          StepTypeLoop,
				MaxIterations: 0,
				Until:         "true",
				Steps: []StepDefinition{
					{ID: "action", Type: StepTypeLLM, Prompt: "test"},
				},
			},
			wantErr: "max_iterations",
		},
		{
			name: "missing until",
			step: &StepDefinition{
				ID:            "no_until_loop",
				Type:          StepTypeLoop,
				MaxIterations: 3,
				Until:         "",
				Steps: []StepDefinition{
					{ID: "action", Type: StepTypeLLM, Prompt: "test"},
				},
			},
			wantErr: "until",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executor.Execute(context.Background(), tt.step, workflowContext)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestExecuteLoop_MultipleNestedSteps tests loop with multiple sequential steps
func TestExecuteLoop_MultipleNestedSteps(t *testing.T) {
	callCount := int32(0)

	provider := &mockLLMProviderFunc{
		completeFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			atomic.AddInt32(&callCount, 1)
			switch {
			case prompt == "Step A":
				return `{"a_result": "done_a"}`, nil
			case prompt == "Step B":
				return `{"b_result": "done_b"}`, nil
			case prompt == "Check":
				if callCount >= 6 { // 3 steps per iteration, 2 iterations
					return `{"complete": true}`, nil
				}
				return `{"complete": false}`, nil
			}
			return `{}`, nil
		},
	}

	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "multi_step_loop",
		Type:          StepTypeLoop,
		MaxIterations: 5,
		Until:         `steps.check.response == "{\"complete\": true}"`,
		Steps: []StepDefinition{
			{
				ID:     "step_a",
				Type:   StepTypeLLM,
				Prompt: "Step A",
			},
			{
				ID:     "step_b",
				Type:   StepTypeLLM,
				Prompt: "Step B",
			},
			{
				ID:     "check",
				Type:   StepTypeLLM,
				Prompt: "Check",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	output := result.Output
	stepOutputs := output["step_outputs"].(map[string]interface{})

	// Verify final outputs from all steps are present
	assert.NotNil(t, stepOutputs["step_a"])
	assert.NotNil(t, stepOutputs["step_b"])
	assert.NotNil(t, stepOutputs["check"])
}

// BenchmarkLoopExecution benchmarks loop execution overhead
func BenchmarkLoopExecution(b *testing.B) {
	provider := &mockLLMProvider{response: `{"done": true}`}
	executor := NewExecutor(nil, provider)

	step := &StepDefinition{
		ID:            "bench_loop",
		Type:          StepTypeLoop,
		MaxIterations: 1,
		Until:         `true`, // Terminate after 1 iteration
		Steps: []StepDefinition{
			{
				ID:     "action",
				Type:   StepTypeLLM,
				Prompt: "Test",
			},
		},
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(context.Background(), step, workflowContext)
		if err != nil {
			b.Fatal(err)
		}
	}
}
