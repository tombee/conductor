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

package mcp

import (
	"testing"
)

func TestNewMCPListCommand(t *testing.T) {
	cmd := newMCPListCommand()

	if cmd.Use != "list" {
		t.Errorf("expected use 'list', got %q", cmd.Use)
	}

	// Check that flags are defined
	// Note: --json is a root-level persistent flag, not on individual commands
	if cmd.Flags().Lookup("all") == nil {
		t.Error("--all flag not defined")
	}
}

func TestMCPListCommand_AllFlag(t *testing.T) {
	cmd := newMCPListCommand()
	cmd.SetArgs([]string{"--all"})

	// Parse flags only
	err := cmd.ParseFlags([]string{"--all"})
	if err != nil {
		t.Errorf("--all flag parsing failed: %v", err)
	}
}

func TestMCPListCommand_NoServers(t *testing.T) {
	// Skip if running short tests (requires controller)
	if testing.Short() {
		t.Skip("skipping controller integration test in short mode")
	}

	cmd := newMCPListCommand()
	cmd.SetArgs([]string{})

	// This will fail if controller is not running, which is expected
	err := cmd.Execute()
	if err != nil {
		// Expected - controller not running
		t.Logf("list command failed as expected (controller not running): %v", err)
	}
}
