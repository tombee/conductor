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

//go:build integration

package debug

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/tracing/storage"
)

// TestSSEDebugFlow_MultiClientObserver tests multiple clients connecting to a debug session
func TestSSEDebugFlow_MultiClientObserver(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create temporary SQLite database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "debug_test.db")

	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Initialize session manager
	manager := NewSessionManager(SessionManagerConfig{
		Store:          store,
		SessionTimeout: 5 * time.Minute,
		MaxEventBuffer: 10,
		MaxObservers:   5,
	})

	// Create a debug session
	session, err := manager.CreateSession(ctx, "run-001", []string{"step1", "step2"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Add session owner
	if err := manager.AddObserver(session.SessionID, "owner-client", true); err != nil {
		t.Fatalf("failed to add owner: %v", err)
	}

	// Add multiple observers
	observers := []string{"observer-1", "observer-2", "observer-3"}
	for _, obsID := range observers {
		if err := manager.AddObserver(session.SessionID, obsID, false); err != nil {
			t.Fatalf("failed to add observer %s: %v", obsID, err)
		}
	}

	// Verify observer count
	count, err := manager.GetObserverCount(session.SessionID)
	if err != nil {
		t.Fatalf("failed to get observer count: %v", err)
	}

	expectedCount := 4 // 1 owner + 3 observers
	if count != expectedCount {
		t.Errorf("expected %d observers, got %d", expectedCount, count)
	}

	// Verify observer permissions
	isObserver, isOwner := manager.IsObserver(session.SessionID, "owner-client")
	if !isObserver || !isOwner {
		t.Error("owner should be both observer and owner")
	}

	isObserver, isOwner = manager.IsObserver(session.SessionID, "observer-1")
	if !isObserver || isOwner {
		t.Error("observer should be observer but not owner")
	}

	// Test observer limit
	for i := 0; i < 10; i++ {
		err := manager.AddObserver(session.SessionID, "overflow-observer", false)
		if err != nil {
			// Should hit max observers limit
			if i < 1 { // We can add 1 more (5 max, currently have 4)
				t.Errorf("unexpected error before hitting limit: %v", err)
			}
			break
		}
		if i >= 1 {
			t.Error("should have hit max observers limit")
			break
		}
	}

	// Remove an observer
	if err := manager.RemoveObserver(session.SessionID, "observer-2"); err != nil {
		t.Fatalf("failed to remove observer: %v", err)
	}

	// Verify count decreased
	count, err = manager.GetObserverCount(session.SessionID)
	if err != nil {
		t.Fatalf("failed to get observer count after removal: %v", err)
	}

	if count != expectedCount-1 {
		t.Errorf("expected %d observers after removal, got %d", expectedCount-1, count)
	}
}

// TestSSEDebugFlow_Reconnection tests session persistence and reconnection
func TestSSEDebugFlow_Reconnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "reconnect_test.db")

	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	defer store.Close()

	manager := NewSessionManager(SessionManagerConfig{
		Store:          store,
		SessionTimeout: 5 * time.Minute,
		MaxEventBuffer: 10,
		MaxObservers:   5,
	})

	// Create session
	session, err := manager.CreateSession(ctx, "run-reconnect", []string{"step1"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	originalSessionID := session.SessionID

	// Update session state
	if err := manager.UpdateSessionState(ctx, session.SessionID, SessionStateRunning); err != nil {
		t.Fatalf("failed to update session state: %v", err)
	}

	if err := manager.UpdateCurrentStep(ctx, session.SessionID, "step1"); err != nil {
		t.Fatalf("failed to update current step: %v", err)
	}

	// Add some events to the buffer
	for i := 0; i < 5; i++ {
		event := Event{
			Type:      "test_event",
			Timestamp: time.Now(),
		}
		if err := manager.AddEvent(session.SessionID, event); err != nil {
			t.Fatalf("failed to add event: %v", err)
		}
	}

	// Simulate disconnect - clear in-memory cache
	manager.sessions = make(map[string]*DebugSession)

	// Reconnect - load from database
	reconnected, err := manager.GetSession(ctx, originalSessionID)
	if err != nil {
		t.Fatalf("failed to reconnect to session: %v", err)
	}

	// Verify session state was persisted
	if reconnected.State != SessionStateRunning {
		t.Errorf("expected state %s, got %s", SessionStateRunning, reconnected.State)
	}

	if reconnected.CurrentStepID != "step1" {
		t.Errorf("expected current step 'step1', got %q", reconnected.CurrentStepID)
	}

	// Verify event buffer was persisted
	if len(reconnected.EventBuffer) != 5 {
		t.Errorf("expected 5 events in buffer, got %d", len(reconnected.EventBuffer))
	}
}

// TestSSEDebugFlow_SessionTimeout tests session timeout handling
func TestSSEDebugFlow_SessionTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "timeout_test.db")

	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Very short timeout for testing
	manager := NewSessionManager(SessionManagerConfig{
		Store:          store,
		SessionTimeout: 100 * time.Millisecond,
		MaxEventBuffer: 10,
		MaxObservers:   5,
	})

	// Create session
	session, err := manager.CreateSession(ctx, "run-timeout", []string{})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Wait for timeout
	time.Sleep(200 * time.Millisecond)

	// Run cleanup
	count, err := manager.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 session cleaned up, got %d", count)
	}

	// Verify session is marked as timed out
	timedOut, err := manager.GetSession(ctx, session.SessionID)
	if err != nil {
		// Session might be deleted from memory, try loading from DB
		timedOut, err = manager.loadSession(ctx, session.SessionID)
		if err != nil {
			t.Fatalf("failed to load timed out session: %v", err)
		}
	}

	if timedOut.State != SessionStateTimeout {
		t.Errorf("expected state %s, got %s", SessionStateTimeout, timedOut.State)
	}
}

