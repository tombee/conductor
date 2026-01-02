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
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/completion"
	"github.com/tombee/conductor/internal/config"
)

func newRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:               "remove <name>",
		Short:             "Remove a provider",
		Long:              "Remove a provider configuration from the config file.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteProviderNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := args[0]

			// Load configuration
			cfgPath, err := getConfigPathOrDefault()
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Check if provider exists
			if _, exists := cfg.Providers[providerName]; !exists {
				return fmt.Errorf("provider %q not found", providerName)
			}

			// Check if any tier mappings reference this provider's models
			var affectedTiers []string
			for tierName, tierRef := range cfg.Tiers {
				provider, _, err := config.ParseModelReference(tierRef)
				if err == nil && provider == providerName {
					affectedTiers = append(affectedTiers, tierName)
				}
			}

			// If models are mapped to tiers, require --force
			if len(affectedTiers) > 0 && !force {
				fmt.Printf("Cannot remove provider %q: models mapped to tiers.\n", providerName)
				for _, tier := range affectedTiers {
					fmt.Printf("  - %s → tier '%s'\n", cfg.Tiers[tier], tier)
				}
				fmt.Println()
				fmt.Println("Use --force to remove and clear tier mappings.")
				return fmt.Errorf("provider has active tier mappings")
			}

			// Confirm removal if not forced
			if !force {
				fmt.Printf("Remove provider %q? [y/N]: ", providerName)
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					fmt.Println("Cancelled")
					return nil
				}
			}

			// Remove provider
			delete(cfg.Providers, providerName)

			// Clear tier mappings that reference this provider
			for _, tier := range affectedTiers {
				delete(cfg.Tiers, tier)
			}
			if len(affectedTiers) > 0 {
				fmt.Printf("\nCleared %d tier mapping(s)\n", len(affectedTiers))
			}

			// If this was the default provider, clear it
			if cfg.DefaultProvider == providerName {
				cfg.DefaultProvider = ""
				fmt.Printf("\nWarning: %q was the default provider. Use 'conductor provider set-default' to set a new default.\n", providerName)
			}

			// Check for agent mappings that reference this provider
			removedMappings := []string{}
			for agent, provider := range cfg.AgentMappings {
				if provider == providerName {
					delete(cfg.AgentMappings, agent)
					removedMappings = append(removedMappings, agent)
				}
			}
			if len(removedMappings) > 0 {
				fmt.Printf("\nRemoved agent mappings for: %s\n", strings.Join(removedMappings, ", "))
			}

			// Save configuration
			if err := config.WriteConfig(cfg, cfgPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("\nProvider %q removed successfully\n", providerName)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt and remove tier mappings")

	return cmd
}
