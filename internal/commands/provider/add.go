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
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/completion"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/permissions"
	"github.com/tombee/conductor/internal/secrets"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
	"github.com/tombee/conductor/pkg/workflow"
)

func newAddCmd() *cobra.Command {
	var (
		providerType string
		apiKey       string
		apiKeyEnv    string
		baseURL      string
		dryRun       bool
	)

	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Add a new provider",
		Long: `Add a new provider configuration interactively or with flags.

When no flags are provided, launches an interactive TUI for configuration.
For scripted deployments, use --type and either --api-key-env or --api-key.

Examples:
  # Interactive mode (TUI)
  conductor provider add

  # Non-interactive with environment variable (preferred for scripts)
  conductor provider add anthropic --type anthropic --api-key-env ANTHROPIC_API_KEY

  # Non-interactive with literal API key (warns about shell history)
  conductor provider add anthropic --type anthropic --api-key sk-ant-...

  # Ollama with custom base URL
  conductor provider add ollama --type ollama --base-url http://localhost:11434`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine provider name
			var providerName string
			if len(args) > 0 {
				providerName = args[0]
			}

			// Load existing configuration
			cfgPath, err := getConfigPathOrDefault()
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				// If config doesn't exist, create a new one
				cfg = config.Default()
			}
			// Ensure Providers map is initialized
			if cfg.Providers == nil {
				cfg.Providers = make(config.ProvidersMap)
			}

			// Check if provider already exists (if name provided)
			if providerName != "" {
				if _, exists := cfg.Providers[providerName]; exists {
					return fmt.Errorf("provider %q already exists. Use 'conductor provider remove %s' first to replace it", providerName, providerName)
				}
			}

			// Dry-run mode requires --type and name to be specified (non-interactive)
			if dryRun {
				if providerType == "" {
					return fmt.Errorf("--type is required in dry-run mode")
				}
				if providerName == "" {
					return fmt.Errorf("provider name is required in dry-run mode")
				}
			}

			// Interactive mode if type not specified
			if providerType == "" {
				return runAddInteractive(cmd, cfg, cfgPath)
			}

			// Non-interactive mode
			if providerName == "" {
				return fmt.Errorf("provider name is required when using --type flag")
			}

			// Validate provider type
			validTypes := map[string]bool{
				"claude-code": true,
				"anthropic":   true,
				"openai":      true,
				"ollama":      true,
			}
			if !validTypes[providerType] {
				return fmt.Errorf("unsupported provider type: %s. Supported: claude-code, anthropic, openai, ollama", providerType)
			}

			// Warn if using unsupported provider type
			config.WarnUnsupportedProvider(providerType)

			// Create provider config
			providerCfg := config.ProviderConfig{
				Type: providerType,
			}

			// Handle API key configuration
			if apiKey != "" && apiKeyEnv != "" {
				return fmt.Errorf("cannot specify both --api-key and --api-key-env")
			}

			if apiKeyEnv != "" {
				// Preferred: read from environment variable
				apiKeyValue := os.Getenv(apiKeyEnv)
				if apiKeyValue == "" {
					return fmt.Errorf("environment variable %s is not set or empty", apiKeyEnv)
				}
				// Store via secrets backend
				secretKey := fmt.Sprintf("providers/%s/api_key", providerName)
				if err := storeSecret(cmd.Context(), secretKey, apiKeyValue); err != nil {
					return fmt.Errorf("failed to store API key: %w", err)
				}
				providerCfg.APIKey = fmt.Sprintf("$secret:%s", secretKey)
			} else if apiKey != "" {
				// Warn about shell history exposure
				fmt.Fprintln(os.Stderr, "Warning: Using --api-key exposes the key in shell history.")
				fmt.Fprintln(os.Stderr, "Consider using --api-key-env instead for better security.")
				// Store via secrets backend
				secretKey := fmt.Sprintf("providers/%s/api_key", providerName)
				if err := storeSecret(cmd.Context(), secretKey, apiKey); err != nil {
					return fmt.Errorf("failed to store API key: %w", err)
				}
				providerCfg.APIKey = fmt.Sprintf("$secret:%s", secretKey)
			}

			// Handle base URL
			if baseURL != "" {
				// Validate base URL format and security
				if err := validateBaseURL(cmd.Context(), baseURL); err != nil {
					return fmt.Errorf("invalid base URL: %w", err)
				}
				providerCfg.BaseURL = baseURL
			}

			// Type-specific configuration and validation
			switch providerType {
			case "claude-code":
				// Verify Claude CLI is available
				ctx := context.Background()
				p := &claudecode.Provider{}
				found, err := p.Detect()
				if err != nil || !found {
					return fmt.Errorf("Claude CLI not found in PATH. Install from: https://claude.ai/download")
				}
				// Run health check
				result := p.HealthCheck(ctx)
				if !result.Healthy() {
					fmt.Fprintf(os.Stderr, "Warning: %s\n", result.Message)
				} else {
					if result.Version != "" {
						fmt.Fprintf(os.Stderr, "Claude CLI detected (version: %s)\n", result.Version)
					}
				}

			case "anthropic", "openai":
				// Require API key
				if providerCfg.APIKey == "" {
					return fmt.Errorf("API key is required for %s provider. Use --api-key-env or --api-key", providerType)
				}

			case "ollama":
				// Ollama doesn't need API key, but can have custom base URL
				if baseURL == "" {
					providerCfg.BaseURL = "http://localhost:11434"
				}
			}

			// Initialize Models map
			if providerCfg.Models == nil {
				providerCfg.Models = make(map[string]config.ModelConfig)
			}

			// Add default models for claude-code provider
			if providerType == "claude-code" {
				providerCfg.Models["haiku"] = config.ModelConfig{}
				providerCfg.Models["sonnet"] = config.ModelConfig{}
				providerCfg.Models["opus"] = config.ModelConfig{}
			}

			// Add provider to config
			cfg.Providers[providerName] = providerCfg

			// If this is the first provider, set as default
			setAsDefault := len(cfg.Providers) == 1 || cfg.DefaultProvider == ""
			if setAsDefault {
				cfg.DefaultProvider = providerName
			}

			// Handle dry-run mode
			if dryRun {
				return providerAddDryRun(cmd, cfgPath, providerName, providerCfg, setAsDefault)
			}

			// Save configuration
			if err := config.WriteConfig(cfg, cfgPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("\nProvider %q added successfully\n", providerName)
			if setAsDefault {
				fmt.Printf("Set as default provider\n")
			}
			fmt.Printf("Config saved to: %s\n", cfgPath)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  1. Test the provider:")
			fmt.Printf("     conductor provider test %s\n", providerName)
			fmt.Println("  2. Add models:")
			fmt.Printf("     conductor model discover %s --register\n", providerName)
			fmt.Println("  3. Configure tier mappings:")
			fmt.Println("     conductor model set-tier fast <provider/model>")
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVar(&providerType, "type", "", "Provider type (claude-code, anthropic, openai, ollama)")
	cmd.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "Environment variable containing API key (preferred for scripts)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key value (use --api-key-env to avoid shell history)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Base URL for API endpoint (optional)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be modified without executing")

	// Register completion for --type flag
	cmd.RegisterFlagCompletionFunc("type", completion.CompleteProviderTypes)

	return cmd
}

