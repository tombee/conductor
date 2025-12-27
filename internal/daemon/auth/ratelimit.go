// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimitConfig contains rate limiting configuration.
type RateLimitConfig struct {
	// RequestsPerSecond is the number of requests allowed per second per user.
	RequestsPerSecond float64

	// BurstSize is the maximum burst size (token bucket capacity).
	BurstSize int

	// Enabled controls whether rate limiting is active.
	Enabled bool
}

// tokenBucket implements a token bucket rate limiter.
type tokenBucket struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64
	lastRefillTime time.Time
	mu             sync.Mutex
}

// newTokenBucket creates a new token bucket.
func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		tokens:         float64(burst),
		maxTokens:      float64(burst),
		refillRate:     rate,
		lastRefillTime: time.Now(),
	}
}

// allow checks if a request is allowed and consumes a token if so.
func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime).Seconds()

	// Refill tokens based on elapsed time
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefillTime = now

	// Try to consume a token
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}

	return false
}

// RateLimiter provides per-user rate limiting.
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*tokenBucket
	config  RateLimitConfig
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	// Set defaults
	if cfg.RequestsPerSecond <= 0 {
		cfg.RequestsPerSecond = 10 // 10 requests per second default
	}
	if cfg.BurstSize <= 0 {
		cfg.BurstSize = 20 // Allow bursts up to 20 requests
	}

	return &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		config:  cfg,
	}
}

// Allow checks if a request from the given user is allowed.
func (rl *RateLimiter) Allow(userID string) bool {
	if !rl.config.Enabled {
		return true
	}

	if userID == "" {
		// For unauthenticated requests, use a shared bucket
		userID = "_anonymous_"
	}

	// Get or create bucket for user
	rl.mu.RLock()
	bucket, exists := rl.buckets[userID]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		bucket, exists = rl.buckets[userID]
		if !exists {
			bucket = newTokenBucket(rl.config.RequestsPerSecond, rl.config.BurstSize)
			rl.buckets[userID] = bucket
		}
		rl.mu.Unlock()
	}

	return bucket.allow()
}

// Cleanup removes buckets for users who haven't made requests recently.
// This prevents memory leaks from accumulating buckets for one-time users.
func (rl *RateLimiter) Cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for userID, bucket := range rl.buckets {
		bucket.mu.Lock()
		age := now.Sub(bucket.lastRefillTime)
		bucket.mu.Unlock()

		if age > maxAge {
			delete(rl.buckets, userID)
		}
	}
}

// Middleware wraps an http.Handler with rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Try to extract user ID from context
		// In a real implementation, this would come from the auth middleware
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			// Fallback to IP-based limiting for unauthenticated requests
			userID = r.RemoteAddr
		}

		if !rl.Allow(userID) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ParseRateLimit parses a rate limit string like "100/hour", "10/minute", "5/second"
// and returns requests per second and burst size.
// Returns an error if the format is invalid.
func ParseRateLimit(limit string) (requestsPerSecond float64, burstSize int, err error) {
	if limit == "" {
		return 0, 0, fmt.Errorf("empty rate limit string")
	}

	parts := strings.Split(limit, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid rate limit format: expected 'count/period' (e.g., '100/hour'), got %q", limit)
	}

	// Parse count
	count, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid count in rate limit %q: %w", limit, err)
	}
	if count <= 0 {
		return 0, 0, fmt.Errorf("rate limit count must be positive, got %d", count)
	}

	// Parse period
	period := strings.TrimSpace(strings.ToLower(parts[1]))
	var seconds float64
	switch period {
	case "second", "sec", "s":
		seconds = 1
	case "minute", "min", "m":
		seconds = 60
	case "hour", "hr", "h":
		seconds = 3600
	case "day", "d":
		seconds = 86400
	default:
		return 0, 0, fmt.Errorf("invalid period in rate limit %q: expected second/minute/hour/day, got %q", limit, period)
	}

	// Calculate requests per second
	requestsPerSecond = float64(count) / seconds

	// Set burst size to the count (allow full period worth of requests in burst)
	// For example, "100/hour" allows bursting 100 requests immediately
	burstSize = count

	return requestsPerSecond, burstSize, nil
}

// NamedRateLimiter provides rate limiting with named buckets.
// Each bucket can have different rate limits (e.g., per-endpoint, per-key).
type NamedRateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*tokenBucket
	configs map[string]RateLimitConfig
}

// NewNamedRateLimiter creates a new named rate limiter.
func NewNamedRateLimiter() *NamedRateLimiter {
	return &NamedRateLimiter{
		buckets: make(map[string]*tokenBucket),
		configs: make(map[string]RateLimitConfig),
	}
}

// AddLimit adds or updates a named rate limit.
// The limitStr should be in the format "count/period" (e.g., "100/hour").
func (nrl *NamedRateLimiter) AddLimit(name string, limitStr string) error {
	rps, burst, err := ParseRateLimit(limitStr)
	if err != nil {
		return err
	}

	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	nrl.configs[name] = RateLimitConfig{
		RequestsPerSecond: rps,
		BurstSize:         burst,
		Enabled:           true,
	}

	// Remove existing bucket to force recreation with new limits
	delete(nrl.buckets, name)

	return nil
}

// RemoveLimit removes a named rate limit.
func (nrl *NamedRateLimiter) RemoveLimit(name string) {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	delete(nrl.configs, name)
	delete(nrl.buckets, name)
}

// Allow checks if a request for the named limit is allowed.
// Returns true if allowed, false if rate limit exceeded.
func (nrl *NamedRateLimiter) Allow(name string) bool {
	// Get or create bucket for this limit
	nrl.mu.RLock()
	config, hasConfig := nrl.configs[name]
	bucket, hasBucket := nrl.buckets[name]
	nrl.mu.RUnlock()

	if !hasConfig {
		// No limit configured for this name, allow by default
		return true
	}

	if !hasBucket {
		nrl.mu.Lock()
		// Double-check after acquiring write lock
		bucket, hasBucket = nrl.buckets[name]
		if !hasBucket {
			bucket = newTokenBucket(config.RequestsPerSecond, config.BurstSize)
			nrl.buckets[name] = bucket
		}
		nrl.mu.Unlock()
	}

	return bucket.allow()
}

// GetStatus returns the current status of a named rate limit.
// Returns remaining tokens, max tokens, and reset time.
func (nrl *NamedRateLimiter) GetStatus(name string) (remaining float64, limit float64, resetAt time.Time, exists bool) {
	nrl.mu.RLock()
	defer nrl.mu.RUnlock()

	config, hasConfig := nrl.configs[name]
	bucket, hasBucket := nrl.buckets[name]

	if !hasConfig {
		return 0, 0, time.Time{}, false
	}

	if !hasBucket {
		// No bucket yet, return full capacity
		return float64(config.BurstSize), float64(config.BurstSize), time.Now(), true
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Refill tokens to get current state
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefillTime).Seconds()
	tokens := bucket.tokens + elapsed*bucket.refillRate
	if tokens > bucket.maxTokens {
		tokens = bucket.maxTokens
	}

	// Calculate when bucket will be full again
	if tokens >= bucket.maxTokens {
		resetAt = now
	} else {
		tokensNeeded := bucket.maxTokens - tokens
		secondsToFull := tokensNeeded / bucket.refillRate
		resetAt = now.Add(time.Duration(secondsToFull * float64(time.Second)))
	}

	return tokens, bucket.maxTokens, resetAt, true
}
