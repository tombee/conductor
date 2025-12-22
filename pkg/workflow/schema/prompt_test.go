package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBuildPromptWithSchema tests T3.1: prompt builder
func TestBuildPromptWithSchema(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"category": map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"bug", "feature"},
			},
		},
		"required": []interface{}{"category"},
	}

	tests := []struct {
		name         string
		prompt       string
		retryAttempt int
		wantContains []string
	}{
		{
			name:         "first attempt - subtle",
			prompt:       "Classify this issue",
			retryAttempt: 0,
			wantContains: []string{
				"Classify this issue",
				"JSON",
				"category",
			},
		},
		{
			name:         "second attempt - more explicit",
			prompt:       "Classify this issue",
			retryAttempt: 1,
			wantContains: []string{
				"Classify this issue",
				"IMPORTANT",
				"didn't match",
				"ONLY with the JSON",
			},
		},
		{
			name:         "third attempt - very explicit",
			prompt:       "Classify this issue",
			retryAttempt: 2,
			wantContains: []string{
				"Classify this issue",
				"CRITICAL",
				"ONLY valid JSON",
				"Example:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildPromptWithSchema(tt.prompt, schema, tt.retryAttempt)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("BuildPromptWithSchema() missing %q in result:\n%s", want, result)
				}
			}
		})
	}
}

// TestExtractJSON tests T3.2: JSON extraction
func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantData map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "clean JSON",
			response: `{"category": "bug"}`,
			wantData: map[string]interface{}{"category": "bug"},
			wantErr:  false,
		},
		{
			name: "JSON with whitespace",
			response: `

				{"category": "bug"}

			`,
			wantData: map[string]interface{}{"category": "bug"},
			wantErr:  false,
		},
		{
			name:     "JSON in markdown code block",
			response: "```json\n" + `{"category": "bug"}` + "\n```",
			wantData: map[string]interface{}{"category": "bug"},
			wantErr:  false,
		},
		{
			name:     "JSON in generic code block",
			response: "```\n" + `{"category": "bug"}` + "\n```",
			wantData: map[string]interface{}{"category": "bug"},
			wantErr:  false,
		},
		{
			name:     "JSON with text before",
			response: `Here is the classification: {"category": "bug"}`,
			wantData: map[string]interface{}{"category": "bug"},
			wantErr:  false,
		},
		{
			name:     "JSON with text after",
			response: `{"category": "bug"} - This is a bug report`,
			wantData: map[string]interface{}{"category": "bug"},
			wantErr:  false,
		},
		{
			name:     "JSON with text before and after",
			response: `The classification result is: {"category": "bug"} as determined by analysis.`,
			wantData: map[string]interface{}{"category": "bug"},
			wantErr:  false,
		},
		{
			name:     "nested JSON object",
			response: `{"user": {"name": "Alice", "age": 30}}`,
			wantData: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "Alice",
					"age":  float64(30),
				},
			},
			wantErr: false,
		},
		{
			name:     "JSON array",
			response: `[{"id": 1}, {"id": 2}]`,
			wantData: nil, // Will be checked differently
			wantErr:  false,
		},
		{
			name:     "invalid JSON",
			response: `This is not JSON at all`,
			wantErr:  true,
		},
		{
			name:     "malformed JSON",
			response: `{"category": bug}`, // Missing quotes
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ExtractJSON(tt.response)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// For non-map data (like arrays), just check it's not nil
			if tt.wantData == nil {
				if data == nil {
					t.Error("ExtractJSON() returned nil data")
				}
				return
			}

			// Compare as JSON strings for easier comparison
			gotJSON, _ := json.Marshal(data)
			wantJSON, _ := json.Marshal(tt.wantData)

			if string(gotJSON) != string(wantJSON) {
				t.Errorf("ExtractJSON() = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

// TestExtractJSONEdgeCases tests edge cases for JSON extraction
func TestExtractJSONEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantErr  bool
	}{
		{
			name:     "JSON with escaped quotes",
			response: `{"message": "He said \"hello\""}`,
			wantErr:  false,
		},
		{
			name:     "JSON with newlines in strings",
			response: `{"text": "line1\nline2"}`,
			wantErr:  false,
		},
		{
			name:     "multiple JSON objects (takes first)",
			response: `{"first": true} {"second": true}`,
			wantErr:  false,
		},
		{
			name:     "empty object",
			response: `{}`,
			wantErr:  false,
		},
		{
			name:     "empty array",
			response: `[]`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ExtractJSON(tt.response)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && data == nil {
				t.Error("ExtractJSON() returned nil data when error not expected")
			}
		})
	}
}

// TestFormatSchemaForPrompt tests schema formatting for human readability
func TestFormatSchemaForPrompt(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"category": map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"bug", "feature", "question"},
			},
			"priority": map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"high", "medium", "low"},
			},
			"summary": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []interface{}{"category", "priority"},
	}

	result := formatSchemaForPrompt(schema)

	// Check that all properties are mentioned
	properties := []string{"category", "priority", "summary"}
	for _, prop := range properties {
		if !strings.Contains(result, prop) {
			t.Errorf("formatSchemaForPrompt() missing property %q", prop)
		}
	}

	// Check that required marker is present
	if !strings.Contains(result, "required") {
		t.Error("formatSchemaForPrompt() should indicate required fields")
	}

	// Check that enum values are shown
	if !strings.Contains(result, "bug") || !strings.Contains(result, "feature") {
		t.Error("formatSchemaForPrompt() should show enum values")
	}
}

// TestBuildExampleJSON tests example JSON generation
func TestBuildExampleJSON(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]interface{}
		check  func(*testing.T, string)
	}{
		{
			name: "simple object",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "number"},
				},
			},
			check: func(t *testing.T, result string) {
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(result), &data); err != nil {
					t.Fatalf("Example JSON is invalid: %v", err)
				}
				if _, ok := data["name"]; !ok {
					t.Error("Example should include 'name' field")
				}
				if _, ok := data["age"]; !ok {
					t.Error("Example should include 'age' field")
				}
			},
		},
		{
			name: "object with enum",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type": "string",
						"enum": []interface{}{"active", "inactive"},
					},
				},
			},
			check: func(t *testing.T, result string) {
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(result), &data); err != nil {
					t.Fatalf("Example JSON is invalid: %v", err)
				}
				status, ok := data["status"].(string)
				if !ok {
					t.Fatal("status should be a string")
				}
				if status != "active" && status != "inactive" {
					t.Errorf("status should be one of the enum values, got %q", status)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildExampleJSON(tt.schema)
			tt.check(t, result)
		})
	}
}

// TestRetryProgressiveness tests that retry prompts become more explicit
func TestRetryProgressiveness(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"result": map[string]interface{}{"type": "string"},
		},
	}

	prompt := "Do the task"

	attempt0 := BuildPromptWithSchema(prompt, schema, 0)
	attempt1 := BuildPromptWithSchema(prompt, schema, 1)
	attempt2 := BuildPromptWithSchema(prompt, schema, 2)

	// Check that attempts contain progressively stronger keywords
	if !strings.Contains(attempt0, "JSON") {
		t.Error("First attempt should mention JSON")
	}

	if !strings.Contains(attempt1, "IMPORTANT") {
		t.Error("Second attempt should contain IMPORTANT")
	}
	if !strings.Contains(attempt1, "didn't match") {
		t.Error("Second attempt should mention previous failure")
	}

	if !strings.Contains(attempt2, "CRITICAL") {
		t.Error("Third attempt should contain CRITICAL")
	}
	if !strings.Contains(attempt2, "Example:") {
		t.Error("Third attempt should include an example")
	}
}
