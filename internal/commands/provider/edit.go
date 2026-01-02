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

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/completion"
	"github.com/tombee/conductor/internal/config"
)

func newEditCmd() *cobra.Command {
	var (
		apiKey    string
		apiKeyEnv string
		baseURL   string
	)

	cmd := &cobra.Command{
		Use:               "edit <name>",
		Short:             "Edit a provider configuration",
		Long:              "Edit an existing provider's configuration interactively or with flags.",
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
			providerCfg, exists := cfg.Providers[providerName]
			if !exists {
				return fmt.Errorf("provider %q not found", providerName)
			}

			// Check if any flags were provided
			hasFlags := cmd.Flags().Changed("api-key") ||
				cmd.Flags().Changed("api-key-env") ||
				cmd.Flags().Changed("base-url")

			if !hasFlags {
				return fmt.Errorf("interactive edit mode not yet implemented - use flags:\n  conductor provider edit %s --api-key-env <VAR>\n  conductor provider edit %s --base-url <URL>", providerName, providerName)
			}

			// Handle API key updates
			if apiKey != "" && apiKeyEnv != "" {
				return fmt.Errorf("cannot specify both --api-key and --api-key-env")
			}

			if apiKeyEnv != "" {
				// Store via secrets backend (implementation needed)
				providerCfg.APIKey = fmt.Sprintf("$secret:providers/%s/api_key", providerName)
				// TODO: Actually store the secret via secrets backend
			} else if apiKey != "" {
				fmt.Println("Warning: Using --api-key exposes the key in shell history.")
				fmt.Println("Consider using --api-key-env instead for better security.")
				// Store via secrets backend (implementation needed)
				providerCfg.APIKey = fmt.Sprintf("$secret:providers/%s/api_key", providerName)
				// TODO: Actually store the secret via secrets backend
			}

			// Handle base URL update
			if baseURL != "" {
				providerCfg.BaseURL = baseURL
			}

			// Update provider in config
			cfg.Providers[providerName] = providerCfg

			// Save configuration
			if err := config.WriteConfig(cfg, cfgPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("\nProvider %q updated successfully\n", providerName)
			fmt.Printf("Config saved to: %s\n", cfgPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "Environment variable containing API key")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key value (use --api-key-env to avoid shell history)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Base URL for API endpoint")

	return cmd
}
