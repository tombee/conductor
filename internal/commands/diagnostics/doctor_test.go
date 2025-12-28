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

func TestDoctorCommand(t *testing.T) {
	tests := []struct {
		name           string
		setupConfig    string
		wantErr        bool
		expectHealthy  bool
		skipIfNoSpawn  bool // Skip if SKIP_SPAWN_TESTS is set (runs external processes)
	}{
		{
			name:           "no config file",
			setupConfig:    "",
			wantErr:        true, // Should fail if no config is set up
			expectHealthy:  false,
		},
		{
			name: "valid config with claude-code",
			setupConfig: `default_provider: claude-code
providers:
  claude-code:
    type: claude-code
`,
			wantErr:       false, // Succeeds when Claude CLI is installed and authenticated
			expectHealthy: true,
			skipIfNoSpawn: true, // Requires running Claude CLI process
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require spawning external processes
			if tt.skipIfNoSpawn && os.Getenv("SKIP_SPAWN_TESTS") != "" {
				t.Skip("skipping test that requires spawning external processes")
			}

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
			cmd := NewDoctorCommand()
			cmd.SetArgs([]string{})

			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("doctor command error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDoctorJSONOutput(t *testing.T) {
	// Skip if SKIP_SPAWN_TESTS is set - this test runs Claude CLI
	if os.Getenv("SKIP_SPAWN_TESTS") != "" {
		t.Skip("skipping test that requires spawning external processes")
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
	_, _, jsonPtr, _ := shared.RegisterFlagPointers()
	rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")

	cmd := NewDoctorCommand()
	rootCmd.AddCommand(cmd)

	// Run with --json flag
	rootCmd.SetArgs([]string{"doctor", "--json"})

	// Should not panic
	_ = rootCmd.Execute()
}

func TestDoctorCommand_NoProviders(t *testing.T) {
	// Skip if SKIP_SPAWN_TESTS is set - doctor auto-detects Claude CLI even without providers
	if os.Getenv("SKIP_SPAWN_TESTS") != "" {
		t.Skip("skipping test that requires spawning external processes")
	}

	// Create temp directory for config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Config with no providers configured
	config := `# Empty config with no providers
log:
  level: info
`
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Override config path
	shared.SetConfigPathForTest(configPath)
	defer func() { shared.SetConfigPathForTest("") }()

	cmd := NewDoctorCommand()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	// Should succeed but report unhealthy state (no error, but recommendations)
	if err != nil {
		t.Logf("doctor command with no providers returned: %v", err)
	}
}

func TestDoctorCommand_InvalidProviderType(t *testing.T) {
	// Create temp directory for config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Config with invalid provider type
	config := `default_provider: invalid
providers:
  invalid:
    type: nonexistent-provider-type
`
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Override config path
	shared.SetConfigPathForTest(configPath)
	defer func() { shared.SetConfigPathForTest("") }()

	cmd := NewDoctorCommand()
	cmd.SetArgs([]string{})

	// Should execute without panicking, even with invalid provider
	_ = cmd.Execute()
}

func TestDoctorCommand_MultipleProviders(t *testing.T) {
	// Skip if SKIP_SPAWN_TESTS is set - this test runs Claude CLI
	if os.Getenv("SKIP_SPAWN_TESTS") != "" {
		t.Skip("skipping test that requires spawning external processes")
	}

	// Create temp directory for config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Config with multiple providers
	config := `default_provider: claude-code
providers:
  claude-code:
    type: claude-code
  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
`
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Override config path
	shared.SetConfigPathForTest(configPath)
	defer func() { shared.SetConfigPathForTest("") }()

	cmd := NewDoctorCommand()
	cmd.SetArgs([]string{})

	// Should check all providers
	_ = cmd.Execute()
}
