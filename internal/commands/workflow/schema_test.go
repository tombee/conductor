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

package workflow

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tombee/conductor/internal/commands/shared"
	"gopkg.in/yaml.v3"
)

func TestSchemaCommand(t *testing.T) {
	cmd := NewSchemaCommand()

	if cmd.Use != "schema" {
		t.Errorf("expected use 'schema', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected short description to be set")
	}
}

func TestSchemaJSONOutput(t *testing.T) {
	cmd := NewSchemaCommand()

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schema command failed: %v", err)
	}

	// Verify output is valid JSON
	var schema map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, buf.String())
	}

	// Verify schema has expected fields
	if _, ok := schema["$schema"]; !ok {
		t.Error("expected schema to have $schema field")
	}
	if _, ok := schema["$id"]; !ok {
		t.Error("expected schema to have $id field")
	}
	if title, ok := schema["title"].(string); !ok || title != "Conductor Workflow Definition" {
		t.Errorf("expected title 'Conductor Workflow Definition', got %q", title)
	}
}

func TestSchemaYAMLOutput(t *testing.T) {
	cmd := NewSchemaCommand()

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--output", "yaml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schema command failed: %v", err)
	}

	// Verify output is valid YAML
	var schema map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("failed to parse YAML output: %v\nOutput: %s", err, buf.String())
	}

	// Verify schema has expected fields
	if _, ok := schema["$schema"]; !ok {
		t.Error("expected schema to have $schema field")
	}
}

func TestSchemaInvalidOutputFormat(t *testing.T) {
	cmd := NewSchemaCommand()

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--output", "xml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}

	// Verify it's an ExitError with code 2
	if exitErr, ok := err.(*shared.ExitError); ok {
		if exitErr.Code != 2 {
			t.Errorf("expected exit code 2 for invalid format, got %d", exitErr.Code)
		}
	} else {
		t.Errorf("expected ExitError, got %T", err)
	}
}

func TestSchemaWriteToFile(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	cmd := NewSchemaCommand()

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--write"})

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schema command failed: %v", err)
	}

	// Verify file was created
	schemaPath := filepath.Join(tmpDir, "schemas", "workflow.schema.json")
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		t.Errorf("expected schema file to be created at %s", schemaPath)
	}

	// Verify file contains valid JSON
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read schema file: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema file is not valid JSON: %v", err)
	}
}

func TestSchemaWriteToFileExistingFile(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create existing schema file
	schemaDir := filepath.Join(tmpDir, "schemas")
	os.MkdirAll(schemaDir, 0755)
	schemaPath := filepath.Join(schemaDir, "workflow.schema.json")
	os.WriteFile(schemaPath, []byte("{}"), 0644)

	cmd := NewSchemaCommand()

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--write"})

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when file exists without --force")
	}

	// Verify it's an ExitError with code 1
	if exitErr, ok := err.(*shared.ExitError); ok {
		if exitErr.Code != 1 {
			t.Errorf("expected exit code 1 for file exists, got %d", exitErr.Code)
		}
	} else {
		t.Errorf("expected ExitError, got %T", err)
	}
}

func TestSchemaWriteToFileWithForce(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create existing schema file
	schemaDir := filepath.Join(tmpDir, "schemas")
	os.MkdirAll(schemaDir, 0755)
	schemaPath := filepath.Join(schemaDir, "workflow.schema.json")
	os.WriteFile(schemaPath, []byte("{}"), 0644)

	cmd := NewSchemaCommand()

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--write", "--force"})

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("schema command failed: %v", err)
	}

	// Verify file was overwritten with valid schema
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read schema file: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema file is not valid JSON: %v", err)
	}

	// Verify it's the real schema, not just "{}"
	if _, ok := schema["$schema"]; !ok {
		t.Error("expected schema to have $schema field after overwrite")
	}
}
