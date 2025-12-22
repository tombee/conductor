package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestValidateType tests T2.2: type validation
func TestValidateType(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		schema  map[string]interface{}
		data    interface{}
		wantErr bool
	}{
		{
			name:    "valid string",
			schema:  map[string]interface{}{"type": "string"},
			data:    "hello",
			wantErr: false,
		},
		{
			name:    "invalid string (number given)",
			schema:  map[string]interface{}{"type": "string"},
			data:    42,
			wantErr: true,
		},
		{
			name:    "valid number (float)",
			schema:  map[string]interface{}{"type": "number"},
			data:    42.5,
			wantErr: false,
		},
		{
			name:    "valid number (int)",
			schema:  map[string]interface{}{"type": "number"},
			data:    42,
			wantErr: false,
		},
		{
			name:    "invalid number (string given)",
			schema:  map[string]interface{}{"type": "number"},
			data:    "42",
			wantErr: true,
		},
		{
			name:    "valid integer",
			schema:  map[string]interface{}{"type": "integer"},
			data:    float64(42),
			wantErr: false,
		},
		{
			name:    "invalid integer (float given)",
			schema:  map[string]interface{}{"type": "integer"},
			data:    42.5,
			wantErr: true,
		},
		{
			name:    "valid boolean",
			schema:  map[string]interface{}{"type": "boolean"},
			data:    true,
			wantErr: false,
		},
		{
			name:    "invalid boolean",
			schema:  map[string]interface{}{"type": "boolean"},
			data:    "true",
			wantErr: true,
		},
		{
			name:    "valid object",
			schema:  map[string]interface{}{"type": "object"},
			data:    map[string]interface{}{"key": "value"},
			wantErr: false,
		},
		{
			name:    "invalid object",
			schema:  map[string]interface{}{"type": "object"},
			data:    []interface{}{},
			wantErr: true,
		},
		{
			name:    "valid array",
			schema:  map[string]interface{}{"type": "array"},
			data:    []interface{}{"a", "b"},
			wantErr: false,
		},
		{
			name:    "invalid array",
			schema:  map[string]interface{}{"type": "array"},
			data:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.schema, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateObject tests T2.3: object properties and required validation
func TestValidateObject(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		schema  map[string]interface{}
		data    interface{}
		wantErr bool
		errPath string
	}{
		{
			name: "valid object with all required fields",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "number"},
				},
				"required": []interface{}{"name", "age"},
			},
			data: map[string]interface{}{
				"name": "Alice",
				"age":  float64(30),
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "number"},
				},
				"required": []interface{}{"name", "age"},
			},
			data: map[string]interface{}{
				"name": "Alice",
			},
			wantErr: true,
			errPath: "$",
		},
		{
			name: "extra fields allowed",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"name"},
			},
			data: map[string]interface{}{
				"name":  "Alice",
				"extra": "field",
			},
			wantErr: false,
		},
		{
			name: "property type mismatch",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "number"},
				},
			},
			data: map[string]interface{}{
				"name": "Alice",
				"age":  "thirty",
			},
			wantErr: true,
			errPath: "$.age",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.schema, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errPath != "" {
				if verr, ok := err.(*ValidationError); ok {
					if !strings.Contains(verr.Path, tt.errPath) {
						t.Errorf("Expected error path containing %q, got %q", tt.errPath, verr.Path)
					}
				}
			}
		})
	}
}

// TestValidateEnum tests T2.4: enum validation
func TestValidateEnum(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		schema  map[string]interface{}
		data    interface{}
		wantErr bool
	}{
		{
			name: "valid enum value",
			schema: map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"bug", "feature", "question"},
			},
			data:    "bug",
			wantErr: false,
		},
		{
			name: "invalid enum value",
			schema: map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"bug", "feature", "question"},
			},
			data:    "invalid",
			wantErr: true,
		},
		{
			name: "enum in object property",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"category": map[string]interface{}{
						"type": "string",
						"enum": []interface{}{"bug", "feature"},
					},
				},
				"required": []interface{}{"category"},
			},
			data: map[string]interface{}{
				"category": "bug",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.schema, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateArray tests T2.5: array items validation
func TestValidateArray(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		schema  map[string]interface{}
		data    interface{}
		wantErr bool
	}{
		{
			name: "valid array of strings",
			schema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			data:    []interface{}{"a", "b", "c"},
			wantErr: false,
		},
		{
			name: "invalid array item type",
			schema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			data:    []interface{}{"a", 42, "c"},
			wantErr: true,
		},
		{
			name: "array of objects",
			schema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{"type": "number"},
					},
					"required": []interface{}{"id"},
				},
			},
			data: []interface{}{
				map[string]interface{}{"id": float64(1)},
				map[string]interface{}{"id": float64(2)},
			},
			wantErr: false,
		},
		{
			name: "array of objects with invalid item",
			schema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{"type": "number"},
					},
					"required": []interface{}{"id"},
				},
			},
			data: []interface{}{
				map[string]interface{}{"id": float64(1)},
				map[string]interface{}{}, // missing required field
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.schema, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateNested tests T2.6: nested schema validation
func TestValidateNested(t *testing.T) {
	validator := NewValidator()

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"user": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"address": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"city": map[string]interface{}{"type": "string"},
							"zip":  map[string]interface{}{"type": "string"},
						},
						"required": []interface{}{"city"},
					},
				},
				"required": []interface{}{"name", "address"},
			},
		},
		"required": []interface{}{"user"},
	}

	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
		errPath string
	}{
		{
			name: "valid nested object",
			data: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "Alice",
					"address": map[string]interface{}{
						"city": "NYC",
						"zip":  "10001",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing deeply nested required field",
			data: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "Alice",
					"address": map[string]interface{}{
						"zip": "10001",
					},
				},
			},
			wantErr: true,
			errPath: "$.user.address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(schema, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errPath != "" {
				if verr, ok := err.(*ValidationError); ok {
					if !strings.Contains(verr.Path, tt.errPath) {
						t.Errorf("Expected error path containing %q, got %q", tt.errPath, verr.Path)
					}
				}
			}
		})
	}
}

