package workflow

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Math function tests

func TestAdd(t *testing.T) {
	tests := []struct {
		name     string
		values   []interface{}
		expected interface{}
		wantErr  bool
	}{
		{"empty", []interface{}{}, 0, false},
		{"single int", []interface{}{5}, int64(5), false},
		{"two ints", []interface{}{2, 3}, int64(5), false},
		{"multiple ints", []interface{}{1, 2, 3, 4}, int64(10), false},
		{"floats", []interface{}{1.5, 2.5}, 4.0, false},
		{"mixed int and float", []interface{}{2, 3.5}, 5.5, false},
		{"negative", []interface{}{-5, 10}, int64(5), false},
		{"string number", []interface{}{2, "3"}, 5.0, false},
		{"invalid type", []interface{}{2, "invalid"}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := add(tt.values...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSub(t *testing.T) {
	tests := []struct {
		name     string
		a, b     interface{}
		expected interface{}
		wantErr  bool
	}{
		{"ints", 10, 3, int64(7), false},
		{"floats", 10.5, 3.2, 7.3, false},
		{"mixed", 10, 3.5, 6.5, false},
		{"negative", 5, 10, int64(-5), false},
		{"string number", 10, "3", 7.0, false},
		{"invalid", 10, "invalid", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sub(tt.a, tt.b)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if expected, ok := tt.expected.(float64); ok {
					assert.InDelta(t, expected, result, 0.0001)
				} else {
					assert.Equal(t, tt.expected, result)
				}
			}
		})
	}
}

func TestMul(t *testing.T) {
	tests := []struct {
		name     string
		values   []interface{}
		expected interface{}
		wantErr  bool
	}{
		{"empty", []interface{}{}, 1, false},
		{"single int", []interface{}{5}, int64(5), false},
		{"two ints", []interface{}{2, 3}, int64(6), false},
		{"multiple ints", []interface{}{2, 3, 4}, int64(24), false},
		{"floats", []interface{}{2.5, 4.0}, 10.0, false},
		{"mixed", []interface{}{2, 3.5}, 7.0, false},
		{"zero", []interface{}{5, 0}, int64(0), false},
		{"negative", []interface{}{-2, 3}, int64(-6), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mul(tt.values...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDiv(t *testing.T) {
	tests := []struct {
		name     string
		a, b     interface{}
		expected int64
		wantErr  bool
	}{
		{"basic", 10, 2, 5, false},
		{"truncate", 10, 3, 3, false},
		{"negative", -10, 3, -3, false},
		{"division by zero", 10, 0, 0, true},
		{"floats truncated", 10.9, 2.1, 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := div(tt.a, tt.b)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "division by zero")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDivf(t *testing.T) {
	tests := []struct {
		name     string
		a, b     interface{}
		expected float64
		wantErr  bool
	}{
		{"basic", 10, 2, 5.0, false},
		{"decimal", 10, 4, 2.5, false},
		{"negative", -10, 4, -2.5, false},
		{"division by zero", 10, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := divf(tt.a, tt.b)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "division by zero")
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.expected, result, 0.0001)
			}
		})
	}
}

func TestMod(t *testing.T) {
	tests := []struct {
		name     string
		a, b     interface{}
		expected int64
		wantErr  bool
	}{
		{"basic", 10, 3, 1, false},
		{"zero remainder", 10, 2, 0, false},
		{"negative", -10, 3, -1, false},
		{"modulo by zero", 10, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mod(tt.a, tt.b)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "division by zero")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		name     string
		values   []interface{}
		expected interface{}
		wantErr  bool
	}{
		{"empty", []interface{}{}, nil, true},
		{"single", []interface{}{5}, int64(5), false},
		{"ints", []interface{}{3, 1, 4, 1, 5}, int64(1), false},
		{"floats", []interface{}{3.5, 1.2, 4.8}, 1.2, false},
		{"mixed", []interface{}{3, 1.5, 4}, 1.5, false},
		{"negative", []interface{}{-5, 0, 5}, int64(-5), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := min(tt.values...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		name     string
		values   []interface{}
		expected interface{}
		wantErr  bool
	}{
		{"empty", []interface{}{}, nil, true},
		{"single", []interface{}{5}, int64(5), false},
		{"ints", []interface{}{3, 1, 4, 1, 5}, int64(5), false},
		{"floats", []interface{}{3.5, 1.2, 4.8}, 4.8, false},
		{"mixed", []interface{}{3, 1.5, 4}, 4.0, false},
		{"negative", []interface{}{-5, 0, 5}, int64(5), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := max(tt.values...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// JSON function tests

func TestToJson(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
		wantErr  bool
	}{
		{"string", "hello", `"hello"`, false},
		{"int", 42, "42", false},
		{"float", 3.14, "3.14", false},
		{"bool", true, "true", false},
		{"map", map[string]interface{}{"name": "Thorin", "level": 3}, `{"level":3,"name":"Thorin"}`, false},
		{"array", []string{"a", "b", "c"}, `["a","b","c"]`, false},
		{"null", nil, "null", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toJson(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.JSONEq(t, tt.expected, result)
			}
		})
	}
}

func TestToJsonPretty(t *testing.T) {
	input := map[string]interface{}{
		"name":  "Thorin",
		"level": 3,
	}

	result, err := toJsonPretty(input)
	require.NoError(t, err)
	assert.Contains(t, result, "\n")
	assert.Contains(t, result, "  ")
}

func TestFromJson(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
		wantErr  bool
	}{
		{"string", `"hello"`, "hello", false},
		{"int", "42", float64(42), false}, // JSON numbers decode to float64
		{"bool", "true", true, false},
		{"map", `{"name":"Thorin"}`, map[string]interface{}{"name": "Thorin"}, false},
		{"array", `["a","b","c"]`, []interface{}{"a", "b", "c"}, false},
		{"null", "null", nil, false},
		{"invalid", "invalid", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fromJson(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestJsonSizeLimits(t *testing.T) {
	// Create a large JSON string
	largeData := strings.Repeat("a", MaxJSONSize+1)

	_, err := fromJson(largeData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum size")
}

// String function tests

func TestJoinFunc(t *testing.T) {
	tests := []struct {
		name     string
		arr      interface{}
		sep      string
		expected string
		wantErr  bool
	}{
		{"strings", []string{"a", "b", "c"}, ", ", "a, b, c", false},
		{"ints", []int{1, 2, 3}, "-", "1-2-3", false},
		{"mixed interfaces", []interface{}{"a", 1, true}, "|", "a|1|true", false},
		{"empty array", []string{}, ",", "", false},
		{"not an array", "not array", ",", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := joinFunc(tt.arr, tt.sep)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTitleCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "Hello"},
		{"HELLO", "Hello"},
		{"hELLO", "Hello"},
		{"", ""},
		{"a", "A"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := titleCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Collection function tests

func TestFirst(t *testing.T) {
	tests := []struct {
		name     string
		arr      interface{}
		expected interface{}
		wantErr  bool
	}{
		{"string array", []string{"a", "b", "c"}, "a", false},
		{"int array", []int{1, 2, 3}, 1, false},
		{"empty array", []string{}, nil, true},
		{"not an array", "not array", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := first(tt.arr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestLast(t *testing.T) {
	tests := []struct {
		name     string
		arr      interface{}
		expected interface{}
		wantErr  bool
	}{
		{"string array", []string{"a", "b", "c"}, "c", false},
		{"int array", []int{1, 2, 3}, 3, false},
		{"empty array", []string{}, nil, true},
		{"not an array", "not array", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := last(tt.arr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
		wantErr  bool
	}{
		{
			"string map",
			map[string]int{"a": 1, "b": 2},
			[]string{"a", "b"},
			false,
		},
		{
			"empty map",
			map[string]int{},
			[]string{},
			false,
		},
		{
			"not a map",
			[]string{"a", "b"},
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := keys(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func TestValues(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []interface{}
		wantErr  bool
	}{
		{
			"string map",
			map[string]int{"a": 1, "b": 2},
			[]interface{}{1, 2},
			false,
		},
		{
			"empty map",
			map[string]int{},
			[]interface{}{},
			false,
		},
		{
			"not a map",
			[]string{"a", "b"},
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := values(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func TestHasKey(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}

	result, err := hasKey(m, "a")
	require.NoError(t, err)
	assert.True(t, result)

	result, err = hasKey(m, "c")
	require.NoError(t, err)
	assert.False(t, result)

	_, err = hasKey("not a map", "key")
	assert.Error(t, err)
}

func TestPluck(t *testing.T) {
	tests := []struct {
		name      string
		arr       interface{}
		fieldName string
		expected  []interface{}
		wantErr   bool
	}{
		{
			"map array",
			[]map[string]interface{}{
				{"name": "Thorin", "level": 3},
				{"name": "Gandalf", "level": 10},
			},
			"name",
			[]interface{}{"Thorin", "Gandalf"},
			false,
		},
		{
			"missing field",
			[]map[string]interface{}{
				{"name": "Thorin"},
				{"level": 10},
			},
			"name",
			[]interface{}{"Thorin"},
			false,
		},
		{
			"empty array",
			[]map[string]interface{}{},
			"name",
			[]interface{}{},
			false,
		},
		{
			"not an array",
			"not array",
			"field",
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pluck(tt.arr, tt.fieldName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// Default and type conversion tests

func TestDefaultFunc(t *testing.T) {
	tests := []struct {
		name       string
		value      interface{}
		defaultVal interface{}
		expected   interface{}
	}{
		{"nil value", nil, "default", "default"},
		{"empty string", "", "default", "default"},
		{"non-empty string", "value", "default", "value"},
		{"zero int", 0, "default", 0},
		{"empty slice", []string{}, "default", "default"},
		{"non-empty slice", []string{"a"}, "default", []string{"a"}},
		{"empty map", map[string]int{}, "default", "default"},
		{"non-empty map", map[string]int{"a": 1}, "default", map[string]int{"a": 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defaultFunc(tt.value, tt.defaultVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name     string
		values   []interface{}
		expected interface{}
	}{
		{"first non-nil", []interface{}{nil, nil, "first", "second"}, "first"},
		{"skip empty string", []interface{}{nil, "", "first"}, "first"},
		{"skip empty slice", []interface{}{[]string{}, "first"}, "first"},
		{"all nil", []interface{}{nil, nil}, nil},
		{"empty args", []interface{}{}, nil},
		{"zero is valid", []interface{}{nil, 0}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coalesce(tt.values...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
		wantErr  bool
	}{
		{"int", 42, 42, false},
		{"float", 42.7, 42, false},
		{"string", "42", 42, false},
		{"invalid string", "invalid", 0, true},
		{"nil", nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toInt(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
		wantErr  bool
	}{
		{"int", 42, 42.0, false},
		{"float", 3.14, 3.14, false},
		{"string", "3.14", 3.14, false},
		{"invalid string", "invalid", 0, true},
		{"nil", nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toFloat(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.expected, result, 0.0001)
			}
		})
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool", true, "true"},
		{"nil", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
		wantErr  bool
	}{
		{"bool true", true, true, false},
		{"bool false", false, false, false},
		{"string true", "true", true, false},
		{"string false", "false", false, false},
		{"string yes", "yes", true, false},
		{"string no", "no", false, false},
		{"string 1", "1", true, false},
		{"string 0", "0", false, false},
		{"int non-zero", 42, true, false},
		{"int zero", 0, false, false},
		{"float non-zero", 3.14, true, false},
		{"float zero", 0.0, false, false},
		{"nil", nil, false, false},
		{"invalid string", "invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toBool(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// Integration tests

func TestTemplateFuncMapIntegration(t *testing.T) {
	ctx := NewTemplateContext()
	ctx.SetInput("roll", 14)
	ctx.SetInput("modifier", 5)
	ctx.SetStepOutput("character", map[string]interface{}{
		"name":  "Thorin",
		"level": 3,
	})

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			"arithmetic",
			"Total: {{add .roll .modifier}}",
			"Total: 19",
		},
		{
			"JSON serialization",
			"{{toJson .steps.character}}",
			`{"level":3,"name":"Thorin"}`,
		},
		{
			"string operations",
			"{{upper .steps.character.name}}",
			"THORIN",
		},
		{
			"chained functions",
			"Level: {{mul .steps.character.level 2}}",
			"Level: 6",
		},
		{
			"default values",
			"{{default .missing \"fallback\"}}",
			"fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveTemplate(tt.template, ctx)
			require.NoError(t, err)
			if strings.Contains(tt.expected, "{") {
				assert.JSONEq(t, tt.expected, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestArraySizeLimits(t *testing.T) {
	// Create an array that exceeds the limit
	largeArray := make([]int, MaxArrayLen+1)

	_, err := joinFunc(largeArray, ",")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")

	_, err = pluck(largeArray, "field")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")
}
