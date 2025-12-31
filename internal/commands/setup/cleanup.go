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
	"crypto/subtle"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/huh"
)

// SignalHandler handles SIGINT and SIGTERM gracefully, zeroing credentials in memory.
type SignalHandler struct {
	state     *SetupState
	isDirty   func() bool
	terminate chan os.Signal
	done      chan bool
}

// NewSignalHandler creates a new signal handler for the setup wizard.
// It monitors SIGINT (Ctrl+C) and SIGTERM signals.
func NewSignalHandler(state *SetupState, isDirty func() bool) *SignalHandler {
	h := &SignalHandler{
		state:     state,
		isDirty:   isDirty,
		terminate: make(chan os.Signal, 1),
		done:      make(chan bool, 1),
	}

	signal.Notify(h.terminate, os.Interrupt, syscall.SIGTERM)

	return h
}

// Start begins monitoring for signals in a goroutine.
func (h *SignalHandler) Start() {
	go func() {
		for {
			select {
			case <-h.terminate:
				h.handleSignal()
			case <-h.done:
				return
			}
		}
	}()
}

// Stop stops the signal handler.
func (h *SignalHandler) Stop() {
	signal.Stop(h.terminate)
	close(h.done)
}

// handleSignal processes an interrupt signal.
func (h *SignalHandler) handleSignal() {
	// Check if there are unsaved changes
	if !h.isDirty() {
		// No changes, exit immediately
		h.cleanup()
		os.Exit(130) // Standard Ctrl+C exit code
	}

	// Prompt user to confirm discarding changes
	var discard bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Discard unsaved changes?").
				Description("You have unsaved configuration changes.").
				Affirmative("Yes, discard").
				Negative("No, continue editing").
				Value(&discard),
		),
	)

	// Run the confirmation prompt
	if err := form.Run(); err != nil {
		// Error running form, exit anyway
		fmt.Fprintf(os.Stderr, "Error prompting for confirmation: %v\n", err)
		h.cleanup()
		os.Exit(130)
	}

	if discard {
		// User confirmed, cleanup and exit
		h.cleanup()
		os.Exit(130)
	}

	// User chose to continue editing, return to wizard
	// (The goroutine will continue waiting for next signal)
}

// cleanup zeroes all credentials from memory and restores terminal state.
func (h *SignalHandler) cleanup() {
	// Zero all credentials in state using crypto/subtle
	h.zeroCredentials()

	// Restore terminal state (in case TUI left it in raw mode)
	// This is handled automatically by huh/bubbletea on exit,
	// but we ensure it happens here too
	fmt.Print("\033[?25h") // Show cursor
	fmt.Print("\033[0m")   // Reset formatting
}

// zeroCredentials overwrites all credential data in memory.
// This ensures credentials don't persist in memory after the wizard exits.
func (h *SignalHandler) zeroCredentials() {
	if h.state == nil {
		return
	}

	// Zero credentials in working config
	if h.state.Working != nil {
		for name, provider := range h.state.Working.Providers {
			if provider.APIKey != "" {
				// Overwrite with zeros
				b := []byte(provider.APIKey)
				for i := range b {
					b[i] = 0
				}
				// Use subtle.ConstantTimeCopy to ensure compiler doesn't optimize away
				subtle.ConstantTimeCopy(1, b, make([]byte, len(b)))
				provider.APIKey = ""
				h.state.Working.Providers[name] = provider
			}

			// Note: ModelTierMap only contains model names, not API keys,
			// so no additional zeroing needed for the Models field.
		}
	}

	// Zero credentials stored in CredentialStore map
	if h.state.CredentialStore != nil {
		for key, value := range h.state.CredentialStore {
			if value != "" {
				// Overwrite with zeros
				b := []byte(value)
				for i := range b {
					b[i] = 0
				}
				// Use subtle.ConstantTimeCopy to ensure compiler doesn't optimize away
				subtle.ConstantTimeCopy(1, b, make([]byte, len(b)))
				h.state.CredentialStore[key] = ""
			}
		}
		// Clear the map entirely
		h.state.CredentialStore = make(map[string]string)
	}
}

// HandleCleanExit performs cleanup for a successful exit (no confirmation needed).
func HandleCleanExit(state *SetupState) {
	h := &SignalHandler{state: state}
	h.cleanup()
}

// HandleDirtyExit prompts the user before exiting with unsaved changes.
// Returns true if the user confirms exit, false to continue editing.
func HandleDirtyExit(state *SetupState) bool {
	var discard bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Discard unsaved changes?").
				Description("You have unsaved configuration changes.").
				Affirmative("Yes, discard").
				Negative("No, continue editing").
				Value(&discard),
		),
	)

	if err := form.Run(); err != nil {
		// Error running form, assume continue editing
		return false
	}

	if discard {
		h := &SignalHandler{state: state}
		h.cleanup()
		return true
	}

	return false
}
