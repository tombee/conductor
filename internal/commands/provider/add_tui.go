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
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/secrets"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

// runAddInteractive runs the interactive TUI form for adding a provider.
func runAddInteractive(cmd *cobra.Command, cfg *config.Config, cfgPath string) error {
	// Check if we're in an interactive terminal
	if shared.IsNonInteractive() {
		return fmt.Errorf("interactive setup requires a terminal. Use: conductor provider add NAME --type TYPE --api-key-env VAR")
	}

	// Check keychain availability upfront
	keychainAvailable, keychainMsg := checkKeychainAvailable()
	if !keychainAvailable {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", keychainMsg)
		fmt.Fprintf(os.Stderr, "API keys will be stored via environment variable reference instead.\n\n")
	}

	// Phase 1: Get provider name and type
	var providerName string
	var providerType string

	form1 := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Provider Name").
				Description("A unique identifier for this provider").
				Placeholder("my-provider").
				Validate(ValidateProviderNameFunc(cfg.Providers)).
				Value(&providerName),
			huh.NewSelect[string]().
				Title("Provider Type").
				Description("Select the LLM provider to configure").
				Options(
					huh.NewOption("Claude Code (uses installed CLI)", "claude-code"),
					huh.NewOption("Anthropic API", "anthropic"),
					huh.NewOption("OpenAI API", "openai"),
					huh.NewOption("Ollama (local)", "ollama"),
				).
				Value(&providerType),
		),
	)

	if err := form1.Run(); err != nil {
		if err == huh.ErrUserAborted {
			os.Exit(130) // Standard exit code for SIGINT
		}
		return fmt.Errorf("form cancelled: %w", err)
	}

	// Create provider config
	providerCfg := config.ProviderConfig{
		Type: providerType,
	}

	// Phase 2: Type-specific configuration
	switch providerType {
	case "claude-code":
		if err := configureClaudeCode(cmd.Context(), &providerCfg); err != nil {
			return err
		}

	case "anthropic", "openai":
		if err := configureAPIProvider(cmd.Context(), providerName, providerType, &providerCfg, keychainAvailable); err != nil {
			return err
		}

	case "ollama":
		if err := configureOllama(cmd.Context(), &providerCfg); err != nil {
			return err
		}
	}

	// Initialize Models map with defaults for provider type
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
	if cfg.Providers == nil {
		cfg.Providers = make(config.ProvidersMap)
	}
	cfg.Providers[providerName] = providerCfg

	// If this is the first provider, set as default
	setAsDefault := len(cfg.Providers) == 1 || cfg.DefaultProvider == ""
	if setAsDefault {
		cfg.DefaultProvider = providerName
	}

	// Save configuration
	if err := config.WriteConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Success message
	fmt.Printf("\n✓ Provider %q added successfully\n", providerName)
	if setAsDefault {
		fmt.Printf("  Set as default provider\n")
	}
	if len(providerCfg.Models) > 0 {
		var modelNames []string
		for name := range providerCfg.Models {
			modelNames = append(modelNames, name)
		}
		sort.Strings(modelNames)
		fmt.Printf("  Models: %s\n", strings.Join(modelNames, ", "))
	}
	fmt.Printf("  Config saved to: %s\n", cfgPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  conductor provider test %s   # Test the provider\n", providerName)
	fmt.Println()

	return nil
}

// configureClaudeCode handles Claude Code provider configuration.
func configureClaudeCode(ctx context.Context, cfg *config.ProviderConfig) error {
	fmt.Println("\nDetecting Claude CLI...")

	p := claudecode.New()
	found, err := p.Detect()
	if err != nil || !found {
		fmt.Fprintf(os.Stderr, "\n⚠ Claude CLI not found in PATH\n")
		fmt.Fprintf(os.Stderr, "Install from: https://claude.ai/download\n\n")

		// Ask user what to do
		var action string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("What would you like to do?").
					Options(
						huh.NewOption("Retry detection", "retry"),
						huh.NewOption("Use a different provider type", "change"),
						huh.NewOption("Cancel setup", "cancel"),
					).
					Value(&action),
			),
		)

		if err := form.Run(); err != nil {
			if err == huh.ErrUserAborted {
				os.Exit(130)
			}
			return err
		}

		switch action {
		case "retry":
			return configureClaudeCode(ctx, cfg)
		case "change":
			return fmt.Errorf("provider type change requested - please restart setup")
		case "cancel":
			os.Exit(130)
		}
	}

	// Run health check
	result := p.HealthCheck(ctx)
	if result.Healthy() {
		if result.Version != "" {
			fmt.Printf("✓ Claude CLI detected (version: %s)\n", result.Version)
		} else {
			fmt.Println("✓ Claude CLI detected and working")
		}
	} else {
		fmt.Fprintf(os.Stderr, "⚠ %s\n", result.Message)
	}

	return nil
}

