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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/completion"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
)

func newTestCmd() *cobra.Command {
	var testAll bool

	cmd := &cobra.Command{
		Use:               "test [name]",
		Short:             "Test provider connectivity",
		ValidArgsFunction: completion.CompleteProviderNames,
		Long: `Run health check on a provider to verify it's working.

Tests three aspects:
  1. Configured - Provider exists in config
  2. Authenticated - Provider credentials are valid
  3. Working - Provider can make successful API calls

Examples:
  # Test specific provider
  conductor provider test claude-code

  # Test all configured providers
  conductor provider test --all

  # Test and get JSON output
  conductor provider test claude-code --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfgPath, err := getConfigPathOrDefault()
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Determine which providers to test
			var providersToTest map[string]config.ProviderConfig
			if testAll {
				providersToTest = cfg.Providers
			} else {
				if len(args) == 0 {
					return fmt.Errorf("provider name required (or use --all to test all providers)")
				}
				providerName := args[0]
				providerCfg, exists := cfg.Providers[providerName]
				if !exists {
					return fmt.Errorf("provider %q not found", providerName)
				}
				providersToTest = map[string]config.ProviderConfig{
					providerName: providerCfg,
				}
			}

			useJSON := shared.GetJSON()

			// Test each provider
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			type TestResult struct {
				Name          string `json:"name"`
				Type          string `json:"type"`
				Configured    bool   `json:"configured"`
				Authenticated bool   `json:"authenticated"`
				Working       bool   `json:"working"`
				LatencyMs     int64  `json:"latency_ms,omitempty"`
				Error         string `json:"error,omitempty"`
			}

			results := make([]TestResult, 0, len(providersToTest))
			for name, providerCfg := range providersToTest {
				if !useJSON && len(providersToTest) > 1 {
					fmt.Printf("\nTesting %s (%s)...\n", name, providerCfg.Type)
				}

				start := time.Now()
				healthResult := checkProviderHealth(ctx, providerCfg)
				latency := time.Since(start).Milliseconds()

				result := TestResult{
					Name:          name,
					Type:          providerCfg.Type,
					Configured:    true, // If we got here, it's configured
					Authenticated: healthResult.Authenticated,
					Working:       healthResult.Working,
					LatencyMs:     latency,
				}

				if healthResult.Error != nil {
					result.Error = healthResult.Error.Error()
				}

				if useJSON {
					results = append(results, result)
				} else {
					// Display detailed progress
					fmt.Printf("  %s Configured\n", renderStatus(result.Configured))
					fmt.Printf("  %s Authenticated\n", renderStatus(result.Authenticated))
					fmt.Printf("  %s Working\n", renderStatus(result.Working))

					if latency > 0 {
						fmt.Printf("\n  %s %dms\n", shared.Muted.Render("Latency:"), latency)
					}

					if healthResult.Healthy() {
						fmt.Printf("\n  %s %s\n", shared.Muted.Render("Status:"), shared.StatusOK.Render("Healthy"))
					} else {
						fmt.Printf("\n  %s %s\n", shared.Muted.Render("Status:"), shared.StatusError.Render("Failed"))
						if result.Error != "" {
							fmt.Printf("  %s %s\n", shared.Muted.Render("Error:"), result.Error)
						}
						if healthResult.Message != "" {
							fmt.Printf("\n%s\n", shared.RenderWarn(healthResult.Message))
						}
					}
				}
			}

			if useJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&testAll, "all", false, "Test all configured providers")

	return cmd
}

// renderStatus returns a colored status indicator
func renderStatus(ok bool) string {
	if ok {
		return shared.StatusOK.Render("[OK]")
	}
	return shared.StatusError.Render("[FAILED]")
}
