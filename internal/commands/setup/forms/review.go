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
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/tombee/conductor/internal/commands/setup"
)

// ReviewAction represents the user's choice on the review screen.
type ReviewAction string

const (
	// ReviewActionSave indicates save and exit
	ReviewActionSave ReviewAction = "save"
	// ReviewActionAddProvider indicates add a new provider
	ReviewActionAddProvider ReviewAction = "add_provider"
	// ReviewActionEditProvider indicates edit a specific provider
	ReviewActionEditProvider ReviewAction = "edit_provider"
	// ReviewActionRemoveProvider indicates remove a provider
	ReviewActionRemoveProvider ReviewAction = "remove_provider"
	// ReviewActionCancel indicates cancel without saving
	ReviewActionCancel ReviewAction = "cancel"
)

// ReviewResult contains the action and any associated data.
type ReviewResult struct {
	Action       ReviewAction
	ProviderName string // Used when Action is ReviewActionEditProvider or ReviewActionRemoveProvider
}

// ShowReviewScreen displays an editable list of configured providers with storage icons.
// Returns the user's chosen action and optionally a provider name to edit/remove.
func ShowReviewScreen(state *setup.SetupState) (*ReviewResult, error) {
	// Build options list
	options := buildReviewOptions(state)

	// Show warning if no providers configured
	var description string
	if len(state.Working.Providers) == 0 {
		description = setup.FormatWarning("⚠ No providers configured yet. Add at least one provider.")
	} else {
		description = fmt.Sprintf("Configured providers (%d)", len(state.Working.Providers))
	}

	var choice string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Review Configuration").
				Description(description),
			huh.NewSelect[string]().
				Title("Select an action:").
				Options(options...).
				Value(&choice),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Parse the choice
	return parseReviewChoice(choice), nil
}

// buildReviewOptions builds the options list for the review screen.
// Format for provider items: "edit:<provider-name>" or "remove:<provider-name>"
// Format for actions: "save", "add_provider", "cancel"
func buildReviewOptions(state *setup.SetupState) []huh.Option[string] {
	options := []huh.Option[string]{}

	// Add provider items with storage icons
	for name, provider := range state.Working.Providers {
		// Determine storage icon
		storageIcon := getStorageIcon(provider.APIKey, state.SecretsBackend)

		// Build label
		label := fmt.Sprintf("%s %s (%s)", storageIcon, name, provider.Type)
		if name == state.Working.DefaultProvider {
			label += " " + setup.FormatSuccess("[default]")
		}

		// Add edit option for this provider
		options = append(options, huh.NewOption("Edit: "+label, "edit:"+name))
	}

	// Add separator if there are providers
	if len(state.Working.Providers) > 0 {
		options = append(options, huh.NewOption("---", "separator"))
	}

	// Add action options
	options = append(options, huh.NewOption("Add another provider", string(ReviewActionAddProvider)))
	options = append(options, huh.NewOption("Save & Exit", string(ReviewActionSave)))
	options = append(options, huh.NewOption("Cancel (don't save)", string(ReviewActionCancel)))

	return options
}

// getStorageIcon returns an icon representing where the secret is stored.
func getStorageIcon(apiKeyRef, defaultBackend string) string {
	if apiKeyRef == "" {
		return "○" // No secret
	}

	// Check if it's a secret reference
	if strings.HasPrefix(apiKeyRef, "$keychain:") {
		return "🔐" // Keychain
	}
	if strings.HasPrefix(apiKeyRef, "$env:") {
		return "📄" // Environment file
	}
	if strings.HasPrefix(apiKeyRef, "$file:") {
		return "📄" // File
	}

	// Default backend
	switch defaultBackend {
	case "keychain":
		return "🔐"
	case "env":
		return "📄"
	default:
		return "🔑"
	}
}

