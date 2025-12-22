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

package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tombee/conductor/pkg/workflow"
	workflowschema "github.com/tombee/conductor/pkg/workflow/schema"
	"gopkg.in/yaml.v3"
)

const (
	maxYAMLSize = 10 * 1024 * 1024 // 10MB limit per NFR9
)

// ValidationResult represents the validation result
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors,omitempty"`
	Warnings []ValidationError `json:"warnings,omitempty"`
}

// ValidationError represents a validation error with location info
type ValidationError struct {
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// handleValidate implements the conductor_validate tool
func (s *Server) handleValidate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check rate limit
	if !s.rateLimiter.AllowCall() {
		return errorResponse("Rate limit exceeded. Please try again later."), nil
	}

	// Extract workflow_yaml argument
	workflowYAML, err := request.RequireString("workflow_yaml")
	if err != nil {
		return errorResponse("Missing or invalid 'workflow_yaml' argument"), nil
	}

	// Check size limit
	if len(workflowYAML) > maxYAMLSize {
		return errorResponse(fmt.Sprintf("Workflow YAML exceeds maximum size of %d bytes", maxYAMLSize)), nil
	}

	// Validate the YAML
	result := validateWorkflowYAML([]byte(workflowYAML))

	// Marshal result to JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to encode validation result: %v", err)), nil
	}

	return textResponse(string(resultJSON)), nil
}

// validateWorkflowYAML validates workflow YAML content
// This is extracted for reuse and testing
func validateWorkflowYAML(data []byte) ValidationResult {
	result := ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
	}

	// Step 1: Validate YAML syntax
	var yamlData interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		line, col := extractYAMLErrorLocation(err)
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Line:       line,
			Column:     col,
			Message:    fmt.Sprintf("YAML syntax error: %v", err),
			Suggestion: "Check for indentation issues, missing colons, or invalid characters",
		})
		return result
	}

	// Step 2: Validate against JSON Schema
	schemaErrors := validateAgainstSchema(yamlData)
	if len(schemaErrors) > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, schemaErrors...)
		return result
	}

	// Step 3: Validate semantic rules via Go validation
	def, err := workflow.ParseDefinition(data)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Line:       0,
			Column:     0,
			Message:    err.Error(),
			Suggestion: "Check that all step references resolve correctly and required fields are present",
		})
		return result
	}

	// Add warnings for best practices
	if def != nil {
		warnings := checkBestPractices(def)
		result.Warnings = append(result.Warnings, warnings...)
	}

	return result
}

// extractYAMLErrorLocation attempts to extract line and column from YAML parse error
func extractYAMLErrorLocation(err error) (line, col int) {
	if typeErr, ok := err.(*yaml.TypeError); ok {
		if len(typeErr.Errors) > 0 {
			var l int
			if _, parseErr := fmt.Sscanf(typeErr.Errors[0], "line %d:", &l); parseErr == nil {
				return l, 0
			}
		}
	}
	return 0, 0
}

// validateAgainstSchema validates data against the workflow JSON Schema
func validateAgainstSchema(data interface{}) []ValidationError {
	var errors []ValidationError

	// Use embedded schema
	schemaBytes := workflowschema.GetEmbeddedSchema()
	var schemaData map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schemaData); err != nil {
		errors = append(errors, ValidationError{
			Line:    0,
			Column:  0,
			Message: fmt.Sprintf("Failed to parse embedded schema: %v", err),
		})
		return errors
	}

	// Validate against schema
	validator := workflowschema.NewValidator()
	if err := validator.Validate(schemaData, data); err != nil {
		if valErr, ok := err.(*workflowschema.ValidationError); ok {
			errors = append(errors, ValidationError{
				Line:       0,
				Column:     0,
				Message:    valErr.Message,
				Suggestion: "Refer to the schema documentation for field requirements",
			})
		} else {
			errors = append(errors, ValidationError{
				Line:    0,
				Column:  0,
				Message: err.Error(),
			})
		}
	}

	return errors
}

// checkBestPractices checks for common best practice issues
func checkBestPractices(def *workflow.Definition) []ValidationError {
	var warnings []ValidationError

	// Check if workflow has a description
	if def.Description == "" {
		warnings = append(warnings, ValidationError{
			Message:    "Workflow lacks a description",
			Suggestion: "Add a description field to document the workflow's purpose",
		})
	}

	// Check for workflows with no steps
	if len(def.Steps) == 0 {
		warnings = append(warnings, ValidationError{
			Message:    "Workflow has no steps defined",
			Suggestion: "Add at least one step to make the workflow functional",
		})
	}

	// Check for workflows with many steps (might be complex)
	if len(def.Steps) > 20 {
		warnings = append(warnings, ValidationError{
			Message:    fmt.Sprintf("Workflow has %d steps, which may be complex to maintain", len(def.Steps)),
			Suggestion: "Consider breaking complex workflows into smaller, reusable workflows",
		})
	}

	return warnings
}
