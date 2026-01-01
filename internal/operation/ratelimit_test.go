package operation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

func TestRateLimit_TokenBucketRefill(t *testing.T) {
	config := &workflow.RateLimitConfig{
		RequestsPerSecond: 10, // 10 requests per second
		Timeout:           5,
	}

	rl := NewRateLimiter(config, "")

	// Initially should have full bucket (10 tokens)
	stats := rl.GetStats()
	availableTokens := stats["available_tokens"].(float64)
	if availableTokens < 10.0 {
		t.Errorf("expected initial tokens >= 10.0, got %f", availableTokens)
	}

	// Acquire 5 tokens
	for i := 0; i < 5; i++ {
		if !rl.tryAcquire() {
			t.Fatalf("failed to acquire token %d", i)
		}
	}

	// Should have ~5 tokens left
	stats = rl.GetStats()
	availableTokens = stats["available_tokens"].(float64)
	if availableTokens > 6.0 || availableTokens < 4.0 {
		t.Errorf("expected ~5 tokens after acquiring 5, got %f", availableTokens)
	}

	// Wait 1 second for refill (should add ~10 tokens)
	time.Sleep(1100 * time.Millisecond)

	// Try acquiring a token to trigger refill
	rl.tryAcquire()

	// Should have refilled to close to max (capped at 20 = 2x requestsPerSecond)
	stats = rl.GetStats()
	availableTokens = stats["available_tokens"].(float64)
	maxTokens := stats["max_tokens"].(float64)

	if maxTokens != 20.0 {
		t.Errorf("expected max_tokens = 20.0, got %f", maxTokens)
	}

	// Should be near max after refill (accounting for the one we just acquired)
	if availableTokens < 13.0 || availableTokens > 20.0 {
		t.Errorf("expected tokens refilled to ~14-20 range, got %f", availableTokens)
	}
}

func TestRateLimit_BurstAllowed(t *testing.T) {
	config := &workflow.RateLimitConfig{
		RequestsPerSecond: 5, // 5 requests per second
		Timeout:           10,
	}

	rl := NewRateLimiter(config, "")

	// Should allow burst of up to 2x = 10 requests rapidly
	successCount := 0
	for i := 0; i < 12; i++ {
		if rl.tryAcquire() {
			successCount++
		}
	}

	// Should have acquired between 5 and 10 tokens (burst capacity)
	if successCount < 5 || successCount > 10 {
		t.Errorf("expected burst of 5-10 requests, got %d", successCount)
	}

	// Verify max tokens is 2x requests per second
	stats := rl.GetStats()
	maxTokens := stats["max_tokens"].(float64)
	if maxTokens != 10.0 {
		t.Errorf("expected max_tokens = 10.0 (2x), got %f", maxTokens)
	}
}

func TestRateLimit_TimeoutEnforced(t *testing.T) {
	config := &workflow.RateLimitConfig{
		RequestsPerSecond: 0.2, // Very slow: 0.2 requests per second (1 every 5 seconds)
		Timeout:           1,   // Very short timeout: 1 second
	}

	rl := NewRateLimiter(config, "")

	// Exhaust all available tokens (should have 0.2-0.4 initially)
	for rl.tryAcquire() {
		// Keep acquiring until no more tokens
	}

	// Wait a tiny bit to ensure we're past any immediate refill
	time.Sleep(50 * time.Millisecond)

	// Now wait should timeout since no tokens are available and refill is too slow
	// At 0.2 req/sec, in 1 second we only get 0.2 tokens, which is < 1.0 needed
	ctx := context.Background()
	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	connErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}

	if connErr.Type != ErrorTypeRateLimit {
		t.Errorf("expected error type %s, got %s", ErrorTypeRateLimit, connErr.Type)
	}

	// Should timeout after ~1 second
	if elapsed < 900*time.Millisecond || elapsed > 2*time.Second {
		t.Errorf("expected timeout after ~1s, took %v", elapsed)
	}

	if connErr.SuggestText == "" {
		t.Error("expected suggestion in timeout error")
	}
}

func TestRateLimit_StatePersistence(t *testing.T) {
	// Create temporary state file
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "ratelimit_state.json")

	config := &workflow.RateLimitConfig{
		RequestsPerSecond: 10,
		Timeout:           5,
	}

	// Create first rate limiter and acquire some tokens
	rl1 := NewRateLimiter(config, stateFile)

	// Acquire 5 tokens
	for i := 0; i < 5; i++ {
		if !rl1.tryAcquire() {
			t.Fatalf("failed to acquire token %d", i)
		}
	}

	stats1 := rl1.GetStats()
	tokensAfterAcquire := stats1["available_tokens"].(float64)

	// Verify state file was created
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Fatal("state file was not created")
	}

	// Read and verify state file contents
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var state RateLimiterState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("failed to unmarshal state: %v", err)
	}

	if state.Tokens < 4.0 || state.Tokens > 6.0 {
		t.Errorf("expected state tokens ~5.0, got %f", state.Tokens)
	}

	// Create second rate limiter loading from same state file
	time.Sleep(100 * time.Millisecond) // Small delay to allow some refill
	rl2 := NewRateLimiter(config, stateFile)

	stats2 := rl2.GetStats()
	tokensAfterReload := stats2["available_tokens"].(float64)

	// Should have similar tokens (plus small refill during the 100ms delay)
	// The refill during 100ms at 10 req/s = ~1 token
	expectedMin := tokensAfterAcquire
	expectedMax := tokensAfterAcquire + 2.0 // Allow for refill + timing variance

	if tokensAfterReload < expectedMin || tokensAfterReload > expectedMax {
		t.Errorf("expected reloaded tokens in range [%f, %f], got %f",
			expectedMin, expectedMax, tokensAfterReload)
	}
}

