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

	// Verify skipped step has proper output structure matching spec FR4
	assert.Equal(t, StepStatusSkipped, result.Status)
	assert.Equal(t, true, result.Output["skipped"])
	assert.Equal(t, "condition evaluated to false", result.Output["reason"])
	assert.Equal(t, "", result.Output["stdout"])
	assert.Equal(t, "", result.Output["stderr"])
	assert.Equal(t, 0, result.Output["exit_code"])
	assert.Equal(t, "", result.Output["content"])
	assert.Equal(t, "skipped", result.Output["status"])
}

// TestExecute_ConditionWithTemplateExpression tests Phase 3: template syntax in conditions
func TestExecute_ConditionWithTemplateExpression(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "analysis complete"})

	step := &StepDefinition{
		ID:   "analyze",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `{{.steps.check.stdout}} == "success"`,
		},
		Prompt: "Analyze the data",
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{
			"check": map[string]interface{}{
				"stdout": "success",
				"stderr": "",
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)
	assert.Contains(t, result.Output["response"], "analysis complete")
}

// TestExecute_ConditionWithTemplateExpression_False tests skipped step with template
func TestExecute_ConditionWithTemplateExpression_False(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "should not run"})

	step := &StepDefinition{
		ID:   "analyze",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `{{.steps.check.stdout}} == "success"`,
		},
		Prompt: "Analyze the data",
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{
			"check": map[string]interface{}{
				"stdout": "failed",
				"stderr": "error occurred",
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSkipped, result.Status)
	assert.Equal(t, true, result.Output["skipped"])
}

// TestExecute_SkippedStepReferencedByDownstream verifies downstream steps can reference skipped steps
func TestExecute_SkippedStepReferencedByDownstream(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "using skipped output"})

	// First step gets skipped
	step1 := &StepDefinition{
		ID:   "optional_step",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `false`, // Always skip
		},
		Prompt: "This won't run",
	}

	workflowContext := map[string]interface{}{
		"inputs": map[string]interface{}{},
	}

	result1, err := executor.Execute(context.Background(), step1, workflowContext)
	require.NoError(t, err)
	assert.Equal(t, StepStatusSkipped, result1.Status)

	// Add skipped step output to context
	workflowContext["steps"] = map[string]interface{}{
		"optional_step": result1.Output,
	}

	// Second step references the skipped step using template syntax
	step2 := &StepDefinition{
		ID:   "next_step",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `{{.steps.optional_step.status}} == "skipped"`,
		},
		Prompt: "Handle skipped case",
	}

	result2, err := executor.Execute(context.Background(), step2, workflowContext)
	require.NoError(t, err)

	// This step should run because the condition evaluates to true
	assert.Equal(t, StepStatusSuccess, result2.Status)
	assert.Contains(t, result2.Output["response"], "using skipped output")
}

// TestExecute_ChainedConditionsWithTemplates tests multiple conditions with template expressions
func TestExecute_ChainedConditionsWithTemplates(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "final step"})

	// First step succeeds
	step1 := &StepDefinition{
		ID:     "step1",
		Type:   StepTypeLLM,
		Prompt: "First step",
	}

	workflowContext := map[string]interface{}{
		"inputs": map[string]interface{}{},
	}

	result1, err := executor.Execute(context.Background(), step1, workflowContext)
	require.NoError(t, err)
	assert.Equal(t, StepStatusSuccess, result1.Status)

	// Second step checks first step's status
	workflowContext["steps"] = map[string]interface{}{
		"step1": map[string]interface{}{
			"response": result1.Output["response"],
			"status":   "success",
		},
	}

	step2 := &StepDefinition{
		ID:   "step2",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `{{.steps.step1.status}} == "success"`,
		},
		Prompt: "Second step",
	}

	result2, err := executor.Execute(context.Background(), step2, workflowContext)
	require.NoError(t, err)
	assert.Equal(t, StepStatusSuccess, result2.Status)

	// Third step checks both previous steps using complex condition
	workflowContext["steps"].(map[string]interface{})["step2"] = map[string]interface{}{
		"response": result2.Output["response"],
		"status":   "success",
	}

	step3 := &StepDefinition{
		ID:   "step3",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `{{.steps.step1.status}} == "success" && {{.steps.step2.status}} == "success"`,
		},
		Prompt: "Final step",
	}

	result3, err := executor.Execute(context.Background(), step3, workflowContext)
	require.NoError(t, err)
	assert.Equal(t, StepStatusSuccess, result3.Status)
	assert.Contains(t, result3.Output["response"], "final step")
}

// TestExecute_SkippedStepDoesNotTriggerOnError verifies skipped steps don't invoke error handlers
func TestExecute_SkippedStepDoesNotTriggerOnError(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "should not run"})

	step := &StepDefinition{
		ID:   "conditional_step",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `false`, // Always skip
		},
		OnError: &ErrorHandlingDefinition{
			Strategy: ErrorStrategyFail,
		},
		Prompt: "This won't run",
	}

	workflowContext := map[string]interface{}{}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err) // No error should be returned

	assert.Equal(t, StepStatusSkipped, result.Status)
	assert.Empty(t, result.Error) // Error field should be empty for skipped steps
	assert.Equal(t, true, result.Output["skipped"])
}

// TestExecute_ConditionWithNestedStepFields tests template expressions accessing nested fields
func TestExecute_ConditionWithNestedStepFields(t *testing.T) {
	executor := NewExecutor(nil, &mockLLMProvider{response: "processing"})

	step := &StepDefinition{
		ID:   "process",
		Type: StepTypeLLM,
		Condition: &ConditionDefinition{
			Expression: `{{.steps.api_call.exit_code}} == 0`,
		},
		Prompt: "Process the API response",
	}

	workflowContext := map[string]interface{}{
		"steps": map[string]interface{}{
			"api_call": map[string]interface{}{
				"stdout":    "API response data",
				"stderr":    "",
				"exit_code": 0,
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)
	require.NoError(t, err)

	assert.Equal(t, StepStatusSuccess, result.Status)
}
