package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStepReferences(t *testing.T) {
	knownSteps := []string{"check", "build", "test", "deploy"}

	tests := []struct {
		name       string
		expression string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid step reference in expr syntax",
			expression: `steps.check.status == "success"`,
			wantErr:    false,
		},
		{
			name:       "valid step reference in template syntax with dot",
			expression: `{{.steps.check.status}} == "success"`,
			wantErr:    false,
		},
		{
			name:       "valid step reference in template syntax without dot",
			expression: `{{steps.build.result}} == "ok"`,
			wantErr:    false,
		},
		{
			name:       "multiple valid step references",
			expression: `steps.check.status == "success" && steps.build.status == "success"`,
			wantErr:    false,
		},
		{
			name:       "mixed template and expr syntax",
			expression: `{{.steps.check.status}} == "success" && steps.build.ok`,
			wantErr:    false,
		},
		{
			name:       "no step references",
			expression: `inputs.mode == "strict"`,
			wantErr:    false,
		},
		{
			name:       "empty expression",
			expression: "",
			wantErr:    false,
		},
		{
			name:       "invalid step reference",
			expression: `steps.missing.status == "success"`,
			wantErr:    true,
			errMsg:     "unknown step(s): missing",
		},
		{
			name:       "multiple invalid step references",
			expression: `steps.missing.status == "success" && steps.invalid.ok`,
			wantErr:    true,
			errMsg:     "unknown step(s)",
		},
		{
			name:       "mix of valid and invalid",
			expression: `steps.check.status == "success" && steps.missing.ok`,
			wantErr:    true,
			errMsg:     "unknown step(s): missing",
		},
		{
			name:       "invalid step in template",
			expression: `{{.steps.missing.value}} == true`,
			wantErr:    true,
			errMsg:     "unknown step(s): missing",
		},
		{
			name:       "step-like word but not a reference",
			expression: `inputs.steps == "value"`,
			wantErr:    false,
		},
		{
			name:       "complex expression with valid steps",
			expression: `(steps.check.status == "success" && steps.build.status == "success") || steps.test.skip`,
			wantErr:    false,
		},
		{
			name:       "step ID with hyphens",
			expression: `steps.build-step.status == "success"`,
			wantErr:    true,
			errMsg:     "unknown step(s): build-step",
		},
		{
			name:       "step ID with underscores",
			expression: `steps.build_step.status == "success"`,
			wantErr:    true,
			errMsg:     "unknown step(s): build_step",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStepReferences(tt.expression, knownSteps)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateStepReferences_EmptyKnownSteps(t *testing.T) {
	// When there are no known steps, any step reference should be invalid
	err := ValidateStepReferences(`steps.check.status == "success"`, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown step(s): check")
}

func TestExtractStepReferences(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       []string
	}{
		{
			name:       "single expr syntax",
			expression: `steps.check.status == "success"`,
			want:       []string{"check"},
		},
		{
			name:       "single template syntax with dot",
			expression: `{{.steps.check.status}}`,
			want:       []string{"check"},
		},
		{
			name:       "single template syntax without dot",
			expression: `{{steps.check.status}}`,
			want:       []string{"check"},
		},
		{
			name:       "multiple different steps",
			expression: `steps.check.status == "success" && steps.build.ok`,
			want:       []string{"check", "build"},
		},
		{
			name:       "duplicate step references",
			expression: `steps.check.status == "success" && steps.check.code == 0`,
			want:       []string{"check"}, // Deduplicated
		},
		{
			name:       "mixed syntax",
			expression: `{{.steps.check.status}} && steps.build.ok`,
			want:       []string{"check", "build"},
		},
		{
			name:       "no step references",
			expression: `inputs.mode == "strict"`,
			want:       []string{},
		},
		{
			name:       "empty expression",
			expression: "",
			want:       []string{},
		},
		{
			name:       "step ID with underscores",
			expression: `steps.build_step.status`,
			want:       []string{"build_step"},
		},
		{
			name:       "step ID with hyphens",
			expression: `steps.build-step.status`,
			want:       []string{"build-step"},
		},
		{
			name:       "complex expression",
			expression: `(steps.a.x > 5 && steps.b.y < 10) || steps.c.z == true`,
			want:       []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractStepReferences(tt.expression)

			// Sort both slices for comparison since order doesn't matter
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
