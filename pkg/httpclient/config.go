package httpclient

import (
	"fmt"
	"time"
)

// Config configures the HTTP client with timeout, retry, and observability settings.
type Config struct {
	// Timeout is the total request timeout (includes retries).
	// Default: 30s. Must be > 0.
	Timeout time.Duration

	// RetryAttempts is the maximum number of retry attempts (0 = no retries).
	// Default: 3. Must be >= 0.
	RetryAttempts int

	// RetryBackoff is the initial backoff delay before first retry.
	// Default: 100ms. Must be > 0 if RetryAttempts > 0.
	RetryBackoff time.Duration

	// MaxBackoff is the maximum backoff delay cap.
	// Default: 30s. Must be >= RetryBackoff.
	MaxBackoff time.Duration

	// UserAgent is the User-Agent header value.
	// Required. Must be non-empty.
	UserAgent string

	// AllowNonIdempotentRetry enables retry for non-idempotent methods (POST, PUT, PATCH, DELETE).
	// Default: false (only retry GET, HEAD, OPTIONS for safety).
	// Set to true only if you handle idempotency with Idempotency-Key headers.
	AllowNonIdempotentRetry bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Timeout:                 30 * time.Second,
		RetryAttempts:           3,
		RetryBackoff:            100 * time.Millisecond,
		MaxBackoff:              30 * time.Second,
		UserAgent:               "conductor-http-client/1.0",
		AllowNonIdempotentRetry: false,
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Timeout must be positive
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be > 0, got %v", c.Timeout)
	}

	// RetryAttempts must be non-negative
	if c.RetryAttempts < 0 {
		return fmt.Errorf("retry_attempts must be >= 0, got %d", c.RetryAttempts)
	}

	// If retries enabled, validate retry config
	if c.RetryAttempts > 0 {
		if c.RetryBackoff <= 0 {
			return fmt.Errorf("retry_backoff must be > 0 when retry_attempts > 0, got %v", c.RetryBackoff)
		}

		if c.MaxBackoff < c.RetryBackoff {
			return fmt.Errorf("max_backoff (%v) must be >= retry_backoff (%v)", c.MaxBackoff, c.RetryBackoff)
		}
	}

	// UserAgent must be non-empty
	if c.UserAgent == "" {
		return fmt.Errorf("user_agent is required and must be non-empty")
	}

	return nil
}
