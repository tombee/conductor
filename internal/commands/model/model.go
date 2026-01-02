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

package model

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the model command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Manage LLM models and tier mappings",
		Long: `Manage registered LLM models and their tier mappings.

Models are registered under providers and can be mapped to abstract tiers
(fast, balanced, strategic) for use in workflows.

Examples:
  # List all registered models
  conductor model list

  # Add a new model
  conductor model add anthropic/claude-3-5-haiku-20241022 --context-window 200000

  # Discover available models from a provider
  conductor model discover anthropic

  # Map a model to a tier
  conductor model set-tier fast anthropic/claude-3-5-haiku-20241022

  # View model information
  conductor model info anthropic/claude-3-5-haiku-20241022`,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newDiscoverCmd())
	cmd.AddCommand(newSetTierCmd())
	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newRemoveCmd())

	// Default to list if no subcommand specified
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newListCmd().RunE(cmd, args)
	}

	return cmd
}
