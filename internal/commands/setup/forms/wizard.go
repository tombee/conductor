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
	"os"

	"github.com/tombee/conductor/internal/commands/setup"
	"github.com/tombee/conductor/internal/config"
)

func init() {
	// Register this package as the wizard runner during package initialization
	setup.SetWizardRunner(wizardRunner{})
}

// wizardRunner implements setup.WizardRunner
type wizardRunner struct{}

// Run executes the main wizard flow.
func (wizardRunner) Run(ctx context.Context, state *setup.SetupState, accessibleMode bool) error {
	// Panic recovery to ensure terminal state is restored
	defer func() {
		if r := recover(); r != nil {
			// Log panic for debugging
			fmt.Fprintf(os.Stderr, "\nPanic in setup wizard: %v\n", r)
			// Terminal state should be automatically restored by bubbletea alt-screen
			panic(r) // Re-panic after logging
		}
	}()

	// Determine if this is first-run (no existing config)
	isFirstRun := state.Original == nil || len(state.Original.Providers) == 0

	// Check if new wizard flow is enabled via feature flag
	useWizardFlow := os.Getenv("CONDUCTOR_SETUP_V2") == "1"

	// For first-run users with the feature flag enabled, use the new wizard flow
	if isFirstRun && useWizardFlow && !accessibleMode {
		flow := NewWizardFlow(ctx, state)
		return flow.Run()
	}

	// Show welcome screen for first-run, or go directly to main menu for returning users
	if isFirstRun && !accessibleMode {
		// Run pre-flight checks
		checks, err := RunPreFlightCheck(ctx)
		if err != nil {
			return fmt.Errorf("pre-flight check failed: %w", err)
		}

		// Show welcome screen
		if err := ShowWelcomeScreen(ctx, checks); err != nil {
			return fmt.Errorf("welcome screen failed: %w", err)
		}
	}

	// Main navigation loop
	for {
		// Show main menu and get user choice
		choice, err := ShowMainMenu(state, isFirstRun)
		if err != nil {
			return fmt.Errorf("main menu failed: %w", err)
		}

		switch choice {
		case MenuProviders:
			// Navigate to providers management
			if err := handleProvidersMenu(ctx, state); err != nil {
				return err
			}

		case MenuIntegrations:
			// Navigate to integrations management
			if err := handleIntegrationsMenu(ctx, state); err != nil {
				return err
			}

		case MenuSettings:
			// Navigate to settings
			if err := handleSettingsMenu(ctx, state); err != nil {
				return err
			}

		case MenuRunWizard:
			// Run the setup wizard flow
			flow := NewWizardFlow(ctx, state)
			if err := flow.Run(); err != nil {
				return err
			}

		case MenuSaveExit:
			// Show review screen before saving
			if err := handleSaveAndExit(ctx, state); err != nil {
				return err
			}
			// If save succeeded, exit the loop
			return nil

		case MenuDiscardExit:
			// Confirm discard if there are unsaved changes
			if state.IsDirty() {
				discard, err := ConfirmDiscardChanges()
				if err != nil {
					return err
				}
				if discard {
					setup.HandleCleanExit(state)
					return nil
				}
				// User chose not to discard, continue wizard
				continue
			}
			// No unsaved changes, exit cleanly
			setup.HandleCleanExit(state)
			return nil
		}
	}
}

