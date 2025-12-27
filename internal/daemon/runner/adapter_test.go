// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

// MockLLMProvider implements workflow.LLMProvider for testing.
type MockLLMProvider struct {
	CompleteFunc func(ctx context.Context, prompt string, options map[string]interface{}) (string, error)
}

func (m *MockLLMProvider) Complete(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, prompt, options)
	}
	return "mock response", nil
}

func TestNewStepExecutorAdapter(t *testing.T) {
	executor := workflow.NewStepExecutor(nil, &MockLLMProvider{})
	adapter := NewStepExecutorAdapter(executor)

	if adapter == nil {
		t.Fatal("NewStepExecutorAdapter returned nil")
	}
	if adapter.executor != executor {
		t.Error("adapter.executor does not match provided executor")
	}
}

func TestStepExecutorAdapter_ExecuteWorkflow_EmptyWorkflow(t *testing.T) {
	executor := workflow.NewStepExecutor(nil, &MockLLMProvider{})
	adapter := NewStepExecutorAdapter(executor)

	def := &workflow.Definition{
		Name:  "test-workflow",
		Steps: []workflow.StepDefinition{},
	}

	ctx := context.Background()
	result, err := adapter.ExecuteWorkflow(ctx, def, nil, ExecutionOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(result.Steps))
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestStepExecutorAdapter_ExecuteWorkflow_SingleStep(t *testing.T) {
	provider := &MockLLMProvider{
		CompleteFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			return "test response", nil
		},
	}
	executor := workflow.NewStepExecutor(nil, provider)
	adapter := NewStepExecutorAdapter(executor)

	def := &workflow.Definition{
		Name: "test-workflow",
		Steps: []workflow.StepDefinition{
			{
				ID:     "step1",
				Name:   "Test Step",
				Type:   workflow.StepTypeLLM,
				Prompt: "Hello",
			},
		},
	}

	ctx := context.Background()
	opts := ExecutionOptions{
		RunID: "test-run-1",
	}

	result, err := adapter.ExecuteWorkflow(ctx, def, map[string]any{"input1": "value1"}, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(result.Steps))
	}
	if result.Steps[0].StepID != "step1" {
		t.Errorf("expected step ID 'step1', got '%s'", result.Steps[0].StepID)
	}
}

func TestStepExecutorAdapter_ExecuteWorkflow_Callbacks(t *testing.T) {
	provider := &MockLLMProvider{
		CompleteFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			return "response", nil
		},
	}
	executor := workflow.NewStepExecutor(nil, provider)
	adapter := NewStepExecutorAdapter(executor)

	def := &workflow.Definition{
		Name: "test-workflow",
		Steps: []workflow.StepDefinition{
			{ID: "step1", Type: workflow.StepTypeLLM, Prompt: "test1"},
			{ID: "step2", Type: workflow.StepTypeLLM, Prompt: "test2"},
		},
	}

	var startCalls []string
	var endCalls []string
	var logMessages []string

	opts := ExecutionOptions{
		RunID: "callback-test",
		OnStepStart: func(stepID string, stepIndex int, total int) {
			startCalls = append(startCalls, stepID)
		},
		OnStepEnd: func(stepID string, result *workflow.StepResult, err error) {
			endCalls = append(endCalls, stepID)
		},
		OnLog: func(level, message, stepID string) {
			logMessages = append(logMessages, message)
		},
	}

	ctx := context.Background()
	_, err := adapter.ExecuteWorkflow(ctx, def, nil, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(startCalls) != 2 {
		t.Errorf("expected 2 start calls, got %d", len(startCalls))
	}
	if len(endCalls) != 2 {
		t.Errorf("expected 2 end calls, got %d", len(endCalls))
	}
	if startCalls[0] != "step1" || startCalls[1] != "step2" {
		t.Errorf("unexpected start call order: %v", startCalls)
	}
	if len(logMessages) == 0 {
		t.Error("expected log messages to be captured")
	}
}

