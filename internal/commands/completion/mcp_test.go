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

package completion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteMCPServerNames(t *testing.T) {
	// Create a temporary config directory with conductor subdirectory
	tmpDir := t.TempDir()
	conductorDir := filepath.Join(tmpDir, "conductor")
	if err := os.MkdirAll(conductorDir, 0700); err != nil {
		t.Fatalf("failed to create test config dir: %v", err)
	}
	mcpConfigPath := filepath.Join(conductorDir, "mcp.yaml")

	// Write a test MCP config with servers
	mcpConfig := `servers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem"]
  github:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
  postgres:
    command: docker
    args: ["run", "mcp-postgres"]
`
	if err := os.WriteFile(mcpConfigPath, []byte(mcpConfig), 0600); err != nil {
		t.Fatalf("failed to write test MCP config: %v", err)
	}

	// Set XDG_CONFIG_HOME to use test config dir
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	completions, directive := CompleteMCPServerNames(nil, nil, "")

	if len(completions) != 3 {
		t.Errorf("expected 3 server names, got %d: %v", len(completions), completions)
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}

	// Verify expected server names are present
	expectedServers := map[string]bool{
		"filesystem": false,
		"github":     false,
		"postgres":   false,
	}

	for _, name := range completions {
		if _, ok := expectedServers[name]; ok {
			expectedServers[name] = true
		}
	}

	for server, found := range expectedServers {
		if !found {
			t.Errorf("expected server %q not found in completions", server)
		}
	}
}

func TestCompleteMCPServerNames_NoConfig(t *testing.T) {
	// Create a temporary directory with no MCP config
	tmpDir := t.TempDir()

	// Set environment to use test config dir
	t.Setenv("CONDUCTOR_CONFIG_DIR", tmpDir)

	completions, directive := CompleteMCPServerNames(nil, nil, "")

	if len(completions) != 0 {
		t.Errorf("expected 0 completions when no config exists, got %d", len(completions))
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}
}

func TestCompleteMCPServerNames_EmptyConfig(t *testing.T) {
	// Create a temporary config directory with conductor subdirectory
	tmpDir := t.TempDir()
	conductorDir := filepath.Join(tmpDir, "conductor")
	if err := os.MkdirAll(conductorDir, 0700); err != nil {
		t.Fatalf("failed to create test config dir: %v", err)
	}
	mcpConfigPath := filepath.Join(conductorDir, "mcp.yaml")

	// Write an empty MCP config
	mcpConfig := `servers: {}
`
	if err := os.WriteFile(mcpConfigPath, []byte(mcpConfig), 0600); err != nil {
		t.Fatalf("failed to write test MCP config: %v", err)
	}

	// Set XDG_CONFIG_HOME to use test config dir
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	completions, directive := CompleteMCPServerNames(nil, nil, "")

	if len(completions) != 0 {
		t.Errorf("expected 0 completions with empty servers map, got %d", len(completions))
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}
}

func TestCompleteMCPServerNames_BadPermissions(t *testing.T) {
	// Create a temporary config directory with conductor subdirectory
	tmpDir := t.TempDir()
	conductorDir := filepath.Join(tmpDir, "conductor")
	if err := os.MkdirAll(conductorDir, 0700); err != nil {
		t.Fatalf("failed to create test config dir: %v", err)
	}
	mcpConfigPath := filepath.Join(conductorDir, "mcp.yaml")

	// Write a test MCP config
	mcpConfig := `servers:
  test-server:
    command: echo
`
	if err := os.WriteFile(mcpConfigPath, []byte(mcpConfig), 0644); err != nil {
		t.Fatalf("failed to write test MCP config: %v", err)
	}

	// Set XDG_CONFIG_HOME to use test config dir
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	completions, directive := CompleteMCPServerNames(nil, nil, "")

	// Should return empty list due to bad permissions
	if len(completions) != 0 {
		t.Errorf("expected 0 completions with bad permissions, got %d", len(completions))
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}
}
