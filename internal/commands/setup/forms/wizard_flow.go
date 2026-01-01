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

	"github.com/tombee/conductor/internal/commands/setup"
)

// WizardStep represents a step in the first-run wizard flow.
type WizardStep int

const (
	// StepWelcome shows the welcome screen and pre-flight checks
	StepWelcome WizardStep = iota
	// StepProviderSelection shows the flattened provider selection
	StepProviderSelection
	// StepProviderConfig configures the selected provider (API key, etc.)
	StepProviderConfig
	// StepBackendSelection chooses the secrets storage backend (first provider only)
	StepBackendSelection
	// StepReview shows the review screen with inline editing
	StepReview
	// StepComplete shows the completion message
	StepComplete
	// StepExit signals the wizard should exit
	StepExit
)

// String returns the human-readable name of the step.
func (s WizardStep) String() string {
	switch s {
	case StepWelcome:
		return "Welcome"
	case StepProviderSelection:
		return "Select Provider"
	case StepProviderConfig:
		return "Configure Provider"
	case StepBackendSelection:
		return "Choose Storage"
	case StepReview:
		return "Review"
	case StepComplete:
		return "Complete"
	case StepExit:
		return "Exit"
	default:
		return "Unknown"
	}
}

// WizardFlow manages the first-run wizard state machine.
type WizardFlow struct {
	// ctx is the context for cancellation
	ctx context.Context
	// state holds the setup configuration being built
	state *setup.SetupState
	// stepHistory tracks the navigation path for back functionality
	stepHistory []WizardStep
	// currentStep is the current wizard step
	currentStep WizardStep
	// selectedProvider tracks the provider type selected in StepProviderSelection
	selectedProvider string
	// selectedProviderName tracks the provider name for StepProviderConfig
	selectedProviderName string
}

// NewWizardFlow creates a new wizard flow instance.
func NewWizardFlow(ctx context.Context, state *setup.SetupState) *WizardFlow {
	return &WizardFlow{
		ctx:          ctx,
		state:        state,
		stepHistory:  []WizardStep{},
		currentStep:  StepWelcome,
	}
}

// Run executes the wizard flow from start to completion.
// Returns nil on successful completion, error on failure or user cancellation.
func (w *WizardFlow) Run() error {
	for w.currentStep != StepExit {
		// Execute the current step
		nextStep, err := w.executeStep()
		if err != nil {
			return fmt.Errorf("step %s failed: %w", w.currentStep, err)
		}

		// Handle back navigation
		if nextStep == StepExit && w.currentStep != StepWelcome {
			// User pressed Esc to go back
			if err := w.Back(); err != nil {
				return err
			}
			continue
		}

		// Normal forward navigation
		if nextStep != w.currentStep {
			w.pushHistory(w.currentStep)
			w.currentStep = nextStep
		}
	}

	return nil
}

// executeStep runs the logic for the current step and returns the next step.
func (w *WizardFlow) executeStep() (WizardStep, error) {
	switch w.currentStep {
	case StepWelcome:
		return w.runWelcomeStep()
	case StepProviderSelection:
		return w.runProviderSelectionStep()
	case StepProviderConfig:
		return w.runProviderConfigStep()
	case StepBackendSelection:
		return w.runBackendSelectionStep()
	case StepReview:
		return w.runReviewStep()
	case StepComplete:
		return w.runCompleteStep()
	default:
		return StepExit, fmt.Errorf("unknown step: %d", w.currentStep)
	}
}

// runWelcomeStep shows the welcome screen and pre-flight checks.
func (w *WizardFlow) runWelcomeStep() (WizardStep, error) {
	// Run pre-flight checks
	checks, err := RunPreFlightCheck(w.ctx)
	if err != nil {
		return StepExit, fmt.Errorf("pre-flight check failed: %w", err)
	}

	// Show welcome screen
	if err := ShowWelcomeScreen(w.ctx, checks); err != nil {
		return StepExit, fmt.Errorf("welcome screen failed: %w", err)
	}

	// Move to provider selection
	return StepProviderSelection, nil
}

