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
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
)

// ModelInfo represents a model for display purposes
type ModelInfo struct {
	ID            string `json:"id"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	ContextWindow int    `json:"context_window,omitempty"`
	Tier          string `json:"tier,omitempty"`
}

func newListCmd() *cobra.Command {
	var showTiers bool

	cmd := &cobra.Command{
		Use:   "list [provider]",
		Short: "List registered models",
		Long: `List all registered models, optionally filtered by provider.

Without a provider argument, shows all models across all providers.
With a provider argument, shows only models for that provider.

Use --tiers to show which models are mapped to tiers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			// Load configuration
			cfgPath, err := getConfigPathOrDefault()
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}
			cfg, err := config.LoadSettings(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Filter by provider if specified
			var filterProvider string
			if len(args) > 0 {
				filterProvider = args[0]
				if _, exists := cfg.Providers[filterProvider]; !exists {
					return fmt.Errorf("provider %q not found", filterProvider)
				}
			}

			useJSON := shared.GetJSON()

			// Build model list
			var models []ModelInfo
			for providerName, providerCfg := range cfg.Providers {
				// Skip if filtering and this isn't the target provider
				if filterProvider != "" && providerName != filterProvider {
					continue
				}

				for modelName, modelCfg := range providerCfg.Models {
					info := ModelInfo{
						ID:            fmt.Sprintf("%s/%s", providerName, modelName),
						Provider:      providerName,
						Model:         modelName,
						ContextWindow: modelCfg.ContextWindow,
					}

					// Find tier mapping if requested
					if showTiers {
						info.Tier = findTierForModel(cfg, providerName, modelName)
					}

					models = append(models, info)
				}
			}

			// Sort models by ID for consistent output
			sort.Slice(models, func(i, j int) bool {
				return models[i].ID < models[j].ID
			})

			// Check if no models found
			if len(models) == 0 {
				if useJSON {
					fmt.Fprintln(out, `{"models":[]}`)
					return nil
				}
				if filterProvider != "" {
					fmt.Fprintf(out, "No models registered for provider %q.\n", filterProvider)
					fmt.Fprintf(out, "\nRun 'conductor model discover %s' to find available models.\n", filterProvider)
				} else {
					fmt.Fprintln(out, "No models registered.")
					fmt.Fprintln(out)
					fmt.Fprintln(out, "Run 'conductor model add <provider/model>' to register a model.")
				}
				return nil
			}

			// Output results
			if useJSON {
				result := map[string][]ModelInfo{"models": models}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			// Table output
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			if showTiers {
				fmt.Fprintln(w, "MODEL\tPROVIDER\tCONTEXT\tTIER")
				for _, m := range models {
					context := "-"
					if m.ContextWindow > 0 {
						context = fmt.Sprintf("%d", m.ContextWindow)
					}
					tier := "-"
					if m.Tier != "" {
						tier = m.Tier
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.ID, m.Provider, context, tier)
				}
			} else {
				fmt.Fprintln(w, "MODEL\tPROVIDER\tCONTEXT")
				for _, m := range models {
					context := "-"
					if m.ContextWindow > 0 {
						context = fmt.Sprintf("%d", m.ContextWindow)
					}
					fmt.Fprintf(w, "%s\t%s\t%s\n", m.ID, m.Provider, context)
				}
			}
			w.Flush()

			return nil
		},
	}

	cmd.Flags().BoolVar(&showTiers, "tiers", false, "Show tier mappings alongside models")

	return cmd
}

// getConfigPathOrDefault returns the config path from the flag or falls back to default
func getConfigPathOrDefault() (string, error) {
	cfgPath := shared.GetConfigPath()
	if cfgPath == "" {
		return config.SettingsPath()
	}
	return cfgPath, nil
}

// findTierForModel finds the tier name for a given provider/model, or returns empty string
func findTierForModel(cfg *config.Config, provider, model string) string {
	targetRef := fmt.Sprintf("%s/%s", provider, model)
	for tierName, tierRef := range cfg.Tiers {
		if tierRef == targetRef {
			return tierName
		}
	}
	return ""
}
