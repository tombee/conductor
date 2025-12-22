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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10,
		BurstSize:         20,
	})

	// Should allow initial burst
	for i := 0; i < 20; i++ {
		assert.True(t, rl.Allow("user1"), "request %d should be allowed", i)
	}

	// Next request should be denied (burst exhausted)
	assert.False(t, rl.Allow("user1"))
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10,
		BurstSize:         10,
	})

	// Exhaust the bucket
	for i := 0; i < 10; i++ {
		rl.Allow("user1")
	}

	// Should be denied
	assert.False(t, rl.Allow("user1"))

	// Wait for refill (100ms should give us 1 token at 10/sec)
	time.Sleep(150 * time.Millisecond)

	// Should allow one more request
	assert.True(t, rl.Allow("user1"))
}

func TestRateLimiter_PerUser(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 5,
		BurstSize:         5,
	})

	// Exhaust user1's bucket
	for i := 0; i < 5; i++ {
		rl.Allow("user1")
	}

	// user1 should be denied
	assert.False(t, rl.Allow("user1"))

	// user2 should still be allowed (separate bucket)
	assert.True(t, rl.Allow("user2"))
}

func TestRateLimiter_Disabled(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Enabled: false,
	})

	// Should always allow when disabled
	for i := 0; i < 1000; i++ {
		assert.True(t, rl.Allow("user1"))
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10,
		BurstSize:         10,
	})

	// Create buckets for multiple users
	rl.Allow("user1")
	rl.Allow("user2")
	rl.Allow("user3")

	assert.Len(t, rl.buckets, 3)

	// Cleanup old buckets (should remove all since they're recent)
	rl.Cleanup(1 * time.Millisecond)

	// Wait a bit
	time.Sleep(5 * time.Millisecond)

	// Cleanup again (should now remove all buckets)
	rl.Cleanup(1 * time.Millisecond)

	assert.Len(t, rl.buckets, 0)
}

func TestTokenBucket_Allow(t *testing.T) {
	tb := newTokenBucket(10, 10)

	// Should allow 10 requests immediately
	for i := 0; i < 10; i++ {
		assert.True(t, tb.allow())
	}

	// 11th request should fail
	assert.False(t, tb.allow())
}

func TestTokenBucket_MaxTokens(t *testing.T) {
	tb := newTokenBucket(100, 10)

	// Wait for potential refill
	time.Sleep(100 * time.Millisecond)

	// Should still only have max tokens (10), not more
	successCount := 0
	for i := 0; i < 20; i++ {
		if tb.allow() {
			successCount++
		}
	}

	assert.LessOrEqual(t, successCount, 11) // 10 initial + maybe 1 from refill
}
