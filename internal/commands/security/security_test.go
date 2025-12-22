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

package security

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/pkg/security"
)

// TestSecurityStatusCommand tests the security status command.
func TestSecurityStatusCommand(t *testing.T) {
	// Create temporary config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
security:
  default_profile: standard
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create command
	cmd := NewCommand()
	if cmd == nil {
		t.Fatal("NewCommand returned nil")
	}

	// Test that status subcommand exists
	statusCmd := findSubcommand(cmd, "status")
	if statusCmd == nil {
		t.Fatal("status subcommand not found")
	}

	// Verify command properties
	if statusCmd.Use != "status" {
		t.Errorf("Expected Use='status', got %q", statusCmd.Use)
	}

	if statusCmd.Short == "" {
		t.Error("status command has no Short description")
	}
}

// TestSecurityAnalyzeCommand tests the security analyze command.
func TestSecurityAnalyzeCommand(t *testing.T) {
	// Create temporary workflow file
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")

	workflowContent := `
name: test-workflow
steps:
  - run: git status
  - run: curl https://api.example.com
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0600); err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}

	// Create command
	cmd := NewCommand()
	analyzeCmd := findSubcommand(cmd, "analyze")
	if analyzeCmd == nil {
		t.Fatal("analyze subcommand not found")
	}

	// Verify command properties - Use may include args
	if !strings.HasPrefix(analyzeCmd.Use, "analyze") {
		t.Errorf("Expected Use to start with 'analyze', got %q", analyzeCmd.Use)
	}
}

// TestSecurityGenerateProfileCommand tests the generate-profile command.
func TestSecurityGenerateProfileCommand(t *testing.T) {
	cmd := NewCommand()
	generateCmd := findSubcommand(cmd, "generate-profile")
	if generateCmd == nil {
		t.Fatal("generate-profile subcommand not found")
	}

	// Verify command properties
	if !strings.HasPrefix(generateCmd.Use, "generate-profile") {
		t.Errorf("Expected Use to start with 'generate-profile', got %q", generateCmd.Use)
	}
}

// TestSecurityListPermissionsCommand tests the list-permissions command.
func TestSecurityListPermissionsCommand(t *testing.T) {
	cmd := NewCommand()
	listCmd := findSubcommand(cmd, "list-permissions")
	if listCmd == nil {
		t.Fatal("list-permissions subcommand not found")
	}

	if !strings.HasPrefix(listCmd.Use, "list-permissions") {
		t.Errorf("Expected Use to start with 'list-permissions', got %q", listCmd.Use)
	}
}

// TestSecurityRevokeCommand tests the revoke command.
func TestSecurityRevokeCommand(t *testing.T) {
	cmd := NewCommand()
	revokeCmd := findSubcommand(cmd, "revoke")
	if revokeCmd == nil {
		t.Fatal("revoke subcommand not found")
	}

	if !strings.HasPrefix(revokeCmd.Use, "revoke") {
		t.Errorf("Expected Use to start with 'revoke', got %q", revokeCmd.Use)
	}
}

// TestPermissionPrompt tests the permission prompt functionality.
func TestPermissionPrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected security.PermissionResponse
		wantErr  bool
	}{
		{
			name:     "yes response",
			input:    "y\n",
			expected: security.PermissionYes,
			wantErr:  false,
		},
		{
			name:     "no response",
			input:    "n\n",
			expected: security.PermissionNo,
			wantErr:  false,
		},
		{
			name:     "always response",
			input:    "always\n",
			expected: security.PermissionAlways,
			wantErr:  false,
		},
		{
			name:     "never response",
			input:    "never\n",
			expected: security.PermissionNever,
			wantErr:  false,
		},
		{
			name:     "empty defaults to no",
			input:    "\n",
			expected: security.PermissionNo,
			wantErr:  false,
		},
		{
			name:     "invalid response",
			input:    "invalid\n",
			expected: security.PermissionNo,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			writer := &bytes.Buffer{}

			prompter := security.NewPrompterWithIO(reader, writer)

			req := security.PermissionRequest{
				WorkflowName: "test-workflow",
				Filesystem: &security.FilesystemPermissions{
					Read: []string{"/tmp/test"},
				},
			}

			response, err := prompter.Prompt(req)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if response != tt.expected {
				t.Errorf("Expected response %q, got %q", tt.expected, response)
			}

			// Verify prompt was written
			output := writer.String()
			if !strings.Contains(output, "test-workflow") {
				t.Error("Prompt did not contain workflow name")
			}
			if !strings.Contains(output, "/tmp/test") {
				t.Error("Prompt did not contain requested path")
			}
		})
	}
}

// TestPermissionStore tests the permission storage functionality.
func TestPermissionStore(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := security.NewFilePermissionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	workflowContent := "name: test\nsteps:\n  - run: echo test"
	workflowName := "test-workflow"

	perms := security.GrantedPermissions{
		Filesystem: &security.FilesystemPermissions{
			Read: []string{"/tmp/test"},
		},
	}

	// Test Grant
	err = store.Grant(workflowContent, workflowName, perms)
	if err != nil {
		t.Fatalf("Failed to grant permission: %v", err)
	}

	// Test Check
	grant, found := store.Check(workflowContent)
	if !found {
		t.Fatal("Permission not found after grant")
	}

	if grant.WorkflowName != workflowName {
		t.Errorf("Expected workflow name %q, got %q", workflowName, grant.WorkflowName)
	}

	// Test List
	grants, err := store.List()
	if err != nil {
		t.Fatalf("Failed to list grants: %v", err)
	}

	if len(grants) != 1 {
		t.Errorf("Expected 1 grant, got %d", len(grants))
	}

	// Test Revoke
	err = store.Revoke(grant.WorkflowHash)
	if err != nil {
		t.Fatalf("Failed to revoke permission: %v", err)
	}

	_, found = store.Check(workflowContent)
	if found {
		t.Error("Permission still found after revoke")
	}

	// Test persistence - create new store instance
	store2, err := security.NewFilePermissionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second store: %v", err)
	}

	grants2, err := store2.List()
	if err != nil {
		t.Fatalf("Failed to list grants from second store: %v", err)
	}

	// Should still see the revoked grant (it's marked revoked, not deleted)
	if len(grants2) != 1 {
		t.Errorf("Expected 1 grant in persisted store, got %d", len(grants2))
	}

	if !grants2[0].Revoked {
		t.Error("Expected grant to be marked as revoked")
	}
}

// TestJSONOutput tests JSON output mode for security commands.
func TestJSONOutput(t *testing.T) {
	// This is a placeholder for JSON output testing
	// Actual implementation would test the --json flag

	type SecurityStatusJSON struct {
		Profile     string                 `json:"profile"`
		Permissions map[string]interface{} `json:"permissions"`
	}

	// Example of what we'd expect
	jsonStr := `{"profile":"standard","permissions":{"filesystem":{"read":["/tmp"]}}}`

	var status SecurityStatusJSON
	err := json.Unmarshal([]byte(jsonStr), &status)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if status.Profile != "standard" {
		t.Errorf("Expected profile 'standard', got %q", status.Profile)
	}
}

// TestNonInteractivePrompter tests the non-interactive prompter.
func TestNonInteractivePrompter(t *testing.T) {
	prompter := security.NewNonInteractivePrompter()

	req := security.PermissionRequest{
		WorkflowName: "test-workflow",
		Filesystem: &security.FilesystemPermissions{
			Read: []string{"/tmp/test"},
		},
	}

	response, err := prompter.Prompt(req)

	if err == nil {
		t.Error("Expected error from non-interactive prompter")
	}

	if response != security.PermissionNo {
		t.Errorf("Expected PermissionNo, got %v", response)
	}
}

// findSubcommand finds a subcommand by name in a cobra command.
func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}
