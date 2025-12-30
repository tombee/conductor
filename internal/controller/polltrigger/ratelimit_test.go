package polltrigger

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter()

	// First call should be allowed
	if !rl.Allow("test") {
		t.Error("first call should be allowed")
	}

	// Record success to update lastPoll
	rl.RecordSuccess("test")

	// Immediate second call should be blocked (min interval)
	if rl.Allow("test") {
		t.Error("immediate second call should be blocked")
	}

	// Wait for min interval
	time.Sleep(11 * time.Second)

	// Should be allowed now
	if !rl.Allow("test") {
		t.Error("call after min interval should be allowed")
	}
}

func TestRateLimiter_Backoff(t *testing.T) {
	rl := NewRateLimiter()

	// First rate limit should trigger 30s backoff
	rl.RecordRateLimit("test", 0)

	if rl.Allow("test") {
		t.Error("should be blocked during backoff")
	}

	backoffUntil, backedOff := rl.GetBackoffStatus("test")
	if !backedOff {
		t.Error("should be in backoff state")
	}
	if backoffUntil.IsZero() {
		t.Error("backoff time should be set")
	}

	// Second rate limit should double backoff
	rl.RecordRateLimit("test", 0)

	backoffUntil2, _ := rl.GetBackoffStatus("test")
	if !backoffUntil2.After(backoffUntil) {
		t.Error("second backoff should be longer")
	}

	// Success should clear backoff
	rl.RecordSuccess("test")
	_, backedOff = rl.GetBackoffStatus("test")
	if backedOff {
		t.Error("backoff should be cleared after success")
	}
}

func TestRateLimiter_RequestBudget(t *testing.T) {
	rl := NewRateLimiter()
	rl.SetMinInterval("test", 0) // Disable min interval
	rl.SetRequestBudget("test", 5)

	// Should allow up to 5 requests
	for i := 0; i < 5; i++ {
		if !rl.Allow("test") {
			t.Errorf("request %d should be allowed", i+1)
		}
		rl.RecordSuccess("test")
	}

	// 6th request should be blocked
	if rl.Allow("test") {
		t.Error("6th request should exceed budget")
	}
}

func TestRateLimiter_RetryAfter(t *testing.T) {
	rl := NewRateLimiter()

	// Record rate limit with custom retry-after
	retryAfter := 5 * time.Minute
	rl.RecordRateLimit("test", retryAfter)

	backoffUntil, backedOff := rl.GetBackoffStatus("test")
	if !backedOff {
		t.Error("should be in backoff state")
	}

	// Backoff should be at least retryAfter duration
	minBackoff := time.Now().Add(retryAfter - time.Second) // Allow 1s tolerance
	if backoffUntil.Before(minBackoff) {
		t.Error("backoff should respect retry-after duration")
	}
}

func TestRateLimiter_MaxBackoff(t *testing.T) {
	rl := NewRateLimiter()

	// Trigger many rate limits to hit max backoff
	for i := 0; i < 10; i++ {
		rl.RecordRateLimit("test", 0)
	}

	backoffUntil, _ := rl.GetBackoffStatus("test")
	maxBackoff := time.Now().Add(10*time.Minute + time.Second) // Allow 1s tolerance

	if backoffUntil.After(maxBackoff) {
		t.Error("backoff should not exceed 10 minutes")
	}
}

func TestRateLimiter_WaitIfNeeded(t *testing.T) {
	rl := NewRateLimiter()
	rl.SetMinInterval("test", 1) // 1 second min interval

	ctx := context.Background()

	// First call should not wait
	start := time.Now()
	if err := rl.WaitIfNeeded(ctx, "test"); err != nil {
		t.Fatalf("WaitIfNeeded failed: %v", err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Error("first call should not wait")
	}

	rl.RecordSuccess("test")

	// Second immediate call should wait
	start = time.Now()
	if err := rl.WaitIfNeeded(ctx, "test"); err != nil {
		t.Fatalf("WaitIfNeeded failed: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 900*time.Millisecond {
		t.Errorf("should have waited ~1s, waited %v", elapsed)
	}
}

func TestRateLimiter_WaitCancellation(t *testing.T) {
	rl := NewRateLimiter()
	rl.RecordRateLimit("test", 1*time.Hour) // Long backoff

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := rl.WaitIfNeeded(ctx, "test")
	if err == nil {
		t.Error("WaitIfNeeded should return error on context cancellation")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestRateLimiter_MultipleIntegrations(t *testing.T) {
	rl := NewRateLimiter()

	// Rate limit one integration shouldn't affect another
	rl.RecordRateLimit("integration1", 1*time.Minute)

	if rl.Allow("integration1") {
		t.Error("integration1 should be blocked")
	}

	if !rl.Allow("integration2") {
		t.Error("integration2 should be allowed")
	}
}