// handleProvidersMenu manages the providers configuration flow.
func handleProvidersMenu(ctx context.Context, state *setup.SetupState) error {
	for {
		choice, err := ShowProvidersMenu(state)
		if err != nil {
			return err
		}

		switch choice {
		case ProviderAddProvider:
			if err := AddProviderFlow(ctx, state); err != nil {
				return err
			}
			state.MarkDirty()

		case ProviderEditProvider:
			providerName, err := SelectProviderForEdit(state)
			if err != nil {
				return err
			}
			if providerName != "" {
				if err := EditProviderFlow(ctx, state, providerName); err != nil {
					return err
				}
			}

		case ProviderRemoveProvider:
			providerName, err := SelectProviderForRemoval(state)
			if err != nil {
				return err
			}
			if providerName != "" {
				if err := RemoveProvider(state, providerName); err != nil {
					return err
				}
			}

		case ProviderSetDefault:
			if err := SelectDefaultProvider(state); err != nil {
				return err
			}

		case ProviderTestAll:
			if err := TestAllProviders(ctx, state); err != nil {
				return err
			}

		case ProviderDone:
			// Return to main menu
			return nil
		}
	}
}

// handleIntegrationsMenu manages the integrations configuration flow.
func handleIntegrationsMenu(ctx context.Context, state *setup.SetupState) error {
	for {
		choice, err := ShowIntegrationsMenu(state)
		if err != nil {
			return err
		}

		switch choice {
		case IntegrationAdd:
			if err := AddIntegrationFlow(ctx, state); err != nil {
				return err
			}
			state.MarkDirty()

		case IntegrationEdit:
			integrationName, err := SelectIntegrationForEdit(state)
			if err != nil {
				if err.Error() == "no integrations configured" {
					fmt.Println("No integrations configured yet")
					continue
				}
				return err
			}
			if integrationName != "" {
				if err := EditIntegrationFlow(ctx, state, integrationName); err != nil {
					return err
				}
			}

		case IntegrationRemove:
			integrationName, err := SelectIntegrationForRemoval(state)
			if err != nil {
				if err.Error() == "no integrations configured" {
					fmt.Println("No integrations configured yet")
					continue
				}
				return err
			}
			if integrationName != "" {
				if err := RemoveIntegration(state, integrationName); err != nil {
					return err
				}
			}

		case IntegrationTestAll:
			if err := TestAllIntegrations(ctx, state); err != nil {
				return err
			}

		case IntegrationDone:
			// Return to main menu
			return nil
		}
	}
}

// handleSettingsMenu manages the settings configuration flow.
func handleSettingsMenu(ctx context.Context, state *setup.SetupState) error {
	for {
		choice, err := ShowSettingsMenu(state)
		if err != nil {
			return err
		}

		switch choice {
		case SettingsChangeDefaultProvider:
			if err := SelectDefaultProvider(state); err != nil {
				return err
			}
			state.MarkDirty()

		case SettingsChangeBackend:
			if err := ChangeDefaultBackend(state); err != nil {
				return err
			}
			state.MarkDirty()

		case SettingsBack:
			// Return to main menu
			return nil
		}
	}
}

// handleSaveAndExit shows the review screen and saves the configuration.
func handleSaveAndExit(ctx context.Context, state *setup.SetupState) error {
	// Show review screen in a loop to allow inline editing
	for {
		result, err := ShowReviewScreen(state)
		if err != nil {
			return err
		}

		switch result.Action {
		case ReviewActionSave:
			// Save the configuration
			if err := config.WriteConfig(state.Working, state.ConfigPath); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			// Mark as clean
			state.Dirty = false

			// Show completion message
			if err := ShowCompletionMessage(state); err != nil {
				return err
			}

			// Clean exit
			setup.HandleCleanExit(state)
			return nil

		case ReviewActionAddProvider:
			// Add a new provider
			if err := AddProviderFlow(ctx, state); err != nil {
				return err
			}
			state.MarkDirty()
			// Return to review screen
			continue

		case ReviewActionEditProvider:
			// Edit the selected provider (T22 - inline edit)
			if err := EditProviderFlow(ctx, state, result.ProviderName); err != nil {
				return err
			}
			// Return to review screen
			continue

		case ReviewActionRemoveProvider:
			// Remove the selected provider
			if err := RemoveProvider(state, result.ProviderName); err != nil {
				return err
			}
			// Return to review screen
			continue

		case ReviewActionCancel:
			// Return to main menu
			return nil
		}
	}
}
