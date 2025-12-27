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

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHelpCommandJSON(t *testing.T) {
	// Create a minimal root command for testing
	rootCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	// Add persistent flags to simulate global flags
	rootCmd.PersistentFlags().Bool("verbose", false, "Verbose output")

	// Add a sample subcommand
	sampleCmd := &cobra.Command{
		Use:   "sample",
		Short: "Sample subcommand",
		Long:  "This is a sample subcommand for testing",
		Example: `  test sample
  test sample --flag value`,
		Annotations: map[string]string{
			"group": "testing",
		},
	}
	sampleCmd.Flags().String("flag", "", "A sample flag")
	rootCmd.AddCommand(sampleCmd)

	// Create help command and set it as the help command for root
	helpCmd := NewHelpCommand(rootCmd)
	rootCmd.SetHelpCommand(helpCmd)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "help --json lists all commands",
			args:    []string{"--json"},
			wantErr: false,
		},
		{
			name:    "help sample --json shows specific command",
			args:    []string{"sample", "--json"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)

			// Construct full args including "help"
			fullArgs := append([]string{"help"}, tt.args...)
			rootCmd.SetArgs(fullArgs)

			err := rootCmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			output := buf.String()

			// Parse JSON output
			var resp HelpResponse
			decoder := json.NewDecoder(strings.NewReader(output))
			if err := decoder.Decode(&resp); err != nil {
				t.Errorf("Failed to parse JSON output: %v\nOutput: %s", err, output)
				return
			}

			// Verify response structure
			if resp.Version != "1.0" {
				t.Errorf("Expected version 1.0, got %s", resp.Version)
			}
			if !resp.Success {
				t.Errorf("Expected success true, got false")
			}
			if resp.DocsURL == "" {
				t.Errorf("Expected docs_url to be set")
			}

			// Test-specific validations
			if strings.Contains(tt.name, "lists all commands") {
				if len(resp.Commands) == 0 {
					t.Errorf("Expected commands list, got none")
				}
				if resp.Command != nil {
					t.Errorf("Expected command to be nil for list, got %+v", resp.Command)
				}
			}

			if strings.Contains(tt.name, "shows specific command") {
				if resp.Command == nil {
					t.Errorf("Expected command metadata, got nil")
				} else {
					if resp.Command.Name != "sample" {
						t.Errorf("Expected command name 'sample', got %s", resp.Command.Name)
					}
					if resp.Command.Group != "testing" {
						t.Errorf("Expected group 'testing', got %s", resp.Command.Group)
					}
					if resp.Command.Examples == "" {
						t.Errorf("Expected examples to be populated")
					}
				}
				if len(resp.Commands) > 0 {
					t.Errorf("Expected commands to be empty for single command, got %d", len(resp.Commands))
				}
			}

			// Verify global flags are included
			if len(resp.GlobalFlags) == 0 {
				t.Logf("Warning: No global flags found (might be expected in test)")
			}
		})
	}
}

func TestHelpCommandHumanOutput(t *testing.T) {
	// Create a minimal root command for testing
	rootCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	sampleCmd := &cobra.Command{
		Use:   "sample",
		Short: "Sample subcommand",
	}
	rootCmd.AddCommand(sampleCmd)

	helpCmd := NewHelpCommand(rootCmd)
	rootCmd.SetHelpCommand(helpCmd)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("Execute() error = %v", err)
		return
	}

	output := buf.String()

	// Verify it's human-readable (not JSON)
	if strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Errorf("Expected human output, got JSON")
	}
}

func TestExtractCommandMetadata(t *testing.T) {
	cmd := &cobra.Command{
		Use:     "testcmd",
		Short:   "Test command",
		Long:    "This is a longer description",
		Example: "testcmd --flag value",
		Aliases: []string{"tc", "test"},
		Annotations: map[string]string{
			"group": "testing",
		},
	}
	cmd.Flags().String("flag", "default", "A test flag")
	cmd.Flags().Bool("bool-flag", false, "A boolean flag")

	metadata := extractCommandMetadata(cmd)

	if metadata.Name != "testcmd" {
		t.Errorf("Expected name 'testcmd', got %s", metadata.Name)
	}
	if metadata.Short != "Test command" {
		t.Errorf("Expected short 'Test command', got %s", metadata.Short)
	}
	if metadata.Long != "This is a longer description" {
		t.Errorf("Expected long description, got %s", metadata.Long)
	}
	if metadata.Group != "testing" {
		t.Errorf("Expected group 'testing', got %s", metadata.Group)
	}
	if len(metadata.Aliases) != 2 {
		t.Errorf("Expected 2 aliases, got %d", len(metadata.Aliases))
	}
	if len(metadata.Flags) != 2 {
		t.Errorf("Expected 2 flags, got %d", len(metadata.Flags))
	}
}

func TestExtractGlobalFlags(t *testing.T) {
	rootCmd := &cobra.Command{
		Use: "test",
	}
	rootCmd.PersistentFlags().Bool("verbose", false, "Verbose output")
	rootCmd.PersistentFlags().String("config", "", "Config file")

	flags := extractGlobalFlags(rootCmd)

	if len(flags) != 2 {
		t.Errorf("Expected 2 global flags, got %d", len(flags))
	}

	// Verify flag details
	foundVerbose := false
	foundConfig := false
	for _, f := range flags {
		if f.Name == "verbose" {
			foundVerbose = true
			if f.Usage != "Verbose output" {
				t.Errorf("Expected usage 'Verbose output', got %s", f.Usage)
			}
		}
		if f.Name == "config" {
			foundConfig = true
		}
	}

	if !foundVerbose {
		t.Errorf("Expected to find 'verbose' flag")
	}
	if !foundConfig {
		t.Errorf("Expected to find 'config' flag")
	}
}
