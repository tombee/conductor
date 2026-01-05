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
	"net/http"
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
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers"
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

	// Phase 1: Get provider type first
	var providerType string

	typeForm := huh.NewForm(
		huh.NewGroup(
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

	if err := typeForm.Run(); err != nil {
		if err == huh.ErrUserAborted {
			os.Exit(130) // Standard exit code for SIGINT
		}
		return fmt.Errorf("form cancelled: %w", err)
	}

	// Phase 2: Get provider name (defaults to provider type)
	var providerName string

	nameForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Provider Name").
				Description("A unique identifier for this provider").
				Placeholder(providerType).
				Validate(ValidateProviderNameOrEmptyFunc(cfg.Providers, providerType)).
				Value(&providerName),
		),
	)

	if err := nameForm.Run(); err != nil {
		if err == huh.ErrUserAborted {
			os.Exit(130) // Standard exit code for SIGINT
		}
		return fmt.Errorf("form cancelled: %w", err)
	}

	// Default to provider type if left empty
	if providerName == "" {
		providerName = providerType
	}

	// Create provider config
	providerCfg := config.ProviderConfig{
		Type: providerType,
	}

	// Phase 3: Type-specific configuration with health check
	for {
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

		// Health check
		fmt.Print(shared.Muted.Render("Testing connection... "))
		if err := runProviderHealthCheck(cmd.Context(), providerType, &providerCfg); err != nil {
			fmt.Println(shared.RenderWarn("failed"))
			fmt.Fprintf(os.Stderr, "  %s\n\n", shared.Muted.Render(err.Error()))

			var action string
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("What would you like to do?").
						Options(
							huh.NewOption("Retry connection", "retry"),
							huh.NewOption("Reconfigure provider", "reconfigure"),
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
				fmt.Println()
				continue
			case "reconfigure":
				fmt.Println()
				continue
			case "cancel":
				os.Exit(130)
			}
		} else {
			fmt.Println(shared.RenderOK("connected"))
			break
		}
	}

	// Initialize Models map
	if providerCfg.Models == nil {
		providerCfg.Models = make(map[string]config.ModelConfig)
	}

	// Get default setup for claude-code (used for tier mappings)
	var setup *llm.SetupConfig
	if providerType == "claude-code" {
		p := claudecode.New()
		setup = p.DefaultSetup()
	}

	// Add provider to config
	if cfg.Providers == nil {
		cfg.Providers = make(config.ProvidersMap)
	}
	cfg.Providers[providerName] = providerCfg

	// Configure tier mappings from provider defaults
	tiersConfigured := false
	if setup != nil && len(setup.TierMappings) > 0 {
		if err := configureTiersFromSetup(cfg, providerName, setup); err != nil {
			return err
		}
		tiersConfigured = len(cfg.Tiers) > 0
	}

	// Save configuration
	if err := config.WriteConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Success message
	fmt.Printf("\n%s\n", shared.RenderOK(fmt.Sprintf("Provider %q added successfully", providerName)))
	if tiersConfigured {
		fmt.Printf("  %s fast→%s, balanced→%s, strategic→%s\n",
			shared.Muted.Render("Tiers:"), cfg.Tiers["fast"], cfg.Tiers["balanced"], cfg.Tiers["strategic"])
	}
	fmt.Printf("  %s %s\n", shared.Muted.Render("Config saved to:"), cfgPath)
	fmt.Println()

	// Auto-run model discovery for supported providers
	modelsDiscovered := false
	if supportsModelDiscovery(providerType) {
		fmt.Print(shared.Muted.Render("Discovering models... "))
		models, err := runModelDiscovery(cmd.Context(), providerType, &providerCfg)
		if err != nil {
			fmt.Println(shared.RenderWarn("failed"))
			fmt.Fprintf(os.Stderr, "  %s\n", shared.Muted.Render(err.Error()))
		} else if len(models) > 0 {
			// Save discovered models
			for _, model := range models {
				providerCfg.Models[model.ID] = config.ModelConfig{
					ContextWindow:      model.MaxTokens,
					InputPricePerMTok:  model.InputPricePerMillion,
					OutputPricePerMTok: model.OutputPricePerMillion,
				}
			}
			cfg.Providers[providerName] = providerCfg
			if err := config.WriteConfig(cfg, cfgPath); err != nil {
				fmt.Println(shared.RenderWarn("failed to save"))
			} else {
				var modelNames []string
				for name := range providerCfg.Models {
					modelNames = append(modelNames, name)
				}
				sort.Strings(modelNames)
				fmt.Println(shared.RenderOK(fmt.Sprintf("found %d", len(models))))
				fmt.Printf("  %s %s\n", shared.Muted.Render("Models:"), strings.Join(modelNames, ", "))
				modelsDiscovered = true
			}
		} else {
			fmt.Println(shared.Muted.Render("none found"))
		}
		fmt.Println()
	}

	// Configure tier mappings if not already set and models were discovered
	if !tiersConfigured && modelsDiscovered && len(providerCfg.Models) > 0 {
		if err := configureTiersInteractive(cfg, providerName, providerCfg.Models, cfgPath); err != nil {
			if err != huh.ErrUserAborted {
				fmt.Fprintf(os.Stderr, "%s\n", shared.RenderWarn(fmt.Sprintf("Failed to configure tiers: %v", err)))
			}
		} else {
			tiersConfigured = len(cfg.Tiers) > 0
		}
	}

	// Next steps (only show if there's something to do)
	if !modelsDiscovered || !tiersConfigured {
		fmt.Println(shared.Header.Render("Next steps:"))
		if !modelsDiscovered {
			fmt.Printf("  %s  %s\n", shared.StatusInfo.Render(fmt.Sprintf("conductor model discover %s", providerName)), shared.Muted.Render("# Discover available models"))
		}
		if !tiersConfigured {
			fmt.Printf("  %s  %s\n", shared.StatusInfo.Render("conductor model set-tier fast <provider/model>"), shared.Muted.Render("# Configure tier mappings"))
		}
		fmt.Println()
	}

	return nil
}

