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

package shared

import (
	"os"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

var (
	// colorEnabled tracks whether color output is enabled
	colorEnabled     = true
	colorInitialized bool
	colorMutex       sync.Mutex
)

// ColorEnabled returns whether color output is enabled.
// Checks NO_COLOR env var, --no-color flag, and FORCE_COLOR.
func ColorEnabled() bool {
	colorMutex.Lock()
	defer colorMutex.Unlock()

	if !colorInitialized {
		initStyles()
		colorInitialized = true
	}
	return colorEnabled
}

// initStyles initializes styles based on color settings.
// Must be called with colorMutex held.
func initStyles() {
	colorEnabled = true

	// Check NO_COLOR environment variable (https://no-color.org/)
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		colorEnabled = false
	}

	// Check --no-color flag (from flags.go)
	if noColorFlag {
		colorEnabled = false
	}

	// Check FORCE_COLOR to override (useful for CI/CD that supports color)
	if _, exists := os.LookupEnv("FORCE_COLOR"); exists {
		colorEnabled = true
	}

	// Update styles based on color setting
	if !colorEnabled {
		// Use plain styles without color
		StatusOK = lipgloss.NewStyle()
		StatusWarn = lipgloss.NewStyle()
		StatusError = lipgloss.NewStyle()
		StatusInfo = lipgloss.NewStyle()
		Muted = lipgloss.NewStyle()
		Bold = lipgloss.NewStyle()
		Header = lipgloss.NewStyle()
	} else {
		// Use colored styles
		StatusOK = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))     // green
		StatusWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))  // orange
		StatusError = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
		StatusInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))   // blue
		Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))       // gray
		Bold = lipgloss.NewStyle().Bold(true)
		Header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")) // blue bold
	}
}

// CLI style colors using lipgloss - initialized with colors, may be reset by initStyles
var (
	// StatusOK styles success indicators
	StatusOK = lipgloss.NewStyle().Foreground(lipgloss.Color("42")) // green

	// StatusWarn styles warning indicators
	StatusWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // orange

	// StatusError styles error indicators
	StatusError = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red

	// StatusInfo styles informational text
	StatusInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // blue

	// Muted styles secondary/less important text
	Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // gray

	// Bold styles emphasized text
	Bold = lipgloss.NewStyle().Bold(true)

	// Header styles section headers
	Header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")) // blue bold
)

// Symbols for status indicators
const (
	SymbolOK    = "✓"
	SymbolWarn  = "⚠"
	SymbolError = "✗"
	SymbolInfo  = "•"
)

// RenderOK renders a success message with green checkmark
func RenderOK(msg string) string {
	_ = ColorEnabled() // ensure styles are initialized
	return StatusOK.Render(SymbolOK) + " " + msg
}

// RenderWarn renders a warning message with orange symbol
func RenderWarn(msg string) string {
	_ = ColorEnabled()
	return StatusWarn.Render(SymbolWarn) + " " + msg
}

// RenderError renders an error message with red X
func RenderError(msg string) string {
	_ = ColorEnabled()
	return StatusError.Render(SymbolError) + " " + msg
}

// RenderStatus renders a status label like [OK] or [FAIL]
func RenderStatus(ok bool, label string) string {
	_ = ColorEnabled()
	if ok {
		return StatusOK.Render("[" + label + "]")
	}
	return StatusError.Render("[" + label + "]")
}

// RenderLabel renders a dim label (for key: value pairs)
func RenderLabel(label string) string {
	_ = ColorEnabled()
	return Muted.Render(label)
}
