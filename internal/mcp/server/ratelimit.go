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

package server

import (
	"sync"
	"time"
)

// RateLimiter implements token bucket rate limiting for MCP tool calls
type RateLimiter struct {
	// Token buckets for different operations
	runBucket  *tokenBucket
	callBucket *tokenBucket
}

// tokenBucket implements a simple token bucket algorithm
type tokenBucket struct {
	mu            sync.Mutex
	tokens        float64
	maxTokens     float64
	refillRate    float64 // tokens per second
	lastRefill    time.Time
}

// NewRateLimiter creates a rate limiter with specified limits
// runsPerMinute: max conductor_run (non-dry-run) calls per minute
// callsPerMinute: max total tool calls per minute
func NewRateLimiter(runsPerMinute, callsPerMinute int) *RateLimiter {
	return &RateLimiter{
		runBucket: &tokenBucket{
			tokens:     float64(runsPerMinute),
			maxTokens:  float64(runsPerMinute),
			refillRate: float64(runsPerMinute) / 60.0,
			lastRefill: time.Now(),
		},
		callBucket: &tokenBucket{
			tokens:     float64(callsPerMinute),
			maxTokens:  float64(callsPerMinute),
			refillRate: float64(callsPerMinute) / 60.0,
			lastRefill: time.Now(),
		},
	}
}

// AllowRun checks if a conductor_run (non-dry-run) call is allowed
func (rl *RateLimiter) AllowRun() bool {
	return rl.runBucket.take(1)
}

// AllowCall checks if any tool call is allowed
func (rl *RateLimiter) AllowCall() bool {
	return rl.callBucket.take(1)
}

// take attempts to take n tokens from the bucket
func (tb *tokenBucket) take(n float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens = min(tb.maxTokens, tb.tokens+elapsed*tb.refillRate)
	tb.lastRefill = now

	// Check if we have enough tokens
	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}

	return false
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
