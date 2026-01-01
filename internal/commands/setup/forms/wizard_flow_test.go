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
	"testing"

	"github.com/tombee/conductor/internal/commands/setup"
	"github.com/tombee/conductor/internal/config"
)

func TestWizardStep_String(t *testing.T) {
	tests := []struct {
		name string
		step WizardStep
		want string
	}{
		{
			name: "welcome step",
			step: StepWelcome,
			want: "Welcome",
		},
		{
			name: "provider selection step",
			step: StepProviderSelection,
			want: "Select Provider",
		},
		{
			name: "provider config step",
			step: StepProviderConfig,
			want: "Configure Provider",
		},
		{
			name: "backend selection step",
			step: StepBackendSelection,
			want: "Choose Storage",
		},
		{
			name: "review step",
			step: StepReview,
			want: "Review",
		},
		{
			name: "complete step",
			step: StepComplete,
			want: "Complete",
		},
		{
			name: "exit step",
			step: StepExit,
			want: "Exit",
		},
		{
			name: "unknown step",
			step: WizardStep(999),
			want: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.step.String()
			if got != tt.want {
				t.Errorf("WizardStep.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewWizardFlow(t *testing.T) {
	ctx := context.Background()
	state := &setup.SetupState{
		Working: &config.Config{},
	}

	flow := NewWizardFlow(ctx, state)

	if flow == nil {
		t.Fatal("NewWizardFlow() returned nil")
	}

	if flow.ctx != ctx {
		t.Error("NewWizardFlow() did not set context")
	}

	if flow.state != state {
		t.Error("NewWizardFlow() did not set state")
	}

	if flow.currentStep != StepWelcome {
		t.Errorf("NewWizardFlow() currentStep = %v, want %v", flow.currentStep, StepWelcome)
	}

	if len(flow.stepHistory) != 0 {
		t.Errorf("NewWizardFlow() stepHistory length = %d, want 0", len(flow.stepHistory))
	}
}

func TestWizardFlow_GetProgress(t *testing.T) {
	tests := []struct {
		name            string
		currentStep     WizardStep
		wantCurrentStep int
		wantTotalSteps  int
		wantStepName    string
	}{
		{
			name:            "welcome step",
			currentStep:     StepWelcome,
			wantCurrentStep: 1,
			wantTotalSteps:  6,
			wantStepName:    "Welcome",
		},
		{
			name:            "provider selection step",
			currentStep:     StepProviderSelection,
			wantCurrentStep: 2,
			wantTotalSteps:  6,
			wantStepName:    "Select Provider",
		},
		{
			name:            "provider config step",
			currentStep:     StepProviderConfig,
			wantCurrentStep: 3,
			wantTotalSteps:  6,
			wantStepName:    "Configure Provider",
		},
		{
			name:            "backend selection step",
			currentStep:     StepBackendSelection,
			wantCurrentStep: 4,
			wantTotalSteps:  6,
			wantStepName:    "Choose Storage",
		},
		{
			name:            "review step",
			currentStep:     StepReview,
			wantCurrentStep: 5,
			wantTotalSteps:  6,
			wantStepName:    "Review",
		},
		{
			name:            "complete step",
			currentStep:     StepComplete,
			wantCurrentStep: 6,
			wantTotalSteps:  6,
			wantStepName:    "Complete",
		},
		{
			name:            "exit step returns zeros",
			currentStep:     StepExit,
			wantCurrentStep: 0,
			wantTotalSteps:  0,
			wantStepName:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := &WizardFlow{
				currentStep: tt.currentStep,
			}

			currentStep, totalSteps, stepName := flow.GetProgress()

			if currentStep != tt.wantCurrentStep {
				t.Errorf("GetProgress() currentStep = %d, want %d", currentStep, tt.wantCurrentStep)
			}

			if totalSteps != tt.wantTotalSteps {
				t.Errorf("GetProgress() totalSteps = %d, want %d", totalSteps, tt.wantTotalSteps)
			}

			if stepName != tt.wantStepName {
				t.Errorf("GetProgress() stepName = %q, want %q", stepName, tt.wantStepName)
			}
		})
	}
}

func TestWizardFlow_pushPopHistory(t *testing.T) {
	flow := &WizardFlow{
		stepHistory: []WizardStep{},
	}

	// Push some steps
	flow.pushHistory(StepWelcome)
	flow.pushHistory(StepProviderSelection)
	flow.pushHistory(StepProviderConfig)

	if len(flow.stepHistory) != 3 {
		t.Errorf("After pushing 3 steps, history length = %d, want 3", len(flow.stepHistory))
	}

	// Pop and verify LIFO order
	step1 := flow.popHistory()
	if step1 != StepProviderConfig {
		t.Errorf("First pop = %v, want %v", step1, StepProviderConfig)
	}

	step2 := flow.popHistory()
	if step2 != StepProviderSelection {
		t.Errorf("Second pop = %v, want %v", step2, StepProviderSelection)
	}

	step3 := flow.popHistory()
	if step3 != StepWelcome {
		t.Errorf("Third pop = %v, want %v", step3, StepWelcome)
	}

	// Pop from empty history should return StepWelcome
	step4 := flow.popHistory()
	if step4 != StepWelcome {
		t.Errorf("Pop from empty history = %v, want %v", step4, StepWelcome)
	}
}

func TestWizardFlow_Back(t *testing.T) {
	tests := []struct {
		name            string
		stepHistory     []WizardStep
		currentStep     WizardStep
		dirty           bool
		wantCurrentStep WizardStep
	}{
		{
			name:            "back from step with history",
			stepHistory:     []WizardStep{StepWelcome, StepProviderSelection},
			currentStep:     StepProviderConfig,
			dirty:           false,
			wantCurrentStep: StepProviderSelection,
		},
		{
			name:            "back from first step with no changes",
			stepHistory:     []WizardStep{},
			currentStep:     StepWelcome,
			dirty:           false,
			wantCurrentStep: StepExit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &setup.SetupState{
				Working: &config.Config{},
				Dirty:   tt.dirty,
			}

			flow := &WizardFlow{
				ctx:          context.Background(),
				state:        state,
				stepHistory:  tt.stepHistory,
				currentStep:  tt.currentStep,
			}

			// For tests that require user interaction (dirty state), skip
			if tt.dirty {
				t.Skip("Skipping test that requires user interaction")
			}

			err := flow.Back()
			if err != nil {
				t.Errorf("Back() error = %v, want nil", err)
			}

			if flow.currentStep != tt.wantCurrentStep {
				t.Errorf("After Back(), currentStep = %v, want %v", flow.currentStep, tt.wantCurrentStep)
			}
		})
	}
}
