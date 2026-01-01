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

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/tombee/conductor/internal/commands/setup"
)

// FooterContext represents different contexts in the wizard that have different keyboard shortcuts.
type FooterContext string

const (
	// FooterContextSelection is for list/menu selection screens
	FooterContextSelection FooterContext = "selection"
	// FooterContextInput is for text input fields
	FooterContextInput FooterContext = "input"
	// FooterContextConfirm is for confirmation dialogs
	FooterContextConfirm FooterContext = "confirm"
)

// Footer renders context-sensitive keyboard shortcuts.
type Footer struct {
	Context FooterContext
}

// Render generates the formatted footer with keyboard shortcuts.
// Returns empty string if accessible mode is detected.
func (f *Footer) Render() string {
	shortcuts := f.getShortcuts()
	if len(shortcuts) == 0 {
		return ""
	}

	// Join shortcuts with separator
	text := strings.Join(shortcuts, " | ")

	// Apply muted styling
	style := lipgloss.NewStyle().
		Foreground(setup.ColorMuted)

	return style.Render(text)
}

// getShortcuts returns the list of keyboard shortcuts for the current context.
func (f *Footer) getShortcuts() []string {
	switch f.Context {
	case FooterContextSelection:
		return []string{
			"Enter: Select",
			"Up/Down: Navigate",
			"Esc: Back",
			"?: Help",
		}
	case FooterContextInput:
		return []string{
			"Enter: Submit",
			"Esc: Cancel",
			"Tab: Next field",
		}
	case FooterContextConfirm:
		return []string{
			"Enter: Confirm",
			"Esc: Cancel",
		}
	default:
		return []string{}
	}
}

// RenderWithCustomShortcuts renders a footer with custom shortcut definitions.
func RenderWithCustomShortcuts(shortcuts []string) string {
	if len(shortcuts) == 0 {
		return ""
	}

	text := strings.Join(shortcuts, " | ")

	style := lipgloss.NewStyle().
		Foreground(setup.ColorMuted)

	return style.Render(text)
}

// NewFooterNote creates a huh.Note field for use as a footer in forms.
// The footer is automatically hidden in accessible mode.
// Usage: Add to the end of your huh.NewGroup() elements.
func NewFooterNote(context FooterContext) *huh.Note {
	footer := &Footer{Context: context}
	text := footer.Render()

	// If empty (e.g., accessible mode), return an empty note
	if text == "" {
		return huh.NewNote()
	}

	return huh.NewNote().
		Title(text)
}