// runProviderSelectionStep shows the flattened provider selection.
func (w *WizardFlow) runProviderSelectionStep() (WizardStep, error) {
	// Show flattened provider selection
	selectedType, err := ShowFlattenedProviderSelection(w.ctx, w.state)
	if err != nil {
		return StepExit, err
	}

	// User selected back or cancelled
	if selectedType == "" {
		return StepExit, nil
	}

	// Store the selected provider type for the next step
	w.selectedProvider = selectedType

	// Move to provider configuration step
	return StepProviderConfig, nil
}

// runProviderConfigStep configures the selected provider.
func (w *WizardFlow) runProviderConfigStep() (WizardStep, error) {
	// Get the provider type
	providerType, ok := setup.GetProviderType(w.selectedProvider)
	if !ok {
		return StepExit, fmt.Errorf("unknown provider type: %s", w.selectedProvider)
	}

	// Route to appropriate configuration flow
	var err error
	if providerType.IsCLI() {
		err = addCLIProviderFlowDirect(w.ctx, w.state, providerType)
	} else {
		err = addAPIProviderFlowDirect(w.ctx, w.state, providerType)
	}

	if err != nil {
		return StepExit, err
	}

	// For first-run, check if we need backend selection
	// This will be implemented in Phase 4
	// For now, proceed to review
	return StepReview, nil
}

// runBackendSelectionStep selects the secrets storage backend.
func (w *WizardFlow) runBackendSelectionStep() (WizardStep, error) {
	// TODO: Implement backend selection (Phase 4)
	return StepReview, nil
}

// runReviewStep shows the review screen with inline editing.
func (w *WizardFlow) runReviewStep() (WizardStep, error) {
	// TODO: Implement review screen (Phase 6)
	// For now, go to complete
	return StepComplete, nil
}

// runCompleteStep shows the completion message.
func (w *WizardFlow) runCompleteStep() (WizardStep, error) {
	// TODO: Show completion message
	return StepExit, nil
}

// Back navigates to the previous step in the history.
// If at the first step, confirms exit if dirty.
func (w *WizardFlow) Back() error {
	// If no history, we're at the start
	if len(w.stepHistory) == 0 {
		// If state is dirty, confirm discard
		if w.state.IsDirty() {
			discard, err := ConfirmDiscardChanges()
			if err != nil {
				return err
			}
			if discard {
				setup.HandleCleanExit(w.state)
				w.currentStep = StepExit
				return nil
			}
			// User chose not to discard, stay at current step
			return nil
		}
		// No changes, exit immediately
		w.currentStep = StepExit
		return nil
	}

	// Pop the last step from history
	w.currentStep = w.popHistory()
	return nil
}

// pushHistory adds a step to the navigation history.
func (w *WizardFlow) pushHistory(step WizardStep) {
	w.stepHistory = append(w.stepHistory, step)
}

// popHistory removes and returns the last step from the navigation history.
func (w *WizardFlow) popHistory() WizardStep {
	if len(w.stepHistory) == 0 {
		return StepWelcome
	}
	step := w.stepHistory[len(w.stepHistory)-1]
	w.stepHistory = w.stepHistory[:len(w.stepHistory)-1]
	return step
}

// GetProgress returns the current step progress for progress bar rendering.
// Returns (currentStep, totalSteps, stepName).
func (w *WizardFlow) GetProgress() (int, int, string) {
	// Map steps to ordinal positions (excluding Exit)
	stepOrder := map[WizardStep]int{
		StepWelcome:           1,
		StepProviderSelection: 2,
		StepProviderConfig:    3,
		StepBackendSelection:  4,
		StepReview:            5,
		StepComplete:          6,
	}

	// Total steps in the wizard
	totalSteps := 6

	// Get current step position
	currentStepNum, ok := stepOrder[w.currentStep]
	if !ok {
		return 0, 0, ""
	}

	return currentStepNum, totalSteps, w.currentStep.String()
}
