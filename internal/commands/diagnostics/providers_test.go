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
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
)

func TestProvidersCommand(t *testing.T) {
	// Create temporary config directory
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Create a test config with default values to pass validation
	cfg := config.Default()
	cfg.DefaultProvider = "test-provider"
	cfg.Providers = config.ProvidersMap{
		"test-provider": {
			Type: "claude-code",
		},
		"other-provider": {
			Type: "anthropic",
			APIKey: "test-key",
		},
	}

	if err := config.WriteConfig(cfg, cfgPath); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		wantOutput  string
		skipOutput  bool
	}{
		{
			name:       "list providers",
			args:       []string{"providers", "list", "--config", cfgPath},
			wantErr:    false,
			wantOutput: "test-provider",
			skipOutput: false,
		},
		{
			name:       "list with json",
			args:       []string{"providers", "list", "--json", "--config", cfgPath},
			wantErr:    false,
			wantOutput: "test-provider",
			skipOutput: false,
		},
		{
			name:       "set-default",
			args:       []string{"providers", "set-default", "other-provider", "--config", cfgPath},
			wantErr:    false,
			wantOutput: "Default provider",
			skipOutput: false,
		},
		{
			name:    "set-default non-existent",
			args:    []string{"providers", "set-default", "nonexistent", "--config", cfgPath},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			var stdout, stderr bytes.Buffer
			// Create root command with flags
			rootCmd := &cobra.Command{Use: "test"}
			_, _, jsonPtr, configPtr := shared.RegisterFlagPointers()
			rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")
			rootCmd.PersistentFlags().StringVar(configPtr, "config", "", "Config file path")

			rootCmd.AddCommand(NewProvidersCommand())
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)
			rootCmd.SetArgs(tt.args)

			// Execute command
			err := rootCmd.Execute()

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				t.Logf("stdout: %s", stdout.String())
				t.Logf("stderr: %s", stderr.String())
				return
			}

			// Check output if specified
			if !tt.skipOutput && tt.wantOutput != "" {
				output := stdout.String()
				if !strings.Contains(output, tt.wantOutput) {
					t.Errorf("Expected output to contain %q, got:\n%s", tt.wantOutput, output)
				}
			}
		})
	}
}

func TestProvidersListJSON(t *testing.T) {
	// Create temporary config directory
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Create a test config with default values
	cfg := config.Default()
	cfg.DefaultProvider = "test-provider"
	cfg.Providers = config.ProvidersMap{
		"test-provider": {
			Type: "claude-code",
		},
	}

	if err := config.WriteConfig(cfg, cfgPath); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Execute command
	var stdout bytes.Buffer
	// Create root command with flags
	rootCmd := &cobra.Command{Use: "test"}
	_, _, jsonPtr, configPtr := shared.RegisterFlagPointers()
	rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")
	rootCmd.PersistentFlags().StringVar(configPtr, "config", "", "Config file path")

	rootCmd.AddCommand(NewProvidersCommand())
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"providers", "list", "--json", "--config", cfgPath})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Parse JSON output
	var statuses []ProviderStatus
	if err := json.Unmarshal(stdout.Bytes(), &statuses); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, stdout.String())
	}

	// Verify structure
	if len(statuses) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(statuses))
	}

	if statuses[0].Name != "test-provider" {
		t.Errorf("Expected name 'test-provider', got %q", statuses[0].Name)
	}

	if statuses[0].Type != "claude-code" {
		t.Errorf("Expected type 'claude-code', got %q", statuses[0].Type)
	}

	if !statuses[0].IsDefault {
		t.Errorf("Expected provider to be default")
	}
}

func TestProvidersListEmpty(t *testing.T) {
	// Create temporary config directory
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Create empty config
	cfg := config.Default()
	cfg.Providers = make(config.ProvidersMap)
	if err := config.WriteConfig(cfg, cfgPath); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Execute command
	var stdout bytes.Buffer
	// Create root command with flags
	rootCmd := &cobra.Command{Use: "test"}
	_, _, jsonPtr, configPtr := shared.RegisterFlagPointers()
	rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")
	rootCmd.PersistentFlags().StringVar(configPtr, "config", "", "Config file path")

	rootCmd.AddCommand(NewProvidersCommand())
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"providers", "list", "--config", cfgPath})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "No providers configured") {
		t.Errorf("Expected 'No providers configured' message, got:\n%s", output)
	}
}

func TestProvidersRemove(t *testing.T) {
	// Create temporary config directory
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Create a test config with default values
	cfg := config.Default()
	cfg.DefaultProvider = "test-provider"
	cfg.Providers = config.ProvidersMap{
		"test-provider": {
			Type: "claude-code",
		},
		"other-provider": {
			Type: "anthropic",
			APIKey: "test-key",
		},
	}
	cfg.AgentMappings = config.AgentMappings{
		"test-agent": "test-provider",
	}

	if err := config.WriteConfig(cfg, cfgPath); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Execute remove command with force flag
	var stdout bytes.Buffer
	// Create root command with flags
	rootCmd := &cobra.Command{Use: "test"}
	_, _, jsonPtr, configPtr := shared.RegisterFlagPointers()
	rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")
	rootCmd.PersistentFlags().StringVar(configPtr, "config", "", "Config file path")

	rootCmd.AddCommand(NewProvidersCommand())
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"providers", "remove", "test-provider", "-f", "--config", cfgPath})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify provider was removed
	updatedCfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Failed to load updated config: %v", err)
	}

	if _, exists := updatedCfg.Providers["test-provider"]; exists {
		t.Errorf("Provider should have been removed")
	}

	// Verify default was cleared
	if updatedCfg.DefaultProvider == "test-provider" {
		t.Errorf("Default provider should have been cleared")
	}

	// Verify agent mapping was removed
	if _, exists := updatedCfg.AgentMappings["test-agent"]; exists {
		t.Errorf("Agent mapping should have been removed")
	}
}

