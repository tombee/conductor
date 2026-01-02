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

func newAddCmd() *cobra.Command {
	var (
		contextWindow int
		inputPrice    float64
		outputPrice   float64
	)

	cmd := &cobra.Command{
		Use:   "add <provider/model>",
		Short: "Register a new model",
		Long: `Register a new model under its provider with optional pricing metadata.

The model reference must use the format "provider/model" (e.g., "anthropic/claude-3-5-haiku-20241022").
The provider must already be configured.

Pricing information is optional and used for cost estimation in future features.

Examples:
  # Add a model with context window
  conductor model add anthropic/claude-3-5-haiku-20241022 --context-window 200000

  # Add a model with full pricing information
  conductor model add anthropic/claude-sonnet-4-20250514 \
    --context-window 200000 \
    --input-price 3.00 \
    --output-price 15.00

  # Add a local model without metadata
  conductor model add ollama/llama3.2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			modelRef := args[0]

			// Parse the model reference
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

				// Initialize models map if needed
				if providerCfg.Models == nil {
					providerCfg.Models = make(map[string]config.ModelConfig)
				}

				// Check if model already exists
				if _, exists := providerCfg.Models[model]; exists {
					return fmt.Errorf("model %q already registered in provider %q", model, provider)
				}

				// Create model configuration
				modelCfg := config.ModelConfig{
					ContextWindow:     contextWindow,
					InputPricePerMTok: inputPrice,
					OutputPricePerMTok: outputPrice,
				}

				// Add model to provider
				providerCfg.Models[model] = modelCfg
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

			fmt.Fprintf(out, "Model %s registered successfully.\n", modelRef)
			return nil
		},
	}

	cmd.Flags().IntVar(&contextWindow, "context-window", 0, "Context window size (tokens)")
	cmd.Flags().Float64Var(&inputPrice, "input-price", 0, "Input price per million tokens (USD)")
	cmd.Flags().Float64Var(&outputPrice, "output-price", 0, "Output price per million tokens (USD)")

	return cmd
}
