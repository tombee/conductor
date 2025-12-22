// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestExampleWorkflowsValidate verifies that all example workflows in the examples directory
// pass schema validation. This ensures the schema accurately represents valid workflows.
func TestExampleWorkflowsValidate(t *testing.T) {
	// Find all example workflow files
	exampleFiles := []string{
		"../../../examples/workflows/minimal.yaml",
		"../../../examples/workflows/code-review.yaml",
		"../../../examples/workflows/tool-workflow.yaml",
		"../../../examples/code-review/workflow.yaml",
		"../../../examples/issue-triage/workflow.yaml",
		"../../../examples/custom-tools-workflow.yaml",
	}

	// Load the embedded schema
	schemaBytes := GetEmbeddedSchema()
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("Failed to parse embedded schema: %v", err)
	}

	validator := NewValidator()

	for _, exampleFile := range exampleFiles {
		t.Run(filepath.Base(exampleFile), func(t *testing.T) {
			// Read the example workflow file
			data, err := os.ReadFile(exampleFile)
			if err != nil {
				// Skip if file doesn't exist (may not be created yet)
				if os.IsNotExist(err) {
					t.Skipf("Example file not found: %s", exampleFile)
					return
				}
				t.Fatalf("Failed to read example file: %v", err)
			}

			// Parse YAML to data structure
			var workflowData interface{}
			if err := yaml.Unmarshal(data, &workflowData); err != nil {
				t.Fatalf("Failed to parse YAML: %v", err)
			}

			// Validate against schema
			if err := validator.Validate(schema, workflowData); err != nil {
				t.Errorf("Example workflow failed validation: %v", err)
			}
		})
	}
}

