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

// Package workspace provides CLI commands for managing workspaces.
package workspace

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the workspace command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage workspaces",
		Long: `Manage workspaces for organizing integrations and workflows.

A workspace is a configuration boundary containing named integrations.
The "default" workspace is used when no workspace is specified.

Workspaces provide isolation for:
  - Integration configurations (GitHub, Slack, Jira, etc.)
  - Credentials and authentication
  - Team-specific settings

Examples:
  # Create a new workspace
  conductor workspace create frontend --description "Frontend team workspace"

  # List all workspaces
  conductor workspace list

  # Switch to a workspace
  conductor workspace use frontend

  # Show current workspace
  conductor workspace current

  # Delete a workspace
  conductor workspace delete frontend`,
	}

	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewUseCommand())
	cmd.AddCommand(NewCurrentCommand())
	cmd.AddCommand(NewDeleteCommand())

	return cmd
}
