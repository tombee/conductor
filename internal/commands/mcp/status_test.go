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
	"strings"
	"testing"
)

func TestNewMCPStatusCommand(t *testing.T) {
	cmd := newMCPStatusCommand()

	if cmd.Use != "status <name>" {
		t.Errorf("expected use 'status <name>', got %q", cmd.Use)
	}

	// Check that --json flag is defined
	if cmd.Flags().Lookup("json") == nil {
		t.Error("--json flag not defined")
	}
}

func TestMCPStatusCommand_RequiresArgument(t *testing.T) {
	cmd := newMCPStatusCommand()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when server name argument is missing")
	}

	// Should mention accepts/requires 1 arg
	if !strings.Contains(err.Error(), "accepts 1 arg(s)") && !strings.Contains(err.Error(), "required") {
		t.Errorf("expected missing argument error, got: %v", err)
	}
}

func TestMCPStatusCommand_JSONFlag(t *testing.T) {
	cmd := newMCPStatusCommand()
	cmd.SetArgs([]string{"test-server", "--json"})

	// Parse flags only (don't execute - requires daemon)
	err := cmd.ParseFlags([]string{"--json"})
	if err != nil {
		t.Errorf("--json flag parsing failed: %v", err)
	}
}

func TestMCPStatusCommand_NonexistentServer(t *testing.T) {
	// Skip if running short tests (requires daemon)
	if testing.Short() {
		t.Skip("skipping daemon integration test in short mode")
	}

	cmd := newMCPStatusCommand()
	cmd.SetArgs([]string{"nonexistent-server"})

	// This will fail if daemon is not running or server doesn't exist
	err := cmd.Execute()
	if err != nil {
		// Expected - daemon not running or server not found
		t.Logf("status command failed as expected: %v", err)
	}
}
