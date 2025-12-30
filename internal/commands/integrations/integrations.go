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

// Package integrations provides CLI commands for managing workspace integrations.
package integrations

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the integrations command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integrations",
		Short: "Manage workspace integrations",
		Long: `Manage integrations for connecting to external services like GitHub, Slack, and Jira.

Integrations are named connections to external services that workflows can use.
Each integration consists of:
  - Type (github, slack, jira, etc.)
  - Authentication configuration
  - Optional base URL and headers

Integration credentials are encrypted at rest and never appear in workflow files.

Examples:
  # Add a GitHub integration
  conductor integrations add github --token '${GITHUB_TOKEN}'

  # Add GitHub Enterprise integration
  conductor integrations add github --name work \
    --base-url "https://github.mycompany.com/api/v3" \
    --token '${WORK_GITHUB_TOKEN}'

  # List all integrations
  conductor integrations list

  # Show integration details (credentials redacted)
  conductor integrations show github

  # Test integration connectivity
  conductor integrations test github

  # Update integration
  conductor integrations update github --token '${NEW_TOKEN}'

  # Remove integration
  conductor integrations remove github

  # Export integrations (for backup)
  conductor integrations export --format yaml`,
	}

	cmd.AddCommand(NewAddCommand())
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewShowCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewRemoveCommand())
	cmd.AddCommand(NewTestCommand())
	cmd.AddCommand(NewExportCommand())

	return cmd
}
