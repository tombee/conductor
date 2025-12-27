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

func TestParseRateLimit(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantRPS     float64
		wantBurst   int
		wantErr     bool
		errContains string
	}{
		{
			name:      "per second",
			input:     "10/second",
			wantRPS:   10.0,
			wantBurst: 10,
		},
		{
			name:      "per minute",
			input:     "60/minute",
			wantRPS:   1.0,
			wantBurst: 60,
		},
		{
			name:      "per hour",
			input:     "3600/hour",
			wantRPS:   1.0,
			wantBurst: 3600,
		},
		{
			name:      "per day",
			input:     "86400/day",
			wantRPS:   1.0,
			wantBurst: 86400,
		},
		{
			name:      "short form second",
			input:     "5/s",
			wantRPS:   5.0,
			wantBurst: 5,
		},
		{
			name:      "short form minute",
			input:     "100/m",
			wantRPS:   100.0 / 60.0,
			wantBurst: 100,
		},
		{
			name:      "short form hour",
			input:     "1000/h",
			wantRPS:   1000.0 / 3600.0,
			wantBurst: 1000,
		},
		{
			name:      "with whitespace",
			input:     " 50 / minute ",
			wantRPS:   50.0 / 60.0,
			wantBurst: 50,
		},
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "invalid format",
			input:       "100",
			wantErr:     true,
			errContains: "invalid rate limit format",
		},
		{
			name:        "invalid count",
			input:       "abc/hour",
			wantErr:     true,
			errContains: "invalid count",
		},
		{
			name:        "negative count",
			input:       "-10/hour",
			wantErr:     true,
			errContains: "must be positive",
		},
		{
			name:        "zero count",
			input:       "0/hour",
			wantErr:     true,
			errContains: "must be positive",
		},
		{
			name:        "invalid period",
			input:       "100/year",
			wantErr:     true,
			errContains: "invalid period",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rps, burst, err := ParseRateLimit(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.InDelta(t, tt.wantRPS, rps, 0.0001, "requests per second mismatch")
			assert.Equal(t, tt.wantBurst, burst, "burst size mismatch")
		})
	}
}

func TestNamedRateLimiter_AddLimit(t *testing.T) {
	nrl := NewNamedRateLimiter()

	err := nrl.AddLimit("endpoint1", "100/hour")
	assert.NoError(t, err)

	// Verify config was added
	assert.Contains(t, nrl.configs, "endpoint1")
	assert.InDelta(t, 100.0/3600.0, nrl.configs["endpoint1"].RequestsPerSecond, 0.0001)
	assert.Equal(t, 100, nrl.configs["endpoint1"].BurstSize)
}

func TestNamedRateLimiter_AddLimit_InvalidFormat(t *testing.T) {
	nrl := NewNamedRateLimiter()

	err := nrl.AddLimit("endpoint1", "invalid")
	assert.Error(t, err)
	assert.NotContains(t, nrl.configs, "endpoint1")
}

func TestNamedRateLimiter_Allow(t *testing.T) {
	nrl := NewNamedRateLimiter()

	// Add a limit for 10/second
	err := nrl.AddLimit("endpoint1", "10/second")
	assert.NoError(t, err)

	// Should allow initial burst
	for i := 0; i < 10; i++ {
		assert.True(t, nrl.Allow("endpoint1"), "request %d should be allowed", i)
	}

	// Next request should be denied
	assert.False(t, nrl.Allow("endpoint1"))
}

func TestNamedRateLimiter_Allow_NoLimit(t *testing.T) {
	nrl := NewNamedRateLimiter()

	// Should allow unlimited requests when no limit configured
	for i := 0; i < 1000; i++ {
		assert.True(t, nrl.Allow("unlimited"), "all requests should be allowed")
	}
}

func TestNamedRateLimiter_Allow_PerEndpoint(t *testing.T) {
	nrl := NewNamedRateLimiter()

	// Add different limits
	nrl.AddLimit("endpoint1", "5/second")
	nrl.AddLimit("endpoint2", "10/second")

	// Exhaust endpoint1
	for i := 0; i < 5; i++ {
		nrl.Allow("endpoint1")
	}

	// endpoint1 should be denied
	assert.False(t, nrl.Allow("endpoint1"))

	// endpoint2 should still be allowed
	assert.True(t, nrl.Allow("endpoint2"))
}

func TestNamedRateLimiter_RemoveLimit(t *testing.T) {
	nrl := NewNamedRateLimiter()

	nrl.AddLimit("endpoint1", "10/second")
	assert.Contains(t, nrl.configs, "endpoint1")

	nrl.RemoveLimit("endpoint1")
	assert.NotContains(t, nrl.configs, "endpoint1")
	assert.NotContains(t, nrl.buckets, "endpoint1")

	// Should now allow unlimited
	for i := 0; i < 100; i++ {
		assert.True(t, nrl.Allow("endpoint1"))
	}
}

func TestNamedRateLimiter_GetStatus(t *testing.T) {
	nrl := NewNamedRateLimiter()

	// Add a limit
	nrl.AddLimit("endpoint1", "10/second")

	// Get status before any requests
	remaining, limit, resetAt, exists := nrl.GetStatus("endpoint1")
	assert.True(t, exists)
	assert.Equal(t, 10.0, limit)
	assert.Equal(t, 10.0, remaining)
	assert.False(t, resetAt.IsZero())

	// Make some requests
	for i := 0; i < 5; i++ {
		nrl.Allow("endpoint1")
	}

	// Get status after requests
	remaining, limit, resetAt, exists = nrl.GetStatus("endpoint1")
	assert.True(t, exists)
	assert.Equal(t, 10.0, limit)
	assert.InDelta(t, 5.0, remaining, 0.5) // Should have ~5 tokens left
	assert.True(t, resetAt.After(time.Now()))
}

func TestNamedRateLimiter_GetStatus_NotExists(t *testing.T) {
	nrl := NewNamedRateLimiter()

	remaining, limit, resetAt, exists := nrl.GetStatus("nonexistent")
	assert.False(t, exists)
	assert.Equal(t, 0.0, remaining)
	assert.Equal(t, 0.0, limit)
	assert.True(t, resetAt.IsZero())
}