// configureClaudeCode handles Claude Code provider configuration.
func configureClaudeCode(ctx context.Context, cfg *config.ProviderConfig) error {
	fmt.Println("\n" + shared.Muted.Render("Detecting Claude CLI..."))

	p := claudecode.New()
	found, err := p.Detect()
	if err != nil || !found {
		fmt.Fprintf(os.Stderr, "\n%s\n", shared.RenderWarn("Claude CLI not found in PATH"))
		fmt.Fprintf(os.Stderr, "%s\n\n", shared.StatusInfo.Render("Install from: https://claude.ai/download"))

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
			fmt.Println(shared.RenderOK(fmt.Sprintf("Claude CLI detected %s", shared.Muted.Render("(version: "+result.Version+")"))))
		} else {
			fmt.Println(shared.RenderOK("Claude CLI detected and working"))
		}
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", shared.RenderWarn(result.Message))
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
	fmt.Println("\n" + shared.Muted.Render("Verifying API key..."))
	if err := runHealthCheck(ctx, providerType, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", shared.RenderWarn(err.Error()))
		fmt.Fprintf(os.Stderr, "%s\n", shared.Muted.Render("Provider will be saved but may not work until the issue is resolved."))
	} else {
		fmt.Println(shared.RenderOK("API key verified"))
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
	fmt.Printf("\n%s\n", shared.RenderOK(fmt.Sprintf("Configured Ollama at %s", baseURL)))

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

// configureTiersFromSetup configures tier mappings using provider defaults.
// If tiers already exist, asks the user if they want to update them.
func configureTiersFromSetup(cfg *config.Config, providerName string, setup *llm.SetupConfig) error {
	if setup == nil || len(setup.TierMappings) == 0 {
		return nil
	}

	// Build the tier mappings with full provider/model references
	newTiers := make(map[string]string)
	for tier, model := range setup.TierMappings {
		newTiers[tier] = fmt.Sprintf("%s/%s", providerName, model)
	}

	// Check if tiers already exist
	if len(cfg.Tiers) > 0 {
		// Ask user if they want to update
		var updateTiers bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Update tier mappings?").
					Description(fmt.Sprintf("Tiers are already configured. Update to use %s?", providerName)).
					Value(&updateTiers),
			),
		)

		if err := form.Run(); err != nil {
			if err == huh.ErrUserAborted {
				os.Exit(130)
			}
			return err
		}

		if !updateTiers {
			return nil
		}
	}

	// Apply the tier mappings
	if cfg.Tiers == nil {
		cfg.Tiers = make(map[string]string)
	}
	for tier, ref := range newTiers {
		cfg.Tiers[tier] = ref
	}

	return nil
}

