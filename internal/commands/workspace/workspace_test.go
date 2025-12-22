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

package workspace

import (
	"testing"
)

// TestWorkspaceCommandsExist verifies all workspace subcommands are registered
func TestWorkspaceCommandsExist(t *testing.T) {
	cmd := NewCommand()

	expectedCommands := []string{
		"create",
		"list",
		"use",
		"current",
		"delete",
	}

	for _, expectedCmd := range expectedCommands {
		found := false
		for _, subCmd := range cmd.Commands() {
			if subCmd.Name() == expectedCmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected workspace subcommand %q not found", expectedCmd)
		}
	}
}

// TestWorkspaceCommandStructure verifies the workspace command structure
func TestWorkspaceCommandStructure(t *testing.T) {
	cmd := NewCommand()

	if cmd.Use != "workspace" {
		t.Errorf("expected Use to be 'workspace', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be non-empty")
	}

	if cmd.Long == "" {
		t.Error("expected Long description to be non-empty")
	}
}

// TestCreateCommandStructure verifies the create command structure
func TestCreateCommandStructure(t *testing.T) {
	cmd := NewCreateCommand()

	if cmd.Use != "create <name>" {
		t.Errorf("expected Use to be 'create <name>', got %q", cmd.Use)
	}

	if cmd.Args == nil {
		t.Error("expected Args validator to be set")
	}

	// Check for description flag
	flag := cmd.Flags().Lookup("description")
	if flag == nil {
		t.Error("expected --description flag to exist")
	}
}

// TestListCommandStructure verifies the list command structure
func TestListCommandStructure(t *testing.T) {
	cmd := NewListCommand()

	if cmd.Use != "list" {
		t.Errorf("expected Use to be 'list', got %q", cmd.Use)
	}
}

// TestUseCommandStructure verifies the use command structure
func TestUseCommandStructure(t *testing.T) {
	cmd := NewUseCommand()

	if cmd.Use != "use <name>" {
		t.Errorf("expected Use to be 'use <name>', got %q", cmd.Use)
	}

	if cmd.Args == nil {
		t.Error("expected Args validator to be set")
	}
}

// TestCurrentCommandStructure verifies the current command structure
func TestCurrentCommandStructure(t *testing.T) {
	cmd := NewCurrentCommand()

	if cmd.Use != "current" {
		t.Errorf("expected Use to be 'current', got %q", cmd.Use)
	}
}

// TestDeleteCommandStructure verifies the delete command structure
func TestDeleteCommandStructure(t *testing.T) {
	cmd := NewDeleteCommand()

	if cmd.Use != "delete <name>" {
		t.Errorf("expected Use to be 'delete <name>', got %q", cmd.Use)
	}

	if cmd.Args == nil {
		t.Error("expected Args validator to be set")
	}
}
