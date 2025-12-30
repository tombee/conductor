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

package filewatcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestBasicFileCreation tests that the watcher detects file creation events.
func TestBasicFileCreation(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "filewatcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create watcher
	w, err := NewWatcher(tmpDir, []string{"created"})
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	// Start watcher
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Wait for event
	select {
	case event := <-w.Events():
		if event == nil {
			t.Fatal("received nil event")
		}
		if event.Event != "created" {
			t.Errorf("expected event type 'created', got %s", event.Event)
		}
		if event.Name != "test.txt" {
			t.Errorf("expected filename 'test.txt', got %s", event.Name)
		}
		if event.Ext != ".txt" {
			t.Errorf("expected extension '.txt', got %s", event.Ext)
		}
		t.Logf("Received event: %+v", event)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for file creation event")
	}
}

// TestEventFiltering tests that event type filtering works correctly.
func TestEventFiltering(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "filewatcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create watcher that only watches "modified" events
	w, err := NewWatcher(tmpDir, []string{"modified"})
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	// Start watcher
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Create a test file (should NOT trigger since we only watch "modified")
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Modify the file (should trigger)
	time.Sleep(100 * time.Millisecond) // Small delay to ensure file is created
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Wait for event
	select {
	case event := <-w.Events():
		if event == nil {
			t.Fatal("received nil event")
		}
		if event.Event != "modified" {
			t.Errorf("expected event type 'modified', got %s", event.Event)
		}
		t.Logf("Received event: %+v", event)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for file modification event")
	}
}