func TestStepExecutorAdapter_ExecuteWorkflow_Cancellation(t *testing.T) {
	provider := &MockLLMProvider{
		CompleteFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			// Simulate slow execution
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return "response", nil
			}
		},
	}
	executor := workflow.NewStepExecutor(nil, provider)
	adapter := NewStepExecutorAdapter(executor)

	def := &workflow.Definition{
		Name: "test-workflow",
		Steps: []workflow.StepDefinition{
			{ID: "step1", Type: workflow.StepTypeLLM, Prompt: "test"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := adapter.ExecuteWorkflow(ctx, def, nil, ExecutionOptions{})

	if err == nil {
		t.Error("expected error from cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected result even on error")
	}
	if result.FinalError == nil {
		t.Error("expected FinalError to be set")
	}
}

func TestStepExecutorAdapter_ExecuteWorkflow_StepFailure(t *testing.T) {
	stepError := errors.New("step execution failed")
	provider := &MockLLMProvider{
		CompleteFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			return "", stepError
		},
	}
	executor := workflow.NewStepExecutor(nil, provider)
	adapter := NewStepExecutorAdapter(executor)

	def := &workflow.Definition{
		Name: "test-workflow",
		Steps: []workflow.StepDefinition{
			{ID: "step1", Type: workflow.StepTypeLLM, Prompt: "test"},
			{ID: "step2", Type: workflow.StepTypeLLM, Prompt: "test2"},
		},
	}

	ctx := context.Background()
	result, err := adapter.ExecuteWorkflow(ctx, def, nil, ExecutionOptions{})

	if err == nil {
		t.Error("expected error from step failure")
	}
	if result == nil {
		t.Fatal("expected result even on error")
	}
	if result.FinalError == nil {
		t.Error("expected FinalError to be set")
	}
	// Step2 should not have been executed
	if len(result.Steps) > 1 {
		t.Errorf("expected at most 1 step result (failed step), got %d", len(result.Steps))
	}
}

func TestStepExecutorAdapter_ExecuteWorkflow_ErrorStrategyIgnore(t *testing.T) {
	step1Called := false
	step2Called := false
	provider := &MockLLMProvider{
		CompleteFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			if prompt == "test1" {
				step1Called = true
				return "", errors.New("first step error")
			}
			if prompt == "test2" {
				step2Called = true
				return "success", nil
			}
			return "success", nil
		},
	}
	executor := workflow.NewStepExecutor(nil, provider)
	adapter := NewStepExecutorAdapter(executor)

	def := &workflow.Definition{
		Name: "test-workflow",
		Steps: []workflow.StepDefinition{
			{
				ID:      "step1",
				Type:    workflow.StepTypeLLM,
				Prompt:  "test1",
				OnError: &workflow.ErrorHandlingDefinition{Strategy: workflow.ErrorStrategyIgnore},
			},
			{ID: "step2", Type: workflow.StepTypeLLM, Prompt: "test2"},
		},
	}

	ctx := context.Background()
	result, err := adapter.ExecuteWorkflow(ctx, def, nil, ExecutionOptions{})

	if err != nil {
		t.Fatalf("expected no error with ignore strategy, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if !step1Called {
		t.Error("expected step1 to be called")
	}
	if !step2Called {
		t.Error("expected step2 to be called (error should be ignored)")
	}
}

func TestStepExecutorAdapter_ExecuteWorkflow_StepOutputs(t *testing.T) {
	provider := &MockLLMProvider{
		CompleteFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			return "step output", nil
		},
	}
	executor := workflow.NewStepExecutor(nil, provider)
	adapter := NewStepExecutorAdapter(executor)

	def := &workflow.Definition{
		Name: "test-workflow",
		Steps: []workflow.StepDefinition{
			{ID: "step1", Type: workflow.StepTypeLLM, Prompt: "test1"},
		},
	}

	ctx := context.Background()
	result, err := adapter.ExecuteWorkflow(ctx, def, nil, ExecutionOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StepOutputs == nil {
		t.Fatal("expected StepOutputs to be populated")
	}
	if _, ok := result.StepOutputs["step1"]; !ok {
		t.Error("expected step1 output in StepOutputs")
	}
}

func TestMockExecutionAdapter_ExecuteWorkflow(t *testing.T) {
	mock := &MockExecutionAdapter{
		ExecuteWorkflowFunc: func(ctx context.Context, def *workflow.Definition, inputs map[string]any, opts ExecutionOptions) (*ExecutionResult, error) {
			return &ExecutionResult{
				StepOutput: &workflow.StepOutput{Data: map[string]any{"result": "custom"}},
				Duration:   time.Second,
			}, nil
		},
	}

	def := &workflow.Definition{Name: "test"}
	inputs := map[string]any{"key": "value"}
	opts := ExecutionOptions{RunID: "test-123"}

	result, err := mock.ExecuteWorkflow(context.Background(), def, inputs, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StepOutput == nil || result.StepOutput.Data == nil {
		t.Error("expected StepOutput with data")
	}

	// Verify call was recorded
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.Calls))
	}
	if mock.Calls[0].Def != def {
		t.Error("call def doesn't match")
	}
	if mock.Calls[0].Inputs["key"] != "value" {
		t.Error("call inputs don't match")
	}
	if mock.Calls[0].Opts.RunID != "test-123" {
		t.Error("call opts don't match")
	}
}

