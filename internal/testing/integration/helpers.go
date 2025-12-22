package integration

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// RetryConfig configures retry behavior for transient failures.
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts (default 3).
	MaxAttempts int

	// InitialDelay is the delay before the first retry (default 2s).
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries (default 8s).
	MaxDelay time.Duration

	// Multiplier is the backoff multiplier (default 2.0 for exponential).
	Multiplier float64

	// ShouldRetry determines if an error is retryable (default: checks common transient errors).
	ShouldRetry func(error) bool
}

// DefaultRetryConfig returns sensible defaults for integration test retries.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 2 * time.Second,
		MaxDelay:     8 * time.Second,
		Multiplier:   2.0,
		ShouldRetry:  IsTransientError,
	}
}

// Retry executes fn with exponential backoff on transient failures.
// Returns the last error if all attempts fail.
func Retry(ctx context.Context, fn func() error, cfg RetryConfig) error {
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context before attempt
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("retry aborted: %w", err)
		}

		// Execute function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !cfg.ShouldRetry(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Don't sleep after last attempt
		if attempt < cfg.MaxAttempts {
			select {
			case <-ctx.Done():
				return fmt.Errorf("retry aborted during backoff: %w", ctx.Err())
			case <-time.After(delay):
				// Calculate next delay with exponential backoff
				delay = time.Duration(float64(delay) * cfg.Multiplier)
				if delay > cfg.MaxDelay {
					delay = cfg.MaxDelay
				}
			}
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded: %w", cfg.MaxAttempts, lastErr)
}

// IsTransientError checks if an error is likely transient and retryable.
// Checks for common network errors, timeouts, and HTTP 429/503 status codes.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context timeout/cancellation (don't retry)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Check for HTTP status codes
	// 429 = rate limit (retry with backoff)
	// 503 = service unavailable (retry)
	// 500 = server error (may be transient)
	// 401, 403, 404 = not transient (authentication/not found)
	type statusErr interface {
		StatusCode() int
	}
	var se statusErr
	if errors.As(err, &se) {
		code := se.StatusCode()
		return code == http.StatusTooManyRequests ||
			code == http.StatusServiceUnavailable ||
			code == http.StatusInternalServerError
	}

	// Default to not retrying unknown errors
	return false
}

// IsPermanentError checks if an error is permanent (authentication, not found, etc).
func IsPermanentError(err error) bool {
	if err == nil {
		return false
	}

	type statusErr interface {
		StatusCode() int
	}
	var se statusErr
	if errors.As(err, &se) {
		code := se.StatusCode()
		return code == http.StatusUnauthorized ||
			code == http.StatusForbidden ||
			code == http.StatusNotFound
	}

	return false
}

// WaitForServer waits for an HTTP server to become available.
// Returns an error if the server doesn't respond within the timeout.
func WaitForServer(ctx context.Context, url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("server did not become available: %w", ctx.Err())
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				continue
			}

			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				return nil
			}
		}
	}
}
