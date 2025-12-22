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

package version

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

func TestVersionCommand(t *testing.T) {
	cmd := NewVersionCommand()

	if cmd.Use != "version" {
		t.Errorf("expected use 'version', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected short description to be set")
	}
}

func TestVersionOutput(t *testing.T) {
	// Set test version
	shared.SetVersion("1.0.0", "test123", "2025-12-22")
	defer shared.SetVersion("dev", "unknown", "unknown")

	cmd := NewVersionCommand()

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("1.0.0")) {
		t.Errorf("expected output to contain version '1.0.0', got: %s", output)
	}
}

func TestVersionJSONOutput(t *testing.T) {
	// Set test version
	shared.SetVersion("1.0.0", "test123", "2025-12-22")
	defer shared.SetVersion("dev", "unknown", "unknown")

	// Create root command with --json flag
	rootCmd := &cobra.Command{Use: "test"}
	_, _, jsonPtr, _ := shared.RegisterFlagPointers()
	rootCmd.PersistentFlags().BoolVar(jsonPtr, "json", false, "JSON output")

	cmd := NewVersionCommand()
	rootCmd.AddCommand(cmd)

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	cmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"version", "--json"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	// Parse JSON
	var info VersionInfo
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, buf.String())
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", info.Version)
	}
	if info.Commit != "test123" {
		t.Errorf("expected commit 'test123', got %q", info.Commit)
	}
	if info.BuildDate != "2025-12-22" {
		t.Errorf("expected build date '2025-12-22', got %q", info.BuildDate)
	}
}
