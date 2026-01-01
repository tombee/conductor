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

// SettingsMenuChoice represents a selection in the settings menu
type SettingsMenuChoice string

const (
	SettingsChangeDefaultProvider SettingsMenuChoice = "change_default_provider"
	SettingsChangeBackend         SettingsMenuChoice = "change_backend"
	SettingsBack                  SettingsMenuChoice = "back"
)

// ShowSettingsMenu displays the settings management screen.
// Shows current values inline and provides options to change them.
func ShowSettingsMenu(state *setup.SetupState) (SettingsMenuChoice, error) {
	var choice string

	// Build settings summary
	settingsSummary := buildSettingsSummary(state)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Settings\n\n"+settingsSummary),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Change default provider", string(SettingsChangeDefaultProvider)),
					huh.NewOption("Change secrets storage backend", string(SettingsChangeBackend)),
					huh.NewOption("Back", string(SettingsBack)),
				).
				Value(&choice),
			NewFooterNote(FooterContextSelection),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return SettingsMenuChoice(choice), nil
}

// buildSettingsSummary builds a formatted summary of current settings
func buildSettingsSummary(state *setup.SetupState) string {
	var lines []string

	// Default provider
	defaultProvider := state.Working.DefaultProvider
	if defaultProvider == "" {
		defaultProvider = "(none set)"
	}
	lines = append(lines, fmt.Sprintf("Default Provider: %s", defaultProvider))

	// Secrets backend
	backend := state.SecretsBackend
	if backend == "" {
		backend = "(none set)"
	}
	lines = append(lines, fmt.Sprintf("Secrets Storage: %s", backend))

	return strings.Join(lines, "\n")
}

// ChangeDefaultBackend allows user to select a new default secrets backend
func ChangeDefaultBackend(state *setup.SetupState) error {
	backends := setup.GetBackendTypes()
	if len(backends) == 0 {
		return fmt.Errorf("no backends available")
	}

	// Build options for available backends only
	options := make([]huh.Option[string], 0)
	for _, backend := range backends {
		if backend.IsAvailable() {
			label := fmt.Sprintf("%s - %s", backend.ID(), backend.Name())
			if backend.ID() == state.SecretsBackend {
				label += " [current]"
			}
			options = append(options, huh.NewOption(label, backend.ID()))
		}
	}

	if len(options) == 0 {
		return fmt.Errorf("no available backends found")
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select default secrets backend:").
				Description("This backend will be used for new credentials").
				Options(options...).
				Value(&selected),
			NewFooterNote(FooterContextSelection),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if selected != state.SecretsBackend {
		state.SecretsBackend = selected
		state.MarkDirty()
	}

	return nil
}
