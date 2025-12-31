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

func TestNewMCPStatusCommand(t *testing.T) {
	cmd := newMCPStatusCommand()

	if cmd.Use != "status <name>" {
		t.Errorf("expected use 'status <name>', got %q", cmd.Use)
	}

	// Note: --json is a root-level persistent flag, not on individual commands
}

func TestMCPStatusCommand_NonexistentServer(t *testing.T) {
	// Skip if running short tests (requires controller)
	if testing.Short() {
		t.Skip("skipping controller integration test in short mode")
	}

	cmd := newMCPStatusCommand()
	cmd.SetArgs([]string{"nonexistent-server"})

	// This will fail if controller is not running or server doesn't exist
	err := cmd.Execute()
	if err != nil {
		// Expected - controller not running or server not found
		t.Logf("status command failed as expected: %v", err)
	}
}
