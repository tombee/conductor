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

package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the visual theme for the setup wizard.
var Theme = huh.ThemeCharm()

// Color definitions for consistent styling
var (
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSuccess   = lipgloss.Color("#10B981") // Green
	ColorWarning   = lipgloss.Color("#F59E0B") // Amber
	ColorError     = lipgloss.Color("#EF4444") // Red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorHighlight = lipgloss.Color("#3B82F6") // Blue
)

// Styles for various UI elements
var (
	// HeaderStyle is for section headers
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	// SubheaderStyle is for subsection headers
	SubheaderStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight).
			MarginTop(1)

	// HelpStyle is for help text and hints
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)

	// SuccessStyle is for success messages
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	// WarningStyle is for warning messages
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	// ErrorStyle is for error messages
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	// StatusLineStyle is for status indicators
	StatusLineStyle = lipgloss.NewStyle().
			MarginLeft(2)

	// BoxStyle is for bordered content boxes
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)
)

// Status indicators

// StatusOK returns a green checkmark indicator.
func StatusOK() string {
	return SuccessStyle.Render("✓")
}

// StatusError returns a red X indicator.
func StatusError() string {
	return ErrorStyle.Render("✗")
}

// StatusWarning returns a yellow warning indicator.
func StatusWarning() string {
	return WarningStyle.Render("⚠")
}

// StatusPending returns a gray dot indicator.
func StatusPending() string {
	return HelpStyle.Render("○")
}

// StatusInfo returns a blue info indicator.
func StatusInfo() string {
	return lipgloss.NewStyle().Foreground(ColorHighlight).Render("ℹ")
}

// StatusBullet returns a purple bullet point for dirty state.
func StatusBullet() string {
	return lipgloss.NewStyle().Foreground(ColorPrimary).Render("•")
}

// MaskCredential masks a credential for display.
// Shows first 5 and last 3 characters, masks the middle.
// Examples:
//   - "sk-abc123xyz789" -> "sk-ab*****789"
//   - "ghp_abc123xyz789def" -> "ghp_a*****def"
func MaskCredential(credential string) string {
	if credential == "" {
		return "(not set)"
	}

	// For very short credentials, show only first few chars
	if len(credential) < 8 {
		return credential[:min(3, len(credential))] + "***"
	}

	// Show prefix (5 chars), mask middle, show suffix (3 chars)
	prefix := credential[:min(5, len(credential))]
	suffix := credential[max(0, len(credential)-3):]

	// Count masked characters
	masked := len(credential) - len(prefix) - len(suffix)
	if masked < 0 {
		masked = 0
	}

	return prefix + strings.Repeat("*", min(masked, 5)) + suffix
}

// FormatProviderStatus formats a provider status line.
// Example: "✓ claude (claude-code) - default"
func FormatProviderStatus(name, providerType string, isDefault bool) string {
	status := StatusOK() + " " + name + " (" + providerType + ")"
	if isDefault {
		status += " " + SuccessStyle.Render("- default")
	}
	return status
}

// FormatIntegrationStatus formats an integration status line.
// Example: "✓ github-main (github)"
func FormatIntegrationStatus(name, integrationType string) string {
	return StatusOK() + " " + name + " (" + integrationType + ")"
}

// FormatHeader formats a section header.
func FormatHeader(text string) string {
	return HeaderStyle.Render(text)
}

// FormatSubheader formats a subsection header.
func FormatSubheader(text string) string {
	return SubheaderStyle.Render(text)
}

// FormatHelp formats help text.
func FormatHelp(text string) string {
	return HelpStyle.Render(text)
}

// FormatSuccess formats a success message.
func FormatSuccess(text string) string {
	return SuccessStyle.Render(text)
}

// FormatWarning formats a warning message.
func FormatWarning(text string) string {
	return WarningStyle.Render(text)
}

// FormatError formats an error message.
func FormatError(text string) string {
	return ErrorStyle.Render(text)
}

// FormatBox wraps content in a bordered box.
func FormatBox(content string) string {
	return BoxStyle.Render(content)
}

// FormatList formats a list of items with bullets.
func FormatList(items []string) string {
	var lines []string
	for _, item := range items {
		lines = append(lines, "  • "+item)
	}
	return strings.Join(lines, "\n")
}

// FormatKeyValue formats a key-value pair.
// Example: "API Key: sk-a*****789"
func FormatKeyValue(key, value string) string {
	return fmt.Sprintf("%-20s %s", key+":", value)
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
