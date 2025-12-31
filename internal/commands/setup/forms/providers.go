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
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/tombee/conductor/internal/commands/setup"
	"github.com/tombee/conductor/internal/commands/setup/actions"
)

// ProvidersMenuChoice represents a selection in the providers menu
type ProvidersMenuChoice string

const (
	ProviderAddProvider    ProvidersMenuChoice = "add"
	ProviderEditProvider   ProvidersMenuChoice = "edit"
	ProviderRemoveProvider ProvidersMenuChoice = "remove"
	ProviderSetDefault     ProvidersMenuChoice = "set_default"
	ProviderTestAll        ProvidersMenuChoice = "test_all"
	ProviderDone           ProvidersMenuChoice = "done"
)

// ShowProvidersMenu displays the providers management screen.
func ShowProvidersMenu(state *setup.SetupState) (ProvidersMenuChoice, error) {
	var choice string

	// Build provider list summary
	providerList := buildProviderListSummary(state)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Providers\n\n"+providerList),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Add provider", string(ProviderAddProvider)),
					huh.NewOption("Edit provider", string(ProviderEditProvider)),
					huh.NewOption("Remove provider", string(ProviderRemoveProvider)),
					huh.NewOption("Set default provider", string(ProviderSetDefault)),
					huh.NewOption("Test all providers", string(ProviderTestAll)),
					huh.NewOption("Done with providers", string(ProviderDone)),
				).
				Value(&choice),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return ProvidersMenuChoice(choice), nil
}

// buildProviderListSummary builds a formatted list of configured providers
func buildProviderListSummary(state *setup.SetupState) string {
	if len(state.Working.Providers) == 0 {
		return "No providers configured yet."
	}

	var lines []string
	lines = append(lines, "Configured providers:")

	for name, provider := range state.Working.Providers {
		marker := "○"
		suffix := ""
		if name == state.Working.DefaultProvider {
			marker = "●"
			suffix = " ← default"
		}
		lines = append(lines, fmt.Sprintf("  %s %s (%s)%s", marker, name, provider.Type, suffix))
	}

	return strings.Join(lines, "\n")
}

// AddProviderFlow guides the user through adding a new provider.
func AddProviderFlow(ctx context.Context, state *setup.SetupState) error {
	// Step 1: Choose provider category (CLI or API)
	var category string
	categoryForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What type of provider?").
				Options(
					huh.NewOption("CLI Providers - Local tools (Claude Code, Ollama)", "cli"),
					huh.NewOption("API Providers - Cloud APIs requiring credentials", "api"),
				).
				Value(&category),
		),
	)

	if err := categoryForm.Run(); err != nil {
		return err
	}

	if category == "cli" {
		return addCLIProviderFlow(ctx, state)
	}
	return addAPIProviderFlow(ctx, state)
}

// addCLIProviderFlow handles adding CLI-based providers
func addCLIProviderFlow(ctx context.Context, state *setup.SetupState) error {
	// Get CLI provider types
	cliProviders := []setup.ProviderType{}
	for _, pt := range setup.GetProviderTypes() {
		if pt.IsCLI() {
			cliProviders = append(cliProviders, pt)
		}
	}

	// Detect which are available
	type cliStatus struct {
		provider setup.ProviderType
		detected bool
		path     string
	}
	statuses := make([]cliStatus, 0, len(cliProviders))

	for _, pt := range cliProviders {
		detected, path, _ := pt.DetectCLI(ctx)
		statuses = append(statuses, cliStatus{
			provider: pt,
			detected: detected,
			path:     path,
		})
	}

	// Build selection options
	options := make([]huh.Option[string], 0, len(statuses)+1)
	for _, status := range statuses {
		label := status.provider.DisplayName()
		if status.detected {
			label += " ✓ Installed"
		} else {
			label += " ✗ Not found"
		}
		options = append(options, huh.NewOption(label, status.provider.Name()))
	}
	options = append(options, huh.NewOption("Back", "back"))

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select CLI provider to add:").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if selected == "back" {
		return nil
	}

	// Add the selected provider
	providerType, ok := setup.GetProviderType(selected)
	if !ok {
		return fmt.Errorf("unknown provider type: %s", selected)
	}

	// Use provider type name as the instance name for CLI providers
	instanceName := selected

	// Create provider config
	providerCfg := providerType.CreateConfig()

	// Add to state
	state.Working.Providers[instanceName] = providerCfg
	if state.Working.DefaultProvider == "" {
		state.Working.DefaultProvider = instanceName
	}
	state.MarkDirty()

	return nil
}

