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
	"errors"
	"fmt"
	"os"

	pkgerrors "github.com/tombee/conductor/pkg/errors"
)

// Exit codes for conductor run command
const (
	ExitSuccess                    = 0
	ExitExecutionFailed            = 1
	ExitInvalidWorkflow            = 2
	ExitMissingInput               = 3
	ExitProviderError              = 4
	ExitMissingInputNonInteractive = 70 // Missing inputs in non-interactive mode (EX_SOFTWARE from sysexits.h)
)

// ExitError is an error that carries an exit code
type ExitError struct {
	Code    int
	Message string
	Cause   error
}

func (e *ExitError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ExitError) Unwrap() error {
	return e.Cause
}

// NewExecutionError creates an error for workflow execution failures
func NewExecutionError(msg string, cause error) *ExitError {
	return &ExitError{
		Code:    ExitExecutionFailed,
		Message: msg,
		Cause:   cause,
	}
}

// NewInvalidWorkflowError creates an error for invalid workflow files
func NewInvalidWorkflowError(msg string, cause error) *ExitError {
	return &ExitError{
		Code:    ExitInvalidWorkflow,
		Message: msg,
		Cause:   cause,
	}
}

// NewMissingInputError creates an error for missing required inputs
func NewMissingInputError(msg string, cause error) *ExitError {
	return &ExitError{
		Code:    ExitMissingInput,
		Message: msg,
		Cause:   cause,
	}
}

// NewProviderError creates an error for provider-related failures
func NewProviderError(msg string, cause error) *ExitError {
	return &ExitError{
		Code:    ExitProviderError,
		Message: msg,
		Cause:   cause,
	}
}

// NewMissingInputNonInteractiveError creates an error for missing inputs in non-interactive mode
func NewMissingInputNonInteractiveError(msg string, cause error) *ExitError {
	return &ExitError{
		Code:    ExitMissingInputNonInteractive,
		Message: msg,
		Cause:   cause,
	}
}

// HandleExitError checks if an error is an ExitError and exits with the appropriate code
func HandleExitError(err error) {
	if err == nil {
		return
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		// Don't print "Error:" prefix if message already starts with it
		msg := exitErr.Error()
		if len(msg) > 0 {
			fmt.Fprintln(os.Stderr, "Error:", msg)
		}

		// Check if the error (or any in the chain) implements UserVisibleError
		printUserVisibleSuggestion(err)

		os.Exit(exitErr.Code)
	}

	// Default to execution failed
	fmt.Fprintln(os.Stderr, "Error:", err.Error())

	// Check if the error implements UserVisibleError
	printUserVisibleSuggestion(err)

	os.Exit(ExitExecutionFailed)
}

// printUserVisibleSuggestion checks if an error implements UserVisibleError
// and prints the suggestion if available.
func printUserVisibleSuggestion(err error) {
	// Walk the error chain to find a UserVisibleError
	for err != nil {
		if userErr, ok := err.(pkgerrors.UserVisibleError); ok {
			if userErr.IsUserVisible() {
				suggestion := userErr.Suggestion()
				if suggestion != "" {
					fmt.Fprintf(os.Stderr, "\nSuggestion: %s\n", suggestion)
				}
			}
			return
		}

		// Continue unwrapping
		err = errors.Unwrap(err)
	}
}
