package workflow

import (
	"sync"
	"testing"
)

// TestConcurrentFactorySet tests that concurrent calls to SetDefaultActionRegistryFactory
// are safe and only the first call succeeds (sync.Once behavior).
func TestConcurrentFactorySet(t *testing.T) {
	// Reset the factory for this test
	// Note: In production, the factory is set once during init() and never reset.
	// This test verifies the sync.Once protection works correctly.

	callCount := 0
	var mu sync.Mutex

	mockFactory := func(workflowDir string) (OperationRegistry, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil, nil
	}

	// Launch multiple goroutines trying to set the factory
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SetDefaultActionRegistryFactory(mockFactory)
		}()
	}

	wg.Wait()

	// The factory should only be set once due to sync.Once
	// We can't easily verify which factory was set, but we can verify
	// that the function completes without data races
}

// TestConcurrentWithWorkflowDir tests that concurrent calls to WithWorkflowDir
// are safe and don't cause data races when accessing the factory.
func TestConcurrentWithWorkflowDir(t *testing.T) {
	// Set up a test factory
	factoryCalls := 0
	var factoryMu sync.Mutex

	testFactory := func(workflowDir string) (OperationRegistry, error) {
		factoryMu.Lock()
		factoryCalls++
		factoryMu.Unlock()
		return &mockOperationRegistry{}, nil
	}

	// Set the factory (only first call will succeed due to sync.Once)
	SetDefaultActionRegistryFactory(testFactory)

	// Launch multiple goroutines calling WithWorkflowDir
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			executor := NewExecutor(nil, nil)
			executor.WithWorkflowDir("/tmp/test")
		}()
	}

	wg.Wait()

	// If we get here without data races, the test passes
	// The race detector will catch any issues
}

// TestStressConcurrentWorkflowExecution tests 1000 concurrent workflow executions
// to verify no race conditions occur under stress.
func TestStressConcurrentWorkflowExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Set up a factory if not already set
	testFactory := func(workflowDir string) (OperationRegistry, error) {
		return &mockOperationRegistry{}, nil
	}
	SetDefaultActionRegistryFactory(testFactory)

	var wg sync.WaitGroup
	numWorkflows := 1000

	for i := 0; i < numWorkflows; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			executor := NewExecutor(nil, nil)
			executor.WithWorkflowDir("/tmp/test")

			// Simulate some work by setting parallel concurrency
			_ = executor.WithParallelConcurrency(5)
		}(i)
	}

	wg.Wait()

	// If we get here without data races, the test passes
}
