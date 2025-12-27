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

package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

// NewProvidersCommand creates the providers command
func NewProvidersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Annotations: map[string]string{
			"group": "diagnostics",
		},
		Short: "Manage LLM provider configurations",
		Long: `Manage configured LLM providers.

Providers connect Conductor to Large Language Model APIs or CLIs.
Each provider has a unique name and can be configured for different use cases.`,
	}

	cmd.AddCommand(newProvidersListCmd())
	cmd.AddCommand(newProvidersAddCmd())
	cmd.AddCommand(newProvidersRemoveCmd())
	cmd.AddCommand(newProvidersTestCmd())
	cmd.AddCommand(newProvidersSetDefaultCmd())

	// Default to list if no subcommand specified
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newProvidersListCmd().RunE(cmd, args)
	}

	return cmd
}

// ProviderStatus represents the status of a provider for display
type ProviderStatus struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	IsDefault bool   `json:"is_default"`
	Message   string `json:"message,omitempty"`
	Version   string `json:"version,omitempty"`
}

func newProvidersListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured providers",
		Long:  "Display all configured providers with their types, status, and default indicator.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			// Load configuration
			cfgPath, err := getConfigPathOrDefault()
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Check global --json flag in addition to local flag
			useJSON := shared.GetJSON() || jsonOutput

			// Check if no providers configured
			if len(cfg.Providers) == 0 {
				if useJSON {
					fmt.Fprintln(out, "[]")
					return nil
				}
				fmt.Fprintln(out, "No providers configured.")
				fmt.Fprintln(out)
				fmt.Fprintln(out, "To add a provider:")
				fmt.Fprintln(out, "  conductor providers add <name>")
				fmt.Fprintln(out)
				fmt.Fprintln(out, "Or run the interactive setup:")
				fmt.Fprintln(out, "  conductor init")
				return nil
			}

			// Gather provider status
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			statuses := make([]ProviderStatus, 0, len(cfg.Providers))
			for name, providerCfg := range cfg.Providers {
				status := ProviderStatus{
					Name:      name,
					Type:      providerCfg.Type,
					IsDefault: name == cfg.DefaultProvider,
				}

				// Run health check for each provider
				healthResult := checkProviderHealth(ctx, providerCfg)
				if healthResult.Healthy() {
					status.Status = "OK"
					status.Version = healthResult.Version
				} else {
					status.Status = "ERROR"
					status.Message = formatHealthError(healthResult)
				}

				statuses = append(statuses, status)
			}

			// Output results
			if useJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(statuses)
			}

			// Table output
			fmt.Fprintln(out)
			fmt.Fprintln(out, "Configured Providers:")
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tTYPE\tSTATUS\tDEFAULT")
			fmt.Fprintln(w, "  ────\t────\t──────\t───────")
			for _, s := range statuses {
				defaultMark := ""
				if s.IsDefault {
					defaultMark = "*"
				}
				statusDisplay := fmt.Sprintf("[%s]", s.Status)
				if s.Status == "ERROR" {
					statusDisplay = fmt.Sprintf("[%s]  (%s)", s.Status, s.Message)
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", s.Name, s.Type, statusDisplay, defaultMark)
			}
			w.Flush()
			fmt.Fprintln(out)
			if cfg.DefaultProvider != "" {
				fmt.Fprintf(out, "Default provider: %s\n", cfg.DefaultProvider)
			} else {
				fmt.Fprintln(out, "No default provider set")
			}
			fmt.Fprintln(out)

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

func newProvidersAddCmd() *cobra.Command {
	var (
		providerType string
		apiKey       string
		configPath   string
		dryRun       bool
	)

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new provider",
		Long: `Add a new provider configuration interactively or with flags.

If provider type is not specified, you will be prompted to choose one.

Examples:
  conductor providers add work-claude
  conductor providers add work-claude --type claude-code
  conductor providers add anthropic-prod --type anthropic --api-key $ANTHROPIC_API_KEY`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := args[0]

			// Load existing configuration
			cfgPath, err := getConfigPathOrDefault()
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				// If config doesn't exist, create a new one
				cfg = config.Default()
				cfg.Providers = make(config.ProvidersMap)
			}

			// Check if provider already exists
			if _, exists := cfg.Providers[providerName]; exists {
				return fmt.Errorf("provider %q already exists. Use 'conductor providers remove %s' first to replace it", providerName, providerName)
			}

			// Dry-run mode requires --type to be specified (non-interactive)
			if dryRun && providerType == "" {
				return fmt.Errorf("--type is required in dry-run mode")
			}

			// Interactive mode if type not specified
			if providerType == "" {
				fmt.Printf("Adding provider: %s\n\n", providerName)

				// Get visible provider types (filtered by support level)
				visibleTypes := config.GetVisibleProviderTypes()

				// Build interactive menu from visible types
				fmt.Println("Select provider type:")
				typeDescriptions := map[string]string{
					"claude-code": "Claude Code CLI (recommended)",
					"anthropic":   "Anthropic API (experimental)",
					"openai":      "OpenAI API (experimental)",
					"ollama":      "Ollama local API (experimental)",
				}

				typeMap := make(map[string]string) // choice -> type
				for i, pType := range visibleTypes {
					choiceNum := fmt.Sprintf("%d", i+1)
					desc := typeDescriptions[pType]
					if desc == "" {
						desc = pType
					}
					fmt.Printf("  %s) %-12s - %s\n", choiceNum, pType, desc)
					typeMap[choiceNum] = pType
				}
				fmt.Print("\nChoice [1]: ")

				var choice string
				fmt.Scanln(&choice)
				if choice == "" {
					choice = "1"
				}

				selectedType, ok := typeMap[choice]
				if !ok {
					return fmt.Errorf("invalid choice: %s", choice)
				}
				providerType = selectedType
			}

			// Warn if using unsupported provider type
			config.WarnUnsupportedProvider(providerType)

			// Create provider config
			providerCfg := config.ProviderConfig{
				Type: providerType,
			}

			// Type-specific configuration
			switch providerType {
			case "claude-code":
				if configPath != "" {
					providerCfg.ConfigPath = configPath
				}
				// Verify Claude CLI is available
				ctx := context.Background()
				p := &claudecode.Provider{}
				found, err := p.Detect()
				if err != nil || !found {
					fmt.Println()
					fmt.Println("Warning: Claude CLI not found in PATH")
					fmt.Println("Install from: https://claude.ai/download")
					fmt.Println()
					fmt.Print("Continue anyway? [y/N]: ")
					var confirm string
					fmt.Scanln(&confirm)
					if strings.ToLower(confirm) != "y" {
						return fmt.Errorf("cancelled")
					}
				} else {
					// Run health check
					result := p.HealthCheck(ctx)
					if !result.Healthy() {
						fmt.Println()
						fmt.Printf("Warning: %s\n", result.Message)
						fmt.Println()
					} else {
						fmt.Println()
						fmt.Println("Claude CLI detected and healthy")
						if result.Version != "" {
							fmt.Printf("Version: %s\n", result.Version)
						}
						fmt.Println()
					}
				}

			case "anthropic", "openai":
				// Prompt for API key if not provided
				if apiKey == "" {
					fmt.Printf("\nEnter API key for %s (or set via environment variable): ", providerType)
					fmt.Scanln(&apiKey)
					if apiKey == "" {
						return fmt.Errorf("API key is required for %s provider", providerType)
					}
				}
				providerCfg.APIKey = apiKey

			case "ollama":
				// Ollama doesn't need additional config for default local installation
				fmt.Println()
				fmt.Println("Ollama provider configured for local API (http://localhost:11434)")

			default:
				return fmt.Errorf("unsupported provider type: %s", providerType)
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
				return providerAddDryRun(cfgPath, providerName, providerCfg, setAsDefault)
			}

			if setAsDefault {
				fmt.Printf("\nSet %s as default provider\n", providerName)
			}

			// Save configuration
			if err := config.WriteConfig(cfg, cfgPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("\nProvider %q added successfully\n", providerName)
			fmt.Printf("Config saved to: %s\n", cfgPath)
			fmt.Println()
			fmt.Println("Test the provider with:")
			fmt.Printf("  conductor providers test %s\n", providerName)
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVar(&providerType, "type", "", "Provider type (claude-code, anthropic, openai, ollama)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for the provider (for API-based providers)")
	cmd.Flags().StringVar(&configPath, "config-path", "", "Custom config path (for claude-code)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be modified without executing")

	return cmd
}

func newProvidersRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a provider",
		Long:  "Remove a provider configuration from the config file.",
		Args:  cobra.ExactArgs(1),
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

			// If this was the default provider, clear it
			if cfg.DefaultProvider == providerName {
				cfg.DefaultProvider = ""
				fmt.Printf("\nWarning: %q was the default provider. Use 'conductor providers set-default' to set a new default.\n", providerName)
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

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func newProvidersTestCmd() *cobra.Command {
	var (
		jsonOutput bool
		testAll    bool
	)

	cmd := &cobra.Command{
		Use:   "test [name]",
		Short: "Test provider connectivity",
		Long: `Run health check on a provider to verify it's working.

Tests three aspects:
  1. Installed - Provider CLI/library is available
  2. Authenticated - Provider has valid credentials
  3. Working - Provider can make successful API calls

See also: conductor providers list, conductor doctor, conductor providers add`,
		Example: `  # Example 1: Test specific provider
  conductor providers test claude-code

  # Example 2: Test all configured providers
  conductor providers test --all

  # Example 3: Test and get JSON output
  conductor providers test claude-code --json

  # Example 4: Verify provider before running workflow
  conductor providers test default && conductor run workflow.yaml`,
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

			// Check global --json flag in addition to local flag
			useJSON := shared.GetJSON() || jsonOutput

			// Test each provider
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			results := make([]ProviderStatus, 0, len(providersToTest))
			for name, providerCfg := range providersToTest {
				if !useJSON && len(providersToTest) > 1 {
					fmt.Printf("\nTesting %s (%s)...\n", name, providerCfg.Type)
				}

				result := checkProviderHealth(ctx, providerCfg)

				status := ProviderStatus{
					Name:    name,
					Type:    providerCfg.Type,
					Version: result.Version,
				}

				if useJSON {
					if result.Healthy() {
						status.Status = "OK"
					} else {
						status.Status = "ERROR"
						status.Message = formatHealthError(result)
					}
					results = append(results, status)
				} else {
					// Display detailed progress
					fmt.Printf("  [%s] Installed\n", checkMark(result.Installed))
					fmt.Printf("  [%s] Authenticated\n", checkMark(result.Authenticated))
					fmt.Printf("  [%s] Working\n", checkMark(result.Working))

					if result.Version != "" {
						fmt.Printf("\n  Version: %s\n", result.Version)
					}

					if result.Healthy() {
						fmt.Println("\n  Status: Healthy")
					} else {
						fmt.Printf("\n  Status: Failed at step '%s'\n", result.ErrorStep)
						fmt.Printf("  Error: %v\n", result.Error)
						if result.Message != "" {
							fmt.Printf("\n%s\n", result.Message)
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

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().BoolVar(&testAll, "all", false, "Test all configured providers")

	return cmd
}

func newProvidersSetDefaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-default <name>",
		Short: "Set the default provider",
		Long: `Set which provider to use by default for workflow execution.

The default provider is used when:
  - No agent mapping is specified for a step
  - No CONDUCTOR_PROVIDER environment variable is set

Example:
  conductor providers set-default claude-code`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
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
				return fmt.Errorf("provider %q not found. Available providers: %v", providerName, keysOf(cfg.Providers))
			}

			// Update default
			oldDefault := cfg.DefaultProvider
			cfg.DefaultProvider = providerName

			// Save configuration
			if err := config.WriteConfig(cfg, cfgPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			if oldDefault != "" {
				fmt.Fprintf(out, "Default provider changed from %q to %q\n", oldDefault, providerName)
			} else {
				fmt.Fprintf(out, "Default provider set to %q\n", providerName)
			}

			return nil
		},
	}

	return cmd
}

// Helper functions

// getConfigPathOrDefault returns the config path from the flag or falls back to XDG default
func getConfigPathOrDefault() (string, error) {
	cfgPath := shared.GetConfigPath()
	if cfgPath == "" {
		return config.ConfigPath()
	}
	return cfgPath, nil
}

// checkProviderHealth runs a health check for the given provider configuration
func checkProviderHealth(ctx context.Context, providerCfg config.ProviderConfig) llm.HealthCheckResult {
	// For now, only implement claude-code health checks
	// Other providers will be added as they're implemented
	switch providerCfg.Type {
	case "claude-code":
		p := &claudecode.Provider{}
		if hc, ok := interface{}(p).(llm.HealthCheckable); ok {
			return hc.HealthCheck(ctx)
		}
	case "anthropic", "openai", "ollama":
		// Not yet implemented - return basic result
		return llm.HealthCheckResult{
			Installed:     true,
			Authenticated: true,
			Working:       false,
			Message:       fmt.Sprintf("Health checks not yet implemented for %s provider", providerCfg.Type),
		}
	}

	return llm.HealthCheckResult{
		Error:   fmt.Errorf("unknown provider type: %s", providerCfg.Type),
		Message: "Unknown provider type",
	}
}

// formatHealthError formats a health check result error message
func formatHealthError(result llm.HealthCheckResult) string {
	if result.Error != nil {
		return result.Error.Error()
	}
	if !result.Installed {
		return "not installed"
	}
	if !result.Authenticated {
		return "not authenticated"
	}
	if !result.Working {
		return "connectivity failed"
	}
	return "unknown error"
}

// checkMark returns a check mark or X based on boolean value
func checkMark(ok bool) string {
	if ok {
		return "OK"
	}
	return "FAILED"
}

// keysOf returns the keys of a ProvidersMap as a slice
func keysOf(m config.ProvidersMap) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// providerAddDryRun shows what would be modified when adding a provider
func providerAddDryRun(cfgPath, providerName string, providerCfg config.ProviderConfig, setAsDefault bool) error {
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
		maskedKey := shared.MaskSensitiveData("api_key", providerCfg.APIKey)
		description += fmt.Sprintf(", api_key: %s", maskedKey)
	}

	// Add information about config path if present
	if providerCfg.ConfigPath != "" {
		description += fmt.Sprintf(", config_path: %s", providerCfg.ConfigPath)
	}

	// Add default provider note
	if setAsDefault {
		description += ", set as default"
	}

	output.DryRunModify(placeholderPath, description)

	// Print the output
	fmt.Println(output.String())

	return nil
}
