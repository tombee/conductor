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

func newSetTierCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-tier <tier> <provider/model>",
		Short: "Map a tier to a specific model",
		Long: `Map an abstract tier (fast, balanced, strategic) to a specific model.

Tiers allow workflows to reference models by capability rather than specific IDs.
This enables easy switching between providers or model versions.

Valid tiers: fast, balanced, strategic

Examples:
  # Map the fast tier to Claude Haiku
  conductor model set-tier fast anthropic/claude-3-5-haiku-20241022

  # Map the balanced tier to a local Ollama model
  conductor model set-tier balanced ollama/llama3.2`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			tierName := args[0]
			modelRef := args[1]

			// Validate tier name
			if err := config.ValidateTierName(tierName); err != nil {
				return fmt.Errorf("invalid tier: %s. Must be one of: fast, balanced, strategic", tierName)
			}

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
					return fmt.Errorf("provider %q not configured. Run 'conductor provider add %s' first", provider, provider)
				}

				// Validate model exists
				if providerCfg.Models == nil {
					return fmt.Errorf("cannot map tier: model %q not registered. Run 'conductor model add %s' first", modelRef, modelRef)
				}
				if _, exists := providerCfg.Models[model]; !exists {
					return fmt.Errorf("cannot map tier: model %q not registered. Run 'conductor model add %s' first", modelRef, modelRef)
				}

				// Initialize tiers map if needed
				if cfg.Tiers == nil {
					cfg.Tiers = make(map[string]string)
				}

				// Set the tier mapping
				cfg.Tiers[tierName] = modelRef

				// Save configuration
				if err := sf.Save(cfg); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}

				return nil
			})

			if err != nil {
				return err
			}

			fmt.Fprintf(out, "Tier %q mapped to %s\n", tierName, modelRef)
			return nil
		},
	}

	return cmd
}
