package llm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"

	pkgerrors "github.com/tombee/conductor/pkg/errors"
)

var (
	// ErrMaxRetriesExceeded indicates all retry attempts have been exhausted.
	ErrMaxRetriesExceeded = errors.New("maximum retry attempts exceeded")
)

// RetryConfig configures retry behavior with exponential backoff.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries).
	MaxRetries int

	// InitialDelay is the delay before the first retry.
	InitialDelay time.Duration

	// MaxDelay caps the backoff delay.
	MaxDelay time.Duration

	// Multiplier is the backoff multiplier (typically 2.0 for exponential).
	Multiplier float64

	// Jitter adds randomness to prevent thundering herd (0.0-1.0).
	Jitter float64

	// AbsoluteTimeout is the maximum total time for all retry attempts.
	// If 0, no absolute timeout is enforced. Default: 2 minutes.
	AbsoluteTimeout time.Duration

	// RetryableErrors is a function that determines if an error should trigger a retry.
	// If nil, uses default logic (transient network and HTTP 5xx errors).
	RetryableErrors func(error) bool
}

// DefaultRetryConfig returns sensible default retry settings.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        10 * time.Second,
		Multiplier:      2.0,
		Jitter:          0.1,
		AbsoluteTimeout: 2 * time.Minute,
		RetryableErrors: nil, // Use default
	}
}

// RetryableProviderWrapper wraps a provider with retry logic.
type RetryableProviderWrapper struct {
	provider Provider
	config   RetryConfig
}

// NewRetryableProvider wraps a provider with retry logic.
func NewRetryableProvider(provider Provider, config RetryConfig) *RetryableProviderWrapper {
	if config.RetryableErrors == nil {
		config.RetryableErrors = isRetryableError
	}

	return &RetryableProviderWrapper{
		provider: provider,
		config:   config,
	}
}

// Name returns the wrapped provider's name.
func (r *RetryableProviderWrapper) Name() string {
	return r.provider.Name()
}

// Capabilities returns the wrapped provider's capabilities.
func (r *RetryableProviderWrapper) Capabilities() Capabilities {
	return r.provider.Capabilities()
}

// Complete executes a completion request with retry logic.
func (r *RetryableProviderWrapper) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := r.calculateBackoff(attempt)
			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := r.provider.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.config.RetryableErrors(err) {
			// Non-retryable error - return original error preserving type
			return nil, err
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	// Max retries exceeded - wrap in ProviderError if not already one
	var provErr *pkgerrors.ProviderError
	if errors.As(lastErr, &provErr) {
		return nil, fmt.Errorf("max retries exceeded after %d attempts: %w", r.config.MaxRetries+1, lastErr)
	}
	return nil, &pkgerrors.ProviderError{
		Provider:   r.provider.Name(),
		Message:    fmt.Sprintf("maximum retry attempts (%d) exceeded", r.config.MaxRetries+1),
		Suggestion: "Check provider availability or increase retry limit",
		Cause:      lastErr,
	}
}

// Stream executes a streaming request with retry logic.
// Note: Streaming retry is more complex as we can't partially replay a stream.
// This implementation retries the entire stream on failure before any chunks are sent.
func (r *RetryableProviderWrapper) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := r.calculateBackoff(attempt)
			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		chunks, err := r.provider.Stream(ctx, req)
		if err == nil {
			return chunks, nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.config.RetryableErrors(err) {
			// Non-retryable error - return original error preserving type
			return nil, err
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	// Max retries exceeded - wrap in ProviderError if not already one
	var provErr *pkgerrors.ProviderError
	if errors.As(lastErr, &provErr) {
		return nil, fmt.Errorf("max retries exceeded after %d attempts: %w", r.config.MaxRetries+1, lastErr)
	}
	return nil, &pkgerrors.ProviderError{
		Provider:   r.provider.Name(),
		Message:    fmt.Sprintf("maximum retry attempts (%d) exceeded", r.config.MaxRetries+1),
		Suggestion: "Check provider availability or increase retry limit",
		Cause:      lastErr,
	}
}

// calculateBackoff computes the delay for a given attempt with jitter.
func (r *RetryableProviderWrapper) calculateBackoff(attempt int) time.Duration {
	// Calculate exponential backoff: initialDelay * multiplier^(attempt-1)
	backoff := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))

	// Cap at max delay
	if backoff > float64(r.config.MaxDelay) {
		backoff = float64(r.config.MaxDelay)
	}

	// Add jitter: backoff * (1 ± jitter)
	if r.config.Jitter > 0 {
		jitterAmount := backoff * r.config.Jitter
		jitterDelta := (rand.Float64() * 2 * jitterAmount) - jitterAmount
		backoff += jitterDelta
	}

	return time.Duration(backoff)
}

// isRetryableError determines if an error should trigger a retry.
// Retryable errors include:
// - HTTP 5xx errors (server errors)
// - HTTP 429 (rate limiting)
// - Timeout errors
// - Temporary network errors
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation/timeout (not retryable)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for ProviderError with HTTP status codes
	var provErr *pkgerrors.ProviderError
	if errors.As(err, &provErr) {
		// Retry on server errors and rate limiting
		return provErr.StatusCode >= 500 || provErr.StatusCode == http.StatusTooManyRequests
	}

	// Check for timeout errors
	var timeoutErr *pkgerrors.TimeoutError
	if errors.As(err, &timeoutErr) {
		return true
	}

	// Check for temporary errors (network issues)
	type temporary interface {
		Temporary() bool
	}
	if temp, ok := err.(temporary); ok {
		return temp.Temporary()
	}

	// Default to not retrying unknown errors
	return false
}
