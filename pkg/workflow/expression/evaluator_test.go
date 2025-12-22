package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluator_ArrayMembership(t *testing.T) {
	e := New()
	ctx := map[string]interface{}{
		"inputs": map[string]interface{}{
			"personas": []interface{}{"security", "performance"},
			"tags":     []interface{}{"go", "cli", "workflow"},
		},
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{
			name: "in operator finds element in array",
			expr: `"security" in inputs.personas`,
			want: true,
		},
		{
			name: "in operator returns false for missing element",
			expr: `"style" in inputs.personas`,
			want: false,
		},
		{
			name: "has function finds element",
			expr: `has(inputs.personas, "performance")`,
			want: true,
		},
		{
			name: "has function returns false for missing",
			expr: `has(inputs.personas, "style")`,
			want: false,
		},
		{
			name: "includes is alias for has",
			expr: `includes(inputs.tags, "cli")`,
			want: true,
		},
		{
			name: "in operator with multiple elements",
			expr: `"go" in inputs.tags`,
			want: true,
		},
		{
			name: "in operator returns false for empty search",
			expr: `"python" in inputs.tags`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluator_Equality(t *testing.T) {
	e := New()
	ctx := map[string]interface{}{
		"inputs": map[string]interface{}{
			"mode":    "strict",
			"count":   5,
			"enabled": true,
			"name":    "test",
		},
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{
			name: "string equality true",
			expr: `inputs.mode == "strict"`,
			want: true,
		},
		{
			name: "string equality false",
			expr: `inputs.mode == "relaxed"`,
			want: false,
		},
		{
			name: "string inequality",
			expr: `inputs.mode != "relaxed"`,
			want: true,
		},
		{
			name: "number equality",
			expr: `inputs.count == 5`,
			want: true,
		},
		{
			name: "number inequality",
			expr: `inputs.count != 0`,
			want: true,
		},
		{
			name: "boolean equality",
			expr: `inputs.enabled == true`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluator_Comparison(t *testing.T) {
	e := New()
	ctx := map[string]interface{}{
		"inputs": map[string]interface{}{
			"count":    10,
			"priority": 5,
			"score":    7.5,
		},
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{
			name: "greater than true",
			expr: `inputs.count > 5`,
			want: true,
		},
		{
			name: "greater than false",
			expr: `inputs.count > 20`,
			want: false,
		},
		{
			name: "less than true",
			expr: `inputs.priority < 10`,
			want: true,
		},
		{
			name: "less than false",
			expr: `inputs.priority < 3`,
			want: false,
		},
		{
			name: "greater than or equal",
			expr: `inputs.count >= 10`,
			want: true,
		},
		{
			name: "less than or equal",
			expr: `inputs.priority <= 5`,
			want: true,
		},
		{
			name: "float comparison",
			expr: `inputs.score > 7.0`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluator_BooleanLogic(t *testing.T) {
	e := New()
	ctx := map[string]interface{}{
		"inputs": map[string]interface{}{
			"a":        true,
			"b":        false,
			"enabled":  true,
			"disabled": false,
			"count":    5,
		},
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{
			name: "logical AND true",
			expr: `inputs.a && inputs.enabled`,
			want: true,
		},
		{
			name: "logical AND false",
			expr: `inputs.a && inputs.b`,
			want: false,
		},
		{
			name: "logical OR true",
			expr: `inputs.a || inputs.b`,
			want: true,
		},
		{
			name: "logical OR both false",
			expr: `inputs.b || inputs.disabled`,
			want: false,
		},
		{
			name: "negation of false",
			expr: `!inputs.b`,
			want: true,
		},
		{
			name: "negation of true",
			expr: `!inputs.a`,
			want: false,
		},
		{
			name: "complex expression",
			expr: `inputs.enabled && !inputs.disabled && inputs.count > 0`,
			want: true,
		},
		{
			name: "parentheses",
			expr: `(inputs.a || inputs.b) && inputs.enabled`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluator_StepReferences(t *testing.T) {
	e := New()
	ctx := map[string]interface{}{
		"inputs": map[string]interface{}{
			"threshold": 80,
		},
		"steps": map[string]interface{}{
			"fetch": map[string]interface{}{
				"content": "some data",
				"status":  "success",
			},
			"analyze": map[string]interface{}{
				"content": "analysis result",
				"score":   95,
				"status":  "success",
			},
		},
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{
			name: "step status check",
			expr: `steps.fetch.status == "success"`,
			want: true,
		},
		{
			name: "step score comparison",
			expr: `steps.analyze.score > inputs.threshold`,
			want: true,
		},
		{
			name: "step content not empty",
			expr: `steps.fetch.content != ""`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluator_EmptyExpression(t *testing.T) {
	e := New()
	ctx := map[string]interface{}{}

	result, err := e.Evaluate("", ctx)
	require.NoError(t, err)
	assert.True(t, result, "empty expression should return true")
}

func TestEvaluator_Caching(t *testing.T) {
	e := New()
	ctx := map[string]interface{}{
		"inputs": map[string]interface{}{
			"x": true,
		},
	}

	expr := `inputs.x == true`

	// First evaluation
	result1, err := e.Evaluate(expr, ctx)
	require.NoError(t, err)
	assert.True(t, result1)

	// Check cache size
	assert.Equal(t, 1, e.CacheSize())

	// Second evaluation (should use cache)
	result2, err := e.Evaluate(expr, ctx)
	require.NoError(t, err)
	assert.True(t, result2)

	// Cache size should still be 1
	assert.Equal(t, 1, e.CacheSize())

	// Different expression
	_, err = e.Evaluate(`inputs.x == false`, ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, e.CacheSize())

	// Clear cache
	e.ClearCache()
	assert.Equal(t, 0, e.CacheSize())
}

func TestEvaluator_Errors(t *testing.T) {
	e := New()

	tests := []struct {
		name    string
		expr    string
		ctx     map[string]interface{}
		wantErr string
	}{
		{
			name:    "syntax error",
			expr:    `inputs.x ==`,
			ctx:     map[string]interface{}{},
			wantErr: "compile expression",
		},
		{
			name:    "non-boolean result",
			expr:    `"string"`,
			ctx:     map[string]interface{}{},
			wantErr: "expected bool", // expr library error message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.Evaluate(tt.expr, tt.ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestEvaluator_NilAndMissingValues(t *testing.T) {
	e := New()
	ctx := map[string]interface{}{
		"inputs": map[string]interface{}{
			"present": "value",
			"nilval":  nil,
		},
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{
			name: "nil comparison",
			expr: `inputs.nilval == nil`,
			want: true,
		},
		{
			name: "present value not nil",
			expr: `inputs.present != nil`,
			want: true,
		},
		{
			name: "missing value is nil",
			expr: `inputs.missing == nil`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluator_LengthFunction(t *testing.T) {
	e := New()
	ctx := map[string]interface{}{
		"inputs": map[string]interface{}{
			"items": []interface{}{"a", "b", "c"},
			"empty": []interface{}{},
			"text":  "hello",
		},
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{
			name: "array length equals 3",
			expr: `length(inputs.items) == 3`,
			want: true,
		},
		{
			name: "empty array length is 0",
			expr: `length(inputs.empty) == 0`,
			want: true,
		},
		{
			name: "array not empty",
			expr: `length(inputs.items) > 0`,
			want: true,
		},
		{
			name: "string length",
			expr: `length(inputs.text) == 5`,
			want: true,
		},
		{
			name: "empty array check",
			expr: `length(inputs.empty) == 0`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(tt.expr, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
