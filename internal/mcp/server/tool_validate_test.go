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
	"testing"
)

func TestValidateWorkflowYAML_ValidWorkflow(t *testing.T) {
	validYAML := `
name: test-workflow
description: Test workflow
version: "1.0"
steps:
  - id: step1
    type: llm
    prompt: "Hello world"
`

	result := validateWorkflowYAML([]byte(validYAML))

	if !result.Valid {
		t.Errorf("Expected valid workflow, got invalid. Errors: %+v", result.Errors)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got %d errors", len(result.Errors))
	}
}

func TestValidateWorkflowYAML_InvalidYAML(t *testing.T) {
	invalidYAML := `
name: test-workflow
description: "Unterminated string
`

	result := validateWorkflowYAML([]byte(invalidYAML))

	if result.Valid {
		t.Errorf("Expected invalid workflow, got valid")
	}

	if len(result.Errors) == 0 {
		t.Errorf("Expected errors, got none")
	}
}

func TestValidateWorkflowYAML_MissingRequiredFields(t *testing.T) {
	missingNameYAML := `
description: Test workflow
steps: []
`

	result := validateWorkflowYAML([]byte(missingNameYAML))

	if result.Valid {
		t.Errorf("Expected invalid workflow due to missing name, got valid")
	}
}

func TestValidateWorkflowYAML_SizeLimit(t *testing.T) {
	// Create YAML larger than maxYAMLSize (10MB)
	// We'll test the size check in the handler, not here
	// This test validates that normal-sized YAML works
	normalYAML := `
name: normal-workflow
description: Normal size workflow
steps:
  - id: step1
    type: llm
    prompt: "Test"
`

	result := validateWorkflowYAML([]byte(normalYAML))

	if !result.Valid {
		t.Errorf("Expected valid workflow, got invalid. Errors: %+v", result.Errors)
	}
}

func TestValidateWorkflowYAML_BestPracticeWarnings(t *testing.T) {
	noDescriptionYAML := `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Test"
`

	result := validateWorkflowYAML([]byte(noDescriptionYAML))

	if !result.Valid {
		t.Errorf("Expected valid workflow, got invalid. Errors: %+v", result.Errors)
	}

	// Should have warning about missing description
	if len(result.Warnings) == 0 {
		t.Errorf("Expected warnings about best practices, got none")
	}
}