func TestMockExecutionAdapter_DefaultBehavior(t *testing.T) {
	mock := &MockExecutionAdapter{}

	result, err := mock.ExecuteWorkflow(context.Background(), &workflow.Definition{}, nil, ExecutionOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected default result")
	}
}

// TestStepExecutorAdapter_TypedOutput verifies that typed StepOutput is populated correctly (SPEC-40).
func TestStepExecutorAdapter_TypedOutput(t *testing.T) {
	provider := &MockLLMProvider{
		CompleteFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			return "typed response", nil
		},
	}
	executor := workflow.NewStepExecutor(nil, provider)
	adapter := NewStepExecutorAdapter(executor)

	def := &workflow.Definition{
		Name: "typed-workflow",
		Steps: []workflow.StepDefinition{
			{ID: "step1", Type: workflow.StepTypeLLM, Prompt: "test"},
		},
	}

	ctx := context.Background()
	result, err := adapter.ExecuteWorkflow(ctx, def, nil, ExecutionOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	// Verify StepOutput is populated
	if result.StepOutput == nil {
		t.Fatal("expected StepOutput to be populated")
	}
	if result.StepOutput.Text == "" {
		t.Error("expected StepOutput.Text to be set")
	}
}

// TestStepExecutorAdapter_TypedInputsOutputs tests end-to-end workflow with typed inputs/outputs (SPEC-40).
func TestStepExecutorAdapter_TypedInputsOutputs(t *testing.T) {
	provider := &MockLLMProvider{
		CompleteFunc: func(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
			return "response from step", nil
		},
	}
	executor := workflow.NewStepExecutor(nil, provider)
	adapter := NewStepExecutorAdapter(executor)

	def := &workflow.Definition{
		Name: "multi-step-typed",
		Steps: []workflow.StepDefinition{
			{ID: "step1", Type: workflow.StepTypeLLM, Prompt: "first step"},
			{ID: "step2", Type: workflow.StepTypeLLM, Prompt: "second step"},
		},
	}

	inputs := map[string]any{
		"text_input":   "hello world",
		"number_input": 42,
		"bool_input":   true,
	}

	ctx := context.Background()
	result, err := adapter.ExecuteWorkflow(ctx, def, inputs, ExecutionOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both steps executed
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}

	// Verify typed StepOutput is available
	if result.StepOutput == nil {
		t.Fatal("expected StepOutput to be populated")
	}

	// Verify step outputs are tracked
	if len(result.StepOutputs) != 2 {
		t.Errorf("expected 2 step outputs, got %d", len(result.StepOutputs))
	}
	if _, ok := result.StepOutputs["step1"]; !ok {
		t.Error("expected step1 output")
	}
	if _, ok := result.StepOutputs["step2"]; !ok {
		t.Error("expected step2 output")
	}

	// Verify metadata is present
	if result.StepOutput.Metadata.Duration <= 0 {
		t.Error("expected positive duration in metadata")
	}
}

// TestOutputConversionHelpers tests the conversion functions between typed and untyped formats (SPEC-40).
func TestOutputConversionHelpers(t *testing.T) {
	t.Run("stepResultToOutput", func(t *testing.T) {
		stepResult := &workflow.StepResult{
			StepID:   "test-step",
			Status:   workflow.StepStatusSuccess,
			Output:   map[string]interface{}{"text": "hello", "data": 123},
			Error:    "",
			Duration: 500 * time.Millisecond,
		}

		output := stepResultToOutput(stepResult)

		if output.Text != "hello" {
			t.Errorf("expected text 'hello', got '%s'", output.Text)
		}
		if output.Error != "" {
			t.Errorf("expected no error, got '%s'", output.Error)
		}
		if output.Metadata.Duration != 500*time.Millisecond {
			t.Errorf("expected duration 500ms, got %v", output.Metadata.Duration)
		}
		if output.Data == nil {
			t.Error("expected Data to be populated")
		}
	})

	t.Run("stepResultToOutput with error", func(t *testing.T) {
		stepResult := &workflow.StepResult{
			StepID:   "failed-step",
			Status:   workflow.StepStatusFailed,
			Error:    "step failed",
			Duration: 100 * time.Millisecond,
		}

		output := stepResultToOutput(stepResult)

		if output.Error != "step failed" {
			t.Errorf("expected error 'step failed', got '%s'", output.Error)
		}
	})

	t.Run("stepResultToOutput nil input", func(t *testing.T) {
		output := stepResultToOutput(nil)

		if output.Text != "" || output.Error != "" {
			t.Error("expected empty StepOutput for nil input")
		}
	})

}
