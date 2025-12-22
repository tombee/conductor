package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: mockLLMProvider is defined in executor_test.go

func TestExecute_ConditionTrue(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	step := &StepDefinition{
		ID:   "test_step",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `"security" in inputs.personas`,
		},
		Prompt: "Test prompt",
	}

	workflowContext := map[string]interface{}{
		"inputs": map[string]interface{}{
			"personas": []interface{}{"security", "performance"},
		},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)
	assert.True(t, result.Status == StepStatusSuccess)
	assert.NotNil(t, result.Output)
}

func TestExecute_ConditionFalse_StepSkipped(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "should not run"})

	step := &StepDefinition{
		ID:   "test_step",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `"style" in inputs.personas`, // "style" not in list
		},
		Prompt: "Test prompt",
	}

	workflowContext := map[string]interface{}{
		"inputs": map[string]interface{}{
			"personas": []interface{}{"security", "performance"},
		},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSkipped, result.Status)
	// StepStatusSkipped is not a failure - step was intentionally skipped
	assert.Equal(t, true, result.Output["skipped"])
	assert.Equal(t, "condition evaluated to false", result.Output["reason"])
}

func TestExecute_NoCondition_StepRuns(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	step := &StepDefinition{
		ID:     "test_step",
		Type:   StepTypeLLM,
		Prompt: "Test prompt",
		// No condition - should always run
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)
	assert.True(t, result.Status == StepStatusSuccess)
}

func TestExecute_ConditionWithHasFunction(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	step := &StepDefinition{
		ID:   "test_step",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `has(inputs.tags, "go")`,
		},
		Prompt: "Test prompt",
	}

	workflowContext := map[string]interface{}{
		"inputs": map[string]interface{}{
			"tags": []interface{}{"go", "cli", "workflow"},
		},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)
}

func TestExecute_ConditionWithEquality(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	tests := []struct {
		name       string
		expression string
		inputs     map[string]interface{}
		wantStatus StepStatus
	}{
		{
			name:       "string equality true",
			expression: `inputs.mode == "strict"`,
			inputs:     map[string]interface{}{"mode": "strict"},
			wantStatus: StepStatusSuccess,
		},
		{
			name:       "string equality false",
			expression: `inputs.mode == "strict"`,
			inputs:     map[string]interface{}{"mode": "relaxed"},
			wantStatus: StepStatusSkipped,
		},
		{
			name:       "number comparison true",
			expression: `inputs.count > 5`,
			inputs:     map[string]interface{}{"count": 10},
			wantStatus: StepStatusSuccess,
		},
		{
			name:       "number comparison false",
			expression: `inputs.count > 5`,
			inputs:     map[string]interface{}{"count": 3},
			wantStatus: StepStatusSkipped,
		},
		{
			name:       "boolean logic",
			expression: `inputs.enabled && !inputs.disabled`,
			inputs:     map[string]interface{}{"enabled": true, "disabled": false},
			wantStatus: StepStatusSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &StepDefinition{
				ID:   "test_step",
				Type: StepTypeLLM,
				Condition: &ConditionDefinition{
					Expression: tt.expression,
				},
				Prompt: "Test prompt",
			}

			workflowContext := map[string]interface{}{
				"inputs": tt.inputs,
			}

			result, err := executor.Execute(context.Background(), step, workflowContext)
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, result.Status, "expression: %s", tt.expression)
		})
	}
}

func TestExecute_ConditionWithStepReferences(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	step := &StepDefinition{
		ID:   "analyze_step",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `steps.fetch.status == "success"`,
		},
		Prompt: "Analyze the fetched data",
	}

	workflowContext := map[string]interface{}{
		"inputs": map[string]interface{}{},
		"steps": map[string]interface{}{
			"fetch": map[string]interface{}{
				"status":  "success",
				"content": "fetched data",
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)
}

func TestExecute_ConditionError(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	step := &StepDefinition{
		ID:   "test_step",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `invalid syntax ==`, // Invalid expression
		},
		Prompt: "Test prompt",
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.Error(t, err)

	assert.Equal(t, StepStatusFailed, result.Status)
	assert.False(t, result.Status == StepStatusSuccess)
	assert.Contains(t, err.Error(), "evaluate condition")
}

func TestExecute_EmptyConditionExpression(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "test response"})

	step := &StepDefinition{
		ID:   "test_step",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: "", // Empty expression should run
		},
		Prompt: "Test prompt",
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)
}

func TestExecute_SkippedStepOutput(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "should not run"})

	step := &StepDefinition{
		ID:   "skipped_step",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `false`, // Always false
		},
		Prompt: "Test prompt",
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	// Verify skipped step has proper output structure
	assert.Equal(t, StepStatusSkipped, result.Status)
	assert.Equal(t, "", result.Output["content"])
	assert.Equal(t, true, result.Output["skipped"])
	assert.NotEmpty(t, result.Output["reason"])
}
