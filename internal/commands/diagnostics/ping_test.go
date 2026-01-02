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

package diagnostics

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

func TestPingCommand(t *testing.T) {
	// Skip if SKIP_SPAWN_TESTS is set - ping runs Claude CLI health check
	if os.Getenv("SKIP_SPAWN_TESTS") != "" {
		t.Skip("skipping test that requires spawning external processes")
	}
	// Skip tests that call os.Exit() in short mode
	if testing.Short() {
		t.Skip("Skipping ping tests in short mode (calls os.Exit)")
	}

	tests := []struct {
		name        string
		setupConfig string
		args        []string
		wantErr     bool
	}{
		{
			name:        "no config file",
			setupConfig: "",
			args:        []string{},
			wantErr:     false, // Falls back to claude-code when no config exists
		},
		{
			name: "ping default provider",
			setupConfig: `default_provider: claude-code
providers:
  claude-code:
    type: claude-code
`,
			args:    []string{},
			wantErr: false, // Command runs, may exit with code 1 if not healthy, but no command error
		},
		{
			name: "ping specific provider",
			setupConfig: `default_provider: claude-code
providers:
  claude-code:
    type: claude-code
  test-provider:
    type: claude-code
`,
			args:    []string{"test-provider"},
			wantErr: false,
		},
		{
			name: "ping unknown provider",
			setupConfig: `default_provider: claude-code
providers:
  claude-code:
    type: claude-code
`,
			args:    []string{"unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for config
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			// Set up config if provided
			if tt.setupConfig != "" {
				if err := os.WriteFile(configPath, []byte(tt.setupConfig), 0600); err != nil {
					t.Fatalf("Failed to write test config: %v", err)
				}
			}

			// Always override config path (even for "no config" test)
			shared.SetConfigPathForTest(configPath)
			defer func() { shared.SetConfigPathForTest("") }()

			// Create and run command
			cmd := NewPingCommand()
			cmd.SetArgs(tt.args)

			// Capture exit calls by checking error
			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("ping command error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPingJSONOutput(t *testing.T) {
	// Skip if SKIP_SPAWN_TESTS is set - ping runs Claude CLI health check
	if os.Getenv("SKIP_SPAWN_TESTS") != "" {
		t.Skip("skipping test that requires spawning external processes")
	}
	// Skip tests that call os.Exit() in short mode
	if testing.Short() {
		t.Skip("Skipping ping tests in short mode (calls os.Exit)")
	}

	// Create temp directory for config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	config := `default_provider: claude-code
providers:
  claude-code:
    type: claude-code
`
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Override config path
	shared.SetConfigPathForTest(configPath)
	defer func() { shared.SetConfigPathForTest("") }()

	// Create root command with --json flag
	rootCmd := &cobra.Command{Use: "test"}
	_, _, jsonPtr, _, _ := shared.RegisterFlagPointers()
	rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")

	cmd := NewPingCommand()
	rootCmd.AddCommand(cmd)

	// Run with --json flag
	rootCmd.SetArgs([]string{"ping", "--json"})

	// Should not panic
	_ = rootCmd.Execute()
}

func TestPingProviderFlag(t *testing.T) {
	// Skip if SKIP_SPAWN_TESTS is set - ping runs Claude CLI health check
	if os.Getenv("SKIP_SPAWN_TESTS") != "" {
		t.Skip("skipping test that requires spawning external processes")
	}
	// Skip tests that call os.Exit() in short mode
	if testing.Short() {
		t.Skip("Skipping ping tests in short mode (calls os.Exit)")
	}

	// Create temp directory for config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	config := `default_provider: default-provider
providers:
  default-provider:
    type: claude-code
  other-provider:
    type: claude-code
`
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Override config path
	shared.SetConfigPathForTest(configPath)
	defer func() { shared.SetConfigPathForTest("") }()

	// Test with --provider flag
	cmd := NewPingCommand()
	cmd.SetArgs([]string{"--provider", "other-provider"})

	err := cmd.Execute()
	// Should not return error (may exit with code 1 if unhealthy)
	if err != nil {
		t.Errorf("ping command with --provider flag failed: %v", err)
	}
}
