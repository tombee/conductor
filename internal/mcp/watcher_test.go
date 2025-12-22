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

package mcp

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_Watch(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.py")

	// Create test file
	if err := os.WriteFile(testFile, []byte("# test"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create mock manager
	manager := NewManager(ManagerConfig{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	defer manager.Close()

	// Create watcher
	watcher, err := NewWatcher(WatcherConfig{
		Manager:       manager,
		DebounceDelay: 50 * time.Millisecond,
		Logger:        slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Watch the file
	if err := watcher.Watch("test-server", []string{testFile}); err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	// Verify the file is being watched
	watcher.mu.RLock()
	paths, exists := watcher.watchedServers["test-server"]
	watcher.mu.RUnlock()

	if !exists {
		t.Fatal("server not found in watched servers")
	}

	if len(paths) != 1 || paths[0] != testFile {
		t.Errorf("expected paths [%s], got %v", testFile, paths)
	}
}

func TestWatcher_Unwatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.py")

	if err := os.WriteFile(testFile, []byte("# test"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	manager := NewManager(ManagerConfig{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	defer manager.Close()

	watcher, err := NewWatcher(WatcherConfig{
		Manager: manager,
		Logger:  slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Watch then unwatch
	if err := watcher.Watch("test-server", []string{testFile}); err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	if err := watcher.Unwatch("test-server"); err != nil {
		t.Fatalf("Unwatch failed: %v", err)
	}

	// Verify the server is no longer watched
	watcher.mu.RLock()
	_, exists := watcher.watchedServers["test-server"]
	watcher.mu.RUnlock()

	if exists {
		t.Error("server should not be in watched servers after unwatch")
	}
}

func TestWatcher_FileChange(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.py")

	if err := os.WriteFile(testFile, []byte("# test"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create manager with a mock server
	manager := NewManager(ManagerConfig{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	defer manager.Close()

	// Create watcher with short debounce
	watcher, err := NewWatcher(WatcherConfig{
		Manager:       manager,
		DebounceDelay: 50 * time.Millisecond,
		Logger:        slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Watch("test-server", []string{testFile}); err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	// Modify the file
	if err := os.WriteFile(testFile, []byte("# modified"), 0600); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Wait for debounce (50ms) + processing time.
	// This sleep is intentional - we need to wait for the debounce timer to fire.
	// Without a callback mechanism in the watcher, we can't observe the restart.
	time.Sleep(200 * time.Millisecond)

	// Smoke test: passes if no crashes occur during file change handling
}

func TestWatcher_MultipleWatches(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.py")
	file2 := filepath.Join(tmpDir, "file2.py")

	for _, file := range []string{file1, file2} {
		if err := os.WriteFile(file, []byte("# test"), 0600); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	manager := NewManager(ManagerConfig{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	defer manager.Close()

	watcher, err := NewWatcher(WatcherConfig{
		Manager: manager,
		Logger:  slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	// Watch both files
	if err := watcher.Watch("test-server", []string{file1, file2}); err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	watcher.mu.RLock()
	paths := watcher.watchedServers["test-server"]
	watcher.mu.RUnlock()

	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
}

func TestWatcher_DebounceMultipleChanges(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.py")

	if err := os.WriteFile(testFile, []byte("# test"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	manager := NewManager(ManagerConfig{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	defer manager.Close()

	watcher, err := NewWatcher(WatcherConfig{
		Manager:       manager,
		DebounceDelay: 100 * time.Millisecond,
		Logger:        slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Watch("test-server", []string{testFile}); err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	// Make multiple rapid changes (20ms simulates realistic editor save timing)
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(testFile, []byte("# modified"), 0600); err != nil {
			t.Fatalf("failed to modify test file: %v", err)
		}
		time.Sleep(20 * time.Millisecond) // Intentional: simulate rapid file saves
	}

	// Wait for debounce (100ms) + processing time.
	// This sleep is intentional - we need to wait for the debounce timer to fire.
	time.Sleep(200 * time.Millisecond)

	// Smoke test: should only trigger one restart due to debouncing
	// Passes if no crashes occur
}