// providerAddDryRun shows what would be modified when adding a provider
func providerAddDryRun(cmd *cobra.Command, cfgPath, providerName string, providerCfg config.ProviderConfig, setAsDefault bool) error {
	// Get config directory for placeholder
	configDir, err := config.ConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	output := shared.NewDryRunOutput()

	// Use placeholder path
	placeholderPath := shared.PlaceholderPath(cfgPath, configDir, "<config-dir>")

	// Build description with masked sensitive values
	description := fmt.Sprintf("add provider '%s' (type: %s)", providerName, providerCfg.Type)

	// Add information about API key if present (but mask it)
	if providerCfg.APIKey != "" {
		if strings.HasPrefix(providerCfg.APIKey, "$secret:") {
			description += fmt.Sprintf(", api_key: %s", providerCfg.APIKey)
		} else {
			maskedKey := shared.MaskSensitiveData("api_key", providerCfg.APIKey)
			description += fmt.Sprintf(", api_key: %s", maskedKey)
		}
	}

	// Add information about base URL if present
	if providerCfg.BaseURL != "" {
		description += fmt.Sprintf(", base_url: %s", providerCfg.BaseURL)
	}

	// Add default provider note
	if setAsDefault {
		description += ", set as default"
	}

	output.DryRunModify(placeholderPath, description)

	// Print the output to the command's output stream
	fmt.Fprintln(cmd.OutOrStdout(), output.String())

	return nil
}

// storeSecret stores a secret value in the keychain backend
func storeSecret(ctx context.Context, key, value string) error {
	// Create keychain backend
	backend := secrets.NewKeychainBackend()
	if !backend.Available() {
		return fmt.Errorf("keychain backend not available - please ensure your system keychain is accessible")
	}

	// Store the secret
	if err := backend.Set(ctx, key, value); err != nil {
		return fmt.Errorf("keychain storage failed: %w", err)
	}

	return nil
}

// validateBaseURL validates that a base URL is properly formatted and not targeting blocked hosts
func validateBaseURL(ctx context.Context, baseURL string) error {
	// Parse URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}

	// Validate scheme
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme, got: %s", u.Scheme)
	}

	// Validate host is present
	if u.Host == "" {
		return fmt.Errorf("URL must include a host")
	}

	// Check against SSRF blocked hosts
	// Note: We check against default blocked hosts to prevent targeting internal services
	// The full permission context is not available here, so we only validate against defaults
	permCtx := &permissions.PermissionContext{
		Network: &workflow.NetworkPermissions{
			// Use empty allowed list (allow all except blocked)
			AllowedHosts: []string{},
			BlockedHosts: []string{},
		},
	}

	if err := permissions.CheckNetwork(ctx, permCtx, u.Host); err != nil {
		return fmt.Errorf("URL targets a blocked host (metadata endpoint or private network): %w", err)
	}

	return nil
}
