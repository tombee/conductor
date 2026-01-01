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
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// NewCommand creates the setup command
func NewCommand() *cobra.Command {
	var accessible bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive wizard to configure Conductor",
		Long: `Launch the interactive setup wizard to configure:
  - LLM providers (Claude Code, Ollama, Anthropic, OpenAI-compatible)
  - Secrets management (keychain, environment variables)
  - Integrations (GitHub, Slack, Jira, Discord, Jenkins)

The wizard provides a TUI (Terminal User Interface) for guided configuration.

Use --accessible for simple text prompts if the TUI doesn't work in your terminal.
You can also set CONDUCTOR_ACCESSIBLE=1 to enable accessible mode.`,
		Annotations: map[string]string{
			"group": "config",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(cmd, accessible)
		},
	}

	cmd.Flags().BoolVar(&accessible, "accessible", false, "Use accessible mode (simple text prompts instead of TUI)")

	return cmd
}

// runSetup executes the setup wizard
func runSetup(cmd *cobra.Command, accessible bool) error {
	// Determine if we should use accessible mode
	accessibleMode := shouldUseAccessibleMode(accessible)

	// Validate terminal size if using TUI mode
	if !accessibleMode {
		if err := validateTerminalSize(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			fmt.Fprintf(os.Stderr, "Tip: Use --accessible flag for non-interactive mode:\n")
			fmt.Fprintf(os.Stderr, "  conductor setup --accessible\n")
			return err
		}
	}

	// Create audit logger
	audit := NewAuditLogger()

	// Load existing config or create new state
	state, err := LoadOrCreateState()
	if err != nil {
		return fmt.Errorf("failed to load setup state: %w", err)
	}

	// Attach audit logger to state for use throughout wizard
	state.Audit = audit

	audit.LogSetupStarted(state.Original != nil)

	// Initialize signal handler for graceful exit
	signalHandler := NewSignalHandler(state, state.IsDirty)
	signalHandler.Start()
	defer signalHandler.Stop()

	// Run the wizard flow
	if err := RunWizard(cmd.Context(), state, accessibleMode); err != nil {
		return err
	}

	return nil
}

// shouldUseAccessibleMode determines if accessible mode should be used.
// Returns true if:
// - --accessible flag is set
// - CONDUCTOR_ACCESSIBLE=1 environment variable is set
// - stdin is not a terminal (e.g., piped input)
func shouldUseAccessibleMode(flagValue bool) bool {
	// Explicit flag takes precedence
	if flagValue {
		return true
	}

	// Check environment variable
	if os.Getenv("CONDUCTOR_ACCESSIBLE") == "1" {
		return true
	}

	// Check if stdin is a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return true
	}

	return false
}

// validateTerminalSize checks if the terminal is large enough for the TUI.
// Minimum size: 40 columns x 15 rows
func validateTerminalSize() error {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Can't determine size, assume it's okay
		return nil
	}

	const minWidth = 40
	const minHeight = 15

	if width < minWidth || height < minHeight {
		return fmt.Errorf("terminal too small (need at least %dx%d, got %dx%d)", minWidth, minHeight, width, height)
	}

	return nil
}
