package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests using real workflow examples from the spec

func TestDnDWorkflowExample(t *testing.T) {
	// Simulate a D&D workflow with dice rolls
	ctx := NewTemplateContext()

	// Step 1: d20 roll
	ctx.SetStepOutput("d20_roll", map[string]interface{}{
		"result": 14,
	})

	// Step 2: load character with modifiers
	ctx.SetStepOutput("load_character", map[string]interface{}{
		"response": map[string]interface{}{
			"name": "Thorin",
			"modifiers": map[string]interface{}{
				"str": 5,
			},
		},
	})

	// Step 3: classify difficulty
	ctx.SetStepOutput("classify", map[string]interface{}{
		"response": map[string]interface{}{
			"difficulty": 15,
		},
	})

	// Template from the spec
	template := `{{$roll := .steps.d20_roll.result}}{{$mod := .steps.load_character.response.modifiers.str}}{{$total := add $roll $mod}}Attack roll: {{$total}} (rolled {{$roll}} + {{$mod}} modifier)
DC was: {{.steps.classify.response.difficulty}}`

	result, err := ResolveTemplate(template, ctx)
	require.NoError(t, err)

	expected := `Attack roll: 19 (rolled 14 + 5 modifier)
DC was: 15`

	assert.Equal(t, expected, result)
}

func TestCharacterDataSerialization(t *testing.T) {
	// Test serializing character data to JSON for prompts
	ctx := NewTemplateContext()

	ctx.SetStepOutput("character", map[string]interface{}{
		"name":  "Thorin",
		"level": 3,
		"items": []string{"sword", "shield", "axe"},
	})

	template := `Character data: {{toJson .steps.character}}`

	result, err := ResolveTemplate(template, ctx)
	require.NoError(t, err)

	// Verify it's valid JSON
	assert.Contains(t, result, `"name":"Thorin"`)
	assert.Contains(t, result, `"level":3`)
	assert.Contains(t, result, `"items":["sword","shield","axe"]`)
}

func TestChainedFunctions(t *testing.T) {
	// Test multiple functions chained together
	ctx := NewTemplateContext()

	ctx.SetInput("characters", []map[string]interface{}{
		{"name": "Thorin", "level": 3},
		{"name": "Gandalf", "level": 10},
		{"name": "Bilbo", "level": 1},
	})

	// Extract names, join them, and uppercase
	template := `{{$names := pluck .characters "name"}}{{upper (join $names ", ")}}`

	result, err := ResolveTemplate(template, ctx)
	require.NoError(t, err)

	assert.Equal(t, "THORIN, GANDALF, BILBO", result)
}

func TestDefaultsInWorkflow(t *testing.T) {
	// Test using default values for optional workflow inputs

	// Test 1: Custom timeout not provided
	ctx1 := NewTemplateContext()
	ctx1.SetInput("custom_timeout", nil)

	template := `Timeout: {{default .custom_timeout 30}}`

	result, err := ResolveTemplate(template, ctx1)
	require.NoError(t, err)
	assert.Equal(t, "Timeout: 30", result)

	// Test 2: With custom timeout
	ctx2 := NewTemplateContext()
	ctx2.SetInput("custom_timeout", 60)

	result, err = ResolveTemplate(template, ctx2)
	require.NoError(t, err)
	assert.Equal(t, "Timeout: 60", result)
}

