package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildContext(t *testing.T) {
	tests := []struct {
		name            string
		workflowContext map[string]interface{}
		wantInputs      bool
		wantSteps       bool
	}{
		{
			name: "extracts inputs and steps",
			workflowContext: map[string]interface{}{
				"inputs": map[string]interface{}{
					"name": "test",
				},
				"steps": map[string]interface{}{
					"fetch": map[string]interface{}{
						"content": "data",
					},
				},
			},
			wantInputs: true,
			wantSteps:  true,
		},
		{
			name:            "handles empty context",
			workflowContext: map[string]interface{}{},
			wantInputs:      true, // Should have empty map
			wantSteps:       true, // Should have empty map
		},
		{
			name: "handles nil inputs",
			workflowContext: map[string]interface{}{
				"steps": map[string]interface{}{},
			},
			wantInputs: true,
			wantSteps:  true,
		},
		{
			name: "ignores internal fields",
			workflowContext: map[string]interface{}{
				"_templateContext": "should be ignored",
				"inputs":           map[string]interface{}{"x": 1},
			},
			wantInputs: true,
			wantSteps:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := BuildContext(tt.workflowContext)

			_, hasInputs := ctx["inputs"]
			assert.Equal(t, tt.wantInputs, hasInputs, "inputs presence")

			_, hasSteps := ctx["steps"]
			assert.Equal(t, tt.wantSteps, hasSteps, "steps presence")
		})
	}
}

func TestBuildContext_ValueAccess(t *testing.T) {
	workflowContext := map[string]interface{}{
		"inputs": map[string]interface{}{
			"personas": []interface{}{"security", "performance"},
			"mode":     "strict",
		},
		"steps": map[string]interface{}{
			"fetch": map[string]interface{}{
				"content": "PR diff data",
				"status":  "success",
			},
		},
	}

	ctx := BuildContext(workflowContext)

	// Check inputs are accessible
	inputs, ok := ctx["inputs"].(map[string]interface{})
	assert.True(t, ok, "inputs should be a map")
	assert.Equal(t, "strict", inputs["mode"])

	personas, ok := inputs["personas"].([]interface{})
	assert.True(t, ok, "personas should be a slice")
	assert.Len(t, personas, 2)

	// Check steps are accessible
	steps, ok := ctx["steps"].(map[string]interface{})
	assert.True(t, ok, "steps should be a map")

	fetch, ok := steps["fetch"].(map[string]interface{})
	assert.True(t, ok, "fetch should be a map")
	assert.Equal(t, "success", fetch["status"])
}

func TestBuildContextFromInputsAndSteps(t *testing.T) {
	inputs := map[string]interface{}{
		"name": "test",
	}
	steps := map[string]interface{}{
		"step1": map[string]interface{}{
			"content": "result",
		},
	}

	ctx := BuildContextFromInputsAndSteps(inputs, steps)

	assert.NotNil(t, ctx["inputs"])
	assert.NotNil(t, ctx["steps"])

	ctxInputs := ctx["inputs"].(map[string]interface{})
	assert.Equal(t, "test", ctxInputs["name"])

	ctxSteps := ctx["steps"].(map[string]interface{})
	step1 := ctxSteps["step1"].(map[string]interface{})
	assert.Equal(t, "result", step1["content"])
}

func TestBuildContextFromInputsAndSteps_NilValues(t *testing.T) {
	ctx := BuildContextFromInputsAndSteps(nil, nil)

	assert.NotNil(t, ctx["inputs"])
	assert.NotNil(t, ctx["steps"])

	// Should be empty maps, not nil
	inputs := ctx["inputs"].(map[string]interface{})
	assert.Empty(t, inputs)

	steps := ctx["steps"].(map[string]interface{})
	assert.Empty(t, steps)
}

// mockConverter is a test helper that implements StepOutputConverter
type mockConverter struct {
	data map[string]interface{}
}

func (m *mockConverter) ToMap() map[string]interface{} {
	return m.data
}

// TestBuildContextFromTypedOutputs tests the StepOutputConverter interface.
// Full integration tests with workflow.StepOutput are in pkg/workflow/types_test.go
// to avoid circular dependencies.
func TestBuildContextFromTypedOutputs(t *testing.T) {
	// Verify mockConverter implements the interface
	var _ StepOutputConverter = (*mockConverter)(nil)

	t.Run("converts step outputs using converter interface", func(t *testing.T) {
		inputs := map[string]any{
			"user":  "alice",
			"count": 42,
		}

		stepOutputs := map[string]StepOutputConverter{
			"step1": &mockConverter{
				data: map[string]interface{}{
					"text":     "step 1 result",
					"response": "step 1 result",
					"status":   "success",
				},
			},
		}

		ctx := BuildContextFromTypedOutputs(inputs, stepOutputs)

		// Verify inputs are accessible
		assert.NotNil(t, ctx["inputs"])
		ctxInputs := ctx["inputs"].(map[string]interface{})
		assert.Equal(t, "alice", ctxInputs["user"])
		assert.Equal(t, 42, ctxInputs["count"])

		// Verify steps are accessible
		assert.NotNil(t, ctx["steps"])
		ctxSteps := ctx["steps"].(map[string]interface{})
		step1 := ctxSteps["step1"].(map[string]interface{})
		assert.Equal(t, "step 1 result", step1["text"])
		assert.Equal(t, "success", step1["status"])
	})

	t.Run("handles nil step outputs", func(t *testing.T) {
		inputs := map[string]any{"key": "value"}
		ctx := BuildContextFromTypedOutputs(inputs, nil)

		assert.NotNil(t, ctx["inputs"])
		assert.NotNil(t, ctx["steps"])
		assert.Empty(t, ctx["steps"])
	})

	t.Run("handles nil converter in map", func(t *testing.T) {
		stepOutputs := map[string]StepOutputConverter{
			"step1": nil,
		}

		ctx := BuildContextFromTypedOutputs(nil, stepOutputs)
		// Should not panic, and step1 should not be in output
		assert.Empty(t, ctx["steps"])
	})
}
