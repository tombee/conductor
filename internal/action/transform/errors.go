// Package transform provides a builtin action for data transformation operations.
package transform

import "fmt"

// ErrorType represents the type of transform action error.
type ErrorType string

const (
	// ErrorTypeParseError indicates invalid JSON/XML syntax.
	ErrorTypeParseError ErrorType = "parse_error"

	// ErrorTypeExpressionError indicates invalid jq expression or timeout.
	ErrorTypeExpressionError ErrorType = "expression_error"

	// ErrorTypeTypeError indicates wrong input type (e.g., split on non-array, circular reference).
	ErrorTypeTypeError ErrorType = "type_error"

	// ErrorTypeEmptyInput indicates null or undefined input when value required.
	ErrorTypeEmptyInput ErrorType = "empty_input"

	// ErrorTypeLimitExceeded indicates array too large, output too large, or expression timeout.
	ErrorTypeLimitExceeded ErrorType = "limit_exceeded"

	// ErrorTypeValidation indicates invalid input parameters.
	ErrorTypeValidation ErrorType = "validation"

	// ErrorTypeInternal indicates an internal error.
	ErrorTypeInternal ErrorType = "internal"
)

// OperationError represents an error from a transform action operation.
type OperationError struct {
	Operation  string
	Message    string
	ErrorType  ErrorType
	Cause      error
	Suggestion string
	Position   int    // Character position for parse errors
	Context    string // Context preview (redacted)
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
	// Transform errors are deterministic and not retryable
	return false
}
