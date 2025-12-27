package httpclient

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// retryTransport wraps an http.RoundTripper to add retry logic with exponential backoff.
type retryTransport struct {
	base                    http.RoundTripper
	maxAttempts             int
	baseBackoff             time.Duration
	maxBackoff              time.Duration
	allowNonIdempotentRetry bool
}

// newRetryTransport creates a new retry transport that wraps the base transport.
func newRetryTransport(base http.RoundTripper, cfg Config) *retryTransport {
	if base == nil {
		base = http.DefaultTransport
	}

	return &retryTransport{
		base:                    base,
		maxAttempts:             cfg.RetryAttempts + 1, // +1 because attempts include initial try
		baseBackoff:             cfg.RetryBackoff,
		maxBackoff:              cfg.MaxBackoff,
		allowNonIdempotentRetry: cfg.AllowNonIdempotentRetry,
	}
}

// RoundTrip implements http.RoundTripper with retry logic.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check if this method is retryable
	isIdempotent := t.isIdempotentMethod(req.Method)
	if !isIdempotent && !t.allowNonIdempotentRetry {
		// Non-idempotent method and retries not explicitly allowed
		// Just execute once without retry
		return t.base.RoundTrip(req)
	}

	var lastErr error
	var lastResp *http.Response

	for attempt := 1; attempt <= t.maxAttempts; attempt++ {
		// Wait before retry (skip on first attempt)
		if attempt > 1 {
			delay := t.calculateBackoff(attempt - 1)

			// Check for Retry-After header from previous response
			if lastResp != nil {
				if retryAfter := t.parseRetryAfter(lastResp); retryAfter > 0 {
					// Use Retry-After if it's less than our calculated delay
					if retryAfter < delay {
						delay = retryAfter
					}
				}
			}

			// Wait with context cancellation support
			select {
			case <-time.After(delay):
				// Continue with retry
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}

		// Execute request
		resp, err := t.base.RoundTrip(req)

		// Success - return response
		if err == nil && !t.shouldRetryStatus(resp.StatusCode) {
			return resp, nil
		}

		// Save for potential retry-after parsing
		lastErr = err
		lastResp = resp

		// Check if error is retryable
		if err != nil && !t.isRetryableError(err) {
			return nil, err
		}

		// Check if status code is retryable
		if err == nil && !t.shouldRetryStatus(resp.StatusCode) {
			return resp, nil
		}

		// Close response body if present (won't be returned)
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		// Check if context was cancelled
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}

// isIdempotentMethod checks if an HTTP method is idempotent.
// Idempotent methods: GET, HEAD, OPTIONS, PUT (if implemented correctly), DELETE (if implemented correctly)
// For safety, we only auto-retry GET, HEAD, OPTIONS by default.
func (t *retryTransport) isIdempotentMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

// shouldRetryStatus determines if an HTTP status code should trigger a retry.
func (t *retryTransport) shouldRetryStatus(statusCode int) bool {
	switch {
	case statusCode >= 500 && statusCode < 600:
		// 5xx server errors are retryable
		return true
	case statusCode == http.StatusRequestTimeout: // 408
		return true
	case statusCode == http.StatusTooManyRequests: // 429
		return true
	default:
		return false
	}
}

// isRetryableError determines if an error should trigger a retry.
func (t *retryTransport) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Context cancellation is not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for net.Error interface (timeout, temporary)
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Retry on timeout or temporary errors
		return netErr.Timeout() || netErr.Temporary()
	}

	// Check for url.Error (wraps underlying errors)
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return t.isRetryableError(urlErr.Err)
	}

	// Check error message for common transient errors
	errMsg := strings.ToLower(err.Error())
	transientKeywords := []string{
		"connection refused",
		"connection reset",
		"no such host",
		"network unreachable",
		"temporary failure in name resolution",
		"eof",
	}

	for _, keyword := range transientKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

// calculateBackoff computes the delay for a given attempt with exponential backoff and jitter.
func (t *retryTransport) calculateBackoff(attempt int) time.Duration {
	// Calculate exponential backoff: baseBackoff * 2^(attempt-1)
	backoff := float64(t.baseBackoff) * math.Pow(2.0, float64(attempt-1))

	// Cap at max backoff
	if backoff > float64(t.maxBackoff) {
		backoff = float64(t.maxBackoff)
	}

	// Add jitter: 0-20% of backoff
	jitterAmount := backoff * 0.2
	jitter := rand.Float64() * jitterAmount

	return time.Duration(backoff + jitter)
}

// parseRetryAfter extracts the Retry-After header value.
// Supports both seconds (integer) and HTTP-date formats.
// Returns 0 if header is missing or invalid.
func (t *retryTransport) parseRetryAfter(resp *http.Response) time.Duration {
	header := resp.Header.Get("Retry-After")
	if header == "" {
		return 0
	}

	// Try parsing as seconds (integer)
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP-date
	if retryTime, err := http.ParseTime(header); err == nil {
		delay := time.Until(retryTime)
		if delay > 0 {
			return delay
		}
	}

	return 0
}
