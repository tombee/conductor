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
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/tombee/conductor/internal/commands/setup"
	"github.com/tombee/conductor/internal/commands/setup/actions"
	"github.com/tombee/conductor/internal/config"
)

// TestAction represents an action the user can take after a connection test.
type TestAction string

const (
	// TestActionContinue indicates the user wants to proceed (after success)
	TestActionContinue TestAction = "continue"
	// TestActionRetry indicates the user wants to retry the test
	TestActionRetry TestAction = "retry"
	// TestActionSkip indicates the user wants to skip testing and continue
	TestActionSkip TestAction = "skip"
	// TestActionEdit indicates the user wants to edit the credentials
	TestActionEdit TestAction = "edit"
	// TestActionCancel indicates the user wants to cancel
	TestActionCancel TestAction = "cancel"
)

// ConnectionTestResult holds the result of running ShowConnectionTest.
type ConnectionTestResult struct {
	// Success indicates whether the test succeeded
	Success bool
	// Action indicates what the user chose to do next
	Action TestAction
	// Message contains the test result message
	Message string
}

// ShowConnectionTest automatically runs a connection test and shows the result.
// On success: displays green checkmark and auto-proceeds after 1 second.
// On failure: shows Retry/Skip/Edit options menu.
// Returns the test result and the user's chosen action.
//
// The timeout is configurable via CONDUCTOR_SETUP_TIMEOUT environment variable
// (in seconds), defaulting to 10 seconds.
func ShowConnectionTest(ctx context.Context, providerType setup.ProviderType, cfg config.ProviderConfig) (*ConnectionTestResult, error) {
	// Get timeout from environment or use default
	timeout := getTestTimeout()

	// Create context with timeout
	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run the test (this is synchronous and will show a brief "Testing..." message)
	// In a real TUI, we'd show a spinner here, but huh v0.8.0 doesn't have built-in spinner support
	// The test itself is fast (< 10s) so a simple synchronous approach is acceptable
	testResult := actions.TestProvider(testCtx, providerType.Name(), cfg)

	// Handle test result
	if testResult.Success {
		return handleSuccessResult(testResult)
	}

	return handleFailureResult(testResult)
}

// handleSuccessResult shows success message and auto-proceeds after 1 second.
func handleSuccessResult(result *actions.TestResult) (*ConnectionTestResult, error) {
	// Show success message
	successMsg := setup.StatusOK() + " Connection test successful\n\n" + result.Message

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(successMsg),
		),
	)

	// Create a channel to signal completion
	done := make(chan error, 1)

	// Run the form in a goroutine
	go func() {
		done <- form.Run()
	}()

	// Wait for either 1 second or form completion
	select {
	case <-time.After(1 * time.Second):
		// Auto-proceed after 1 second
		return &ConnectionTestResult{
			Success: true,
			Action:  TestActionContinue,
			Message: result.Message,
		}, nil
	case err := <-done:
		if err != nil {
			return nil, err
		}
		return &ConnectionTestResult{
			Success: true,
			Action:  TestActionContinue,
			Message: result.Message,
		}, nil
	}
}

// handleFailureResult shows error message and presents action options.
func handleFailureResult(result *actions.TestResult) (*ConnectionTestResult, error) {
	// Build error message
	errorMsg := setup.StatusError() + " Connection test failed\n\n"
	errorMsg += result.Message
	if result.ErrorDetails != "" {
		errorMsg += "\n" + setup.FormatHelp("Details: "+result.ErrorDetails)
	}

	var action string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(errorMsg),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Retry test", string(TestActionRetry)),
					huh.NewOption("Skip test and continue anyway", string(TestActionSkip)),
					huh.NewOption("Edit credentials", string(TestActionEdit)),
					huh.NewOption("Cancel", string(TestActionCancel)),
				).
				Value(&action),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	return &ConnectionTestResult{
		Success: false,
		Action:  TestAction(action),
		Message: result.Message,
	}, nil
}

// getTestTimeout returns the test timeout from environment variable or default.
func getTestTimeout() time.Duration {
	timeoutStr := os.Getenv("CONDUCTOR_SETUP_TIMEOUT")
	if timeoutStr == "" {
		return 10 * time.Second
	}

	timeoutSec, err := strconv.Atoi(timeoutStr)
	if err != nil || timeoutSec <= 0 {
		return 10 * time.Second
	}

	return time.Duration(timeoutSec) * time.Second
}

// AddVerifiedIndicatorToCLIProvider adds a [Verified] indicator to CLI provider names
// after successful detection.
func AddVerifiedIndicatorToCLIProvider(providerName string) string {
	return providerName + " " + setup.FormatSuccess("[Verified]")
}