// addAPIProviderFlow handles adding API-based providers
func addAPIProviderFlow(ctx context.Context, state *setup.SetupState) error {
	// Get API provider types
	apiProviders := []setup.ProviderType{}
	for _, pt := range setup.GetProviderTypes() {
		if !pt.IsCLI() {
			apiProviders = append(apiProviders, pt)
		}
	}

	// Build selection options
	options := make([]huh.Option[string], 0, len(apiProviders)+1)
	for _, pt := range apiProviders {
		options = append(options, huh.NewOption(
			pt.DisplayName()+" - "+pt.Description(),
			pt.Name(),
		))
	}
	options = append(options, huh.NewOption("Back", "back"))

	var selectedType string
	typeForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select API provider type:").
				Options(options...).
				Value(&selectedType),
		),
	)

	if err := typeForm.Run(); err != nil {
		return err
	}

	if selectedType == "back" {
		return nil
	}

	providerType, ok := setup.GetProviderType(selectedType)
	if !ok {
		return fmt.Errorf("unknown provider type: %s", selectedType)
	}

	// Step 2: Get instance name
	var instanceName string
	defaultName := selectedType
	if defaultName == "openai-compatible" {
		defaultName = "" // Force user to provide a descriptive name
	}

	nameForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Provider name (unique identifier):")).
				Description("Examples: anthropic, openai, truefoundry, azure-openai").
				Value(&instanceName).
				Placeholder(defaultName).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("provider name is required")
					}
					if _, exists := state.Working.Providers[s]; exists {
						return fmt.Errorf("provider %q already exists", s)
					}
					return nil
				}),
		),
	)

	if err := nameForm.Run(); err != nil {
		return err
	}

	if instanceName == "" {
		instanceName = defaultName
	}

	// Step 3: Configure provider-specific fields
	providerCfg := providerType.CreateConfig()

	// If provider requires base URL, prompt for it
	if providerType.RequiresBaseURL() {
		var baseURL string
		defaultURL := providerType.DefaultBaseURL()

		urlForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Base URL:").
					Value(&baseURL).
					Placeholder(defaultURL).
					Validate(func(s string) error {
						if s == "" && defaultURL == "" {
							return fmt.Errorf("base URL is required")
						}
						// TODO: Add URL validation
						return nil
					}),
			),
		)

		if err := urlForm.Run(); err != nil {
			return err
		}

		if baseURL == "" {
			baseURL = defaultURL
		}
		// TODO: BaseURL field doesn't exist in config.ProviderConfig yet
		// Will need to add this field or use a different approach
		// providerCfg.BaseURL = baseURL
		_ = baseURL // Suppress unused variable warning for now
	}

	// Step 4: API Key configuration
	if providerType.RequiresAPIKey() {
		// TODO: Implement API key + backend selection flow
		// For now, just prompt for the key
		var apiKey string

		keyForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("API Key:").
					EchoMode(huh.EchoModePassword).
					Value(&apiKey).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("API key is required")
						}
						return nil
					}),
			),
		)

		if err := keyForm.Run(); err != nil {
			return err
		}

		// Store in credential store for later persistence
		credKey := fmt.Sprintf("provider:%s:api_key", instanceName)
		state.CredentialStore[credKey] = apiKey

		// For now, store reference in config (TODO: implement backend selection)
		providerCfg.APIKey = fmt.Sprintf("$secret:%s_API_KEY", strings.ToUpper(instanceName))
	}

	// Add to state
	state.Working.Providers[instanceName] = providerCfg
	if state.Working.DefaultProvider == "" {
		state.Working.DefaultProvider = instanceName
	}
	state.MarkDirty()

	return nil
}

// SelectProviderForEdit shows a list of providers and returns the selected one
func SelectProviderForEdit(state *setup.SetupState) (string, error) {
	if len(state.Working.Providers) == 0 {
		return "", fmt.Errorf("no providers configured")
	}

	options := make([]huh.Option[string], 0, len(state.Working.Providers)+1)
	for name, provider := range state.Working.Providers {
		label := fmt.Sprintf("%s (%s)", name, provider.Type)
		if name == state.Working.DefaultProvider {
			label += " [default]"
		}
		options = append(options, huh.NewOption(label, name))
	}
	options = append(options, huh.NewOption("Back", ""))

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select provider to edit:").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return selected, nil
}