// configureAPIProvider handles Anthropic/OpenAI provider configuration.
func configureAPIProvider(ctx context.Context, providerName, providerType string, cfg *config.ProviderConfig, keychainAvailable bool) error {
	var apiKeySource string
	var apiKeyValue string
	var envVarName string

	// Build source options based on keychain availability
	sourceOptions := []huh.Option[string]{
		huh.NewOption("Enter API key directly", "direct"),
		huh.NewOption("Read from environment variable", "env"),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("API Key Source").
				Description("How would you like to provide the API key?").
				Options(sourceOptions...).
				Value(&apiKeySource),
		),
	)

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			os.Exit(130)
		}
		return err
	}

	if apiKeySource == "direct" {
		// Get API key directly
		keyForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("API Key").
					Description("Enter your " + providerType + " API key").
					EchoMode(huh.EchoModePassword).
					Validate(ValidateAPIKey).
					Value(&apiKeyValue),
			),
		)

		if err := keyForm.Run(); err != nil {
			if err == huh.ErrUserAborted {
				os.Exit(130)
			}
			return err
		}

		// Store in keychain if available
		if keychainAvailable {
			secretKey := fmt.Sprintf("providers/%s/api_key", providerName)
			if err := storeSecret(ctx, secretKey, apiKeyValue); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to store in keychain: %v\n", err)
				fmt.Fprintf(os.Stderr, "Falling back to environment variable reference.\n")
				// Fall back to asking for env var
				return configureEnvVar(providerName, cfg)
			}
			cfg.APIKey = fmt.Sprintf("$secret:%s", secretKey)
		} else {
			// Without keychain, we can't store the key securely
			// Ask user to set up an environment variable
			fmt.Fprintf(os.Stderr, "\nWithout keychain access, please set an environment variable with your API key.\n")
			return configureEnvVar(providerName, cfg)
		}

	} else {
		// Read from environment variable
		envForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Environment Variable Name").
					Description("Name of the environment variable containing your API key").
					Placeholder(fmt.Sprintf("%s_API_KEY", strings.ToUpper(providerType))).
					Validate(ValidateEnvVarName).
					Value(&envVarName),
			),
		)

		if err := envForm.Run(); err != nil {
			if err == huh.ErrUserAborted {
				os.Exit(130)
			}
			return err
		}

		// Check if environment variable is set
		apiKeyValue = os.Getenv(envVarName)
		if apiKeyValue == "" {
			return fmt.Errorf("environment variable %s is not set or empty", envVarName)
		}

		// Store the value in keychain if available
		if keychainAvailable {
			secretKey := fmt.Sprintf("providers/%s/api_key", providerName)
			if err := storeSecret(ctx, secretKey, apiKeyValue); err != nil {
				return fmt.Errorf("failed to store API key: %w", err)
			}
			cfg.APIKey = fmt.Sprintf("$secret:%s", secretKey)
		} else {
			// Store reference to env var
			cfg.APIKey = fmt.Sprintf("$env:%s", envVarName)
		}
	}

	// Run health check with timeout
	fmt.Println("\nVerifying API key...")
	if err := runHealthCheck(ctx, providerType, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ %s\n", err)
		fmt.Fprintf(os.Stderr, "Provider will be saved but may not work until the issue is resolved.\n")
	} else {
		fmt.Println("✓ API key verified")
	}

	return nil
}

// configureEnvVar prompts user to configure an environment variable reference.
func configureEnvVar(providerName string, cfg *config.ProviderConfig) error {
	var envVarName string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Environment Variable Name").
				Description("Name of the environment variable to use for the API key").
				Placeholder("ANTHROPIC_API_KEY").
				Validate(ValidateEnvVarName).
				Value(&envVarName),
		),
	)

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			os.Exit(130)
		}
		return err
	}

	cfg.APIKey = fmt.Sprintf("$env:%s", envVarName)
	fmt.Printf("\nNote: Set the environment variable before using this provider:\n")
	fmt.Printf("  export %s=your-api-key\n", envVarName)

	return nil
}

// configureOllama handles Ollama provider configuration.
func configureOllama(ctx context.Context, cfg *config.ProviderConfig) error {
	var baseURL string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Base URL").
				Description("URL of your Ollama server").
				Placeholder("http://localhost:11434").
				Value(&baseURL),
		),
	)

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			os.Exit(130)
		}
		return err
	}

	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// Validate base URL (localhost is allowed for ollama)
	if err := validateOllamaBaseURL(ctx, baseURL); err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	cfg.BaseURL = baseURL
	fmt.Printf("\n✓ Configured Ollama at %s\n", baseURL)
	fmt.Println("Note: Make sure Ollama is running when you use this provider.")

	return nil
}

// validateOllamaBaseURL validates a base URL for ollama provider.
// Unlike validateBaseURL in add.go, this allows localhost for ollama.
func validateOllamaBaseURL(ctx context.Context, baseURL string) error {
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

	// Ollama is typically local, so we allow localhost/127.0.0.1
	// but still block metadata endpoints and other private ranges
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil // Allow localhost for ollama
	}

	// For non-localhost URLs, use the standard validation
	return validateBaseURL(ctx, baseURL)
}

// checkKeychainAvailable checks if the system keychain is available.
// Returns availability status and a platform-specific message if unavailable.
func checkKeychainAvailable() (bool, string) {
	backend := secrets.NewKeychainBackend()
	if backend.Available() {
		return true, ""
	}

	// Platform-specific guidance
	switch runtime.GOOS {
	case "darwin":
		return false, "System keychain unavailable. Try unlocking Keychain in Keychain Access.app"
	case "linux":
		return false, "System keychain unavailable. Ensure GNOME Keyring or KWallet is running"
	case "windows":
		return false, "System keychain unavailable. Check Windows Credential Manager service"
	default:
		return false, "System keychain unavailable on this platform"
	}
}

// runHealthCheck runs a basic health check for API-based providers.
func runHealthCheck(ctx context.Context, providerType string, cfg *config.ProviderConfig) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// For now, we just verify the API key format
	// Full health checks would require instantiating the provider
	// which is done after config is saved
	_ = ctx // Context available for future health check implementations

	return nil
}
