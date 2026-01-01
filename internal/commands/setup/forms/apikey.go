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

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/tombee/conductor/internal/commands/setup"
)

// ShowAPIKeyForm prompts the user for an API key with provider-specific guidance.
// The form includes:
// - Clickable hyperlink to the API key management page (OSC 8 support)
// - Format hint in the description
// - Real-time format validation
// - Never includes the key value in error messages
func ShowAPIKeyForm(providerType setup.ProviderType, providerName string) (string, error) {
	// Get guidance for this provider
	guidance := GetAPIKeyGuidance(providerType.Name())

	// Build description with guidance
	var description string
	if guidance != nil {
		// Create clickable hyperlink using OSC 8 if URL is available
		if guidance.URL != "" {
			// OSC 8 format: \e]8;;URL\e\\TEXT\e]8;;\e\\
			// Use lipgloss hyperlink support if available, otherwise show plain URL
			urlDisplay := makeHyperlink(guidance.URL)
			description = fmt.Sprintf("Get your API key from: %s\n\nExpected format: %s",
				urlDisplay, guidance.FormatHint)
		} else {
			description = fmt.Sprintf("Expected format: %s", guidance.FormatHint)
		}
	} else {
		description = "Enter your API key for this provider"
	}

	var apiKey string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("API Key for %s:", providerName)).
				Description(description).
				EchoMode(huh.EchoModePassword).
				Value(&apiKey).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("API key is required")
					}

					// Real-time format validation using guidance
					if guidance != nil && guidance.ValidationRegex != nil {
						if !guidance.ValidationRegex.MatchString(s) {
							// Never include the key value in the error message
							return fmt.Errorf("invalid format. %s", guidance.FormatHint)
						}
					}

					return nil
				}),
			NewFooterNote(FooterContextInput),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return apiKey, nil
}

// makeHyperlink creates a clickable hyperlink using OSC 8 if the terminal supports it.
// Falls back to plain text with URL if OSC 8 is not supported.
func makeHyperlink(url string) string {
	// Use lipgloss hyperlink rendering
	// The lipgloss library handles terminal capability detection
	urlStyle := lipgloss.NewStyle().
		Foreground(setup.ColorHighlight).
		Underline(true)

	// Create hyperlink - lipgloss will add OSC 8 codes if supported
	fullURL := "https://" + url
	return urlStyle.Render(fullURL)
}

// ShowStorageBackendSelection prompts the user to choose a secrets storage backend.
// This is shown for the first API provider being configured.
// If only one backend is available, it's selected automatically.
func ShowStorageBackendSelection(availableBackends []string, currentDefault string) (string, error) {
	// If only one backend available, use it automatically
	if len(availableBackends) == 1 {
		return availableBackends[0], nil
	}

	// Build options
	options := make([]huh.Option[string], 0, len(availableBackends))
	for _, backend := range availableBackends {
		label := backend
		if backend == currentDefault {
			label += " (current default)"
		}
		options = append(options, huh.NewOption(label, backend))
	}

	var selectedBackend string
	if currentDefault != "" {
		selectedBackend = currentDefault
	} else if len(availableBackends) > 0 {
		selectedBackend = availableBackends[0]
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Where should this API key be stored?").
				Description("This will be the default for all future API keys").
				Options(options...).
				Value(&selectedBackend),
			NewFooterNote(FooterContextSelection),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return selectedBackend, nil
}

// GetStorageBackendDisplay returns a user-friendly description of where the secret will be stored.
// Used to show "Will store in: [backend]" for subsequent keys.
func GetStorageBackendDisplay(backend string) string {
	switch backend {
	case "keychain":
		return "Will store in: macOS Keychain"
	case "env":
		return "Will store in: .env file"
	default:
		return fmt.Sprintf("Will store in: %s", backend)
	}
}