// parseReviewChoice parses the user's choice and returns a ReviewResult.
func parseReviewChoice(choice string) *ReviewResult {
	// Check for edit/remove actions
	if strings.HasPrefix(choice, "edit:") {
		providerName := strings.TrimPrefix(choice, "edit:")
		return &ReviewResult{
			Action:       ReviewActionEditProvider,
			ProviderName: providerName,
		}
	}
	if strings.HasPrefix(choice, "remove:") {
		providerName := strings.TrimPrefix(choice, "remove:")
		return &ReviewResult{
			Action:       ReviewActionRemoveProvider,
			ProviderName: providerName,
		}
	}

	// Direct action
	return &ReviewResult{
		Action: ReviewAction(choice),
	}
}

// buildReviewSummary creates a formatted summary of the configuration.
func buildReviewSummary(state *setup.SetupState) string {
	var b strings.Builder

	// Providers section
	b.WriteString("\n")
	b.WriteString("Providers:\n")
	if len(state.Working.Providers) == 0 {
		b.WriteString("  (none configured)\n")
	} else {
		for name, provider := range state.Working.Providers {
			isDefault := ""
			if name == state.Working.DefaultProvider {
				isDefault = " (default)"
			}
			b.WriteString(fmt.Sprintf("  • %s: %s%s\n", name, provider.Type, isDefault))

			// Show masked API key if present
			if provider.APIKey != "" {
				if strings.HasPrefix(provider.APIKey, "$secret:") || strings.HasPrefix(provider.APIKey, "$env:") {
					b.WriteString(fmt.Sprintf("    API Key: %s\n", provider.APIKey))
				} else {
					b.WriteString(fmt.Sprintf("    API Key: %s\n", maskCredential(provider.APIKey)))
				}
			}

			// Show ConfigPath if present
			if provider.ConfigPath != "" {
				b.WriteString(fmt.Sprintf("    Config Path: %s\n", provider.ConfigPath))
			}

			// Show models if configured (ModelTierMap structure)
			if provider.Models.Fast != "" || provider.Models.Balanced != "" || provider.Models.Strategic != "" {
				b.WriteString("    Models:\n")
				if provider.Models.Fast != "" {
					b.WriteString(fmt.Sprintf("      fast: %s\n", provider.Models.Fast))
				}
				if provider.Models.Balanced != "" {
					b.WriteString(fmt.Sprintf("      balanced: %s\n", provider.Models.Balanced))
				}
				if provider.Models.Strategic != "" {
					b.WriteString(fmt.Sprintf("      strategic: %s\n", provider.Models.Strategic))
				}
			}
		}
	}

	// Integrations section
	b.WriteString("\n")
	b.WriteString("Integrations:\n")
	// Note: Integration types will be added when integration config is implemented
	b.WriteString("  (integration review not yet implemented)\n")

	// Settings section
	b.WriteString("\n")
	b.WriteString("Settings:\n")
	b.WriteString("  (settings review not yet implemented)\n")

	// Config file location
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Config will be saved to: %s\n", state.ConfigPath))

	return b.String()
}

// maskCredential masks a credential value, showing first 3 and last 4 characters.
// Example: "sk-1234567890abcdef" -> "sk-1•••••cdef"
func maskCredential(value string) string {
	if len(value) <= 10 {
		// Too short to mask meaningfully, mask everything except first char
		if len(value) <= 1 {
			return "*"
		}
		return string(value[0]) + strings.Repeat("•", len(value)-1)
	}

	// Show first 3 and last 4 characters
	prefix := value[:3]
	suffix := value[len(value)-4:]
	masked := prefix + strings.Repeat("•", 5) + suffix

	return masked
}

// ConfirmSave shows a final confirmation before saving the configuration.
func ConfirmSave(state *setup.SetupState) (bool, error) {
	var confirm bool

	message := fmt.Sprintf("Save configuration to %s?", state.ConfigPath)
	if state.Original != nil {
		message += "\n\nA backup of your existing config will be created."
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Affirmative("Save").
				Negative("Cancel").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return false, err
	}

	return confirm, nil
}