// supportsModelDiscovery returns true if the provider type supports auto-discovery.
func supportsModelDiscovery(providerType string) bool {
	switch providerType {
	case "claude-code", "ollama":
		return true
	default:
		return false
	}
}

// runProviderHealthCheck performs a health check on the provider.
func runProviderHealthCheck(ctx context.Context, providerType string, cfg *config.ProviderConfig) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	switch providerType {
	case "claude-code":
		p := claudecode.New()
		result := p.HealthCheck(ctx)
		if !result.Healthy() {
			return fmt.Errorf("%s", result.Message)
		}
		return nil
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		// Simple health check - try to reach the API
		resp, err := http.Get(baseURL + "/api/tags")
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}
		return nil
	default:
		return fmt.Errorf("health check not implemented for %s", providerType)
	}
}

// runModelDiscovery discovers available models from a provider.
func runModelDiscovery(ctx context.Context, providerType string, cfg *config.ProviderConfig) ([]llm.ModelInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	switch providerType {
	case "claude-code":
		p := claudecode.New()
		caps := p.Capabilities()
		return caps.Models, nil
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		creds := llm.OllamaCredentials{BaseURL: baseURL}
		provider, err := providers.NewOllamaWithCredentials(creds)
		if err != nil {
			return nil, err
		}
		discoverer, ok := provider.(llm.ModelDiscoverer)
		if !ok {
			return nil, fmt.Errorf("provider does not support model discovery")
		}
		return discoverer.DiscoverModels(ctx)
	default:
		return nil, fmt.Errorf("discovery not implemented for %s", providerType)
	}
}

// configureTiersInteractive prompts the user to configure tier mappings using discovered models.
func configureTiersInteractive(cfg *config.Config, providerName string, models map[string]config.ModelConfig, cfgPath string) error {
	// Build list of model options
	var modelNames []string
	for name := range models {
		modelNames = append(modelNames, name)
	}
	sort.Strings(modelNames)

	// Create options including "Skip" option
	buildOptions := func() []huh.Option[string] {
		options := make([]huh.Option[string], 0, len(modelNames)+1)
		options = append(options, huh.NewOption[string]("(skip)", ""))
		for _, name := range modelNames {
			options = append(options, huh.NewOption[string](name, name))
		}
		return options
	}

	fmt.Println("Configure Model Tiers")
	fmt.Println(shared.Muted.Render("Select which models to use for each tier, or skip to configure later."))
	fmt.Println()

	tiers := []struct {
		name  string
		key   string
		desc  string
		value string
	}{
		{"Fast tier", "fast", "Low-latency model for simple tasks", ""},
		{"Balanced tier", "balanced", "General-purpose model (default for most workflows)", ""},
		{"Strategic tier", "strategic", "Most capable model for complex reasoning", ""},
	}

	for i := range tiers {
		var selected string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(tiers[i].name).
					Description(tiers[i].desc).
					Options(buildOptions()...).
					Value(&selected),
			),
		)

		if err := form.Run(); err != nil {
			return err
		}
		tiers[i].value = selected
	}

	// Apply tier mappings
	if cfg.Tiers == nil {
		cfg.Tiers = make(map[string]string)
	}

	tiersSet := 0
	for _, tier := range tiers {
		if tier.value != "" {
			cfg.Tiers[tier.key] = fmt.Sprintf("%s/%s", providerName, tier.value)
			tiersSet++
		}
	}

	if tiersSet == 0 {
		fmt.Println(shared.Muted.Render("No tiers configured. You can set them later with 'conductor model set-tier'."))
		return nil
	}

	// Save configuration
	if err := config.WriteConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("failed to save tier config: %w", err)
	}

	fmt.Printf("%s\n", shared.RenderOK(fmt.Sprintf("Configured %d tier(s)", tiersSet)))
	for _, tier := range tiers {
		if tier.value != "" {
			fmt.Printf("  %s %s/%s\n", shared.Muted.Render(tier.key+" →"), providerName, tier.value)
		}
	}
	fmt.Println()

	return nil
}
