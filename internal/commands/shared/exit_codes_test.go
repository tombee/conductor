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
	"testing"

	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/internal/mcp"
	pkgerrors "github.com/tombee/conductor/pkg/errors"
)

// mockUserVisibleError is a test implementation of UserVisibleError
type mockUserVisibleError struct {
	message    string
	suggestion string
	visible    bool
}

func (e *mockUserVisibleError) Error() string {
	return e.message
}

func (e *mockUserVisibleError) IsUserVisible() bool {
	return e.visible
}

func (e *mockUserVisibleError) UserMessage() string {
	return e.message
}

func (e *mockUserVisibleError) Suggestion() string {
	return e.suggestion
}

func TestPrintUserVisibleSuggestion_ConnectorError(t *testing.T) {
	// Test that connector.Error implements UserVisibleError correctly
	connErr := &connector.Error{
		Type:        connector.ErrorTypeAuth,
		Message:     "authentication failed",
		SuggestText: "Check your API credentials",
	}

	// Verify it implements the interface
	var userErr pkgerrors.UserVisibleError = connErr
	if !userErr.IsUserVisible() {
		t.Error("expected connector.Error to be user visible")
	}

	if userErr.UserMessage() != "authentication failed" {
		t.Errorf("expected user message 'authentication failed', got %q", userErr.UserMessage())
	}

	if userErr.Suggestion() != "Check your API credentials" {
		t.Errorf("expected suggestion 'Check your API credentials', got %q", userErr.Suggestion())
	}
}

func TestPrintUserVisibleSuggestion_MCPError(t *testing.T) {
	// Test that mcp.MCPError implements UserVisibleError correctly
	mcpErr := mcp.NewMCPError(mcp.ErrorCodeNotFound, "server not found").
		WithDetail("server 'test' does not exist").
		WithSuggestions("Check server name with 'conductor mcp list'")

	// Verify it implements the interface
	var userErr pkgerrors.UserVisibleError = mcpErr
	if !userErr.IsUserVisible() {
		t.Error("expected mcp.MCPError to be user visible")
	}

	expectedMsg := "server not found: server 'test' does not exist"
	if userErr.UserMessage() != expectedMsg {
		t.Errorf("expected user message %q, got %q", expectedMsg, userErr.UserMessage())
	}

	expectedSuggestion := "Check server name with 'conductor mcp list'"
	if userErr.Suggestion() != expectedSuggestion {
		t.Errorf("expected suggestion %q, got %q", expectedSuggestion, userErr.Suggestion())
	}
}

func TestPrintUserVisibleSuggestion_WrappedError(t *testing.T) {
	// Test that suggestions work when errors are wrapped
	innerErr := &connector.Error{
		Type:        connector.ErrorTypeTimeout,
		Message:     "request timed out",
		SuggestText: "Increase timeout configuration",
	}

	wrappedErr := fmt.Errorf("operation failed: %w", innerErr)

	// The printUserVisibleSuggestion function should walk the error chain
	// and find the UserVisibleError. We can't directly test the function
	// since it outputs to stderr, but we can verify the error chain works.
	var connErr *connector.Error
	if !errors.As(wrappedErr, &connErr) {
		t.Fatal("expected to unwrap connector.Error from wrapped error")
	}

	if connErr.Suggestion() != "Increase timeout configuration" {
		t.Errorf("expected suggestion from wrapped error, got %q", connErr.Suggestion())
	}
}

func TestPrintUserVisibleSuggestion_NoSuggestion(t *testing.T) {
	// Test error with empty suggestion
	connErr := &connector.Error{
		Type:        connector.ErrorTypeServer,
		Message:     "internal server error",
		SuggestText: "", // Empty suggestion
	}

	var userErr pkgerrors.UserVisibleError = connErr
	if userErr.Suggestion() != "" {
		t.Errorf("expected empty suggestion, got %q", userErr.Suggestion())
	}
}

func TestPrintUserVisibleSuggestion_NonUserVisibleError(t *testing.T) {
	// Test with a regular error that doesn't implement UserVisibleError
	regularErr := errors.New("some internal error")

	// This should not panic when passed to printUserVisibleSuggestion
	// We can't directly test the function output, but we can verify
	// that the error doesn't implement UserVisibleError
	var userErr pkgerrors.UserVisibleError
	if errors.As(regularErr, &userErr) {
		t.Error("regular error should not implement UserVisibleError")
	}
}