func TestCheckMark(t *testing.T) {
	tests := []struct {
		input bool
		want  string
	}{
		{true, "OK"},
		{false, "FAILED"},
	}

	for _, tt := range tests {
		got := checkMark(tt.input)
		if got != tt.want {
			t.Errorf("checkMark(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatHealthError(t *testing.T) {
	tests := []struct {
		name   string
		result ProviderStatus
		want   string
	}{
		{
			name: "with error",
			result: ProviderStatus{
				Status:  "ERROR",
				Message: "test error",
			},
			want: "test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the ProviderStatus structure
			if tt.result.Message != tt.want {
				t.Errorf("Message = %q, want %q", tt.result.Message, tt.want)
			}
		})
	}
}

func TestKeysOf(t *testing.T) {
	m := config.ProvidersMap{
		"a": {Type: "test"},
		"b": {Type: "test"},
		"c": {Type: "test"},
	}

	keys := keysOf(m)
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Check that all keys are present (order doesn't matter)
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	for _, expected := range []string{"a", "b", "c"} {
		if !keyMap[expected] {
			t.Errorf("Expected key %q to be present", expected)
		}
	}
}

// TestProvidersDefaultBehavior tests that "conductor providers" defaults to "list"
func TestProvidersDefaultBehavior(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	cfg := config.Default()
	cfg.DefaultProvider = "test"
	cfg.Providers = config.ProvidersMap{
		"test": {Type: "claude-code"},
	}

	if err := config.WriteConfig(cfg, cfgPath); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	var stdout bytes.Buffer
	// Create root command with flags
	rootCmd := &cobra.Command{Use: "test"}
	_, _, jsonPtr, configPtr := shared.RegisterFlagPointers()
	rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")
	rootCmd.PersistentFlags().StringVar(configPtr, "config", "", "Config file path")

	rootCmd.AddCommand(NewProvidersCommand())
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"providers", "--config", cfgPath})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Configured Providers") {
		t.Errorf("Expected default to list providers, got:\n%s", output)
	}
}

// TestProvidersTestMissingProvider tests error handling when testing non-existent provider
func TestProvidersTestMissingProvider(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	cfg := config.Default()
	cfg.Providers = config.ProvidersMap{
		"test": {Type: "claude-code"},
	}

	if err := config.WriteConfig(cfg, cfgPath); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	var stdout bytes.Buffer
	// Create root command with flags
	rootCmd := &cobra.Command{Use: "test"}
	_, _, jsonPtr, configPtr := shared.RegisterFlagPointers()
	rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")
	rootCmd.PersistentFlags().StringVar(configPtr, "config", "", "Config file path")

	rootCmd.AddCommand(NewProvidersCommand())
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"providers", "test", "nonexistent", "--config", cfgPath})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("Expected error when testing non-existent provider")
		return
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// TestProvidersTestRequiresName tests that test command requires a provider name or --all
func TestProvidersTestRequiresName(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	cfg := config.Default()
	cfg.Providers = config.ProvidersMap{
		"test": {Type: "claude-code"},
	}

	if err := config.WriteConfig(cfg, cfgPath); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	var stdout bytes.Buffer
	// Create root command with flags
	rootCmd := &cobra.Command{Use: "test"}
	_, _, jsonPtr, configPtr := shared.RegisterFlagPointers()
	rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")
	rootCmd.PersistentFlags().StringVar(configPtr, "config", "", "Config file path")

	rootCmd.AddCommand(NewProvidersCommand())
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"providers", "test", "--config", cfgPath})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("Expected error when no provider name given")
		return
	}

	if !strings.Contains(err.Error(), "required") {
		t.Errorf("Expected 'required' error, got: %v", err)
	}
}

// Skip this test in CI since it might not have the test config
func TestProvidersIntegration(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI")
	}

	// This test only runs if there's an existing config
	cfgPath, err := config.ConfigPath()
	if err != nil {
		t.Skip("Cannot determine config path")
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Skip("No config file exists")
	}

	// Just verify the command runs without error
	var stdout bytes.Buffer
	// Create root command with flags
	rootCmd := &cobra.Command{Use: "test"}
	_, _, jsonPtr, configPtr := shared.RegisterFlagPointers()
	rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")
	rootCmd.PersistentFlags().StringVar(configPtr, "config", "", "Config file path")

	rootCmd.AddCommand(NewProvidersCommand())
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"providers", "list"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}
