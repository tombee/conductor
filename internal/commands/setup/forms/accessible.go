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

package forms

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tombee/conductor/internal/commands/setup"
	"github.com/tombee/conductor/internal/commands/setup/validation"
	"github.com/tombee/conductor/internal/config"
)

// AccessibleWizard provides a text-based wizard flow for accessible mode.
// No ANSI codes, no cursor movement - just numbered prompts and plain text.
type AccessibleWizard struct {
	ctx     context.Context
	state   *setup.SetupState
	scanner *bufio.Scanner
}

// NewAccessibleWizard creates a new accessible wizard instance.
func NewAccessibleWizard(ctx context.Context, state *setup.SetupState) *AccessibleWizard {
	return &AccessibleWizard{
		ctx:     ctx,
		state:   state,
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// Run executes the accessible wizard flow.
func (w *AccessibleWizard) Run() error {
	fmt.Println("=== Conductor Setup Wizard (Accessible Mode) ===")
	fmt.Println()

	// Step 1: Welcome
	if err := w.showWelcome(); err != nil {
		return err
	}

	// Step 2: Select Provider
	providerType, err := w.selectProvider()
	if err != nil {
		return err
	}
	if providerType == "" {
		return nil // User cancelled
	}

	// Step 3: Configure Provider
	if err := w.configureProvider(providerType); err != nil {
		return err
	}

	// Step 4: Review
	if err := w.reviewConfiguration(); err != nil {
		return err
	}

	// Step 5: Save
	if err := w.saveConfiguration(); err != nil {
		return err
	}

	// Step 6: Complete
	w.showCompletion()

	return nil
}

// showWelcome displays the welcome message.
func (w *AccessibleWizard) showWelcome() error {
	fmt.Println("Step 1 of 5: Welcome")
	fmt.Println()
	fmt.Println("This wizard will guide you through configuring Conductor.")
	fmt.Println()
	fmt.Println("You'll configure:")
	fmt.Println("  - LLM provider (Claude Code, Ollama, Anthropic, OpenAI)")
	fmt.Println("  - Secrets storage (keychain or environment variables)")
	fmt.Println()
	fmt.Println("Press Enter to continue...")

	if !w.scanner.Scan() {
		return fmt.Errorf("failed to read input")
	}

	return nil
}

// selectProvider prompts the user to select a provider.
func (w *AccessibleWizard) selectProvider() (string, error) {
	fmt.Println()
	fmt.Println("Step 2 of 5: Select Provider")
	fmt.Println()

	allProviders := setup.GetProviderTypes()

	// Separate CLI and API providers
	var cliProviders, apiProviders []setup.ProviderType
	for _, pt := range allProviders {
		if pt.IsCLI() {
			cliProviders = append(cliProviders, pt)
		} else {
			apiProviders = append(apiProviders, pt)
		}
	}

	fmt.Println("Choose an LLM provider:")
	fmt.Println()

	optionNum := 1
	optionMap := make(map[int]string)

	// Show CLI providers
	if len(cliProviders) > 0 {
		fmt.Println("Local Providers (CLI tools):")
		for _, pt := range cliProviders {
			fmt.Printf("  %d. %s - %s\n", optionNum, pt.DisplayName(), pt.Description())
			optionMap[optionNum] = pt.Name()
			optionNum++
		}
		fmt.Println()
	}

	// Show API providers
	if len(apiProviders) > 0 {
		fmt.Println("Cloud API Providers:")
		for _, pt := range apiProviders {
			fmt.Printf("  %d. %s - %s\n", optionNum, pt.DisplayName(), pt.Description())
			optionMap[optionNum] = pt.Name()
			optionNum++
		}
		fmt.Println()
	}

	fmt.Println("  0. Cancel and exit")
	fmt.Println()

	// Get user choice
	for {
		fmt.Print("Enter your choice (0-" + strconv.Itoa(optionNum-1) + "): ")
		if !w.scanner.Scan() {
			return "", fmt.Errorf("failed to read input")
		}

		choice := strings.TrimSpace(w.scanner.Text())
		choiceNum, err := strconv.Atoi(choice)
		if err != nil {
			fmt.Println("Invalid input. Please enter a number.")
			continue
		}

		if choiceNum == 0 {
			return "", nil // User cancelled
		}

		if providerType, ok := optionMap[choiceNum]; ok {
			return providerType, nil
		}

		fmt.Printf("Invalid choice. Please enter a number between 0 and %d.\n", optionNum-1)
	}
}

// configureProvider configures the selected provider.
func (w *AccessibleWizard) configureProvider(providerTypeName string) error {
	fmt.Println()
	fmt.Println("Step 3 of 5: Configure Provider")
	fmt.Println()

	providerType, ok := setup.GetProviderType(providerTypeName)
	if !ok {
		return fmt.Errorf("unknown provider type: %s", providerTypeName)
	}

	// CLI providers are auto-configured
	if providerType.IsCLI() {
		return w.configureCLIProvider(providerType)
	}

	// API providers need configuration
	return w.configureAPIProvider(providerType)
}

// configureCLIProvider configures a CLI provider.
func (w *AccessibleWizard) configureCLIProvider(providerType setup.ProviderType) error {
	instanceName := providerType.Name()

	fmt.Printf("Configuring %s...\n", providerType.DisplayName())
	fmt.Println()
	fmt.Println("This provider uses a local CLI tool and doesn't require additional configuration.")
	fmt.Println()

	// Create provider config
	providerCfg := providerType.CreateConfig()

	// Add to state
	w.state.Working.Providers[instanceName] = providerCfg
	if w.state.Working.DefaultProvider == "" {
		w.state.Working.DefaultProvider = instanceName
	}
	w.state.MarkDirty()

	fmt.Println("Provider configured successfully.")
	return nil
}

// configureAPIProvider configures an API provider.
func (w *AccessibleWizard) configureAPIProvider(providerType setup.ProviderType) error {
	fmt.Printf("Configuring %s...\n", providerType.DisplayName())
	fmt.Println()

	// Step 1: Get instance name
	instanceName, err := w.promptProviderName(providerType)
	if err != nil {
		return err
	}

	// Step 2: Configure provider-specific fields
	providerCfg := providerType.CreateConfig()

	// If provider requires base URL, prompt for it
	if providerType.RequiresBaseURL() {
		baseURL, err := w.promptBaseURL(providerType)
		if err != nil {
			return err
		}
		providerCfg.BaseURL = baseURL
	}

	// Step 3: API Key configuration with inline backend selection
	if providerType.RequiresAPIKey() {
		apiKey, err := w.promptAPIKey(providerType, instanceName)
		if err != nil {
			return err
		}

		// Check if this is the first API provider (need to select backend)
		needsBackendSelection := w.state.SecretsBackend == ""

		// Select backend if needed
		var selectedBackend string
		if needsBackendSelection {
			selectedBackend, err = w.selectBackend()
			if err != nil {
				return err
			}

			// Set as default backend for future API keys
			w.state.SecretsBackend = selectedBackend
			w.state.MarkDirty()

			// Log backend selection
			if w.state.Audit != nil {
				w.state.Audit.LogBackendSelected(selectedBackend)
			}
		} else {
			// Use existing default backend
			selectedBackend = w.state.SecretsBackend
		}

		// Store in credential store for later persistence
		credKey := fmt.Sprintf("provider:%s:api_key", instanceName)
		w.state.CredentialStore[credKey] = apiKey

		// Store reference in config with backend prefix
		providerCfg.APIKey = fmt.Sprintf("$%s:%s_API_KEY", selectedBackend, strings.ToUpper(instanceName))
	}

	// Add to state
	w.state.Working.Providers[instanceName] = providerCfg
	if w.state.Working.DefaultProvider == "" {
		w.state.Working.DefaultProvider = instanceName
	}
	w.state.MarkDirty()

	fmt.Println()
	fmt.Println("Provider configured successfully.")
	return nil
}

// promptProviderName prompts for the provider instance name.
func (w *AccessibleWizard) promptProviderName(providerType setup.ProviderType) (string, error) {
	defaultName := providerType.Name()
	if defaultName == "openai-compatible" {
		defaultName = "" // Force user to provide a descriptive name
	}

	fmt.Println("Enter a unique name for this provider.")
	fmt.Println("Examples: anthropic, openai, truefoundry, azure-openai")
	fmt.Println()

	for {
		if defaultName != "" {
			fmt.Printf("Provider name [%s]: ", defaultName)
		} else {
			fmt.Print("Provider name: ")
		}

		if !w.scanner.Scan() {
			return "", fmt.Errorf("failed to read input")
		}

		name := strings.TrimSpace(w.scanner.Text())
		if name == "" && defaultName != "" {
			name = defaultName
		}

		if name == "" {
			fmt.Println("Provider name is required.")
			continue
		}

		if _, exists := w.state.Working.Providers[name]; exists {
			fmt.Printf("Provider %q already exists. Please choose a different name.\n", name)
			continue
		}

		return name, nil
	}
}

// promptBaseURL prompts for the base URL.
func (w *AccessibleWizard) promptBaseURL(providerType setup.ProviderType) (string, error) {
	defaultURL := providerType.DefaultBaseURL()

	fmt.Println()
	fmt.Println("Enter the base URL for this provider.")
	if defaultURL != "" {
		fmt.Printf("Default: %s\n", defaultURL)
	}
	fmt.Println()

	for {
		if defaultURL != "" {
			fmt.Printf("Base URL [%s]: ", defaultURL)
		} else {
			fmt.Print("Base URL: ")
		}

		if !w.scanner.Scan() {
			return "", fmt.Errorf("failed to read input")
		}

		url := strings.TrimSpace(w.scanner.Text())
		if url == "" && defaultURL != "" {
			url = defaultURL
		}

		if url == "" && defaultURL == "" {
			fmt.Println("Base URL is required.")
			continue
		}

		if url != "" {
			if err := validation.ValidateURL(url); err != nil {
				fmt.Printf("Invalid URL: %v\n", err)
				continue
			}
		}

		return url, nil
	}
}

// promptAPIKey prompts for the API key.
func (w *AccessibleWizard) promptAPIKey(providerType setup.ProviderType, instanceName string) (string, error) {
	fmt.Println()
	fmt.Println("Enter the API key for this provider.")
	fmt.Println("Your input will be hidden for security.")
	fmt.Println()

	for {
		fmt.Print("API Key: ")

		// In accessible mode, we can't hide input, so we just warn the user
		if !w.scanner.Scan() {
			return "", fmt.Errorf("failed to read input")
		}

		apiKey := strings.TrimSpace(w.scanner.Text())
		if apiKey == "" {
			fmt.Println("API key is required.")
			continue
		}

		return apiKey, nil
	}
}

// selectBackend prompts the user to select a secrets backend.
func (w *AccessibleWizard) selectBackend() (string, error) {
	fmt.Println()
	fmt.Println("Step 4 of 5: Choose Storage")
	fmt.Println()
	fmt.Println("Where should API keys be stored?")
	fmt.Println()
	fmt.Println("  1. Keychain - Secure system keychain (recommended)")
	fmt.Println("  2. Environment variables - Store in .env file")
	fmt.Println()

	for {
		fmt.Print("Enter your choice (1-2): ")
		if !w.scanner.Scan() {
			return "", fmt.Errorf("failed to read input")
		}

		choice := strings.TrimSpace(w.scanner.Text())
		switch choice {
		case "1":
			return "keychain", nil
		case "2":
			return "env", nil
		default:
			fmt.Println("Invalid choice. Please enter 1 or 2.")
		}
	}
}

// reviewConfiguration shows the configuration review.
func (w *AccessibleWizard) reviewConfiguration() error {
	fmt.Println()
	fmt.Println("Step 5 of 5: Review Configuration")
	fmt.Println()
	fmt.Println("Please review your configuration:")
	fmt.Println()

	// Show providers
	fmt.Println("Providers:")
	if len(w.state.Working.Providers) == 0 {
		fmt.Println("  (none)")
	} else {
		for name, provider := range w.state.Working.Providers {
			marker := " "
			if name == w.state.Working.DefaultProvider {
				marker = "*"
			}
			fmt.Printf("  %s %s (%s)\n", marker, name, provider.Type)
		}
	}
	fmt.Println()

	// Show secrets backend
	if w.state.SecretsBackend != "" {
		fmt.Printf("Secrets storage: %s\n", w.state.SecretsBackend)
		fmt.Println()
	}

	// Confirm
	for {
		fmt.Print("Save this configuration? (y/n): ")
		if !w.scanner.Scan() {
			return fmt.Errorf("failed to read input")
		}

		choice := strings.ToLower(strings.TrimSpace(w.scanner.Text()))
		switch choice {
		case "y", "yes":
			return nil
		case "n", "no":
			return fmt.Errorf("configuration cancelled by user")
		default:
			fmt.Println("Please enter 'y' or 'n'.")
		}
	}
}

// saveConfiguration saves the configuration to disk.
func (w *AccessibleWizard) saveConfiguration() error {
	fmt.Println()
	fmt.Println("Saving configuration...")

	if err := config.WriteConfig(w.state.Working, w.state.ConfigPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	w.state.Dirty = false
	fmt.Println("Configuration saved successfully.")

	return nil
}

// showCompletion displays the completion message.
func (w *AccessibleWizard) showCompletion() {
	fmt.Println()
	fmt.Println("=== Setup Complete ===")
	fmt.Println()
	fmt.Println("Conductor has been configured successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Create a workflow YAML file")
	fmt.Println("  2. Run: conductor run <workflow.yaml>")
	fmt.Println()
	fmt.Println("For help, run: conductor --help")
	fmt.Println()
}
