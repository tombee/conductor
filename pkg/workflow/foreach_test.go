package workflow

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestExecutor_Foreach(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	executor := NewStepExecutor(nil, nil).WithLogger(logger)

	tests := []struct {
		name           string
		step           *StepDefinition
		workflowCtx    map[string]interface{}
		wantErr        bool
		checkResults   func(t *testing.T, output map[string]interface{})
		errContains    string
	}{
		{
			name: "basic foreach with array",
			step: &StepDefinition{
				ID:   "test_foreach",
				Type: StepTypeParallel,
				Foreach: "{{.steps.data_step.response}}",
				Steps: []StepDefinition{
					{
						ID:   "process",
						Type: StepTypeCondition,
						Condition: &ConditionDefinition{
							Expression: "true", // Always succeeds
						},
					},
				},
			},
			workflowCtx: map[string]interface{}{
				"_templateContext": &TemplateContext{
					Steps: map[string]map[string]interface{}{
						"data_step": {
							"response": []interface{}{"a", "b", "c"},
						},
					},
				},
			},
			wantErr: false,
			checkResults: func(t *testing.T, output map[string]interface{}) {
				results, ok := output["results"].([]interface{})
				if !ok {
					t.Errorf("expected results to be array, got %T", output["results"])
					return
				}
				if len(results) != 3 {
					t.Errorf("expected 3 results, got %d", len(results))
				}
			},
		},
		{
			name: "foreach with empty array",
			step: &StepDefinition{
				ID:   "test_foreach",
				Type: StepTypeParallel,
				Foreach: "{{.steps.data_step.response}}",
				Steps: []StepDefinition{
					{
						ID:   "process",
						Type: StepTypeLLM,
						Prompt: "Item: {{.item}}",
					},
				},
			},
			workflowCtx: map[string]interface{}{
				"_templateContext": &TemplateContext{
					Steps: map[string]map[string]interface{}{
						"data_step": {
							"response": []interface{}{},
						},
					},
				},
			},
			wantErr: false,
			checkResults: func(t *testing.T, output map[string]interface{}) {
				results, ok := output["results"].([]interface{})
				if !ok {
					t.Errorf("expected results to be array, got %T", output["results"])
					return
				}
				if len(results) != 0 {
					t.Errorf("expected 0 results for empty array, got %d", len(results))
				}
			},
		},
		{
			name: "foreach with non-array input (object)",
			step: &StepDefinition{
				ID:   "test_foreach",
				Type: StepTypeParallel,
				Foreach: "{{.steps.data_step.response}}",
				Steps: []StepDefinition{
					{
						ID:   "process",
						Type: StepTypeLLM,
						Prompt: "Item: {{.item}}",
					},
				},
			},
			workflowCtx: map[string]interface{}{
				"_templateContext": &TemplateContext{
					Steps: map[string]map[string]interface{}{
						"data_step": {
							"response": map[string]interface{}{"key": "value"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "foreach requires array input, got object",
		},
		{
			name: "foreach with non-array input (string)",
			step: &StepDefinition{
				ID:   "test_foreach",
				Type: StepTypeParallel,
				Foreach: "{{.steps.data_step.response}}",
				Steps: []StepDefinition{
					{
						ID:   "process",
						Type: StepTypeLLM,
						Prompt: "Item: {{.item}}",
					},
				},
			},
			workflowCtx: map[string]interface{}{
				"_templateContext": &TemplateContext{
					Steps: map[string]map[string]interface{}{
						"data_step": {
							"response": "not an array",
						},
					},
				},
			},
			wantErr:     true,
			errContains: "foreach requires array input, got string",
		},
		{
			name: "foreach with null input",
			step: &StepDefinition{
				ID:   "test_foreach",
				Type: StepTypeParallel,
				Foreach: "{{.steps.data_step.response}}",
				Steps: []StepDefinition{
					{
						ID:   "process",
						Type: StepTypeLLM,
						Prompt: "Item: {{.item}}",
					},
				},
			},
			workflowCtx: map[string]interface{}{
				"_templateContext": &TemplateContext{
					Steps: map[string]map[string]interface{}{
						"data_step": {
							"response": nil,
						},
					},
				},
			},
			wantErr:     true,
			errContains: "foreach requires array input, got null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			output, err := executor.executeForeach(ctx, tt.step, nil, tt.workflowCtx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errContains != "" {
					if !strings.Contains(err.Error(), tt.errContains) {
						t.Errorf("error = %v, want error containing %q", err, tt.errContains)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkResults != nil {
				tt.checkResults(t, output)
			}
		})
	}
}
