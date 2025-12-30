package polltrigger

import (
	"context"
	"testing"
	"time"
)

func TestStateManager_CreateAndGet(t *testing.T) {
	ctx := context.Background()

	// Use in-memory database for testing
	sm, err := NewStateManager(StateConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer sm.Close()

	// Test creating a new state
	state := &PollState{
		TriggerID:     "test-trigger-1",
		WorkflowPath:  "/workflows/test.yaml",
		Integration:   "pagerduty",
		LastPollTime:  time.Now().Add(-1 * time.Hour),
		HighWaterMark: time.Now().Add(-30 * time.Minute),
		SeenEvents: map[string]int64{
			"event1": time.Now().Add(-1 * time.Hour).Unix(),
			"event2": time.Now().Add(-30 * time.Minute).Unix(),
		},
		ErrorCount: 0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Save the state
	if err := sm.SaveState(ctx, state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Retrieve the state
	retrieved, err := sm.GetState(ctx, "test-trigger-1")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected state to be retrieved, got nil")
	}

	// Verify fields
	if retrieved.TriggerID != state.TriggerID {
		t.Errorf("TriggerID mismatch: got %s, want %s", retrieved.TriggerID, state.TriggerID)
	}
	if retrieved.WorkflowPath != state.WorkflowPath {
		t.Errorf("WorkflowPath mismatch: got %s, want %s", retrieved.WorkflowPath, state.WorkflowPath)
	}
	if retrieved.Integration != state.Integration {
		t.Errorf("Integration mismatch: got %s, want %s", retrieved.Integration, state.Integration)
	}
	if len(retrieved.SeenEvents) != len(state.SeenEvents) {
		t.Errorf("SeenEvents count mismatch: got %d, want %d", len(retrieved.SeenEvents), len(state.SeenEvents))
	}
}

func TestStateManager_Update(t *testing.T) {
	ctx := context.Background()

	sm, err := NewStateManager(StateConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer sm.Close()

	// Create initial state
	state := &PollState{
		TriggerID:    "test-trigger-2",
		WorkflowPath: "/workflows/test.yaml",
		Integration:  "slack",
		ErrorCount:   0,
		SeenEvents:   make(map[string]int64),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := sm.SaveState(ctx, state); err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}

	// Update the state
	state.ErrorCount = 5
	state.LastError = "connection timeout"
	state.SeenEvents["event3"] = time.Now().Unix()

	if err := sm.SaveState(ctx, state); err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}

	// Retrieve and verify
	retrieved, err := sm.GetState(ctx, "test-trigger-2")
	if err != nil {
		t.Fatalf("Failed to get updated state: %v", err)
	}

	if retrieved.ErrorCount != 5 {
		t.Errorf("ErrorCount mismatch: got %d, want 5", retrieved.ErrorCount)
	}
	if retrieved.LastError != "connection timeout" {
		t.Errorf("LastError mismatch: got %s, want 'connection timeout'", retrieved.LastError)
	}
	if len(retrieved.SeenEvents) != 1 {
		t.Errorf("SeenEvents count mismatch: got %d, want 1", len(retrieved.SeenEvents))
	}
}

func TestStateManager_Delete(t *testing.T) {
	ctx := context.Background()

	sm, err := NewStateManager(StateConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer sm.Close()

	// Create a state
	state := &PollState{
		TriggerID:    "test-trigger-3",
		WorkflowPath: "/workflows/test.yaml",
		Integration:  "jira",
		SeenEvents:   make(map[string]int64),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := sm.SaveState(ctx, state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Delete the state
	if err := sm.DeleteState(ctx, "test-trigger-3"); err != nil {
		t.Fatalf("Failed to delete state: %v", err)
	}

	// Verify it's gone
	retrieved, err := sm.GetState(ctx, "test-trigger-3")
	if err != nil {
		t.Fatalf("Failed to get state after delete: %v", err)
	}
	if retrieved != nil {
		t.Error("Expected state to be deleted, but it still exists")
	}
}

func TestStateManager_PruneSeenEvents(t *testing.T) {
	now := time.Now().Unix()

	state := &PollState{
		TriggerID:  "test-trigger-4",
		SeenEvents: map[string]int64{
			"old1":   now - 90000, // >24h old
			"old2":   now - 86500, // >24h old
			"recent": now - 3600,  // 1h old
			"new":    now - 60,    // 1m old
		},
	}

	sm := &StateManager{}
	sm.PruneSeenEvents(state, 86400, 10000) // 24h TTL, 10k max

	if len(state.SeenEvents) != 2 {
		t.Errorf("Expected 2 events after pruning, got %d", len(state.SeenEvents))
	}

	if _, exists := state.SeenEvents["old1"]; exists {
		t.Error("old1 should have been pruned")
	}
	if _, exists := state.SeenEvents["old2"]; exists {
		t.Error("old2 should have been pruned")
	}
	if _, exists := state.SeenEvents["recent"]; !exists {
		t.Error("recent should still exist")
	}
	if _, exists := state.SeenEvents["new"]; !exists {
		t.Error("new should still exist")
	}
}

func TestStateManager_PruneSeenEvents_MaxCount(t *testing.T) {
	now := time.Now().Unix()

	// Create state with more than max events
	state := &PollState{
		TriggerID:  "test-trigger-5",
		SeenEvents: make(map[string]int64),
	}

	// Add 15 events (all recent, within TTL)
	for i := 0; i < 15; i++ {
		eventID := string(rune('a' + i))
		state.SeenEvents[eventID] = now - int64(i)*60 // Each 1 minute apart
	}

	sm := &StateManager{}
	sm.PruneSeenEvents(state, 86400, 10) // 24h TTL, max 10 events

	if len(state.SeenEvents) != 10 {
		t.Errorf("Expected 10 events after max count pruning, got %d", len(state.SeenEvents))
	}
}

func TestStateManager_GetState_NotExists(t *testing.T) {
	ctx := context.Background()

	sm, err := NewStateManager(StateConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer sm.Close()

	// Try to get a non-existent state
	state, err := sm.GetState(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetState returned error for non-existent state: %v", err)
	}
	if state != nil {
		t.Error("Expected nil state for non-existent trigger, got non-nil")
	}
}
