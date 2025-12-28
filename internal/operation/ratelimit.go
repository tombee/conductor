package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

// RateLimiter enforces rate limits using a token bucket algorithm.
type RateLimiter struct {
	mu              sync.Mutex
	config          *workflow.RateLimitConfig
	tokens          float64
	lastRefill      time.Time
	stateFilePath   string
	requestsPerSec  float64
	maxTokens       float64
	refillInterval  time.Duration
}

// RateLimiterState represents the persistent state of a rate limiter.
type RateLimiterState struct {
	Tokens     float64   `json:"tokens"`
	LastRefill time.Time `json:"last_refill"`
}

// NewRateLimiter creates a new rate limiter from configuration.
func NewRateLimiter(config *workflow.RateLimitConfig, stateFilePath string) *RateLimiter {
	if config == nil {
		return nil
	}

	// Calculate requests per second from config
	requestsPerSec := config.RequestsPerSecond
	if requestsPerSec == 0 && config.RequestsPerMinute > 0 {
		requestsPerSec = float64(config.RequestsPerMinute) / 60.0
	}

	rl := &RateLimiter{
		config:         config,
		tokens:         requestsPerSec, // Start with full bucket
		lastRefill:     time.Now(),
		stateFilePath:  stateFilePath,
		requestsPerSec: requestsPerSec,
		maxTokens:      requestsPerSec * 2, // Allow burst up to 2 seconds worth
		refillInterval: time.Second,
	}

	// Load persisted state if available
	if stateFilePath != "" {
		rl.loadState()
	}

	return rl
}

// Wait blocks until a token is available or timeout is reached.
// Returns an error if the timeout is exceeded.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	if rl == nil {
		return nil // No rate limiting configured
	}

	timeout := time.Duration(rl.config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}

	deadline := time.Now().Add(timeout)

	for {
		// Try to acquire a token
		if rl.tryAcquire() {
			return nil
		}

		// Check if we've exceeded the timeout
		if time.Now().After(deadline) {
			return &Error{
				Type:       ErrorTypeRateLimit,
				Message:    fmt.Sprintf("rate limit timeout after %v", timeout),
				SuggestText: "Increase rate_limit.timeout or reduce request frequency",
			}
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Continue loop
		}
	}
}

// tryAcquire attempts to acquire a token from the bucket.
// Returns true if a token was acquired, false otherwise.
func (rl *RateLimiter) tryAcquire() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on elapsed time
	rl.refill()

	// Check if we have tokens available
	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		rl.saveState()
		return true
	}

	return false
}

// refill adds tokens to the bucket based on elapsed time.
// Must be called with lock held.
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)

	// Calculate tokens to add based on elapsed time
	tokensToAdd := elapsed.Seconds() * rl.requestsPerSec

	// Add tokens up to max capacity
	rl.tokens = min(rl.tokens+tokensToAdd, rl.maxTokens)
	rl.lastRefill = now
}

// loadState loads the rate limiter state from disk.
func (rl *RateLimiter) loadState() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	data, err := os.ReadFile(rl.stateFilePath)
	if err != nil {
		// File doesn't exist or can't be read, use defaults
		return
	}

	var state RateLimiterState
	if err := json.Unmarshal(data, &state); err != nil {
		// Invalid state file, use defaults
		return
	}

	// Restore state
	rl.tokens = state.Tokens
	rl.lastRefill = state.LastRefill

	// Refill tokens for time elapsed since last save
	rl.refill()
}

// saveState persists the rate limiter state to disk.
// Must be called with lock held.
func (rl *RateLimiter) saveState() {
	if rl.stateFilePath == "" {
		return
	}

	state := RateLimiterState{
		Tokens:     rl.tokens,
		LastRefill: rl.lastRefill,
	}

	data, err := json.Marshal(state)
	if err != nil {
		// Failed to marshal, skip save
		return
	}

	// Best-effort save, ignore errors
	_ = os.WriteFile(rl.stateFilePath, data, 0600)
}

// GetStats returns current rate limiter statistics.
func (rl *RateLimiter) GetStats() map[string]interface{} {
	if rl == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill before reporting stats
	rl.refill()

	return map[string]interface{}{
		"enabled":           true,
		"requests_per_sec":  rl.requestsPerSec,
		"available_tokens":  rl.tokens,
		"max_tokens":        rl.maxTokens,
		"requests_per_min":  rl.config.RequestsPerMinute,
		"timeout":           rl.config.Timeout,
	}
}

// min returns the minimum of two float64 values.
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
