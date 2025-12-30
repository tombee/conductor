package polltrigger

import (
	"context"
	"sync"
	"time"
)

// RateLimiter provides rate limiting for integration API calls.
// It enforces per-integration rate limits with exponential backoff on 429 errors.
type RateLimiter struct {
	mu sync.Mutex

	// Per-integration limits
	limits map[string]*integrationLimit
}

// integrationLimit tracks rate limiting state for a single integration.
type integrationLimit struct {
	// Minimum poll interval in seconds
	minInterval int

	// Last successful poll time
	lastPoll time.Time

	// Backoff state
	backoffUntil time.Time
	backoffCount int

	// Request budget tracking
	requestsPerMinute int
	requestWindow     time.Time
	requestCount      int
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limits: make(map[string]*integrationLimit),
	}
}

// Allow checks if a poll is allowed for the given integration.
// Returns true if the poll can proceed, false if rate limited.
func (r *RateLimiter) Allow(integration string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit := r.getOrCreateLimit(integration)

	now := time.Now()

	// Check if we're in backoff period
	if now.Before(limit.backoffUntil) {
		return false
	}

	// Check minimum interval
	if limit.minInterval > 0 {
		elapsed := now.Sub(limit.lastPoll).Seconds()
		if elapsed < float64(limit.minInterval) {
			return false
		}
	}

	// Check request budget (if configured)
	if limit.requestsPerMinute > 0 {
		// Reset window if needed
		if now.Sub(limit.requestWindow) >= time.Minute {
			limit.requestWindow = now
			limit.requestCount = 0
		}

		// Check if we've exceeded budget
		if limit.requestCount >= limit.requestsPerMinute {
			return false
		}
	}

	return true
}

// RecordSuccess records a successful poll for rate limiting.
func (r *RateLimiter) RecordSuccess(integration string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit := r.getOrCreateLimit(integration)
	limit.lastPoll = time.Now()

	// Increment request count
	if limit.requestsPerMinute > 0 {
		limit.requestCount++
	}

	// Clear backoff on success
	limit.backoffCount = 0
	limit.backoffUntil = time.Time{}
}

// RecordRateLimit records a 429 rate limit response and applies exponential backoff.
func (r *RateLimiter) RecordRateLimit(integration string, retryAfter time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit := r.getOrCreateLimit(integration)
	limit.backoffCount++

	// Exponential backoff: 30s, 60s, 120s, 240s, 480s (max 10m)
	backoffDuration := time.Duration(30<<uint(limit.backoffCount-1)) * time.Second
	if backoffDuration > 10*time.Minute {
		backoffDuration = 10 * time.Minute
	}

	// Use retryAfter if provided and greater
	if retryAfter > backoffDuration {
		backoffDuration = retryAfter
	}

	limit.backoffUntil = time.Now().Add(backoffDuration)
}

// RecordError records a general error (not rate limit).
func (r *RateLimiter) RecordError(integration string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit := r.getOrCreateLimit(integration)
	limit.lastPoll = time.Now()

	// Increment request count even on error
	if limit.requestsPerMinute > 0 {
		limit.requestCount++
	}
}

// SetMinInterval sets the minimum poll interval for an integration.
func (r *RateLimiter) SetMinInterval(integration string, intervalSeconds int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit := r.getOrCreateLimit(integration)
	limit.minInterval = intervalSeconds
}

// SetRequestBudget sets the requests per minute budget for an integration.
func (r *RateLimiter) SetRequestBudget(integration string, requestsPerMinute int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit := r.getOrCreateLimit(integration)
	limit.requestsPerMinute = requestsPerMinute
	limit.requestWindow = time.Now()
	limit.requestCount = 0
}

// GetBackoffStatus returns the backoff status for an integration.
// Returns the backoff end time and whether the integration is currently backed off.
func (r *RateLimiter) GetBackoffStatus(integration string) (time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit, exists := r.limits[integration]
	if !exists {
		return time.Time{}, false
	}

	if time.Now().Before(limit.backoffUntil) {
		return limit.backoffUntil, true
	}

	return time.Time{}, false
}

// WaitIfNeeded blocks until the rate limit allows the poll to proceed.
// Returns an error if the context is cancelled while waiting.
func (r *RateLimiter) WaitIfNeeded(ctx context.Context, integration string) error {
	for {
		if r.Allow(integration) {
			return nil
		}

		// Calculate wait time
		r.mu.Lock()
		limit := r.getOrCreateLimit(integration)
		waitUntil := limit.backoffUntil
		if waitUntil.IsZero() || time.Now().After(waitUntil) {
			// Calculate based on minimum interval
			nextAllowed := limit.lastPoll.Add(time.Duration(limit.minInterval) * time.Second)
			waitUntil = nextAllowed
		}
		r.mu.Unlock()

		waitDuration := time.Until(waitUntil)
		if waitDuration <= 0 {
			waitDuration = 100 * time.Millisecond
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
			// Try again
		}
	}
}

// getOrCreateLimit gets or creates an integration limit entry.
// Must be called with r.mu held.
func (r *RateLimiter) getOrCreateLimit(integration string) *integrationLimit {
	limit, exists := r.limits[integration]
	if !exists {
		limit = &integrationLimit{
			minInterval:       10, // Default: 10 seconds
			requestsPerMinute: 0,  // Default: no budget limit
			requestWindow:     time.Now(),
		}
		r.limits[integration] = limit
	}
	return limit
}
