//go:build integration

package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/daemon/backend"
	"github.com/tombee/conductor/internal/testing/integration"
)

// TestMemoryRunLifecycle tests the full run lifecycle with memory backend.
func TestMemoryRunLifecycle(t *testing.T) {
	be := New()
	defer be.Close()

	ctx := context.Background()

	// Test CreateRun
	now := time.Now()
	run := &backend.Run{
		ID:            "test-run-1",
		WorkflowID:    "test-workflow",
		Workflow:      "name: test\nsteps:\n  - name: step1",
		Status:        "pending",
		CorrelationID: "corr-123",
		Inputs: map[string]any{
			"key": "value",
		},
		StartedAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := be.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Test GetRun
	retrieved, err := be.GetRun(ctx, "test-run-1")
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved.ID != run.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, run.ID)
	}
	if retrieved.WorkflowID != run.WorkflowID {
		t.Errorf("WorkflowID mismatch: got %s, want %s", retrieved.WorkflowID, run.WorkflowID)
	}
	if retrieved.Status != run.Status {
		t.Errorf("Status mismatch: got %s, want %s", retrieved.Status, run.Status)
	}

	// Test UpdateRun
	retrieved.Status = "running"
	retrieved.CurrentStep = "step1"
	retrieved.Completed = 1
	retrieved.Total = 3
	retrieved.UpdatedAt = time.Now()

	err = be.UpdateRun(ctx, retrieved)
	if err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}

	// Verify update persisted
	updated, err := be.GetRun(ctx, "test-run-1")
	if err != nil {
		t.Fatalf("GetRun after update failed: %v", err)
	}

	if updated.Status != "running" {
		t.Errorf("Status not updated: got %s, want running", updated.Status)
	}
	if updated.CurrentStep != "step1" {
		t.Errorf("CurrentStep not updated: got %s, want step1", updated.CurrentStep)
	}
	if updated.Completed != 1 {
		t.Errorf("Completed not updated: got %d, want 1", updated.Completed)
	}

	// Test ListRuns
	runs, err := be.ListRuns(ctx, backend.RunFilter{})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}

	if len(runs) != 1 {
		t.Errorf("Expected 1 run, got %d", len(runs))
	}

	// Test DeleteRun
	err = be.DeleteRun(ctx, "test-run-1")
	if err != nil {
		t.Fatalf("DeleteRun failed: %v", err)
	}

	// Verify deletion
	_, err = be.GetRun(ctx, "test-run-1")
	if err == nil {
		t.Error("Expected error getting deleted run, got nil")
	}
}

// TestMemoryCheckpointPersistence tests checkpoint storage and retrieval.
func TestMemoryCheckpointPersistence(t *testing.T) {
	be := New()
	defer be.Close()

	ctx := context.Background()

	// Create a run first
	now := time.Now()
	run := &backend.Run{
		ID:         "test-run-cp",
		WorkflowID: "test-workflow",
		Workflow:   "name: test",
		Status:     "running",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err := be.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Save checkpoint
	checkpoint := &backend.Checkpoint{
		RunID:     "test-run-cp",
		StepID:    "step-1",
		StepIndex: 0,
		Context: map[string]any{
			"variable": "value",
			"count":    42,
		},
		CreatedAt: time.Now(),
	}

	err = be.SaveCheckpoint(ctx, "test-run-cp", checkpoint)
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	// Retrieve checkpoint
	retrieved, err := be.GetCheckpoint(ctx, "test-run-cp")
	if err != nil {
		t.Fatalf("GetCheckpoint failed: %v", err)
	}

	if retrieved.RunID != checkpoint.RunID {
		t.Errorf("RunID mismatch: got %s, want %s", retrieved.RunID, checkpoint.RunID)
	}
	if retrieved.StepID != checkpoint.StepID {
		t.Errorf("StepID mismatch: got %s, want %s", retrieved.StepID, checkpoint.StepID)
	}
	if retrieved.StepIndex != checkpoint.StepIndex {
		t.Errorf("StepIndex mismatch: got %d, want %d", retrieved.StepIndex, checkpoint.StepIndex)
	}

	// Verify context data
	if val, ok := retrieved.Context["variable"].(string); !ok || val != "value" {
		t.Errorf("Context variable mismatch: got %v, want 'value'", retrieved.Context["variable"])
	}
}

// TestMemoryConcurrentAccess tests concurrent reads and writes.
func TestMemoryConcurrentAccess(t *testing.T) {
	be := New()
	defer be.Close()

	ctx := context.Background()
	cleanup := integration.NewCleanupManager(t)

	// Create multiple runs concurrently
	const numRuns = 10
	errChan := make(chan error, numRuns)

	for i := 0; i < numRuns; i++ {
		go func(index int) {
			runID := fmt.Sprintf("concurrent-run-%d", index)
			now := time.Now()

			run := &backend.Run{
				ID:         runID,
				WorkflowID: "test-workflow",
				Workflow:   "name: test",
				Status:     "pending",
				CreatedAt:  now,
				UpdatedAt:  now,
			}

			if createErr := be.CreateRun(ctx, run); createErr != nil {
				errChan <- createErr
				return
			}

			// Update the run
			run.Status = "running"
			if updateErr := be.UpdateRun(ctx, run); updateErr != nil {
				errChan <- updateErr
				return
			}

			errChan <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < numRuns; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}

	// Verify all runs were created
	runs, err := be.ListRuns(ctx, backend.RunFilter{})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}

	if len(runs) != numRuns {
		t.Errorf("Expected %d runs, got %d", numRuns, len(runs))
	}

	// Verify all runs are in "running" status
	for _, run := range runs {
		if run.Status != "running" {
			t.Errorf("Run %s has incorrect status: %s", run.ID, run.Status)
		}
	}

	// Cleanup
	for _, run := range runs {
		runID := run.ID
		cleanup.Add("run:"+runID, func() error {
			return be.DeleteRun(ctx, runID)
		})
	}
}

// TestMemoryRunFiltering tests run listing with filters.
func TestMemoryRunFiltering(t *testing.T) {
	be := New()
	defer be.Close()

	ctx := context.Background()
	now := time.Now()

	// Create runs with different statuses and workflows
	runs := []*backend.Run{
		{
			ID:         "run-pending-1",
			WorkflowID: "workflow-a",
			Workflow:   "workflow-a",
			Status:     "pending",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:         "run-running-1",
			WorkflowID: "workflow-a",
			Workflow:   "workflow-a",
			Status:     "running",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:         "run-completed-1",
			WorkflowID: "workflow-b",
			Workflow:   "workflow-b",
			Status:     "completed",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}

	for _, run := range runs {
		if err := be.CreateRun(ctx, run); err != nil {
			t.Fatalf("CreateRun failed: %v", err)
		}
	}

	// Test status filter
	filtered, err := be.ListRuns(ctx, backend.RunFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("ListRuns with status filter failed: %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("Expected 1 pending run, got %d", len(filtered))
	}

	// Test workflow filter
	filtered, err = be.ListRuns(ctx, backend.RunFilter{Workflow: "workflow-a"})
	if err != nil {
		t.Fatalf("ListRuns with workflow filter failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("Expected 2 runs for workflow-a, got %d", len(filtered))
	}

	// Test limit
	filtered, err = be.ListRuns(ctx, backend.RunFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListRuns with limit failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("Expected 2 runs with limit, got %d", len(filtered))
	}
}
