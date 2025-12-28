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

package lifecycle

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHealthChecker_Check(t *testing.T) {
	t.Run("returns success for healthy endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		checker := NewHealthChecker(server.URL)
		result := checker.Check(context.Background())

		if !result.Success {
			t.Errorf("Check() success = false, want true (error: %v)", result.Error)
		}
		if result.StatusCode != http.StatusOK {
			t.Errorf("Check() status = %d, want %d", result.StatusCode, http.StatusOK)
		}
		if result.ResponseTime <= 0 {
			t.Error("Check() response time should be positive")
		}
	})

	t.Run("returns failure for unhealthy endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		checker := NewHealthChecker(server.URL)
		result := checker.Check(context.Background())

		if result.Success {
			t.Error("Check() success = true, want false")
		}
		if result.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("Check() status = %d, want %d", result.StatusCode, http.StatusServiceUnavailable)
		}
	})

	t.Run("returns error for connection failure", func(t *testing.T) {
		// Use a non-existent endpoint
		checker := NewHealthChecker("http://localhost:99999/health")
		result := checker.Check(context.Background())

		if result.Success {
			t.Error("Check() success = true, want false")
		}
		if result.Error == nil {
			t.Error("Check() error = nil, want non-nil")
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		// Create a server that delays response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(1 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		checker := NewHealthChecker(server.URL)
		result := checker.Check(ctx)

		if result.Success {
			t.Error("Check() success = true, want false (should timeout)")
		}
		if result.Error == nil {
			t.Error("Check() error = nil, want timeout error")
		}
	})
}

func TestHealthChecker_WaitUntilHealthy(t *testing.T) {
	t.Run("returns immediately for healthy endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		checker := NewHealthChecker(server.URL)
		start := time.Now()

		err := checker.WaitUntilHealthy(5 * time.Second)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("WaitUntilHealthy() error = %v", err)
		}
		if duration > 1*time.Second {
			t.Errorf("WaitUntilHealthy() took %v, should be nearly instant", duration)
		}
	})

	t.Run("waits and succeeds when endpoint becomes healthy", func(t *testing.T) {
		var attempts atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Become healthy after 3 attempts
			if attempts.Add(1) >= 3 {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		}))
		defer server.Close()

		checker := NewHealthChecker(server.URL)
		err := checker.WaitUntilHealthy(5 * time.Second)

		if err != nil {
			t.Errorf("WaitUntilHealthy() error = %v", err)
		}
		if attempts.Load() < 3 {
			t.Errorf("Expected at least 3 attempts, got %d", attempts.Load())
		}
	})

	t.Run("times out for persistently unhealthy endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		checker := NewHealthChecker(server.URL)
		start := time.Now()

		err := checker.WaitUntilHealthy(500 * time.Millisecond)
		duration := time.Since(start)

		if !errors.Is(err, ErrHealthCheckTimeout) {
			t.Errorf("WaitUntilHealthy() error = %v, want ErrHealthCheckTimeout", err)
		}
		if duration < 500*time.Millisecond {
			t.Errorf("WaitUntilHealthy() returned too early: %v", duration)
		}
	})

	t.Run("uses exponential backoff", func(t *testing.T) {
		var requestTimes []time.Time

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestTimes = append(requestTimes, time.Now())
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		checker := NewHealthChecker(server.URL).WithBackoff(50*time.Millisecond, 200*time.Millisecond, 2.0)
		checker.WaitUntilHealthy(1 * time.Second)

		// Verify backoff intervals increase
		if len(requestTimes) < 3 {
			t.Fatalf("Expected at least 3 requests, got %d", len(requestTimes))
		}

		// Check first interval (should be ~50ms)
		interval1 := requestTimes[1].Sub(requestTimes[0])
		if interval1 < 40*time.Millisecond || interval1 > 100*time.Millisecond {
			t.Errorf("First interval = %v, want ~50ms", interval1)
		}

		// Check second interval (should be ~100ms due to 2x multiplier)
		interval2 := requestTimes[2].Sub(requestTimes[1])
		if interval2 < 80*time.Millisecond || interval2 > 150*time.Millisecond {
			t.Errorf("Second interval = %v, want ~100ms", interval2)
		}

		// Third interval should be capped at maxInterval (200ms)
		if len(requestTimes) >= 4 {
			interval3 := requestTimes[3].Sub(requestTimes[2])
			if interval3 < 150*time.Millisecond || interval3 > 250*time.Millisecond {
				t.Errorf("Third interval = %v, want ~200ms (capped)", interval3)
			}
		}
	})
}

func TestHealthChecker_WaitUntilHealthyWithCallback(t *testing.T) {
	t.Run("calls callback for each attempt", func(t *testing.T) {
		var attempts atomic.Int32
		var callbackCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if attempts.Add(1) >= 3 {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		}))
		defer server.Close()

		checker := NewHealthChecker(server.URL)
		err := checker.WaitUntilHealthyWithCallback(5*time.Second, func(result *HealthCheckResult, attempt int) {
			callbackCount.Add(1)
			if attempt != int(callbackCount.Load()) {
				t.Errorf("Callback attempt = %d, want %d", attempt, callbackCount.Load())
			}
		})

		if err != nil {
			t.Errorf("WaitUntilHealthyWithCallback() error = %v", err)
		}
		if callbackCount.Load() != attempts.Load() {
			t.Errorf("Callback count = %d, want %d", callbackCount.Load(), attempts.Load())
		}
	})

	t.Run("callback receives result information", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		checker := NewHealthChecker(server.URL)
		var receivedResult *HealthCheckResult

		err := checker.WaitUntilHealthyWithCallback(5*time.Second, func(result *HealthCheckResult, attempt int) {
			receivedResult = result
		})

		if err != nil {
			t.Errorf("WaitUntilHealthyWithCallback() error = %v", err)
		}
		if receivedResult == nil {
			t.Fatal("Callback was not called")
		}
		if !receivedResult.Success {
			t.Error("Callback received unsuccessful result")
		}
		if receivedResult.StatusCode != http.StatusOK {
			t.Errorf("Callback received status = %d, want %d", receivedResult.StatusCode, http.StatusOK)
		}
	})
}

func TestHealthChecker_CustomBackoff(t *testing.T) {
	t.Run("respects custom backoff parameters", func(t *testing.T) {
		var requestTimes []time.Time

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestTimes = append(requestTimes, time.Now())
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		// Custom: 100ms initial, 3x multiplier, 500ms max
		checker := NewHealthChecker(server.URL).WithBackoff(100*time.Millisecond, 500*time.Millisecond, 3.0)
		checker.WaitUntilHealthy(1500 * time.Millisecond)

		if len(requestTimes) < 2 {
			t.Fatalf("Expected at least 2 requests, got %d", len(requestTimes))
		}

		// First interval should be ~100ms
		interval1 := requestTimes[1].Sub(requestTimes[0])
		if interval1 < 80*time.Millisecond || interval1 > 150*time.Millisecond {
			t.Errorf("First interval = %v, want ~100ms", interval1)
		}
	})
}

func TestHealthChecker_WithHTTPClient(t *testing.T) {
	t.Run("uses custom HTTP client", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Custom client with short timeout
		client := &http.Client{
			Timeout: 50 * time.Millisecond,
		}

		checker := NewHealthChecker(server.URL).WithHTTPClient(client)
		result := checker.Check(context.Background())

		// Should timeout due to short client timeout
		if result.Success {
			t.Error("Check() success = true, want false (should timeout)")
		}
		if result.Error == nil {
			t.Error("Check() error = nil, want timeout error")
		}
	})
}
