package sdk

import "fmt"

// TruncateError represents an error during code truncation.
type TruncateError struct {
	Code    string
	Message string
}

func (e *TruncateError) Error() string {
	return fmt.Sprintf("truncate error [%s]: %s", e.Code, e.Message)
}

// Error codes for truncation failures.
const (
	ErrCodeInputTooLarge   = "INPUT_TOO_LARGE"
	ErrCodeInvalidOptions  = "INVALID_OPTIONS"
)

// Common truncation errors.
var (
	// ErrInputTooLarge indicates the input exceeds MaxBytes limit.
	ErrInputTooLarge = &TruncateError{
		Code:    ErrCodeInputTooLarge,
		Message: "input exceeds maximum size limit",
	}

	// ErrInvalidOptions indicates invalid truncation options were provided.
	ErrInvalidOptions = &TruncateError{
		Code:    ErrCodeInvalidOptions,
		Message: "invalid truncation options",
	}
)

// NewInputTooLargeError creates an error for inputs exceeding size limits.
// Does not include actual size to prevent information leakage.
func NewInputTooLargeError() *TruncateError {
	return &TruncateError{
		Code:    ErrCodeInputTooLarge,
		Message: "input exceeds maximum size limit",
	}
}

// NewInvalidOptionsError creates an error for invalid option values.
// Does not include field names or values to prevent information leakage.
func NewInvalidOptionsError(reason string) *TruncateError {
	return &TruncateError{
		Code:    ErrCodeInvalidOptions,
		Message: fmt.Sprintf("invalid options: %s", reason),
	}
}