func TestExitError_Unwrap(t *testing.T) {
	// Test that ExitError properly wraps cause errors
	innerErr := errors.New("inner error")
	exitErr := NewExecutionError("execution failed", innerErr)

	unwrapped := errors.Unwrap(exitErr)
	if unwrapped != innerErr {
		t.Errorf("expected unwrapped error to be innerErr, got %v", unwrapped)
	}
}

func TestExitError_WithUserVisibleCause(t *testing.T) {
	// Test ExitError wrapping a UserVisibleError
	connErr := &connector.Error{
		Type:        connector.ErrorTypeNotFound,
		Message:     "resource not found",
		SuggestText: "Verify the resource ID",
	}

	exitErr := NewExecutionError("operation failed", connErr)

	// Verify we can unwrap to get the UserVisibleError
	var userErr pkgerrors.UserVisibleError
	if !errors.As(exitErr, &userErr) {
		t.Fatal("expected to unwrap UserVisibleError from ExitError")
	}

	if userErr.Suggestion() != "Verify the resource ID" {
		t.Errorf("expected suggestion from cause error, got %q", userErr.Suggestion())
	}
}

func TestExitCodes_Values(t *testing.T) {
	// Verify exit codes match documented values
	tests := []struct {
		name     string
		code     int
		expected int
	}{
		{"ExitSuccess", ExitSuccess, 0},
		{"ExitExecutionFailed", ExitExecutionFailed, 1},
		{"ExitInvalidWorkflow", ExitInvalidWorkflow, 2},
		{"ExitMissingInput", ExitMissingInput, 3},
		{"ExitProviderError", ExitProviderError, 4},
		{"ExitMissingInputNonInteractive", ExitMissingInputNonInteractive, 70},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.expected {
				t.Errorf("%s = %d, expected %d", tt.name, tt.code, tt.expected)
			}
		})
	}
}

func TestExitError_Constructors(t *testing.T) {
	tests := []struct {
		name         string
		constructor  func() *ExitError
		expectedCode int
		expectedMsg  string
	}{
		{
			name: "NewExecutionError",
			constructor: func() *ExitError {
				return NewExecutionError("execution failed", errors.New("inner"))
			},
			expectedCode: ExitExecutionFailed,
			expectedMsg:  "execution failed",
		},
		{
			name: "NewInvalidWorkflowError",
			constructor: func() *ExitError {
				return NewInvalidWorkflowError("invalid workflow", errors.New("parse error"))
			},
			expectedCode: ExitInvalidWorkflow,
			expectedMsg:  "invalid workflow",
		},
		{
			name: "NewMissingInputError",
			constructor: func() *ExitError {
				return NewMissingInputError("missing input", errors.New("required"))
			},
			expectedCode: ExitMissingInput,
			expectedMsg:  "missing input",
		},
		{
			name: "NewProviderError",
			constructor: func() *ExitError {
				return NewProviderError("provider error", errors.New("connection failed"))
			},
			expectedCode: ExitProviderError,
			expectedMsg:  "provider error",
		},
		{
			name: "NewMissingInputNonInteractiveError",
			constructor: func() *ExitError {
				return NewMissingInputNonInteractiveError("non-interactive", errors.New("no prompt"))
			},
			expectedCode: ExitMissingInputNonInteractive,
			expectedMsg:  "non-interactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor()

			if err.Code != tt.expectedCode {
				t.Errorf("expected code %d, got %d", tt.expectedCode, err.Code)
			}

			if err.Message != tt.expectedMsg {
				t.Errorf("expected message %q, got %q", tt.expectedMsg, err.Message)
			}

			if err.Cause == nil {
				t.Error("expected Cause to be set")
			}
		})
	}
}

func TestExitError_Error(t *testing.T) {
	tests := []struct {
		name     string
		exitErr  *ExitError
		expected string
	}{
		{
			name: "with cause",
			exitErr: &ExitError{
				Code:    1,
				Message: "test error",
				Cause:   errors.New("inner error"),
			},
			expected: "test error: inner error",
		},
		{
			name: "without cause",
			exitErr: &ExitError{
				Code:    1,
				Message: "test error",
				Cause:   nil,
			},
			expected: "test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.exitErr.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, expected %q", got, tt.expected)
			}
		})
	}
}