// SelectProviderForRemoval shows a list of providers and returns the selected one
func SelectProviderForRemoval(state *setup.SetupState) (string, error) {
	if len(state.Working.Providers) == 0 {
		return "", fmt.Errorf("no providers configured")
	}

	options := make([]huh.Option[string], 0, len(state.Working.Providers)+1)
	for name, provider := range state.Working.Providers {
		label := fmt.Sprintf("%s (%s)", name, provider.Type)
		if name == state.Working.DefaultProvider {
			label += " [default] ⚠ Warning: removing default"
		}
		options = append(options, huh.NewOption(label, name))
	}
	options = append(options, huh.NewOption("Back", ""))

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select provider to remove:").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return selected, nil
}

// ConfirmRemoveProvider confirms removal of a provider
func ConfirmRemoveProvider(name string, isDefault bool) (bool, error) {
	var confirm bool
	message := fmt.Sprintf("Remove provider %q?", name)
	if isDefault {
		message += "\n⚠ This is the default provider. You'll need to select a new default."
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return false, err
	}

	return confirm, nil
}

// RemoveProvider removes a provider and handles default provider selection
func RemoveProvider(state *setup.SetupState, name string) error {
	isDefault := name == state.Working.DefaultProvider

	// Confirm removal
	confirmed, err := ConfirmRemoveProvider(name, isDefault)
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	// Remove the provider
	delete(state.Working.Providers, name)
	state.MarkDirty()

	// If we removed the default, select a new one
	if isDefault && len(state.Working.Providers) > 0 {
		// Pick the first available provider
		for newDefault := range state.Working.Providers {
			state.Working.DefaultProvider = newDefault
			break
		}
	} else if len(state.Working.Providers) == 0 {
		state.Working.DefaultProvider = ""
	}

	return nil
}

// SelectDefaultProvider allows user to select which provider should be default
func SelectDefaultProvider(state *setup.SetupState) error {
	if len(state.Working.Providers) == 0 {
		return fmt.Errorf("no providers configured")
	}

	if len(state.Working.Providers) == 1 {
		// Only one provider, make it default
		for name := range state.Working.Providers {
			state.Working.DefaultProvider = name
			state.MarkDirty()
		}
		return nil
	}

	options := make([]huh.Option[string], 0, len(state.Working.Providers))
	for name, provider := range state.Working.Providers {
		label := fmt.Sprintf("%s (%s)", name, provider.Type)
		if name == state.Working.DefaultProvider {
			label += " [current default]"
		}
		options = append(options, huh.NewOption(label, name))
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select default provider:").
				Description("The default provider is used when workflows don't specify one").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if selected != state.Working.DefaultProvider {
		state.Working.DefaultProvider = selected
		state.MarkDirty()
	}

	return nil
}

// EditProviderFlow guides the user through editing an existing provider
func EditProviderFlow(ctx context.Context, state *setup.SetupState, providerName string) error {
	provider, ok := state.Working.Providers[providerName]
	if !ok {
		return fmt.Errorf("provider %q not found", providerName)
	}

	providerType, ok := setup.GetProviderType(provider.Type)
	if !ok {
		return fmt.Errorf("unknown provider type: %s", provider.Type)
	}

	// Build menu for editable fields
	var choice string
	options := []huh.Option[string]{
		huh.NewOption("Test connection", "test"),
		huh.NewOption("Done editing", "done"),
	}

	// Add API key option if provider requires it
	if providerType.RequiresAPIKey() {
		options = append([]huh.Option[string]{
			huh.NewOption("Change API key", "api_key"),
		}, options...)
	}

	// Add base URL option if provider requires it
	if providerType.RequiresBaseURL() {
		options = append([]huh.Option[string]{
			huh.NewOption("Change base URL", "base_url"),
		}, options...)
	}

	for {
		// Show current config
		var configLines []string
		configLines = append(configLines, fmt.Sprintf("Provider: %s", providerName))
		configLines = append(configLines, fmt.Sprintf("Type: %s", provider.Type))
		if provider.APIKey != "" {
			backend, key := parseCredentialRef(provider.APIKey)
			if backend != "" {
				configLines = append(configLines, fmt.Sprintf("API Key: %s:%s", backend, key))
			} else {
				configLines = append(configLines, "API Key: (set)")
			}
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("Edit Provider\n\n"+strings.Join(configLines, "\n")),
				huh.NewSelect[string]().
					Title("What would you like to do?").
					Options(options...).
					Value(&choice),
			),
		)

		if err := form.Run(); err != nil {
			return err
		}

		switch choice {
		case "api_key":
			if err := updateProviderAPIKey(ctx, state, providerName); err != nil {
				return err
			}

		case "base_url":
			if err := updateProviderBaseURL(state, providerName); err != nil {
				return err
			}

		case "test":
			if err := testSingleProvider(ctx, state, providerName); err != nil {
				return err
			}

		case "done":
			return nil
		}

		// Reload provider after changes
		provider = state.Working.Providers[providerName]
	}
}

