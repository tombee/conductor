package transport

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// RetryConfig configures retry behavior for transport operations.
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts (default: 3)
	MaxAttempts int

	// InitialBackoff is the initial backoff duration (default: 1s)
	InitialBackoff time.Duration

	// MaxBackoff is the maximum backoff duration (default: 30s)
	MaxBackoff time.Duration

	// BackoffFactor is the exponential backoff multiplier (default: 2.0)
	BackoffFactor float64

	// RetryableErrors is the list of HTTP status codes that should be retried
	// Default: [408, 429, 500, 502, 503, 504]
	RetryableErrors []int
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:     3,
		InitialBackoff:  1 * time.Second,
		MaxBackoff:      30 * time.Second,
		BackoffFactor:   2.0,
		RetryableErrors: []int{408, 429, 500, 502, 503, 504},
	}
}

// Validate checks if the retry configuration is valid.
func (c *RetryConfig) Validate() error {
	if c.MaxAttempts < 1 {
		return fmt.Errorf("max_attempts must be at least 1, got %d", c.MaxAttempts)
	}
	if c.InitialBackoff < 0 {
		return fmt.Errorf("initial_backoff must be non-negative, got %v", c.InitialBackoff)
	}
	if c.MaxBackoff < c.InitialBackoff {
		return fmt.Errorf("max_backoff (%v) must be >= initial_backoff (%v)", c.MaxBackoff, c.InitialBackoff)
	}
	if c.BackoffFactor < 1.0 {
		return fmt.Errorf("backoff_factor must be >= 1.0, got %f", c.BackoffFactor)
	}
	return nil
}

// IsRetryable returns true if the given status code should be retried.
func (c *RetryConfig) IsRetryable(statusCode int) bool {
	for _, code := range c.RetryableErrors {
		if code == statusCode {
			return true
		}
	}
	return false
}

// ExecuteFunc is a function that executes a single request attempt.
// Returns the response and an error. If the error is a TransportError,
// retry logic will check if it's retryable.
type ExecuteFunc func(ctx context.Context) (*Response, error)

// Execute runs the given function with retry logic.
// Implements exponential backoff with jitter and Retry-After header handling.
//
// Retry behavior:
// - Retries on retryable status codes (408, 429, 5xx)
// - Retries on connection errors and timeouts
// - Does NOT retry on 4xx errors (except 408, 429)
// - Respects Retry-After header when present
// - Stops immediately on context cancellation
func Execute(ctx context.Context, config *RetryConfig, fn ExecuteFunc) (*Response, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	var resp *Response

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Execute the request
		resp, lastErr = fn(ctx)

		// Success - return immediately
		if lastErr == nil {
			// Add retry count to metadata
			if resp.Metadata == nil {
				resp.Metadata = make(map[string]interface{})
			}
			resp.Metadata[MetadataRetryCount] = attempt - 1
			return resp, nil
		}

		// Check if we should retry
		shouldRetry, retryAfter := shouldRetryError(lastErr, config)

		// Don't retry if:
		// - This was the last attempt
		// - Error is not retryable
		// - Context is cancelled
		if attempt >= config.MaxAttempts || !shouldRetry {
			return nil, lastErr
		}

		// Check context before sleeping
		if ctx.Err() != nil {
			return nil, &TransportError{
				Type:      ErrorTypeCancelled,
				Message:   "request cancelled before retry",
				Retryable: false,
				Cause:     ctx.Err(),
			}
		}

		// Calculate backoff delay
		delay := calculateBackoff(config, attempt, retryAfter)

		// Sleep for the backoff duration (interruptible by context)
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return nil, &TransportError{
				Type:      ErrorTypeCancelled,
				Message:   "request cancelled during retry backoff",
				Retryable: false,
				Cause:     ctx.Err(),
			}
		}
	}

	// All retries exhausted
	return nil, lastErr
}

// shouldRetryError determines if an error should be retried and extracts Retry-After if present.
func shouldRetryError(err error, config *RetryConfig) (shouldRetry bool, retryAfter time.Duration) {
	// Check if it's a TransportError
	transportErr, ok := err.(*TransportError)
	if !ok {
		// Unknown error type - don't retry
		return false, 0
	}

	// Don't retry if explicitly marked as non-retryable
	if !transportErr.Retryable {
		return false, 0
	}

	// For HTTP status code errors, check if the code is retryable
	if transportErr.StatusCode > 0 {
		if !config.IsRetryable(transportErr.StatusCode) {
			return false, 0
		}

		// Extract Retry-After header for 429 or 503 responses
		if transportErr.StatusCode == 429 || transportErr.StatusCode == 503 {
			retryAfter = extractRetryAfter(transportErr)
		}
	}

	return true, retryAfter
}

// calculateBackoff calculates the backoff delay for a retry attempt.
// Implements exponential backoff with jitter and Retry-After handling.
//
// Formula: delay = min(InitialBackoff * (BackoffFactor ^ (attempt - 1)), MaxBackoff) + jitter
// Jitter: random [0ms, 100ms]
func calculateBackoff(config *RetryConfig, attempt int, retryAfter time.Duration) time.Duration {
	// Calculate base delay with exponential backoff
	baseDelay := float64(config.InitialBackoff) * pow(config.BackoffFactor, attempt-1)

	// Cap at MaxBackoff
	if baseDelay > float64(config.MaxBackoff) {
		baseDelay = float64(config.MaxBackoff)
	}

	delay := time.Duration(baseDelay)

	// If Retry-After is specified, use max(calculated_delay, retry_after) capped at MaxBackoff
	if retryAfter > 0 {
		if retryAfter > delay {
			delay = retryAfter
		}
		// Cap Retry-After at MaxBackoff to avoid waiting indefinitely
		if delay > config.MaxBackoff {
			delay = config.MaxBackoff
		}
	}

	// Add jitter (0-100ms)
	jitter := time.Duration(rand.Int63n(101)) * time.Millisecond

	return delay + jitter
}

// extractRetryAfter extracts the Retry-After header from transport error metadata.
// Returns 0 if not present or invalid.
//
// Supports two formats:
// - Numeric: seconds to wait (e.g., "120")
// - HTTP-date: absolute time (e.g., "Wed, 21 Oct 2015 07:28:00 GMT")
func extractRetryAfter(err *TransportError) time.Duration {
	if err.Metadata == nil {
		return 0
	}

	retryAfterRaw, ok := err.Metadata["retry_after"]
	if !ok {
		return 0
	}

	retryAfterStr, ok := retryAfterRaw.(string)
	if !ok {
		return 0
	}

	// Try numeric format first (seconds)
	if seconds, err := strconv.ParseInt(retryAfterStr, 10, 64); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try HTTP-date format
	retryTime, parseErr := http.ParseTime(retryAfterStr)
	if parseErr != nil {
		// Malformed Retry-After - ignore and use calculated backoff
		return 0
	}

	// Calculate delay from now
	delay := time.Until(retryTime)
	if delay < 0 {
		// Retry-After is in the past - retry immediately
		return 0
	}

	return delay
}

// pow calculates base^exp for integer exponents.
// Used for exponential backoff calculation.
func pow(base float64, exp int) float64 {
	if exp == 0 {
		return 1.0
	}
	result := 1.0
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}
