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

	"github.com/charmbracelet/huh"
	"github.com/tombee/conductor/internal/commands/setup"
	"golang.org/x/term"
)

// PreFlightCheckResult contains the results of pre-flight checks
type PreFlightCheckResult struct {
	IsInteractive      bool
	TerminalSize       string
	TerminalSizeOK     bool
	ExistingConfig     bool
	KeychainAvailable  bool
	AutoDetectedClaude bool
	ClaudeCodePath     string
}

// RunPreFlightCheck performs pre-flight checks before showing the welcome screen.
// It verifies:
// - Terminal is interactive
// - Terminal size is adequate (minimum 40x15)
// - Detects existing config
// - Checks keychain availability
func RunPreFlightCheck(ctx context.Context) (*PreFlightCheckResult, error) {
	result := &PreFlightCheckResult{}

	// Check if terminal is interactive
	result.IsInteractive = term.IsTerminal(int(os.Stdin.Fd()))

	// Check terminal size
	if result.IsInteractive {
		width, height, err := term.GetSize(int(os.Stdout.Fd()))
		if err == nil {
			result.TerminalSize = fmt.Sprintf("%dx%d", width, height)
			result.TerminalSizeOK = width >= 40 && height >= 15
		} else {
			// If we can't get terminal size, assume it's OK
			result.TerminalSizeOK = true
		}
	}

	// Check for existing config
	// TODO: Get config path from internal/config
	// For now, check default location
	configPath := os.ExpandEnv("$HOME/.config/conductor/config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		result.ExistingConfig = true
	}

	// Check keychain availability
	keychainBackend, ok := setup.GetBackendType("keychain")
	if ok {
		result.KeychainAvailable = keychainBackend.IsAvailable()
	}

	// Auto-detect Claude Code CLI
	claudeType, ok := setup.GetProviderType("claude-code")
	if ok {
		detected, path, err := claudeType.DetectCLI(ctx)
		if err == nil && detected {
			result.AutoDetectedClaude = true
			result.ClaudeCodePath = path
		}
	}

	return result, nil
}

// ShowWelcomeScreen displays the welcome screen for first-time setup.
// It shows pre-flight check results and auto-detected providers.
func ShowWelcomeScreen(ctx context.Context, checks *PreFlightCheckResult) error {
	// Build welcome message
	var message string
	if checks.AutoDetectedClaude {
		message = fmt.Sprintf(`Welcome to Conductor! ⚡

Let's get you set up in 2 minutes.

✓ Claude Code CLI detected at: %s

Press Enter to continue or 's' to skip auto-detection`, checks.ClaudeCodePath)
	} else {
		message = `Welcome to Conductor! ⚡

Let's get you set up in 2 minutes.

Press Enter to continue...`
	}

	var skipAutoDetect bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(message),
			huh.NewConfirm().
				Title("Skip auto-detection?").
				Value(&skipAutoDetect).
				Affirmative("Skip").
				Negative("Continue with auto-detect"),
			NewFooterNote(FooterContextConfirm),
		),
	)

	return form.Run()
}

// ShowReturningUserMenu shows the main menu for users with existing config.
// This is different from first-run flow which goes directly to provider setup.
func ShowReturningUserMenu(state *setup.SetupState) (MenuChoice, error) {
	var choice string

	// Build current configuration summary
	providerCount := len(state.Working.Providers)
	defaultProvider := state.Working.DefaultProvider
	if defaultProvider == "" && providerCount > 0 {
		// Get first provider as default
		for name := range state.Working.Providers {
			defaultProvider = name
			break
		}
	}

	// TODO: Count integrations once integration support is added
	integrationCount := 0

	summary := fmt.Sprintf(`Current configuration:
  • %d provider(s) (default: %s)
  • %d integration(s)
  • Secrets: %s (default)`,
		providerCount,
		defaultProvider,
		integrationCount,
		state.SecretsBackend,
	)

	dirtyIndicator := ""
	if state.IsDirty() {
		dirtyIndicator = " ●"
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("⚡ Conductor Setup\n\n"+summary),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Providers - Configure LLM access", string(MenuProviders)),
					huh.NewOption("Integrations - GitHub, Slack, etc.", string(MenuIntegrations)),
					huh.NewOption("Settings - Secrets backend, etc.", string(MenuSettings)),
					huh.NewOption("Run Setup Wizard", string(MenuRunWizard)),
					huh.NewOption("Save & Exit"+dirtyIndicator, string(MenuSaveExit)),
					huh.NewOption("Exit (discard changes)", string(MenuDiscardExit)),
				).
				Value(&choice),
			NewFooterNote(FooterContextSelection),
		),
	).WithProgramOptions(setup.WithAltScreen())

	if err := form.Run(); err != nil {
		return "", err
	}

	return MenuChoice(choice), nil
}

// MenuChoice represents a menu selection
type MenuChoice string

const (
	MenuProviders    MenuChoice = "providers"
	MenuIntegrations MenuChoice = "integrations"
	MenuSettings     MenuChoice = "settings"
	MenuRunWizard    MenuChoice = "run_wizard"
	MenuSaveExit     MenuChoice = "save"
	MenuDiscardExit  MenuChoice = "discard"
)

// ValidateTerminalSize validates that the terminal is large enough for the TUI.
// Returns an error if the terminal is too small.
func ValidateTerminalSize() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("setup requires an interactive terminal")
	}

	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// If we can't get size, assume it's OK
		return nil
	}

	if width < 40 || height < 15 {
		return fmt.Errorf("terminal too small (minimum 40x15, got %dx%d)", width, height)
	}

	return nil
}
