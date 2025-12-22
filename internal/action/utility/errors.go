// Package utility provides a builtin action for utility functions (random, ID, math).
package utility

import "fmt"

// ErrorType represents the type of utility action error.
type ErrorType string

const (
	// ErrorTypeValidation indicates invalid input parameters.
	ErrorTypeValidation ErrorType = "validation"

	// ErrorTypeRange indicates value out of allowed range.
	ErrorTypeRange ErrorType = "range"

	// ErrorTypeEmpty indicates empty input when value required.
	ErrorTypeEmpty ErrorType = "empty"

	// ErrorTypeType indicates wrong input type.
	ErrorTypeType ErrorType = "type"

	// ErrorTypeInternal indicates an internal error.
	ErrorTypeInternal ErrorType = "internal"
)

// OperationError represents an error from a utility action operation.
type OperationError struct {
	Operation  string
	Message    string
	ErrorType  ErrorType
	Cause      error
	Suggestion string
}

// Error implements the error interface.
func (e *OperationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Operation, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Operation, e.Message)
}

// Unwrap returns the underlying cause.
func (e *OperationError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if the error may succeed on retry.
func (e *OperationError) IsRetryable() bool {
	// Utility errors are deterministic and not retryable
	return false
}
