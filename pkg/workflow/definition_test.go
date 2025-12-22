package workflow

import (
	"testing"
)

func TestParseDefinition(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid workflow",
			yaml: `
name: test-workflow
version: "1.0"
description: A test workflow
inputs:
  - name: input1
    type: string
    required: true
steps:
  - id: step1
    name: First Step
    type: action
    action: test-tool
outputs:
  - name: result
    type: string
    value: $.step1.output
`,
			wantErr: false,
		},
		{
			name: "missing name",
			yaml: `
version: "1.0"
steps:
  - id: step1
    name: First Step
    type: action
    action: test-tool
`,
			wantErr: true,
		},
		{
			name: "missing version",
			yaml: `
name: test-workflow
steps:
  - id: step1
    name: First Step
    type: action
    action: test-tool
`,
			wantErr: true,
		},
		{
			name: "no steps",
			yaml: `
name: test-workflow
version: "1.0"
steps: []
`,
			wantErr: true,
		},
		{
			name: "duplicate step IDs",
			yaml: `
name: test-workflow
version: "1.0"
steps:
  - id: step1
    name: First Step
    type: action
    action: test-tool
  - id: step1
    name: Second Step
    type: action
    action: test-tool
`,
			wantErr: true,
		},
		{
			name: "invalid input type",
			yaml: `
name: test-workflow
version: "1.0"
inputs:
  - name: input1
    type: invalid-type
    required: true
steps:
  - id: step1
    name: First Step
    type: action
    action: test-tool
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && def == nil {
				t.Error("ParseDefinition() returned nil definition")
			}
		})
	}
}

func TestStepDefinitionValidate(t *testing.T) {
	tests := []struct {
		name    string
		step    StepDefinition
		wantErr bool
	}{
		{
			name: "valid action step",
			step: StepDefinition{
				ID:     "step1",
				Name:   "Test Step",
				Type:   StepTypeAction,
				Action: "test-tool",
			},
			wantErr: false,
		},
		{
			name: "valid LLM step",
			step: StepDefinition{
				ID:     "step1",
				Name:   "LLM Step",
				Type:   StepTypeLLM,
				Action: "complete",
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			step: StepDefinition{
				Name:   "Test Step",
				Type:   StepTypeAction,
				Action: "test-tool",
			},
			wantErr: true,
		},
		{
			name: "missing action for action step",
			step: StepDefinition{
				ID:   "step1",
				Name: "Test Step",
				Type: StepTypeAction,
			},
			wantErr: true,
		},
		{
			name: "missing condition for condition step",
			step: StepDefinition{
				ID:   "step1",
				Name: "Condition Step",
				Type: StepTypeCondition,
			},
			wantErr: true,
		},
		{
			name: "invalid step type",
			step: StepDefinition{
				ID:   "step1",
				Name: "Test Step",
				Type: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("StepDefinition.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRetryDefinitionValidate(t *testing.T) {
	tests := []struct {
		name    string
		retry   RetryDefinition
		wantErr bool
	}{
		{
			name: "valid retry",
			retry: RetryDefinition{
				MaxAttempts:       3,
				BackoffBase:       1,
				BackoffMultiplier: 2.0,
			},
			wantErr: false,
		},
		{
			name: "invalid max attempts",
			retry: RetryDefinition{
				MaxAttempts:       0,
				BackoffBase:       1,
				BackoffMultiplier: 2.0,
			},
			wantErr: true,
		},
		{
			name: "invalid backoff base",
			retry: RetryDefinition{
				MaxAttempts:       3,
				BackoffBase:       0,
				BackoffMultiplier: 2.0,
			},
			wantErr: true,
		},
		{
			name: "invalid backoff multiplier",
			retry: RetryDefinition{
				MaxAttempts:       3,
				BackoffBase:       1,
				BackoffMultiplier: 0.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.retry.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RetryDefinition.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestErrorHandlingDefinitionValidate(t *testing.T) {
	tests := []struct {
		name    string
		handler ErrorHandlingDefinition
		wantErr bool
	}{
		{
			name: "valid fail strategy",
			handler: ErrorHandlingDefinition{
				Strategy: ErrorStrategyFail,
			},
			wantErr: false,
		},
		{
			name: "valid fallback strategy",
			handler: ErrorHandlingDefinition{
				Strategy:     ErrorStrategyFallback,
				FallbackStep: "fallback-step",
			},
			wantErr: false,
		},
		{
			name: "invalid strategy",
			handler: ErrorHandlingDefinition{
				Strategy: "invalid",
			},
			wantErr: true,
		},
		{
			name: "fallback without fallback step",
			handler: ErrorHandlingDefinition{
				Strategy: ErrorStrategyFallback,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.handler.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ErrorHandlingDefinition.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
