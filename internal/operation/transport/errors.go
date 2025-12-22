package transport

import (
	"fmt"
)

// ErrorType classifies transport errors for routing and retry decisions.
type ErrorType string

const (
	// ErrorTypeConnection indicates network or DNS errors
	ErrorTypeConnection ErrorType = "connection"

	// ErrorTypeTimeout indicates request timeout or deadline exceeded
	ErrorTypeTimeout ErrorType = "timeout"

	// ErrorTypeAuth indicates authentication failure (401, 403, invalid credentials)
	ErrorTypeAuth ErrorType = "auth"

	// ErrorTypeRateLimit indicates rate limiting (429 Too Many Requests)
	ErrorTypeRateLimit ErrorType = "rate_limit"

	// ErrorTypeServer indicates server errors (5xx)
	ErrorTypeServer ErrorType = "server"

	// ErrorTypeClient indicates client errors (4xx, non-retryable)
	ErrorTypeClient ErrorType = "client"

	// ErrorTypeInvalidReq indicates request validation error (invalid method, URL, etc.)
	ErrorTypeInvalidReq ErrorType = "invalid_request"

	// ErrorTypeCancelled indicates context was cancelled
	ErrorTypeCancelled ErrorType = "cancelled"
)

// TransportError represents a structured error from transport execution.
// All transport implementations should return TransportError for failures
// to enable consistent error handling and retry logic.
type TransportError struct {
	// Type classifies the error for routing and retry decisions
	Type ErrorType

	// StatusCode is the HTTP status code if applicable
	// Zero for non-HTTP errors (connection, timeout, etc.)
	StatusCode int

	// Message is a user-facing error message with credentials redacted
	// Should be safe to log and display to users
	Message string

	// RequestID is the request ID from the service (AWS, GitHub, etc.)
	// Used for debugging and support requests
	RequestID string

	// Retryable indicates whether the error is retryable
	Retryable bool

	// Cause is the underlying error
	// May contain sensitive data - use Message for user-facing errors
	Cause error

	// Metadata contains service-specific debugging details
	// Used for structured logging, not user display
	Metadata map[string]interface{}
}

// Error implements the error interface.
func (e *TransportError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("%s error (status %d): %s", e.Type, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error for error chain inspection.
func (e *TransportError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if the error should be retried.
func (e *TransportError) IsRetryable() bool {
	return e.Retryable
}

// IsStatusCode returns true if the error has the given HTTP status code.
func (e *TransportError) IsStatusCode(code int) bool {
	return e.StatusCode == code
}

// IsType returns true if the error is of the given type.
func (e *TransportError) IsType(t ErrorType) bool {
	return e.Type == t
}