func TestRateLimit_Concurrent(t *testing.T) {
	config := &workflow.RateLimitConfig{
		RequestsPerSecond: 20, // 20 requests per second
		Timeout:           5,
	}

	rl := NewRateLimiter(config, "")

	// Launch multiple goroutines trying to acquire tokens concurrently
	const goroutines = 10
	const attemptsPerGoroutine = 5

	var wg sync.WaitGroup
	successCount := make([]int, goroutines)
	errors := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			for j := 0; j < attemptsPerGoroutine; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				err := rl.Wait(ctx)
				cancel()

				if err == nil {
					successCount[idx]++
				} else {
					errors[idx] = err
					break
				}
			}
		}(i)
	}

	wg.Wait()

	// Count total successes
	totalSuccess := 0
	for i, count := range successCount {
		totalSuccess += count
		if errors[i] != nil {
			t.Logf("goroutine %d encountered error after %d successes: %v", i, count, errors[i])
		}
	}

	// With burst capacity (40 tokens max) and refill during execution,
	// most goroutines should succeed. Exact count depends on timing,
	// but we should get at least the burst capacity worth
	minExpected := 30 // Conservative: burst capacity minus some contention overhead

	if totalSuccess < minExpected {
		t.Errorf("expected at least %d successful acquisitions across all goroutines, got %d",
			minExpected, totalSuccess)
	}

	t.Logf("Successfully acquired %d tokens across %d concurrent goroutines", totalSuccess, goroutines)
}

func TestRateLimit_NilRateLimiter(t *testing.T) {
	// Nil rate limiter should not block
	var rl *RateLimiter
	ctx := context.Background()

	err := rl.Wait(ctx)
	if err != nil {
		t.Errorf("nil rate limiter should not return error, got: %v", err)
	}

	stats := rl.GetStats()
	if stats["enabled"].(bool) {
		t.Error("nil rate limiter should report enabled=false")
	}
}

func TestRateLimit_ContextCancellation(t *testing.T) {
	config := &workflow.RateLimitConfig{
		RequestsPerSecond: 1, // Very slow
		Timeout:           10,
	}

	rl := NewRateLimiter(config, "")

	// Exhaust all tokens
	for rl.tryAcquire() {
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}

	// Should return quickly when context is cancelled (not wait for rate limit timeout)
	if elapsed > 500*time.Millisecond {
		t.Errorf("expected quick return on context cancellation, took %v", elapsed)
	}
}

func TestRateLimit_RequestsPerMinuteConversion(t *testing.T) {
	config := &workflow.RateLimitConfig{
		RequestsPerMinute: 120, // 120 per minute = 2 per second
		Timeout:           5,
	}

	rl := NewRateLimiter(config, "")

	stats := rl.GetStats()
	requestsPerSec := stats["requests_per_sec"].(float64)

	// Should convert 120 req/min to 2 req/sec
	expected := 2.0
	if requestsPerSec < expected-0.1 || requestsPerSec > expected+0.1 {
		t.Errorf("expected requests_per_sec = %f, got %f", expected, requestsPerSec)
	}

	// Max tokens should be 2x requests per second
	maxTokens := stats["max_tokens"].(float64)
	if maxTokens != 4.0 {
		t.Errorf("expected max_tokens = 4.0 (2x2), got %f", maxTokens)
	}
}

func TestRateLimit_DefaultTimeout(t *testing.T) {
	config := &workflow.RateLimitConfig{
		RequestsPerSecond: 1,
		Timeout:           0, // No timeout specified
	}

	rl := NewRateLimiter(config, "")

	// Exhaust all tokens
	for rl.tryAcquire() {
	}

	// Should use default 30 second timeout
	ctx := context.Background()
	start := time.Now()

	// Use a context with shorter timeout to avoid waiting 30 seconds
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	// Should timeout from context, not rate limiter
	if err != context.DeadlineExceeded {
		t.Errorf("expected context timeout, got: %v", err)
	}

	if elapsed > 2*time.Second {
		t.Errorf("took too long: %v", elapsed)
	}
}