// TestSchemaRejectsInvalidWorkflows tests that the schema correctly rejects workflows
// with common authoring errors. This ensures the schema provides helpful validation.
//
// NOTE: The current validator supports basic JSON Schema features (type, required, enum, properties, items)
// but does not support advanced features like $ref, $defs, allOf, or if/then/else conditionals.
// These tests focus on validations that the simple validator can perform.
func TestSchemaRejectsInvalidWorkflows(t *testing.T) {
	// Load the embedded schema
	schemaBytes := GetEmbeddedSchema()
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("Failed to parse embedded schema: %v", err)
	}

	validator := NewValidator()

	tests := []struct {
		name        string
		workflow    string
		wantErr     bool
		errKeyword  string // Expected validation keyword (required, enum, type)
		errContains string // Expected substring in error message
	}{
		{
			name: "missing required name field",
			workflow: `
steps:
  - id: step1
    type: llm
    prompt: "Hello"
`,
			wantErr:     true,
			errKeyword:  "required",
			errContains: "name",
		},
		{
			name: "missing required steps array",
			workflow: `
name: test-workflow
description: "Test workflow"
`,
			wantErr:     true,
			errKeyword:  "required",
			errContains: "steps",
		},
		{
			name: "steps is wrong type (not array)",
			workflow: `
name: test-workflow
steps: "not-an-array"
`,
			wantErr:     true,
			errKeyword:  "type",
			errContains: "array",
		},
		{
			name: "name is wrong type (not string)",
			workflow: `
name: 123
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantErr:     true,
			errKeyword:  "type",
			errContains: "string",
		},
		{
			name: "valid minimal workflow",
			workflow: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Hello world"
`,
			wantErr: false,
		},
		{
			name: "valid workflow with all fields",
			workflow: `
name: test-workflow
description: "A complete test workflow"
version: "1.0"
inputs:
  - name: user_input
    type: string
    required: true
    description: "User input text"
steps:
  - id: analyze
    type: llm
    model: balanced
    prompt: "Analyze: {{.user_input}}"
  - id: file_read
    type: tool
    tool: file
    inputs:
      path: "test.txt"
outputs:
  - name: result
    type: string
    value: "$.steps.analyze.response"
    description: "Analysis result"
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML to data structure
			var workflowData interface{}
			if err := yaml.Unmarshal([]byte(tt.workflow), &workflowData); err != nil {
				t.Fatalf("Failed to parse test workflow YAML: %v", err)
			}

			// Validate against schema
			err := validator.Validate(schema, workflowData)

			if tt.wantErr && err == nil {
				t.Errorf("Expected validation error, but got none")
				return
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Expected no validation error, but got: %v", err)
				return
			}

			if tt.wantErr && err != nil {
				// Verify error has expected properties
				verr, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("Expected *ValidationError, got %T: %v", err, err)
					return
				}

				// Check keyword matches
				if tt.errKeyword != "" && verr.Keyword != tt.errKeyword {
					t.Errorf("Expected error keyword %q, got %q", tt.errKeyword, verr.Keyword)
				}

				// Check error message contains expected text
				if tt.errContains != "" {
					errMsg := err.Error()
					if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(tt.errContains)) {
						t.Errorf("Expected error to contain %q, got: %s", tt.errContains, errMsg)
					}
				}
			}
		})
	}
}

// TestSchemaErrorMessagesAreHelpful verifies that validation errors contain useful
// information for workflow authors, including field paths and expected values.
func TestSchemaErrorMessagesAreHelpful(t *testing.T) {
	// Load the embedded schema
	schemaBytes := GetEmbeddedSchema()
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("Failed to parse embedded schema: %v", err)
	}

	validator := NewValidator()

	tests := []struct {
		name             string
		workflow         string
		wantPath         string // Expected path in error (e.g., "$", "$.steps")
		wantFieldInError string // Field name that should appear in error
		wantExpectedInfo string // Info about what was expected
	}{
		{
			name: "missing name shows field path",
			workflow: `
steps:
  - id: test
    type: llm
    prompt: "test"
`,
			wantPath:         "$",
			wantFieldInError: "name",
			wantExpectedInfo: "required",
		},
		{
			name: "missing steps shows field path",
			workflow: `
name: test
description: "test"
`,
			wantPath:         "$",
			wantFieldInError: "steps",
			wantExpectedInfo: "required",
		},
		{
			name: "wrong type for name",
			workflow: `
name: 123
steps:
  - id: test
    type: llm
    prompt: "test"
`,
			wantPath:         "$.name",
			wantFieldInError: "string",
			wantExpectedInfo: "expected",
		},
		{
			name: "wrong type for steps",
			workflow: `
name: test
steps: "should-be-array"
`,
			wantPath:         "$.steps",
			wantFieldInError: "array",
			wantExpectedInfo: "expected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML to data structure
			var workflowData interface{}
			if err := yaml.Unmarshal([]byte(tt.workflow), &workflowData); err != nil {
				t.Fatalf("Failed to parse test workflow YAML: %v", err)
			}

			// Validate against schema
			err := validator.Validate(schema, workflowData)
			if err == nil {
				t.Fatal("Expected validation error, got nil")
			}

			verr, ok := err.(*ValidationError)
			if !ok {
				t.Fatalf("Expected *ValidationError, got %T: %v", err, err)
			}

			// Check that error contains the expected path
			if !strings.Contains(verr.Path, tt.wantPath) {
				t.Errorf("Expected error path to contain %q, got %q", tt.wantPath, verr.Path)
			}

			// Check that error message contains field name
			errMsg := err.Error()
			if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(tt.wantFieldInError)) {
				t.Errorf("Expected error to mention field %q, got: %s", tt.wantFieldInError, errMsg)
			}

			// Check that error contains information about what was expected
			if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(tt.wantExpectedInfo)) {
				t.Errorf("Expected error to contain %q, got: %s", tt.wantExpectedInfo, errMsg)
			}
		})
	}
}

// TestSchemaValidatesBasicStructure tests that the schema validates
// the basic structure of workflows using the available validator features.
//
// NOTE: Advanced validations like conditional requirements (e.g., "llm requires prompt")
// and enum constraints (e.g., "model must be fast/balanced/strategic") are defined in
// the JSON Schema but require a more complete validator implementation. The current
// validator provides basic type and required field validation. Full schema validation
// (including conditionals and enums) is available in IDEs via the YAML Language Server.
func TestSchemaValidatesBasicStructure(t *testing.T) {
	// Load the embedded schema
	schemaBytes := GetEmbeddedSchema()
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("Failed to parse embedded schema: %v", err)
	}

	validator := NewValidator()

	tests := []struct {
		name     string
		workflow string
		wantErr  bool
	}{
		{
			name: "valid workflow with various step types",
			workflow: `
name: test
steps:
  - id: step1
    type: llm
    prompt: "Analyze this"
  - id: step2
    github.create_issue:
      title: "Test issue"
  - id: step3
    type: tool
    tool: file
    inputs:
      path: "test.txt"
  - id: step4
    type: condition
    condition:
      expression: "$.value == 'yes'"
`,
			wantErr: false,
		},
		{
			name: "valid workflow with inputs and outputs",
			workflow: `
name: test
description: "Test workflow"
inputs:
  - name: input1
    type: string
    required: true
steps:
  - id: step1
    type: llm
    prompt: "Process {{.input1}}"
outputs:
  - name: result
    type: string
    value: "$.steps.step1.response"
`,
			wantErr: false,
		},
		{
			name: "valid workflow with agents and tools",
			workflow: `
name: test
agents:
  reviewer:
    prefers: anthropic
    capabilities: ["long-context"]
tools:
  - name: custom_tool
    type: http
    method: GET
    url: "https://api.example.com"
    description: "Custom tool"
steps:
  - id: step1
    type: llm
    agent: reviewer
    prompt: "Review"
    tools: ["custom_tool"]
`,
			wantErr: false,
		},
		{
			name: "empty steps array allowed (will fail Go validation)",
			workflow: `
name: test
steps: []
`,
			wantErr: false, // Basic validator doesn't check minItems
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML to data structure
			var workflowData interface{}
			if err := yaml.Unmarshal([]byte(tt.workflow), &workflowData); err != nil {
				t.Fatalf("Failed to parse test workflow YAML: %v", err)
			}

			// Validate against schema
			err := validator.Validate(schema, workflowData)

			if tt.wantErr && err == nil {
				t.Errorf("Expected validation error, but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Expected no validation error, but got: %v", err)
			}
		})
	}
}
