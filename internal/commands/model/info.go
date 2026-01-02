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
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
)

// ModelInfoOutput represents model information for display or JSON output
type ModelInfoOutput struct {
	Model         string   `json:"model"`
	Provider      string   `json:"provider"`
	ContextWindow *int     `json:"context_window"`
	InputPrice    *float64 `json:"input_price"`
	OutputPrice   *float64 `json:"output_price"`
	Tiers         []string `json:"tiers"`
}

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <provider/model>",
		Short: "Show detailed information about a model",
		Long: `Display model metadata including pricing, context window, and tier mappings.

Shows all available information about a registered model, including:
  - Context window size (tokens)
  - Input/output pricing (USD per million tokens)
  - Which tiers (if any) map to this model

Examples:
  # Show info for a specific model
  conductor model info anthropic/claude-3-5-haiku-20241022

  # Get JSON output
  conductor model info anthropic/claude-3-5-haiku-20241022 --json`,
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
			cfg, err := config.LoadSettings(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Get model config
			modelCfg, err := cfg.GetModelConfig(provider, model)
			if err != nil {
				return fmt.Errorf("model not found: %s. Run 'conductor model list' to see registered models", modelRef)
			}

			// Find tier mappings
			var tiers []string
			for tierName, tierRef := range cfg.Tiers {
				if tierRef == modelRef {
					tiers = append(tiers, tierName)
				}
			}

			useJSON := shared.GetJSON()

			// Prepare output
			info := ModelInfoOutput{
				Model:    modelRef,
				Provider: provider,
				Tiers:    tiers,
			}

			// Add optional fields only if they have values
			if modelCfg.ContextWindow > 0 {
				info.ContextWindow = &modelCfg.ContextWindow
			}
			if modelCfg.InputPricePerMTok > 0 {
				info.InputPrice = &modelCfg.InputPricePerMTok
			}
			if modelCfg.OutputPricePerMTok > 0 {
				info.OutputPrice = &modelCfg.OutputPricePerMTok
			}

			if useJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}

			// Human-readable output
			fmt.Fprintf(out, "Model Name:       %s\n", modelRef)
			fmt.Fprintf(out, "Provider:         %s\n", provider)

			if info.ContextWindow != nil {
				fmt.Fprintf(out, "Context Window:   %d tokens\n", *info.ContextWindow)
			} else {
				fmt.Fprintf(out, "Context Window:   N/A\n")
			}

			if info.InputPrice != nil {
				fmt.Fprintf(out, "Input Price:      $%.2f per million tokens\n", *info.InputPrice)
			} else {
				fmt.Fprintf(out, "Input Price:      N/A\n")
			}

			if info.OutputPrice != nil {
				fmt.Fprintf(out, "Output Price:     $%.2f per million tokens\n", *info.OutputPrice)
			} else {
				fmt.Fprintf(out, "Output Price:     N/A\n")
			}

			if len(tiers) > 0 {
				fmt.Fprintf(out, "Mapped Tiers:     %v\n", tiers)
			} else {
				fmt.Fprintf(out, "Mapped Tiers:     None\n")
			}

			return nil
		},
	}

	return cmd
}