// updateProviderAPIKey updates the API key for a provider
func updateProviderAPIKey(ctx context.Context, state *setup.SetupState, providerName string) error {
	provider := state.Working.Providers[providerName]

	var apiKey string
	keyForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("New API Key:").
				EchoMode(huh.EchoModePassword).
				Value(&apiKey).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("API key is required")
					}
					return nil
				}),
		),
	)

	if err := keyForm.Run(); err != nil {
		return err
	}

	// Store in credential store
	credKey := fmt.Sprintf("provider:%s:api_key", providerName)
	state.CredentialStore[credKey] = apiKey

	// Update config reference
	provider.APIKey = fmt.Sprintf("$secret:%s_API_KEY", strings.ToUpper(providerName))
	state.Working.Providers[providerName] = provider
	state.MarkDirty()

	return nil
}

// updateProviderBaseURL updates the base URL for a provider
func updateProviderBaseURL(state *setup.SetupState, providerName string) error {
	provider := state.Working.Providers[providerName]
	providerType, _ := setup.GetProviderType(provider.Type)

	var baseURL string
	urlForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Base URL:").
				Value(&baseURL).
				Placeholder(providerType.DefaultBaseURL()).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("base URL is required")
					}
					return nil
				}),
		),
	)

	if err := urlForm.Run(); err != nil {
		return err
	}

	// Store base URL in credential store for now
	credKey := fmt.Sprintf("provider:%s:base_url", providerName)
	state.CredentialStore[credKey] = baseURL
	state.MarkDirty()

	return nil
}

// testSingleProvider tests a single provider connection
func testSingleProvider(ctx context.Context, state *setup.SetupState, providerName string) error {
	provider, ok := state.Working.Providers[providerName]
	if !ok {
		return fmt.Errorf("provider %q not found", providerName)
	}

	// Import actions package
	result := actions.TestProvider(ctx, provider.Type, provider)

	// Display result
	message := fmt.Sprintf("Testing %s...\n\n%s", providerName, result.Message)
	if !result.Success && result.ErrorDetails != "" {
		message += "\n\nError: " + result.ErrorDetails
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(message),
			huh.NewConfirm().
				Title("Press Enter to continue").
				Affirmative("Continue").
				Negative(""),
		),
	)

	return form.Run()
}

// TestAllProviders tests all configured providers
func TestAllProviders(ctx context.Context, state *setup.SetupState) error {
	if len(state.Working.Providers) == 0 {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("No providers configured yet."),
				huh.NewConfirm().
					Title("Press Enter to go back").
					Affirmative("Back").
					Negative(""),
			),
		)
		return form.Run()
	}

	// Test each provider
	var results []string
	for name, provider := range state.Working.Providers {
		result := actions.TestProvider(ctx, provider.Type, provider)
		status := result.StatusIcon
		results = append(results, fmt.Sprintf("%s %s (%s)", status, name, provider.Type))
	}

	message := "Test Results\n\n" + strings.Join(results, "\n")

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(message),
			huh.NewConfirm().
				Title("Press Enter to continue").
				Affirmative("Continue").
				Negative(""),
		),
	)

	return form.Run()
}

// parseCredentialRef parses a credential reference like "$secret:KEY"
func parseCredentialRef(ref string) (string, string) {
	if !strings.HasPrefix(ref, "$") {
		return "", ""
	}
	parts := strings.SplitN(ref[1:], ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
