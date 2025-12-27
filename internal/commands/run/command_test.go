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

package run

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

	if cmd.Use != "run <workflow>" {
		t.Errorf("expected use 'run <workflow>', got %q", cmd.Use)
	}

	// Check that key flags are defined
	expectedFlags := []string{"input", "input-file", "dry-run", "quiet", "verbose", "help-inputs"}
	for _, flag := range expectedFlags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("--%s flag not defined", flag)
		}
	}
}

func TestRunCommand_MissingWorkflowArg(t *testing.T) {
	cmd := NewCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when workflow argument is missing")
	}

	// Cobra's error for missing args should mention "accepts 1 arg(s)"
	if !strings.Contains(err.Error(), "accepts 1 arg(s)") && !strings.Contains(err.Error(), "required") {
		t.Errorf("expected missing argument error, got: %v", err)
	}
}

func TestRunCommand_NonexistentWorkflowFile(t *testing.T) {
	cmd := NewCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"/nonexistent/workflow.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent workflow file")
	}

	// Check exit code
	if exitErr, ok := err.(*shared.ExitError); ok {
		if exitErr.Code != shared.ExitInvalidWorkflow {
			t.Errorf("expected exit code %d, got %d", shared.ExitInvalidWorkflow, exitErr.Code)
		}
	}
}

func TestRunCommand_InvalidYAML(t *testing.T) {
	// Create temp directory with invalid YAML file
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `name: test
description: "unclosed string
steps: []
`
	if err := os.WriteFile(workflowPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd := NewCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{workflowPath})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}

	// Check exit code
	if exitErr, ok := err.(*shared.ExitError); ok {
		if exitErr.Code != shared.ExitInvalidWorkflow {
			t.Errorf("expected exit code %d, got %d", shared.ExitInvalidWorkflow, exitErr.Code)
		}
	}
}

func TestRunCommand_DryRun(t *testing.T) {
	cmd := NewCommand()
	// Parse flags only - don't execute
	err := cmd.ParseFlags([]string{"--dry-run", "../testdata/valid_workflow.yaml"})
	if err != nil {
		t.Errorf("--dry-run flag parsing failed: %v", err)
	}

	// Verify flag was set
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("--dry-run flag not defined")
	}
}

func TestRunCommand_HelpInputs(t *testing.T) {
	cmd := NewCommand()
	// Parse flags only - don't execute
	err := cmd.ParseFlags([]string{"--help-inputs", "../testdata/with_inputs.yaml"})
	if err != nil {
		t.Errorf("--help-inputs flag parsing failed: %v", err)
	}

	// Verify flag was set
	helpInputsFlag := cmd.Flags().Lookup("help-inputs")
	if helpInputsFlag == nil {
		t.Error("--help-inputs flag not defined")
	}
}

func TestRunCommand_InputFlagParsing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "single input",
			args:    []string{"--input", "key=value"},
			wantErr: false,
		},
		{
			name:    "multiple inputs",
			args:    []string{"--input", "key1=value1", "--input", "key2=value2"},
			wantErr: false,
		},
		{
			name:    "short flag",
			args:    []string{"-i", "key=value"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand()
			// Parse flags only - don't execute
			args := append(tt.args, "../testdata/valid_workflow.yaml")
			err := cmd.ParseFlags(args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunCommand_ConflictingFlags(t *testing.T) {
	// Test that --quiet and --verbose can both be parsed (no flag conflict at parse time)
	cmd := NewCommand()
	// Parse flags only - don't execute
	err := cmd.ParseFlags([]string{"--quiet", "--verbose", "../testdata/valid_workflow.yaml"})
	if err != nil {
		errMsg := err.Error()
		// Should not be a flag-related error
		if strings.Contains(errMsg, "conflicting") || strings.Contains(errMsg, "mutually exclusive") {
			t.Errorf("unexpected flag conflict: %v", err)
		}
	}
}

func TestRunCommand_InputFileFlag(t *testing.T) {
	// Create temp input file
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "inputs.json")
	inputData := `{"key": "value"}`
	if err := os.WriteFile(inputFile, []byte(inputData), 0644); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	cmd := NewCommand()
	// Parse flags only - don't execute
	err := cmd.ParseFlags([]string{"--input-file", inputFile, "../testdata/valid_workflow.yaml"})
	if err != nil {
		t.Errorf("--input-file flag parsing failed: %v", err)
	}

	// Verify flag was set
	inputFileFlag := cmd.Flags().Lookup("input-file")
	if inputFileFlag == nil {
		t.Error("--input-file flag not defined")
	}
}

func TestRunCommand_DaemonFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "daemon flag",
			args: []string{"--daemon"},
		},
		{
			name: "daemon short flag",
			args: []string{"-d"},
		},
		{
			name: "background flag",
			args: []string{"--background"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand()
			// Parse flags only - don't execute
			args := append(tt.args, "../testdata/valid_workflow.yaml")
			err := cmd.ParseFlags(args)
			if err != nil {
				t.Errorf("daemon flag not recognized: %v", err)
			}
		})
	}
}

func TestRunCommand_SecurityFlags(t *testing.T) {
	cmd := NewCommand()
	// Parse flags only - don't execute
	err := cmd.ParseFlags([]string{
		"--security", "strict",
		"--allow-hosts", "example.com",
		"--allow-paths", "/tmp",
		"../testdata/valid_workflow.yaml",
	})
	if err != nil {
		t.Errorf("security flags not recognized: %v", err)
	}

	// Verify flags exist
	for _, flag := range []string{"security", "allow-hosts", "allow-paths"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("--%s flag not defined", flag)
		}
	}
}
