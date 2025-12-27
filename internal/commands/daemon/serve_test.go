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

package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tombee/conductor/internal/commands/shared"
)

func TestNewServeCommand(t *testing.T) {
	cmd := NewServeCommand()

	if cmd.Use != "serve" {
		t.Errorf("expected use 'serve', got %q", cmd.Use)
	}

	// Check that --port flag is defined
	if cmd.Flags().Lookup("port") == nil {
		t.Error("--port flag not defined")
	}
}

func TestServeCommand_PortFlagParsing(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "default port",
			args: []string{},
		},
		{
			name: "custom port",
			args: []string{"--port", "8080"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewServeCommand()
			cmd.SetArgs(tt.args)

			// Parse flags only (don't execute)
			err := cmd.ParseFlags(tt.args)
			if err != nil {
				t.Errorf("flag parsing failed: %v", err)
			}

			// Verify port flag was parsed
			portFlag := cmd.Flags().Lookup("port")
			if portFlag == nil {
				t.Error("--port flag not found after parsing")
			}
		})
	}
}

func TestServeCommand_MissingConfig(t *testing.T) {
	// Skip if running short tests (integration test)
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp directory for non-existent config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yaml")

	// Override config path
	shared.SetConfigPathForTest(configPath)
	defer func() { shared.SetConfigPathForTest("") }()

	cmd := NewServeCommand()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when config file is missing")
	}

	// Should mention configuration in error
	if !strings.Contains(err.Error(), "config") {
		t.Errorf("expected error about config, got: %v", err)
	}
}

func TestServeCommand_InvalidConfig(t *testing.T) {
	// Skip if running short tests (integration test)
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp directory with invalid config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write invalid YAML
	invalidConfig := `default_provider: test
providers:
  test
    type: invalid
`
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0600); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Override config path
	shared.SetConfigPathForTest(configPath)
	defer func() { shared.SetConfigPathForTest("") }()

	cmd := NewServeCommand()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when config file is invalid")
	}
}

func TestServeCommand_ValidConfig(t *testing.T) {
	// Skip if running short tests (integration test requiring server startup)
	if testing.Short() {
		t.Skip("skipping server startup test in short mode")
	}

	// Create temp directory with valid config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write minimal valid config
	validConfig := `default_provider: claude-code
providers:
  claude-code:
    type: claude-code
server:
  port: 0  # Use random available port
log:
  level: info
  format: text
`
	if err := os.WriteFile(configPath, []byte(validConfig), 0600); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Override config path
	shared.SetConfigPathForTest(configPath)
	defer func() { shared.SetConfigPathForTest("") }()

	// Note: This test would start a server, which we skip in unit tests
	// The test is here to document the expected behavior
	t.Skip("skipping actual server startup - requires graceful shutdown mechanism")
}
