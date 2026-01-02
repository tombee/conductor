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
	"github.com/charmbracelet/lipgloss"
)

// CLI style colors using lipgloss
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
	return StatusOK.Render(SymbolOK) + " " + msg
}

// RenderWarn renders a warning message with orange symbol
func RenderWarn(msg string) string {
	return StatusWarn.Render(SymbolWarn) + " " + msg
}

// RenderError renders an error message with red X
func RenderError(msg string) string {
	return StatusError.Render(SymbolError) + " " + msg
}

// RenderStatus renders a status label like [OK] or [FAIL]
func RenderStatus(ok bool, label string) string {
	if ok {
		return StatusOK.Render("[" + label + "]")
	}
	return StatusError.Render("[" + label + "]")
}

// RenderLabel renders a dim label (for key: value pairs)
func RenderLabel(label string) string {
	return Muted.Render(label)
}
