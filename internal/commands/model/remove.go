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
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/config"
)

func newRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <provider/model>",
		Short: "Remove a model from the registry",
		Long: `Remove a model from the registry.

If the model is mapped to any tiers, the command will fail unless --force is used.
Using --force will remove the model without clearing tier mappings, which may
leave orphaned tier references.

Examples:
  # Remove a model
  conductor model remove ollama/llama3.2

  # Force remove a model that's mapped to a tier
  conductor model remove anthropic/claude-3-5-haiku-20241022 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			modelRef := args[0]

			// Parse model reference
			provider, model, err := config.ParseModelReference(modelRef)
			if err != nil {
				return err
			}

			// Load configuration
			cfgPath, err := getConfigPathOrDefault()
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}

			// Load, modify, and save with locking
			sf, err := config.NewSettingsFile(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to create settings file: %w", err)
			}

			err = sf.WithLock(func() error {
				cfg, err := sf.Load()
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}

				// Validate provider exists
				providerCfg, exists := cfg.Providers[provider]
				if !exists {
					return fmt.Errorf("provider %q not found", provider)
				}

				// Validate model exists
				if providerCfg.Models == nil {
					return fmt.Errorf("model %q not found", modelRef)
				}
				if _, exists := providerCfg.Models[model]; !exists {
					return fmt.Errorf("model %q not found", modelRef)
				}

				// Check if model is mapped to any tiers
				var mappedTiers []string
				for tierName, tierRef := range cfg.Tiers {
					if tierRef == modelRef {
						mappedTiers = append(mappedTiers, tierName)
					}
				}

				if len(mappedTiers) > 0 && !force {
					return fmt.Errorf("cannot remove model: mapped to tier(s) %v. Use 'conductor model set-tier <tier> <other-model>' first or --force", mappedTiers)
				}

				// Remove the model
				delete(providerCfg.Models, model)
				cfg.Providers[provider] = providerCfg

				// Save configuration
				if err := sf.Save(cfg); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}

				return nil
			})

			if err != nil {
				return err
			}

			fmt.Fprintf(out, "Model %s removed successfully.\n", modelRef)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force removal even if model is mapped to tiers")

	return cmd
}
