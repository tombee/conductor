package http

import "fmt"

// Error types for workflow error handling

// InvalidURLError represents an invalid URL error.
type InvalidURLError struct {
	URL    string
	Reason string
}

func (e *InvalidURLError) Error() string {
	return fmt.Sprintf("invalid URL %s: %s", e.URL, e.Reason)
}

// SecurityBlockedError represents a security policy violation.
type SecurityBlockedError struct {
	URL    string
	Reason string
}

func (e *SecurityBlockedError) Error() string {
	return fmt.Sprintf("security policy blocked URL %s: %s", e.URL, e.Reason)
}

// TimeoutError represents a request timeout.
type TimeoutError struct {
	URL     string
	Timeout string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("request to %s timed out after %s", e.URL, e.Timeout)
}

// NetworkError represents a network-level error.
type NetworkError struct {
	URL    string
	Reason string
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error for %s: %s", e.URL, e.Reason)
}
