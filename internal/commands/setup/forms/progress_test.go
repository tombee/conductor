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
	"strings"
	"testing"
)

func TestProgressBar_Render(t *testing.T) {
	tests := []struct {
		name        string
		currentStep int
		totalSteps  int
		stepName    string
		wantStep    string
		wantBar     bool
	}{
		{
			name:        "first step of four",
			currentStep: 1,
			totalSteps:  4,
			stepName:    "Welcome",
			wantStep:    "Step 1/4: Welcome",
			wantBar:     true,
		},
		{
			name:        "middle step",
			currentStep: 2,
			totalSteps:  4,
			stepName:    "Configure Provider",
			wantStep:    "Step 2/4: Configure Provider",
			wantBar:     true,
		},
		{
			name:        "last step",
			currentStep: 4,
			totalSteps:  4,
			stepName:    "Review",
			wantStep:    "Step 4/4: Review",
			wantBar:     true,
		},
		{
			name:        "without step name",
			currentStep: 2,
			totalSteps:  5,
			stepName:    "",
			wantStep:    "Step 2/5",
			wantBar:     true,
		},
		{
			name:        "invalid step returns empty",
			currentStep: 0,
			totalSteps:  4,
			stepName:    "Test",
			wantStep:    "",
			wantBar:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := &ProgressBar{
				CurrentStep: tt.currentStep,
				TotalSteps:  tt.totalSteps,
				StepName:    tt.stepName,
			}

			got := pb.Render()

			// For invalid cases, expect empty string
			if tt.wantStep == "" {
				if got != "" {
					t.Errorf("Render() = %q, want empty string", got)
				}
				return
			}

			// Check step text is present (ignoring ANSI codes)
			if !strings.Contains(got, tt.wantStep) {
				t.Errorf("Render() missing step text %q in output", tt.wantStep)
			}

			// Check progress bar is present
			if tt.wantBar && !strings.Contains(got, "[") {
				t.Errorf("Render() missing progress bar")
			}

			// Check percentage is present
			if tt.wantBar && !strings.Contains(got, "%") {
				t.Errorf("Render() missing percentage")
			}
		})
	}
}

func TestProgressBar_RenderCompact(t *testing.T) {
	tests := []struct {
		name        string
		currentStep int
		totalSteps  int
		stepName    string
		want        string
	}{
		{
			name:        "with step name",
			currentStep: 2,
			totalSteps:  4,
			stepName:    "Configure Provider",
			want:        "Step 2/4: Configure Provider",
		},
		{
			name:        "without step name",
			currentStep: 3,
			totalSteps:  6,
			stepName:    "",
			want:        "Step 3/6",
		},
		{
			name:        "invalid step returns empty",
			currentStep: 0,
			totalSteps:  4,
			stepName:    "Test",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := &ProgressBar{
				CurrentStep: tt.currentStep,
				TotalSteps:  tt.totalSteps,
				StepName:    tt.stepName,
			}

			got := pb.RenderCompact()

			if tt.want == "" {
				if got != "" {
					t.Errorf("RenderCompact() = %q, want empty string", got)
				}
				return
			}

			// Check step text is present (ignoring ANSI codes)
			if !strings.Contains(got, tt.want) {
				t.Errorf("RenderCompact() missing %q in output", tt.want)
			}

			// Compact should not have progress bar
			if strings.Contains(got, "[") {
				t.Errorf("RenderCompact() should not contain progress bar")
			}
		})
	}
}

func TestProgressBar_ProgressBarFill(t *testing.T) {
	tests := []struct {
		name        string
		currentStep int
		totalSteps  int
		wantArrow   bool
	}{
		{
			name:        "25% progress",
			currentStep: 1,
			totalSteps:  4,
			wantArrow:   true,
		},
		{
			name:        "50% progress",
			currentStep: 2,
			totalSteps:  4,
			wantArrow:   true,
		},
		{
			name:        "100% progress",
			currentStep: 4,
			totalSteps:  4,
			wantArrow:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := &ProgressBar{
				CurrentStep: tt.currentStep,
				TotalSteps:  tt.totalSteps,
				StepName:    "Test",
			}

			got := pb.Render()

			// Check for arrow indicator in progress bar
			if tt.wantArrow && !strings.Contains(got, ">") {
				t.Errorf("Render() missing arrow indicator in progress bar")
			}

			// Check bar is properly formatted
			if !strings.Contains(got, "[") || !strings.Contains(got, "]") {
				t.Errorf("Render() progress bar not properly formatted")
			}
		})
	}
}
