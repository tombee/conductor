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
	"github.com/charmbracelet/huh"
	"github.com/tombee/conductor/internal/commands/setup"
)

// ShowMainMenu displays the main menu based on whether this is first-run or returning user.
// Returns the selected menu choice.
func ShowMainMenu(state *setup.SetupState, isFirstRun bool) (MenuChoice, error) {
	if isFirstRun {
		// First-run flow goes straight to provider setup
		// No main menu shown
		return MenuProviders, nil
	}

	// Returning user - show full main menu
	return ShowReturningUserMenu(state)
}

// ConfirmDiscardChanges asks the user to confirm discarding unsaved changes.
// This is shown when user tries to exit with dirty state.
func ConfirmDiscardChanges() (bool, error) {
	var discard bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("You have unsaved changes. Discard them?").
				Description("All changes will be lost if you exit now.").
				Affirmative("Discard changes").
				Negative("Go back").
				Value(&discard),
			NewFooterNote(FooterContextConfirm),
		),
	)

	if err := form.Run(); err != nil {
		return false, err
	}

	return discard, nil
}

// ShowCompletionMessage shows the setup completion message with next steps.
func ShowCompletionMessage(state *setup.SetupState) error {
	providerCount := len(state.Working.Providers)
	defaultProvider := state.Working.DefaultProvider

	// Build provider list
	var providerNames string
	if providerCount == 1 {
		providerNames = "● " + defaultProvider + " (default)"
	} else {
		providerNames = "● " + defaultProvider + " (default)"
		for name := range state.Working.Providers {
			if name != defaultProvider {
				providerNames += "\n    ○ " + name
			}
		}
	}

	message := `✓ Setup Complete!

Configuration saved to:
  ` + state.ConfigPath + `

Providers configured:
    ` + providerNames + `

Next steps:
  conductor run <workflow.yaml>   Run a workflow
  conductor setup                 Modify config

Try running a workflow to test your setup!`

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(message),
			huh.NewConfirm().
				Title("Press Enter to finish").
				Affirmative("Done").
				Negative(""),
		),
	)

	return form.Run()
}

// ShowHelpOverlay displays a help overlay with keyboard shortcuts.
// Triggered by pressing '?' in the TUI.
func ShowHelpOverlay() error {
	message := `Keyboard Shortcuts

Navigation:
  ↑/↓      Navigate up/down
  Enter    Select/Confirm
  Esc      Go back
  q        Quit (prompts if unsaved changes)

Forms:
  Tab         Next field
  Shift+Tab   Previous field

Other:
  ?        Show this help
  Ctrl+C   Cancel/Exit`

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(message),
			huh.NewConfirm().
				Title("Press any key to close").
				Affirmative("Close").
				Negative(""),
		),
	)

	return form.Run()
}