func TestArrayManipulation(t *testing.T) {
	// Test array operations in workflow
	ctx := NewTemplateContext()

	ctx.SetInput("items", []string{"health_potion", "mana_potion", "sword", "shield"})

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			"count items",
			"Item count: {{len .items}}",
			"Item count: 4",
		},
		{
			"first item",
			"First: {{first .items}}",
			"First: health_potion",
		},
		{
			"last item",
			"Last: {{last .items}}",
			"Last: shield",
		},
		{
			"join items",
			"{{join .items \", \"}}",
			"health_potion, mana_potion, sword, shield",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveTemplate(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapOperations(t *testing.T) {
	// Test map operations in workflow
	ctx := NewTemplateContext()

	ctx.SetInput("stats", map[string]interface{}{
		"strength":     15,
		"dexterity":    12,
		"intelligence": 8,
	})

	template := `Has wisdom: {{hasKey .stats "wisdom"}}, Has strength: {{hasKey .stats "strength"}}`

	result, err := ResolveTemplate(template, ctx)
	require.NoError(t, err)

	assert.Equal(t, "Has wisdom: false, Has strength: true", result)
}

func TestStringFormatting(t *testing.T) {
	// Test string formatting operations
	ctx := NewTemplateContext()

	ctx.SetInput("raw_name", "  thorin oakenshield  ")
	ctx.SetInput("file_path", "/api/v1/characters")

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			"trim spaces",
			"{{trim .raw_name}}",
			"thorin oakenshield",
		},
		{
			"title case",
			"{{title (trim .raw_name)}}",
			"Thorin oakenshield",
		},
		{
			"uppercase",
			"{{upper (trim .raw_name)}}",
			"THORIN OAKENSHIELD",
		},
		{
			"trim prefix",
			"{{trimPrefix .file_path \"/\"}}",
			"api/v1/characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveTemplate(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMathInWorkflow(t *testing.T) {
	// Test various math operations in workflow context
	ctx := NewTemplateContext()

	ctx.SetInput("base_damage", 10)
	ctx.SetInput("multiplier", 2)
	ctx.SetInput("bonus", 5)

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			"multiplication and addition",
			"{{add (mul .base_damage .multiplier) .bonus}}",
			"25",
		},
		{
			"division",
			"{{div .bonus 2}}",
			"2",
		},
		{
			"float division",
			"{{divf .base_damage 3}}",
			"3.3333333333333335",
		},
		{
			"modulo",
			"{{mod .base_damage 3}}",
			"1",
		},
		{
			"min/max",
			"Min: {{min .base_damage .multiplier .bonus}}, Max: {{max .base_damage .multiplier .bonus}}",
			"Min: 2, Max: 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveTemplate(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeConversionInWorkflow(t *testing.T) {
	// Test type conversion in workflow
	ctx := NewTemplateContext()

	ctx.SetInput("string_number", "42")
	ctx.SetInput("string_bool", "true")
	ctx.SetInput("number", 123)

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			"string to int",
			"{{add (toInt .string_number) 8}}",
			"50",
		},
		{
			"number to string",
			"Value: {{toString .number}}",
			"Value: 123",
		},
		{
			"string to bool",
			"{{toBool .string_bool}}",
			"true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveTemplate(tt.template, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoalesceWorkflow(t *testing.T) {
	// Test coalesce for fallback values
	ctx := NewTemplateContext()

	ctx.SetInput("primary", nil)
	ctx.SetInput("secondary", "")
	ctx.SetInput("fallback", "default_value")

	template := `Value: {{coalesce .primary .secondary .fallback}}`

	result, err := ResolveTemplate(template, ctx)
	require.NoError(t, err)

	assert.Equal(t, "Value: default_value", result)

	// Now with secondary set
	ctx.SetInput("secondary", "secondary_value")

	result, err = ResolveTemplate(template, ctx)
	require.NoError(t, err)

	assert.Equal(t, "Value: secondary_value", result)
}

func TestJSONRoundTrip(t *testing.T) {
	// Test JSON serialization and deserialization
	ctx := NewTemplateContext()

	ctx.SetInput("data", map[string]interface{}{
		"name":  "Test",
		"count": 5,
	})

	// Serialize to JSON
	template1 := `{{toJson .data}}`
	json, err := ResolveTemplate(template1, ctx)
	require.NoError(t, err)

	// Store the JSON string
	ctx.SetInput("json_string", json)

	// Parse it back and access a field
	template2 := `{{$parsed := fromJson .json_string}}{{index $parsed "name"}}`
	result, err := ResolveTemplate(template2, ctx)
	require.NoError(t, err)

	assert.Equal(t, "Test", result)
}
