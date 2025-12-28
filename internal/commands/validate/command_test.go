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

package validate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tombee/conductor/internal/commands/shared"
)

func TestNewCommand(t *testing.T) {
	cmd := NewCommand()

	if cmd.Use != "validate <workflow>" {
		t.Errorf("expected use 'validate <workflow>', got %q", cmd.Use)
	}

	// Check that flags are defined
	if cmd.Flags().Lookup("schema") == nil {
		t.Error("--schema flag not defined")
	}
	// Note: --json flag is global and added by root command, not locally
}

func TestValidateValidWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "valid.yaml")

	validWorkflow := `name: test-workflow
description: Test workflow
version: "1.0"

steps:
  - id: step1
    name: Test Step
    type: llm
    prompt: "Hello"
    inputs:
      model: fast
`

	if err := os.WriteFile(workflowPath, []byte(validWorkflow), 0644); err != nil {
		t.Fatalf("failed to create test workflow: %v", err)
	}

	cmd := NewCommand()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{workflowPath})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected valid workflow to pass, got error: %v\nStdout: %s\nStderr: %s", err, outBuf.String(), errBuf.String())
	}

	// Check stdout for success message (fmt.Println writes to cmd.OutOrStdout())
	output := outBuf.String()
	if output == "" {
		// Try stderr if stdout is empty
		output = errBuf.String()
	}
	if !strings.Contains(output, "[OK]") {
		t.Errorf("expected success message with [OK], got stdout: %q, stderr: %q", outBuf.String(), errBuf.String())
	}
}

func TestValidateInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidWorkflow := `name: test
description: "unclosed string
steps: []
`

	if err := os.WriteFile(workflowPath, []byte(invalidWorkflow), 0644); err != nil {
		t.Fatalf("failed to create test workflow: %v", err)
	}

	cmd := NewCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{workflowPath})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected invalid YAML to fail validation")
	}

	// Check exit code
	if exitErr, ok := err.(*shared.ExitError); ok {
		if exitErr.Code != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.Code)
		}
	}
}

func TestValidateMissingFile(t *testing.T) {
	cmd := NewCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"/nonexistent/file.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected missing file to fail validation")
	}

	// Check exit code
	if exitErr, ok := err.(*shared.ExitError); ok {
		if exitErr.Code != 2 {
			t.Errorf("expected exit code 2 for missing file, got %d", exitErr.Code)
		}
	}
}

func TestValidateJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "valid.yaml")

	validWorkflow := `name: test-workflow
description: Test workflow
version: "1.0"

steps:
  - id: step1
    name: Test Step
    type: llm
    prompt: "Hello"
    inputs:
      model: fast
`

	if err := os.WriteFile(workflowPath, []byte(validWorkflow), 0644); err != nil {
		t.Fatalf("failed to create test workflow: %v", err)
	}

	// Enable JSON output via global flag (simulated)
	shared.SetJSONForTest(true)
	defer shared.SetJSONForTest(false) // Reset after test

	cmd := NewCommand()
	cmd.SetArgs([]string{workflowPath})

	// Verify command succeeds - JSON output goes to os.Stdout
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected valid workflow to pass, got error: %v", err)
	}
}

func TestValidateJSONOutputWithErrors(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "invalid.yaml")

	// Missing required field: name
	invalidWorkflow := `description: Test workflow
version: "1.0"

steps:
  - id: step1
    name: Test Step
    type: llm
    prompt: "Hello"
    inputs:
      model: fast
`

	if err := os.WriteFile(workflowPath, []byte(invalidWorkflow), 0644); err != nil {
		t.Fatalf("failed to create test workflow: %v", err)
	}

	// Enable JSON output via command flag
	cmd := NewCommand()
	cmd.SetArgs([]string{workflowPath, "--json"})

	// Verify command fails as expected - JSON output goes to os.Stdout
	err := cmd.Execute()
	if err == nil {
		t.Error("expected invalid workflow to fail validation")
	}
}

func TestValidateWithCustomSchema(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	schemaPath := filepath.Join(tmpDir, "schema.json")

	workflow := `name: test
description: Test
version: "1.0"
steps:
  - id: step1
    type: llm
    prompt: "Hello"
    inputs:
      model: fast
`

	// Create a minimal schema that requires name field
	schema := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string"}
  }
}`

	if err := os.WriteFile(workflowPath, []byte(workflow), 0644); err != nil {
		t.Fatalf("failed to create test workflow: %v", err)
	}
	if err := os.WriteFile(schemaPath, []byte(schema), 0644); err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}

	cmd := NewCommand()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--schema", schemaPath, workflowPath})

	err := cmd.Execute()
	// This should pass as the workflow has the "name" field
	if err != nil {
		t.Errorf("expected validation to pass with custom schema, got error: %v\nStdout: %s\nStderr: %s", err, outBuf.String(), errBuf.String())
	}
}

func TestValidateErrorFormat(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "invalid.yaml")

	// Workflow missing required field
	invalidWorkflow := `description: Test workflow
version: "1.0"
steps: []
`

	if err := os.WriteFile(workflowPath, []byte(invalidWorkflow), 0644); err != nil {
		t.Fatalf("failed to create test workflow: %v", err)
	}

	cmd := NewCommand()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{workflowPath})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected invalid workflow to fail validation")
	}

	output := errBuf.String()
	// Error output should include file path
	if !strings.Contains(output, workflowPath) {
		t.Errorf("expected error output to contain file path, got: %s", output)
	}
	// Error output should include "error:"
	if !strings.Contains(output, "error:") {
		t.Errorf("expected error output to contain 'error:', got: %s", output)
	}
}

func TestExtractModelTiers(t *testing.T) {
	tests := []struct {
		name     string
		workflow string
		expected []string
	}{
		{
			name: "single model tier",
			workflow: `name: test
description: Test
version: "1.0"
steps:
  - id: step1
    type: llm
    prompt: "Hello"
    inputs:
      model: fast
`,
			expected: []string{"fast"},
		},
		{
			name: "multiple model tiers",
			workflow: `name: test
description: Test
version: "1.0"
steps:
  - id: step1
    type: llm
    prompt: "Hello"
    inputs:
      model: fast
  - id: step2
    type: llm
    prompt: "World"
    inputs:
      model: advanced
`,
			expected: []string{"fast", "advanced"},
		},
		{
			name: "no LLM steps",
			workflow: `name: test
description: Test
version: "1.0"
steps:
  - id: step1
    type: http
    inputs:
      url: "https://example.com"
`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			workflowPath := filepath.Join(tmpDir, "workflow.yaml")

			if err := os.WriteFile(workflowPath, []byte(tt.workflow), 0644); err != nil {
				t.Fatalf("failed to create test workflow: %v", err)
			}

			cmd := NewCommand()
			var outBuf bytes.Buffer
			cmd.SetOut(&outBuf)
			cmd.SetErr(&outBuf)
			cmd.SetArgs([]string{workflowPath})

			// Execute command
			_ = cmd.Execute()

			output := outBuf.String()
			for _, tier := range tt.expected {
				if !strings.Contains(output, tier) {
					t.Errorf("expected output to contain model tier %q, got: %s", tier, output)
				}
			}
		})
	}
}
