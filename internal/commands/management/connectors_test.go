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

package management

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestConnectorsListCommand(t *testing.T) {
	// Test that the command is created correctly
	cmd := NewConnectorsCommand()
	if cmd == nil {
		t.Fatal("NewConnectorsCommand() returned nil")
	}

	if cmd.Use != "connectors" {
		t.Errorf("expected Use to be 'connectors', got %q", cmd.Use)
	}

	// Check subcommands exist
	subcommands := cmd.Commands()
	if len(subcommands) < 3 {
		t.Errorf("expected at least 3 subcommands, got %d", len(subcommands))
	}

	// Verify list command exists
	var listCmd *cobra.Command
	for _, sub := range subcommands {
		if sub.Use == "list [workflow]" {
			listCmd = sub
			break
		}
	}

	if listCmd == nil {
		t.Error("list subcommand not found")
	}
}

func TestConnectorsShowCommand(t *testing.T) {
	// Test that show command exists
	cmd := NewConnectorsCommand()
	var showCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if strings.HasPrefix(sub.Use, "show") {
			showCmd = sub
			break
		}
	}

	if showCmd == nil {
		t.Fatal("show subcommand not found")
	}

	if !strings.HasPrefix(showCmd.Use, "show <name>") {
		t.Errorf("expected Use to start with 'show <name>', got %q", showCmd.Use)
	}
}

func TestConnectorsOperationCommand(t *testing.T) {
	// Test that operation command exists
	cmd := NewConnectorsCommand()
	var opCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if strings.HasPrefix(sub.Use, "operation") {
			opCmd = sub
			break
		}
	}

	if opCmd == nil {
		t.Fatal("operation subcommand not found")
	}

	if !strings.HasPrefix(opCmd.Use, "operation <connector.operation>") {
		t.Errorf("expected Use to start with 'operation <connector.operation>', got %q", opCmd.Use)
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"file", "fiel", 2},    // delete 'e' and move 'l'
		{"github", "gitlab", 2}, // substitute 'u' with 'l', 'b' with 'a'
		{"shell", "shelf", 1},   // substitute 'l' with 'f'
		{"transform", "transfer", 2}, // delete 'o', substitute 'm' with 'e'
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := levenshteinDistance(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestSuggestConnectors(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		workflowYAML string
		wantContains []string
		maxResults   int
	}{
		{
			name:         "suggest for typo in file",
			input:        "fiel",
			wantContains: []string{"file"},
			maxResults:   3, // Allow up to 3 suggestions
		},
		{
			name:         "suggest for partial match",
			input:        "shel",
			wantContains: []string{"shell"},
			maxResults:   1,
		},
		{
			name:         "suggest multiple options",
			input:        "http",
			wantContains: []string{"http"},
			maxResults:   1,
		},
		{
			name:  "suggest from configured connectors",
			input: "githu", // typo (distance 1 instead of 2)
			workflowYAML: `
name: test
connectors:
  github:
    base_url: https://api.github.com
    operations:
      test:
        method: GET
        path: /test
steps:
  - id: dummy
    type: llm
    prompt: test
`,
			wantContains: []string{"github"},
			maxResults:   3,
		},
		{
			name:         "no suggestions for very different input",
			input:        "verydifferent",
			wantContains: []string{},
			maxResults:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var workflowPath string
			if tt.workflowYAML != "" {
				tmpDir := t.TempDir()
				workflowPath = filepath.Join(tmpDir, "workflow.yaml")
				if err := os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0644); err != nil {
					t.Fatalf("failed to write workflow file: %v", err)
				}
			}

			suggestions := suggestConnectors(tt.input, workflowPath)

			if len(suggestions) > 3 {
				t.Errorf("expected at most 3 suggestions, got %d", len(suggestions))
			}

			for _, want := range tt.wantContains {
				found := false
				for _, suggestion := range suggestions {
					if suggestion == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("suggestions missing %q, got: %v", want, suggestions)
				}
			}

			if tt.maxResults > 0 && len(suggestions) > tt.maxResults {
				t.Errorf("expected at most %d suggestions, got %d", tt.maxResults, len(suggestions))
			}
		})
	}
}

func TestGetBuiltinOperationMetadata(t *testing.T) {
	// Test file.read description
	desc := getBuiltinOperationDescription("file", "read")
	if desc == "" {
		t.Error("expected description for file.read, got empty string")
	}

	// Test file.read parameters
	params := getBuiltinOperationParameters("file", "read")
	if len(params) == 0 {
		t.Error("expected parameters for file.read, got none")
	}

	// Check path parameter exists and is required
	foundPath := false
	for _, param := range params {
		if param.Name == "path" {
			foundPath = true
			if !param.Required {
				t.Error("expected path parameter to be required")
			}
		}
	}
	if !foundPath {
		t.Error("expected path parameter in file.read operation")
	}

	// Test file.read examples
	examples := getBuiltinOperationExamples("file", "read")
	if len(examples) == 0 {
		t.Error("expected examples for file.read, got none")
	}
}

func TestGetOperationInfoBuiltin(t *testing.T) {
	// Test getting file.read operation info
	info, err := getOperationInfo("file", "read", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Connector != "file" {
		t.Errorf("expected connector 'file', got %q", info.Connector)
	}

	if info.Operation != "read" {
		t.Errorf("expected operation 'read', got %q", info.Operation)
	}

	if len(info.Parameters) == 0 {
		t.Error("expected parameters, got none")
	}
}

func TestGetOperationInfoConfigured(t *testing.T) {
	// Create temp workflow file
	workflowYAML := `
name: test-workflow
connectors:
  github:
    base_url: https://api.github.com
    auth:
      type: bearer
      token: ${GITHUB_TOKEN}
    operations:
      create_issue:
        method: POST
        path: /repos/{owner}/{repo}/issues
        request_schema:
          type: object
          properties:
            owner:
              type: string
            repo:
              type: string
          required:
            - owner
            - repo
steps:
  - id: dummy
    type: llm
    prompt: test
`
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	// Test getting configured operation info
	info, err := getOperationInfo("github", "create_issue", workflowPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Connector != "github" {
		t.Errorf("expected connector 'github', got %q", info.Connector)
	}

	if info.Operation != "create_issue" {
		t.Errorf("expected operation 'create_issue', got %q", info.Operation)
	}

	if !strings.Contains(info.Description, "POST") {
		t.Errorf("expected description to contain 'POST', got %q", info.Description)
	}
}
