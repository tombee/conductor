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

	"github.com/charmbracelet/lipgloss"
	"github.com/tombee/conductor/internal/commands/setup"
)

// ProgressBar renders step indicators for the wizard flow.
type ProgressBar struct {
	CurrentStep int
	TotalSteps  int
	StepName    string
}

// Render generates the formatted progress indicator with step number and progress bar.
// Format: "Step N/M: Step Name"
//
//	"[===>    ] XX%"
func (p *ProgressBar) Render() string {
	if p.CurrentStep < 1 || p.TotalSteps < 1 {
		return ""
	}

	// Step counter line
	stepLine := fmt.Sprintf("Step %d/%d", p.CurrentStep, p.TotalSteps)
	if p.StepName != "" {
		stepLine += ": " + p.StepName
	}

	// Progress bar
	barWidth := 10
	filled := int(float64(p.CurrentStep) / float64(p.TotalSteps) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	var bar strings.Builder
	bar.WriteString("[")
	for i := 0; i < barWidth; i++ {
		if i < filled-1 {
			bar.WriteString("=")
		} else if i == filled-1 {
			bar.WriteString(">")
		} else {
			bar.WriteString(" ")
		}
	}
	bar.WriteString("]")

	percentage := int(float64(p.CurrentStep) / float64(p.TotalSteps) * 100)
	barLine := fmt.Sprintf("%s %d%%", bar.String(), percentage)

	// Apply styling
	stepStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(setup.ColorPrimary)

	barStyle := lipgloss.NewStyle().
		Foreground(setup.ColorMuted)

	return stepStyle.Render(stepLine) + "\n" + barStyle.Render(barLine)
}

// RenderCompact generates a compact progress indicator without the progress bar.
// Format: "Step N/M: Step Name"
func (p *ProgressBar) RenderCompact() string {
	if p.CurrentStep < 1 || p.TotalSteps < 1 {
		return ""
	}

	stepLine := fmt.Sprintf("Step %d/%d", p.CurrentStep, p.TotalSteps)
	if p.StepName != "" {
		stepLine += ": " + p.StepName
	}

	stepStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(setup.ColorPrimary)

	return stepStyle.Render(stepLine)
}
