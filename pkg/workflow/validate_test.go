package workflow

import (
	"testing"
)

func TestValidateExpressionInjection(t *testing.T) {
	tests := []struct {
		name    string
		step    *StepDefinition
		wantErr bool
	}{
		{
			name: "safe literal expression",
			step: &StepDefinition{
				ID:   "test",
				Type: StepTypeBuiltin,
				Inputs: map[string]interface{}{
					"expr": ".items[0].name",
				},
			},
			wantErr: false,
		},
		{
			name: "safe expression with functions",
			step: &StepDefinition{
				ID:   "test",
				Type: StepTypeBuiltin,
				Inputs: map[string]interface{}{
					"expr": "map(.price) | add",
				},
			},
			wantErr: false,
		},
		{
			name: "injection attempt with template expression",
			step: &StepDefinition{
				ID:   "test",
				Type: StepTypeBuiltin,
				Inputs: map[string]interface{}{
					"expr": ".items | map({{.malicious}})",
				},
			},
			wantErr: true,
		},
		{
			name: "injection attempt with workflow variable",
			step: &StepDefinition{
				ID:   "test",
				Type: StepTypeBuiltin,
				Inputs: map[string]interface{}{
					"expr": "{{.steps.user_input.output}}",
				},
			},
			wantErr: true,
		},
		{
			name: "step without expr field",
			step: &StepDefinition{
				ID:   "test",
				Type: StepTypeBuiltin,
				Inputs: map[string]interface{}{
					"data": "some data",
				},
			},
			wantErr: false,
		},
		{
			name: "step without inputs",
			step: &StepDefinition{
				ID:   "test",
				Type: StepTypeBuiltin,
			},
			wantErr: false,
		},
		{
			name: "non-string expr (handled elsewhere)",
			step: &StepDefinition{
				ID:   "test",
				Type: StepTypeBuiltin,
				Inputs: map[string]interface{}{
					"expr": 123,
				},
			},
			wantErr: false, // Type validation happens elsewhere
		},
		{
			name: "expression with only opening brackets",
			step: &StepDefinition{
				ID:   "test",
				Type: StepTypeBuiltin,
				Inputs: map[string]interface{}{
					"expr": ".items | select(.name == \"{{\")",
				},
			},
			wantErr: false, // Only rejected if both {{ and }} present
		},
		{
			name: "expression with only closing brackets",
			step: &StepDefinition{
				ID:   "test",
				Type: StepTypeBuiltin,
				Inputs: map[string]interface{}{
					"expr": ".items | select(.name == \"}}\")",
				},
			},
			wantErr: false, // Only rejected if both {{ and }} present
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExpressionInjection(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExpressionInjection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNestedForeach(t *testing.T) {
	tests := []struct {
		name    string
		step    *StepDefinition
		wantErr bool
	}{
		{
			name: "simple parallel without foreach",
			step: &StepDefinition{
				ID:   "parallel_step",
				Type: StepTypeParallel,
				Steps: []StepDefinition{
					{ID: "step1", Type: StepTypeLLM},
					{ID: "step2", Type: StepTypeLLM},
				},
			},
			wantErr: false,
		},
		{
			name: "parallel with foreach",
			step: &StepDefinition{
				ID:      "parallel_step",
				Type:    StepTypeParallel,
				Foreach: "{{.items}}",
				Steps: []StepDefinition{
					{ID: "step1", Type: StepTypeLLM},
				},
			},
			wantErr: false,
		},
		{
			name: "nested parallel with foreach inside foreach",
			step: &StepDefinition{
				ID:      "outer",
				Type:    StepTypeParallel,
				Foreach: "{{.items}}",
				Steps: []StepDefinition{
					{
						ID:      "inner",
						Type:    StepTypeParallel,
						Foreach: "{{.item.subitems}}",
						Steps: []StepDefinition{
							{ID: "step1", Type: StepTypeLLM},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "deeply nested foreach",
			step: &StepDefinition{
				ID:      "outer",
				Type:    StepTypeParallel,
				Foreach: "{{.items}}",
				Steps: []StepDefinition{
					{
						ID:   "middle",
						Type: StepTypeParallel,
						Steps: []StepDefinition{
							{
								ID:      "inner",
								Type:    StepTypeParallel,
								Foreach: "{{.item.subitems}}",
								Steps: []StepDefinition{
									{ID: "step1", Type: StepTypeLLM},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "non-parallel step",
			step: &StepDefinition{
				ID:   "llm_step",
				Type: StepTypeLLM,
			},
			wantErr: false,
		},
		{
			name: "parallel inside parallel without foreach",
			step: &StepDefinition{
				ID:   "outer",
				Type: StepTypeParallel,
				Steps: []StepDefinition{
					{
						ID:   "inner",
						Type: StepTypeParallel,
						Steps: []StepDefinition{
							{ID: "step1", Type: StepTypeLLM},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "foreach on non-parallel step inside foreach",
			step: &StepDefinition{
				ID:      "outer",
				Type:    StepTypeParallel,
				Foreach: "{{.items}}",
				Steps: []StepDefinition{
					{
						ID:      "step1",
						Type:    StepTypeLLM,
						Foreach: "{{.item.subitems}}", // This shouldn't be possible, but test validation
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNestedForeach(tt.step, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNestedForeach() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
