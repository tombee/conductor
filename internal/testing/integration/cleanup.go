package integration

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

// CleanupFunc is a function that cleans up a test resource.
type CleanupFunc func() error

// CleanupManager tracks test resources and ensures proper cleanup.
// It automatically registers with t.Cleanup() and verifies all cleanups succeed.
type CleanupManager struct {
	t         *testing.T
	mu        sync.Mutex
	resources []cleanupEntry
}

type cleanupEntry struct {
	name    string
	cleanup CleanupFunc
}

// NewCleanupManager creates a new cleanup manager for a test.
// It automatically registers cleanup with t.Cleanup().
func NewCleanupManager(t *testing.T) *CleanupManager {
	t.Helper()

	cm := &CleanupManager{
		t:         t,
		resources: make([]cleanupEntry, 0),
	}

	// Register cleanup handler
	t.Cleanup(func() {
		cm.runAll()
	})

	return cm
}

// Add registers a cleanup function for a named resource.
// Cleanup functions run in reverse order (LIFO) to handle dependencies.
func (cm *CleanupManager) Add(name string, cleanup CleanupFunc) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.resources = append(cm.resources, cleanupEntry{
		name:    name,
		cleanup: cleanup,
	})
}

// runAll executes all cleanup functions in reverse order.
// Logs errors but continues cleanup to ensure all resources are released.
func (cm *CleanupManager) runAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Run in reverse order (LIFO)
	for i := len(cm.resources) - 1; i >= 0; i-- {
		entry := cm.resources[i]
		if err := entry.cleanup(); err != nil {
			cm.t.Errorf("Cleanup failed for %s: %v", entry.name, err)
		}
	}
}

// Count returns the number of registered cleanup functions.
func (cm *CleanupManager) Count() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.resources)
}

// CleanupFile creates a cleanup function for removing a file.
// Returns a function suitable for use with Add().
func CleanupFile(path string) CleanupFunc {
	return func() error {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove file %s: %w", path, err)
		}
		return nil
	}
}