// TestValidationErrorFormat tests T2.7: error formatting
func TestValidationErrorFormat(t *testing.T) {
	validator := NewValidator()

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
		name           string
		data           interface{}
		wantErrKeyword string
		wantErrPath    string
	}{
		{
			name:           "type error",
			data:           map[string]interface{}{"category": 42},
			wantErrKeyword: "type",
			wantErrPath:    "$.category",
		},
		{
			name:           "enum error",
			data:           map[string]interface{}{"category": "invalid"},
			wantErrKeyword: "enum",
			wantErrPath:    "$.category",
		},
		{
			name:           "required error",
			data:           map[string]interface{}{},
			wantErrKeyword: "required",
			wantErrPath:    "$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(schema, tt.data)
			if err == nil {
				t.Fatal("Expected validation error, got nil")
			}

			verr, ok := err.(*ValidationError)
			if !ok {
				t.Fatalf("Expected *ValidationError, got %T", err)
			}

			if verr.Keyword != tt.wantErrKeyword {
				t.Errorf("Keyword = %q, want %q", verr.Keyword, tt.wantErrKeyword)
			}

			if !strings.Contains(verr.Path, tt.wantErrPath) {
				t.Errorf("Path %q does not contain %q", verr.Path, tt.wantErrPath)
			}

			// Check that error message is descriptive
			errMsg := err.Error()
			if !strings.Contains(errMsg, verr.Path) {
				t.Errorf("Error message should contain path, got: %s", errMsg)
			}
			if !strings.Contains(errMsg, verr.Keyword) {
				t.Errorf("Error message should contain keyword, got: %s", errMsg)
			}
		})
	}
}

// TestRealWorldSchemas tests validation with real-world examples
func TestRealWorldSchemas(t *testing.T) {
	validator := NewValidator()

	t.Run("classification schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"bug", "feature", "question", "other"},
				},
				"priority": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"critical", "high", "medium", "low"},
				},
				"summary": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []interface{}{"category", "priority"},
		}

		// Parse valid JSON response
		validJSON := `{"category": "bug", "priority": "high", "summary": "Login fails"}`
		var data interface{}
		if err := json.Unmarshal([]byte(validJSON), &data); err != nil {
			t.Fatal(err)
		}

		err := validator.Validate(schema, data)
		if err != nil {
			t.Errorf("Expected valid, got error: %v", err)
		}

		// Parse invalid JSON response
		invalidJSON := `{"category": "invalid", "priority": "high"}`
		if err := json.Unmarshal([]byte(invalidJSON), &data); err != nil {
			t.Fatal(err)
		}

		err = validator.Validate(schema, data)
		if err == nil {
			t.Error("Expected validation error for invalid category")
		}
	})

	t.Run("decision schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"decision": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"approve", "reject", "escalate"},
				},
				"reasoning": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []interface{}{"decision", "reasoning"},
		}

		validJSON := `{"decision": "approve", "reasoning": "Meets all criteria"}`
		var data interface{}
		if err := json.Unmarshal([]byte(validJSON), &data); err != nil {
			t.Fatal(err)
		}

		err := validator.Validate(schema, data)
		if err != nil {
			t.Errorf("Expected valid, got error: %v", err)
		}
	})

	t.Run("extraction schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":    map[string]interface{}{"type": "string"},
				"email":   map[string]interface{}{"type": "string"},
				"company": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"name", "email", "company"},
		}

		validJSON := `{"name": "Alice", "email": "alice@example.com", "company": "ACME"}`
		var data interface{}
		if err := json.Unmarshal([]byte(validJSON), &data); err != nil {
			t.Fatal(err)
		}

		err := validator.Validate(schema, data)
		if err != nil {
			t.Errorf("Expected valid, got error: %v", err)
		}
	})
}
