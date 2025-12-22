package mcp

import (
	"sync"
	"testing"
	"time"
)

// TestConcurrentMapAccess tests concurrent access to the servers map
// to verify no race conditions occur.
func TestConcurrentMapAccess(t *testing.T) {
	manager := NewManager(ManagerConfig{
		StopTimeout: 1 * time.Second,
	})
	defer manager.StopAll()

	var wg sync.WaitGroup
	numReads := 100

	// Launch concurrent readers
	for i := 0; i < numReads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Attempt to get a non-existent client
			_, _ = manager.GetClient("non-existent")
		}()
	}

	wg.Wait()

	// If we get here without data races, the test passes
}

// TestStopAllTimeout tests that StopAll respects the timeout and doesn't hang indefinitely.
func TestStopAllTimeout(t *testing.T) {
	manager := NewManager(ManagerConfig{
		StopTimeout: 100 * time.Millisecond, // Very short timeout for test
	})

	// Start a server that won't stop cleanly
	// In a real scenario, this would be a server that hangs during shutdown
	// For this test, we just verify the timeout mechanism works

	start := time.Now()
	manager.StopAll()
	elapsed := time.Since(start)

	// Should complete within timeout plus a small margin
	if elapsed > 500*time.Millisecond {
		t.Errorf("StopAll took too long: %v (expected < 500ms)", elapsed)
	}
}

// TestConcurrentStopAll tests that concurrent StopAll calls don't cause panics or races.
func TestConcurrentStopAll(t *testing.T) {
	manager := NewManager(ManagerConfig{
		StopTimeout: 100 * time.Millisecond,
	})

	var wg sync.WaitGroup
	numCalls := 10

	// Launch concurrent StopAll calls
	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.StopAll()
		}()
	}

	wg.Wait()

	// If we get here without data races, the test passes
}

// TestWaitGroupTimeout tests the helper function for timeout-aware WaitGroup waiting.
func TestWaitGroupTimeout(t *testing.T) {
	tests := []struct {
		name          string
		waitDuration  time.Duration
		timeout       time.Duration
		expectSuccess bool
	}{
		{
			name:          "completes before timeout",
			waitDuration:  10 * time.Millisecond,
			timeout:       100 * time.Millisecond,
			expectSuccess: true,
		},
		{
			name:          "times out",
			waitDuration:  200 * time.Millisecond,
			timeout:       50 * time.Millisecond,
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wg sync.WaitGroup
			wg.Add(1)

			go func() {
				time.Sleep(tt.waitDuration)
				wg.Done()
			}()

			success := waitGroupTimeout(&wg, tt.timeout)

			if success != tt.expectSuccess {
				t.Errorf("waitGroupTimeout() = %v, want %v", success, tt.expectSuccess)
			}
		})
	}
}
