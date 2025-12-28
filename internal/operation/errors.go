package operation

import (
	"fmt"
	"net/http"
	"regexp"
)

// ErrorType classifies operation errors for appropriate handling.
type ErrorType string

const (
	// ErrorTypeAuth indicates authentication or authorization failure (401, 403)
	ErrorTypeAuth ErrorType = "auth_error"

	// ErrorTypeNotFound indicates resource not found (404)
	ErrorTypeNotFound ErrorType = "not_found"

	// ErrorTypeValidation indicates invalid request data (400, 422)
	ErrorTypeValidation ErrorType = "validation_error"

	// ErrorTypeRateLimit indicates rate limit exceeded (429)
	ErrorTypeRateLimit ErrorType = "rate_limited"

	// ErrorTypeServer indicates server-side error (500, 502, 503, 504)
	ErrorTypeServer ErrorType = "server_error"

	// ErrorTypeTimeout indicates operation timeout
	ErrorTypeTimeout ErrorType = "timeout"

	// ErrorTypeConnection indicates network/DNS error
	ErrorTypeConnection ErrorType = "connection_error"

	// ErrorTypeTransform indicates response transform failure
	ErrorTypeTransform ErrorType = "transform_error"

	// ErrorTypeSSRF indicates SSRF protection blocked the request
	ErrorTypeSSRF ErrorType = "ssrf_blocked"

	// ErrorTypePathInjection indicates path traversal attempt blocked
	ErrorTypePathInjection ErrorType = "path_injection"

	// ErrorTypeNotImplemented indicates feature not yet implemented
	ErrorTypeNotImplemented ErrorType = "not_implemented"
)

// Error represents an operation execution error with classification.
type Error struct {
	// Type classifies the error for retry logic
	Type ErrorType

	// Message is the human-readable error description
	Message string

	// StatusCode is the HTTP status code (if applicable)
	StatusCode int

	// RetryAfter indicates when to retry (for rate limit errors)
	RetryAfter int

	// SuggestText provides guidance on how to resolve the error.
	// Renamed from Suggestion to avoid conflict with Suggestion() method.
	SuggestText string

	// RequestID from the external service
	RequestID string

	// CorrelationID links this error to workflow execution
	CorrelationID string

	// Cause is the underlying error
	Cause error
}

// Error implements the error interface.
func (e *Error) Error() string {
	msg := fmt.Sprintf("OperationError: %s", e.Message)

	if e.Type != "" {
		msg = fmt.Sprintf("%s (type: %s)", msg, e.Type)
	}

	if e.StatusCode > 0 {
		msg = fmt.Sprintf("%s [HTTP %d]", msg, e.StatusCode)
	}

	if e.RequestID != "" {
		msg = fmt.Sprintf("%s (request-id: %s)", msg, e.RequestID)
	}

	if e.Cause != nil {
		msg = fmt.Sprintf("%s: %v", msg, e.Cause)
	}

	return msg
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *Error) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if this error type should be retried.
func (e *Error) IsRetryable() bool {
	switch e.Type {
	case ErrorTypeRateLimit, ErrorTypeServer, ErrorTypeTimeout, ErrorTypeConnection:
		return true
	default:
		return false
	}
}

// IsUserVisible implements pkg/errors.UserVisibleError.
// Operation errors are always user-visible.
func (e *Error) IsUserVisible() bool {
	return true
}

// UserMessage implements pkg/errors.UserVisibleError.
// Returns a user-friendly message without technical details.
func (e *Error) UserMessage() string {
	return e.Message
}

// Suggestion implements pkg/errors.UserVisibleError.
// Returns actionable guidance for resolving the error.
func (e *Error) Suggestion() string {
	return e.SuggestText
}

// ClassifyHTTPError classifies an HTTP status code into an error type.
func ClassifyHTTPError(statusCode int, responseBody string) ErrorType {
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return ErrorTypeAuth
	case statusCode == http.StatusNotFound:
		return ErrorTypeNotFound
	case statusCode == http.StatusBadRequest || statusCode == http.StatusUnprocessableEntity:
		return ErrorTypeValidation
	case statusCode == http.StatusTooManyRequests:
		return ErrorTypeRateLimit
	case statusCode >= 500:
		return ErrorTypeServer
	default:
		return ErrorTypeValidation
	}
}

// ErrorFromHTTPStatus creates an Error from an HTTP response.
// Response body is NOT included in the error message to avoid leaking sensitive data.
// The body should be logged separately with the request ID for debugging.
func ErrorFromHTTPStatus(statusCode int, statusText, responseBody, requestID string) *Error {
	errType := ClassifyHTTPError(statusCode, responseBody)

	err := &Error{
		Type:       errType,
		StatusCode: statusCode,
		Message:    fmt.Sprintf("%d %s", statusCode, statusText),
		RequestID:  requestID,
	}

	// Add type-specific suggestions
	switch errType {
	case ErrorTypeAuth:
		err.SuggestText = "Check authentication credentials and permissions"
	case ErrorTypeNotFound:
		err.SuggestText = "Verify the resource exists and the path is correct"
	case ErrorTypeValidation:
		err.SuggestText = "Check request inputs against operation schema. See logs for details"
		// NOTE: Response body is intentionally NOT included in the error message
		// It should be logged separately with request_id for debugging
	case ErrorTypeRateLimit:
		err.SuggestText = "Wait for rate limit window or configure rate_limit in operation"
	case ErrorTypeServer:
		err.SuggestText = "Retry or contact the service provider"
	}

	return err
}

// NewTransformError creates an error for response transform failures.
func NewTransformError(expression string, cause error) *Error {
	return &Error{
		Type:        ErrorTypeTransform,
		Message:     fmt.Sprintf("response transform failed: %s", expression),
		Cause:       cause,
		SuggestText: "Check jq expression syntax and ensure it matches the response structure",
	}
}

// ipAddressPattern matches IPv4 addresses.
var ipAddressPattern = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)

// redactIPAddresses replaces IP addresses in a string with [REDACTED_IP].
func redactIPAddresses(s string) string {
	return ipAddressPattern.ReplaceAllString(s, "[REDACTED_IP]")
}

// NewSSRFError creates an error for SSRF protection blocks.
// The error message shown to users is sanitized to avoid leaking internal IP addresses.
func NewSSRFError(host string) *Error {
	// Sanitize the host in the user-facing message
	sanitizedHost := redactIPAddresses(host)

	return &Error{
		Type:        ErrorTypeSSRF,
		Message:     fmt.Sprintf("request blocked by security policy (host: %s)", sanitizedHost),
		SuggestText: "Add host to allowed_hosts if access is intentional",
	}
}

// NewPathInjectionError creates an error for path traversal attempts.
// The error message does not include the full value to avoid leaking attempted attack vectors.
func NewPathInjectionError(param, value string) *Error {
	return &Error{
		Type:        ErrorTypePathInjection,
		Message:     fmt.Sprintf("path parameter %q contains invalid characters", param),
		SuggestText: "Remove path traversal sequences (../, %2e%2e) from path parameters",
	}
}

// NewConnectionError creates an error for network/DNS failures.
func NewConnectionError(cause error) *Error {
	return &Error{
		Type:        ErrorTypeConnection,
		Message:     "connection failed",
		Cause:       cause,
		SuggestText: "Check network connectivity and DNS resolution",
	}
}

// NewTimeoutError creates an error for operation timeouts.
func NewTimeoutError(timeout int) *Error {
	return &Error{
		Type:        ErrorTypeTimeout,
		Message:     fmt.Sprintf("operation timed out after %d seconds", timeout),
		SuggestText: "Increase timeout or check service responsiveness",
	}
}
