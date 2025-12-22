package mcp

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestConcurrentFileChangeEvents tests that concurrent file change events
// don't cause race conditions in the watcher.
func TestConcurrentFileChangeEvents(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.js")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(ManagerConfig{
		StopTimeout: 1 * time.Second,
	})
	defer manager.StopAll()

	watcher, err := NewWatcher(WatcherConfig{
		Manager:       manager,
		DebounceDelay: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	// Add a watch for a test server
	if err := watcher.Watch("test-server", []string{testFile}); err != nil {
		t.Fatal(err)
	}

	// Simulate concurrent file change events
	var wg sync.WaitGroup
	numEvents := 100

	for i := 0; i < numEvents; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			watcher.handleFileChange(testFile)
		}()
	}

	wg.Wait()

	// Give debounce timers time to fire
	time.Sleep(200 * time.Millisecond)

	// If we get here without data races, the test passes
}

// TestConcurrentScheduleRestart tests that concurrent calls to scheduleRestart
// are safe and properly handle timer cancellation.
func TestConcurrentScheduleRestart(t *testing.T) {
	manager := NewManager(ManagerConfig{
		StopTimeout: 1 * time.Second,
	})
	defer manager.StopAll()

	watcher, err := NewWatcher(WatcherConfig{
		Manager:       manager,
		DebounceDelay: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	var wg sync.WaitGroup
	numCalls := 50

	// Concurrent calls to scheduleRestart for the same server
	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			watcher.scheduleRestart("test-server")
		}()
	}

	wg.Wait()

	// Give debounce timers time to fire
	time.Sleep(200 * time.Millisecond)

	// If we get here without data races, the test passes
}

// TestDebounceUnderConcurrentChanges verifies that the debounce behavior
// works correctly when multiple file changes happen concurrently.
func TestDebounceUnderConcurrentChanges(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-debounce-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.js")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(ManagerConfig{
		StopTimeout: 1 * time.Second,
	})
	defer manager.StopAll()

	watcher, err := NewWatcher(WatcherConfig{
		Manager:       manager,
		DebounceDelay: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	if err := watcher.Watch("test-server", []string{testFile}); err != nil {
		t.Fatal(err)
	}

	// Rapidly trigger file changes
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Stagger the changes slightly
			time.Sleep(time.Duration(id) * 5 * time.Millisecond)
			watcher.handleFileChange(testFile)
		}(i)
	}

	wg.Wait()

	// Wait for debounce to complete
	time.Sleep(300 * time.Millisecond)

	// The test passes if we get here without races or panics
	// The debounce should have coalesced the changes
}

// TestConcurrentAddRemoveWatch tests concurrent watch addition and removal.
func TestConcurrentAddRemoveWatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-watch-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.js")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(ManagerConfig{
		StopTimeout: 1 * time.Second,
	})
	defer manager.StopAll()

	watcher, err := NewWatcher(WatcherConfig{
		Manager:       manager,
		DebounceDelay: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	var wg sync.WaitGroup
	numOperations := 20

	for i := 0; i < numOperations; i++ {
		wg.Add(2)

		// Add watch
		go func(id int) {
			defer wg.Done()
			serverName := "test-server"
			_ = watcher.Watch(serverName, []string{testFile})
		}(i)

		// Remove watch
		go func(id int) {
			defer wg.Done()
			_ = watcher.Unwatch("test-server")
		}(i)
	}

	wg.Wait()

	// If we get here without data races, the test passes
}
