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
	"net/http"
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
