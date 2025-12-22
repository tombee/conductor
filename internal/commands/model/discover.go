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
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

func newDiscoverCmd() *cobra.Command {
	var register bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "discover <provider>",
		Short: "Auto-discover available models from a provider",
		Long: `Query a provider's API to discover available models.

Supported providers:
  - claude-code: Returns haiku, sonnet, opus (CLI handles version mapping internally)
  - anthropic: (not yet implemented) Queries GET /v1/models
  - ollama: (not yet implemented) Queries GET /api/tags for locally installed models
  - openai: (not yet implemented) Queries GET /v1/models

The --register flag automatically adds discovered models to your configuration.
Use --yes to skip confirmation prompts.

Examples:
  # Discover models from claude-code provider
  conductor model discover claude-code

  # Discover and auto-register models
  conductor model discover claude-code --register --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			providerName := args[0]

			// Load configuration
			cfgPath, err := getConfigPathOrDefault()
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}
			cfg, err := config.LoadSettings(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Validate provider exists
			_, exists := cfg.Providers[providerName]
			if !exists {
				return fmt.Errorf("provider %q not configured. Run 'conductor provider add %s' first", providerName, providerName)
			}

			// Create a timeout context for discovery (10 seconds as per spec)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Discover models
			models, err := discoverModels(ctx, providerName)
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					return fmt.Errorf("discovery failed: %s: request timeout", providerName)
				}
				// Check for common connection errors
				if strings.Contains(err.Error(), "connection refused") {
					return fmt.Errorf("discovery failed: %s: connection refused", providerName)
				}
				return fmt.Errorf("discovery failed: %s: %w", providerName, err)
			}

			if len(models) == 0 {
				fmt.Fprintf(out, "No models discovered for provider %q.\n", providerName)
				return nil
			}

			// Display discovered models
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MODEL NAME\tCONTEXT WINDOW\tSTATUS")

			limit := len(models)
			displayLimit := false
			if limit > 50 {
				displayLimit = true
				limit = 50
			}

			for i, model := range models {
				if i >= limit {
					break
				}
				contextWindow := "-"
				if model.MaxTokens > 0 {
					contextWindow = fmt.Sprintf("%d", model.MaxTokens)
				}
				status := "Available"
				fmt.Fprintf(w, "%s\t%s\t%s\n", model.ID, contextWindow, status)
			}
			w.Flush()

			if displayLimit {
				fmt.Fprintf(out, "\n(showing %d of %d models)\n", limit, len(models))
			}

			// Register models if requested
			if register {
				// Prompt for confirmation unless --yes is set
				if !yes {
					fmt.Fprintf(out, "\nRegister these %d models? [y/N]: ", len(models))
					reader := bufio.NewReader(os.Stdin)
					response, err := reader.ReadString('\n')
					if err != nil {
						return fmt.Errorf("failed to read confirmation: %w", err)
					}
					response = strings.TrimSpace(strings.ToLower(response))
					if response != "y" && response != "yes" {
						fmt.Fprintln(out, "Registration cancelled.")
						return nil
					}
				}

				// Register models
				sf, err := config.NewSettingsFile(cfgPath)
				if err != nil {
					return fmt.Errorf("failed to create settings file: %w", err)
				}

				registered := 0
				err = sf.WithLock(func() error {
					cfg, err := sf.Load()
					if err != nil {
						return fmt.Errorf("failed to load config: %w", err)
					}

					providerCfg := cfg.Providers[providerName]
					if providerCfg.Models == nil {
						providerCfg.Models = make(map[string]config.ModelConfig)
					}

					for _, model := range models {
						// Skip if already registered
						if _, exists := providerCfg.Models[model.ID]; exists {
							continue
						}

						// Add model
						providerCfg.Models[model.ID] = config.ModelConfig{
							ContextWindow:      model.MaxTokens,
							InputPricePerMTok:  model.InputPricePerMillion,
							OutputPricePerMTok: model.OutputPricePerMillion,
						}
						registered++
					}

					cfg.Providers[providerName] = providerCfg

					// Save configuration
					if err := sf.Save(cfg); err != nil {
						return fmt.Errorf("failed to save config: %w", err)
					}

					return nil
				})

				if err != nil {
					return err
				}

				if registered > 0 {
					fmt.Fprintf(out, "\nRegistered %d new models.\n", registered)
				} else {
					fmt.Fprintln(out, "\nAll models were already registered.")
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&register, "register", false, "Automatically register discovered models")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts when registering")

	return cmd
}

// discoverModels queries a provider to discover available models
func discoverModels(ctx context.Context, providerName string) ([]llm.ModelInfo, error) {
	// For now, only claude-code is implemented
	// Other providers will be added as they're implemented
	switch providerName {
	case "claude-code":
		p := claudecode.New()
		caps := p.Capabilities()
		return caps.Models, nil
	case "anthropic":
		return nil, fmt.Errorf("model discovery not yet implemented for anthropic provider")
	case "openai":
		return nil, fmt.Errorf("model discovery not yet implemented for openai provider")
	case "ollama":
		return nil, fmt.Errorf("model discovery not yet implemented for ollama provider")
	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerName)
	}
}