// TestSSEDebugFlow_SessionCleanup tests cleanup of old completed sessions
func TestSSEDebugFlow_SessionCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cleanup_test.db")

	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	defer store.Close()

	manager := NewSessionManager(SessionManagerConfig{
		Store:          store,
		SessionTimeout: 1 * time.Hour,
		MaxEventBuffer: 10,
		MaxObservers:   5,
	})

	// Create sessions with different states and ages
	sessions := []struct {
		runID     string
		state     SessionState
		createdAt time.Time
	}{
		{"run-old-completed", SessionStateCompleted, time.Now().Add(-25 * time.Hour)},
		{"run-old-failed", SessionStateFailed, time.Now().Add(-26 * time.Hour)},
		{"run-old-killed", SessionStateKilled, time.Now().Add(-30 * time.Hour)},
		{"run-recent-completed", SessionStateCompleted, time.Now().Add(-1 * time.Hour)},
		{"run-active", SessionStateRunning, time.Now()},
	}

	for _, s := range sessions {
		session, err := manager.CreateSession(ctx, s.runID, []string{})
		if err != nil {
			t.Fatalf("failed to create session for %s: %v", s.runID, err)
		}

		// Update state
		if err := manager.UpdateSessionState(ctx, session.SessionID, s.state); err != nil {
			t.Fatalf("failed to update state for %s: %v", s.runID, err)
		}

		// Manually update created_at in database for old sessions
		if s.createdAt.Before(time.Now().Add(-1 * time.Hour)) {
			query := "UPDATE debug_sessions SET created_at = ? WHERE session_id = ?"
			if _, err := store.DB().ExecContext(ctx, query, s.createdAt.UnixNano(), session.SessionID); err != nil {
				t.Fatalf("failed to update created_at: %v", err)
			}
		}
	}

	// Run cleanup
	count, err := manager.CleanupCompletedSessions(ctx)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Should clean up 3 old sessions (completed, failed, killed from >24h ago)
	expectedCleaned := 3
	if count != expectedCleaned {
		t.Errorf("expected %d sessions cleaned, got %d", expectedCleaned, count)
	}

	// Verify the recent and active sessions are still present
	recentStillExists := false
	activeStillExists := false

	allSessions, err := manager.ListSessions(ctx)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}

	for _, session := range allSessions {
		if session.RunID == "run-recent-completed" {
			recentStillExists = true
		}
		if session.RunID == "run-active" {
			activeStillExists = true
		}
	}

	if !recentStillExists {
		t.Error("recent completed session should not be cleaned up")
	}
	if !activeStillExists {
		t.Error("active session should not be cleaned up")
	}
}

// TestSSEDebugFlow_EventBufferLimit tests event buffer size limiting
func TestSSEDebugFlow_EventBufferLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "buffer_test.db")

	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	defer store.Close()

	bufferSize := 5
	manager := NewSessionManager(SessionManagerConfig{
		Store:          store,
		SessionTimeout: 1 * time.Hour,
		MaxEventBuffer: bufferSize,
		MaxObservers:   5,
	})

	session, err := manager.CreateSession(ctx, "run-buffer-test", []string{})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Add more events than buffer size
	for i := 0; i < bufferSize*2; i++ {
		event := Event{
			Type:      "test_event",
			Timestamp: time.Now(),
			Data: map[string]any{
				"index": i,
			},
		}
		if err := manager.AddEvent(session.SessionID, event); err != nil {
			t.Fatalf("failed to add event %d: %v", i, err)
		}
	}

	// Retrieve event buffer
	buffer, err := manager.GetEventBuffer(session.SessionID)
	if err != nil {
		t.Fatalf("failed to get event buffer: %v", err)
	}

	// Verify buffer size is limited
	if len(buffer) != bufferSize {
		t.Errorf("expected buffer size %d, got %d", bufferSize, len(buffer))
	}

	// Verify we kept the most recent events (indices 5-9)
	firstEvent := buffer[0]
	if idx, ok := firstEvent.Data["index"].(int); ok {
		expectedFirstIndex := bufferSize // Event at index 5
		if idx != expectedFirstIndex {
			t.Errorf("expected first event index %d, got %d", expectedFirstIndex, idx)
		}
	} else {
		t.Error("first event missing index in data")
	}
}
