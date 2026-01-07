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
				Type: StepTypeIntegration,
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
				Type: StepTypeIntegration,
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
				Type: StepTypeIntegration,
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
				Type: StepTypeIntegration,
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
				Type: StepTypeIntegration,
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
				Type: StepTypeIntegration,
			},
			wantErr: false,
		},
		{
			name: "non-string expr (handled elsewhere)",
			step: &StepDefinition{
				ID:   "test",
				Type: StepTypeIntegration,
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
				Type: StepTypeIntegration,
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
				Type: StepTypeIntegration,
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

func TestValidateParallelNestingDepth(t *testing.T) {
	tests := []struct {
		name    string
		step    *StepDefinition
		wantErr bool
	}{
		{
			name: "single level parallel is valid",
			step: &StepDefinition{
				ID:   "p1",
				Type: StepTypeParallel,
				Steps: []StepDefinition{
					{ID: "s1", Type: StepTypeLLM},
					{ID: "s2", Type: StepTypeLLM},
				},
			},
			wantErr: false,
		},
		{
			name: "two levels parallel is valid",
			step: &StepDefinition{
				ID:   "p1",
				Type: StepTypeParallel,
				Steps: []StepDefinition{
					{
						ID:   "p2",
						Type: StepTypeParallel,
						Steps: []StepDefinition{
							{ID: "s1", Type: StepTypeLLM},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "three levels parallel is valid (at max)",
			step: &StepDefinition{
				ID:   "p1",
				Type: StepTypeParallel,
				Steps: []StepDefinition{
					{
						ID:   "p2",
						Type: StepTypeParallel,
						Steps: []StepDefinition{
							{
								ID:   "p3",
								Type: StepTypeParallel,
								Steps: []StepDefinition{
									{ID: "s1", Type: StepTypeLLM},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "four levels parallel exceeds max",
			step: &StepDefinition{
				ID:   "p1",
				Type: StepTypeParallel,
				Steps: []StepDefinition{
					{
						ID:   "p2",
						Type: StepTypeParallel,
						Steps: []StepDefinition{
							{
								ID:   "p3",
								Type: StepTypeParallel,
								Steps: []StepDefinition{
									{
										ID:   "p4",
										Type: StepTypeParallel,
										Steps: []StepDefinition{
											{ID: "s1", Type: StepTypeLLM},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "parallel inside loop is tracked",
			step: &StepDefinition{
				ID:            "loop1",
				Type:          StepTypeLoop,
				MaxIterations: 5,
				Until:         "true",
				Steps: []StepDefinition{
					{
						ID:   "p1",
						Type: StepTypeParallel,
						Steps: []StepDefinition{
							{
								ID:   "p2",
								Type: StepTypeParallel,
								Steps: []StepDefinition{
									{
										ID:   "p3",
										Type: StepTypeParallel,
										Steps: []StepDefinition{
											{
												ID:   "p4",
												Type: StepTypeParallel,
												Steps: []StepDefinition{
													{ID: "s1", Type: StepTypeLLM},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParallelNestingDepth(tt.step, 0)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateParallelNestingDepth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMaxConcurrency(t *testing.T) {
	tests := []struct {
		name           string
		maxConcurrency int
		wantErr        bool
	}{
		{
			name:           "zero is valid (uses default)",
			maxConcurrency: 0,
			wantErr:        false,
		},
		{
			name:           "positive value is valid",
			maxConcurrency: 5,
			wantErr:        false,
		},
		{
			name:           "max value (100) is valid",
			maxConcurrency: 100,
			wantErr:        false,
		},
		{
			name:           "negative is invalid",
			maxConcurrency: -1,
			wantErr:        true,
		},
		{
			name:           "exceeds max (101) is invalid",
			maxConcurrency: 101,
			wantErr:        true,
		},
		{
			name:           "way exceeds max is invalid",
			maxConcurrency: 1000,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &StepDefinition{
				ID:             "test",
				Type:           StepTypeParallel,
				MaxConcurrency: tt.maxConcurrency,
			}
			err := ValidateMaxConcurrency(step)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMaxConcurrency() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateForeachArraySize(t *testing.T) {
	tests := []struct {
		name     string
		arrayLen int
		wantErr  bool
	}{
		{
			name:     "small array is valid",
			arrayLen: 10,
			wantErr:  false,
		},
		{
			name:     "medium array is valid",
			arrayLen: 1000,
			wantErr:  false,
		},
		{
			name:     "at limit is valid",
			arrayLen: 10000,
			wantErr:  false,
		},
		{
			name:     "exceeds limit is invalid",
			arrayLen: 10001,
			wantErr:  true,
		},
		{
			name:     "way exceeds limit is invalid",
			arrayLen: 100000,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateForeachArraySize(tt.arrayLen, "test_step")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateForeachArraySize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
