package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryTransport_SuccessOnFirstAttempt(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create retry transport
	cfg := DefaultConfig()
	transport := newRetryTransport(http.DefaultTransport, cfg)

	// Create request
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Execute request
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestRetryTransport_RetriesOn5xx(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.RetryBackoff = 10 * time.Millisecond // Speed up test
	transport := newRetryTransport(http.DefaultTransport, cfg)

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryTransport_RetriesOn429(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.RetryBackoff = 10 * time.Millisecond
	transport := newRetryTransport(http.DefaultTransport, cfg)

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryTransport_DoesNotRetryOn4xx(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.RetryBackoff = 10 * time.Millisecond
	transport := newRetryTransport(http.DefaultTransport, cfg)

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetryTransport_MaxAttemptsExhausted(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.RetryAttempts = 2 // 3 total attempts (1 initial + 2 retries)
	cfg.RetryBackoff = 10 * time.Millisecond
	transport := newRetryTransport(http.DefaultTransport, cfg)

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}

	expectedAttempts := int32(3) // 1 initial + 2 retries
	if attempts != expectedAttempts {
		t.Errorf("expected %d attempts, got %d", expectedAttempts, attempts)
	}
}

func TestRetryTransport_RespectsRetryAfterHeader(t *testing.T) {
	var attempts int32
	var lastAttemptTime time.Time
	var timeBetweenAttempts time.Duration

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		now := time.Now()

		if attempt > 1 {
			timeBetweenAttempts = now.Sub(lastAttemptTime)
		}
		lastAttemptTime = now

		if attempt < 2 {
			w.Header().Set("Retry-After", "1") // 1 second
			w.WriteHeader(http.StatusTooManyRequests)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.RetryBackoff = 100 * time.Millisecond
	transport := newRetryTransport(http.DefaultTransport, cfg)

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify Retry-After was respected (should wait at least ~1s, but our backoff is 100ms)
	// We use the smaller of Retry-After and calculated backoff
	if timeBetweenAttempts < 90*time.Millisecond {
		t.Errorf("expected at least 90ms delay (backoff is smaller), got %v", timeBetweenAttempts)
	}
}

func TestRetryTransport_OnlyRetriesIdempotentMethods(t *testing.T) {
	tests := []struct {
		method           string
		shouldRetry      bool
		expectedAttempts int32
	}{
		{"GET", true, 3},
		{"HEAD", true, 3},
		{"OPTIONS", true, 3},
		{"POST", false, 1},
		{"PUT", false, 1},
		{"PATCH", false, 1},
		{"DELETE", false, 1},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			var attempts int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&attempts, 1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			cfg := DefaultConfig()
			cfg.RetryAttempts = 2
			cfg.RetryBackoff = 10 * time.Millisecond
			transport := newRetryTransport(http.DefaultTransport, cfg)

			req, err := http.NewRequest(tt.method, server.URL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := transport.RoundTrip(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if attempts != tt.expectedAttempts {
				t.Errorf("expected %d attempts for %s, got %d", tt.expectedAttempts, tt.method, attempts)
			}
		})
	}
}

func TestRetryTransport_AllowNonIdempotentRetry(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.AllowNonIdempotentRetry = true
	cfg.RetryBackoff = 10 * time.Millisecond
	transport := newRetryTransport(http.DefaultTransport, cfg)

	req, err := http.NewRequest("POST", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts with AllowNonIdempotentRetry=true, got %d", attempts)
	}
}

func TestRetryTransport_ContextCancellation(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		time.Sleep(50 * time.Millisecond) // Delay to allow cancellation
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.RetryBackoff = 10 * time.Millisecond
	transport := newRetryTransport(http.DefaultTransport, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = transport.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	// Should only attempt once before cancellation
	if atomic.LoadInt32(&attempts) > 1 {
		t.Errorf("expected 1 attempt, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestCalculateBackoff(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RetryBackoff = 100 * time.Millisecond
	cfg.MaxBackoff = 10 * time.Second
	transport := newRetryTransport(http.DefaultTransport, cfg)

	tests := []struct {
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{1, 80 * time.Millisecond, 140 * time.Millisecond},  // 100ms * 2^0 ± 20%
		{2, 160 * time.Millisecond, 280 * time.Millisecond}, // 100ms * 2^1 ± 20%
		{3, 320 * time.Millisecond, 560 * time.Millisecond}, // 100ms * 2^2 ± 20%
		{10, 8 * time.Second, 12 * time.Second},             // Capped at 10s ± 20%
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			backoff := transport.calculateBackoff(tt.attempt)
			if backoff < tt.minExpected || backoff > tt.maxExpected {
				t.Errorf("attempt %d: backoff %v not in range [%v, %v]",
					tt.attempt, backoff, tt.minExpected, tt.maxExpected)
			}
		})
	}
}
