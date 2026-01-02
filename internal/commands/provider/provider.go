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

package provider

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the provider command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage LLM provider configurations",
		Long: `Manage configured LLM providers.

Providers connect Conductor to Large Language Model APIs or CLIs.
Each provider has a unique name and can be configured for different use cases.

Examples:
  # List all configured providers
  conductor provider list

  # Add a new provider interactively
  conductor provider add

  # Add a provider with flags
  conductor provider add anthropic --type anthropic --api-key-env ANTHROPIC_API_KEY

  # Test provider connectivity
  conductor provider test anthropic

  # Remove a provider
  conductor provider remove anthropic`,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newEditCmd())
	cmd.AddCommand(newTestCmd())

	// Default to list if no subcommand specified
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newListCmd().RunE(cmd, args)
	}

	return cmd
}
