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
type ReviewAction int

const (
	ReviewActionSave ReviewAction = iota
	ReviewActionEditProviders
	ReviewActionEditIntegrations
	ReviewActionEditSettings
	ReviewActionCancel
)

// ShowReviewScreen displays a summary of all configuration changes before save.
// Returns the user's choice: save, edit providers, edit integrations, edit settings, or cancel.
func ShowReviewScreen(state *setup.SetupState) (ReviewAction, error) {
	// Build review summary
	summary := buildReviewSummary(state)

	var choice string
	options := []huh.Option[string]{
		huh.NewOption("Save configuration", "save"),
		huh.NewOption("Edit providers", "providers"),
		huh.NewOption("Edit integrations", "integrations"),
		huh.NewOption("Edit settings", "settings"),
		huh.NewOption("Cancel (don't save)", "cancel"),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Review Configuration").
				Description(summary),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(options...).
				Value(&choice),
		),
	)

	if err := form.Run(); err != nil {
		return ReviewActionCancel, err
	}

	switch choice {
	case "save":
		return ReviewActionSave, nil
	case "providers":
		return ReviewActionEditProviders, nil
	case "integrations":
		return ReviewActionEditIntegrations, nil
	case "settings":
		return ReviewActionEditSettings, nil
	case "cancel":
		return ReviewActionCancel, nil
	default:
		return ReviewActionCancel, nil
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
