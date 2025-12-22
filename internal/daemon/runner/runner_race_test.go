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

package runner

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/daemon/backend/memory"
	"github.com/tombee/conductor/internal/daemon/checkpoint"
	"github.com/tombee/conductor/pkg/workflow"
)

// setupTestRunner creates a runner for testing.
func setupTestRunner(t *testing.T) *Runner {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "runner-race-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		time.Sleep(10 * time.Millisecond)
		os.RemoveAll(tmpDir)
	})

	be := memory.New()
	cm, err := checkpoint.NewManager(checkpoint.ManagerConfig{
		Dir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create checkpoint manager: %v", err)
	}

	r := New(Config{
		MaxParallel:    2,
		DefaultTimeout: 30 * time.Second,
	}, be, cm)

	// Set up a mock adapter for testing
	mockAdapter := &MockExecutionAdapter{
		ExecuteWorkflowFunc: func(ctx context.Context, def *workflow.Definition, inputs map[string]any, opts ExecutionOptions) (*ExecutionResult, error) {
			// Simulate quick execution with cancellation support
			select {
			case <-ctx.Done():
				return &ExecutionResult{
					Output:      make(map[string]any),
					Duration:    time.Millisecond,
					Steps:       []workflow.StepResult{},
					StepOutputs: make(map[string]any),
					FinalError:  ctx.Err(),
				}, ctx.Err()
			case <-time.After(10 * time.Millisecond):
				return &ExecutionResult{
					Output:      map[string]any{"response": "test"},
					Duration:    10 * time.Millisecond,
					Steps:       []workflow.StepResult{},
					StepOutputs: make(map[string]any),
				}, nil
			}
		},
	}
	r.SetAdapter(mockAdapter)

	return r
}

var testWorkflow = []byte(`name: race-test
version: "1.0"
description: Test workflow for race detection
inputs: []
steps:
  - id: step1
    name: Test Step
    type: llm
    prompt: "test"
outputs: []
`)

// TestConcurrentCancel tests concurrent Cancel() calls on the same run.
// Verifies that sync.Once prevents panic and state remains consistent.
func TestConcurrentCancel(t *testing.T) {
	r := setupTestRunner(t)

	// Submit a run
	run, err := r.Submit(context.Background(), SubmitRequest{
		WorkflowYAML: testWorkflow,
	})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	// Cancel from 10 goroutines concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Cancel(run.ID) // Ignore error - may already be cancelled
		}()
	}

	wg.Wait()

	// Wait a bit for execute to finish
	time.Sleep(50 * time.Millisecond)

	// Verify final state is consistent
	finalRun, err := r.Get(run.ID)
	if err != nil {
		t.Fatalf("Failed to get run: %v", err)
	}

	// Should be cancelled or completed (if execution finished before cancel)
	if finalRun.Status != RunStatusCancelled && finalRun.Status != RunStatusCompleted {
		t.Errorf("Expected status Cancelled or Completed, got %s", finalRun.Status)
	}

	// If cancelled, CompletedAt should be set
	if finalRun.Status == RunStatusCancelled && finalRun.CompletedAt == nil {
		t.Error("Status is Cancelled but CompletedAt is nil")
	}
}

// TestCancelBeforeExecute tests cancelling a run before execute() starts.
func TestCancelBeforeExecute(t *testing.T) {
	r := setupTestRunner(t)

	// Submit multiple runs to fill the semaphore
	runs := make([]*RunSnapshot, 0)
	for i := 0; i < 5; i++ {
		run, err := r.Submit(context.Background(), SubmitRequest{
			WorkflowYAML: testWorkflow,
		})
		if err != nil {
			t.Fatalf("Failed to submit run: %v", err)
		}
		runs = append(runs, run)
	}

	// Cancel immediately (some may still be pending)
	for _, run := range runs {
		_ = r.Cancel(run.ID)
	}

	// Wait for all to complete
	time.Sleep(200 * time.Millisecond)

	// Verify all runs ended in a valid state
	for _, run := range runs {
		finalRun, err := r.Get(run.ID)
		if err != nil {
			t.Errorf("Failed to get run %s: %v", run.ID, err)
			continue
		}

		if finalRun.Status != RunStatusCancelled && finalRun.Status != RunStatusCompleted {
			t.Errorf("Run %s has invalid final status: %s", run.ID, finalRun.Status)
		}
	}
}

// TestConcurrentGetDuringExecution tests concurrent Get() calls while execute() updates Progress.
func TestConcurrentGetDuringExecution(t *testing.T) {
	r := setupTestRunner(t)

	run, err := r.Submit(context.Background(), SubmitRequest{
		WorkflowYAML: testWorkflow,
	})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	// Start 100 concurrent Get() calls
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snapshot, err := r.Get(run.ID)
			if err != nil {
				t.Errorf("Get failed: %v", err)
				return
			}

			// Verify snapshot is consistent
			if snapshot.Progress != nil {
				if snapshot.Progress.Completed > snapshot.Progress.Total {
					t.Errorf("Invalid progress: completed=%d > total=%d",
						snapshot.Progress.Completed, snapshot.Progress.Total)
				}
			}

			// Verify status transitions are valid
			validStatuses := map[RunStatus]bool{
				RunStatusPending:   true,
				RunStatusRunning:   true,
				RunStatusCompleted: true,
				RunStatusFailed:    true,
				RunStatusCancelled: true,
			}
			if !validStatuses[snapshot.Status] {
				t.Errorf("Invalid status: %s", snapshot.Status)
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentListDuringLogAppends tests List() while logs are being appended.
func TestConcurrentListDuringLogAppends(t *testing.T) {
	r := setupTestRunner(t)

	// Submit a run
	_, err := r.Submit(context.Background(), SubmitRequest{
		WorkflowYAML: testWorkflow,
	})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	// Call List() repeatedly while execution is happening
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snapshots := r.List(ListFilter{})

			// Verify each snapshot
			for _, s := range snapshots {
				// Log count should never be negative
				if len(s.Logs) < 0 {
					t.Errorf("Invalid log count: %d", len(s.Logs))
				}
			}
		}()
	}

	wg.Wait()
}

// TestSnapshotImmutability tests that modifying a snapshot doesn't affect internal state.
func TestSnapshotImmutability(t *testing.T) {
	r := setupTestRunner(t)

	run, err := r.Submit(context.Background(), SubmitRequest{
		WorkflowYAML: testWorkflow,
	})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	// Wait a bit for some logs to be generated
	time.Sleep(50 * time.Millisecond)

	// Get first snapshot
	snapshot1, err := r.Get(run.ID)
	if err != nil {
		t.Fatalf("Failed to get run: %v", err)
	}

	originalLogCount := len(snapshot1.Logs)

	// Modify the snapshot's logs
	if len(snapshot1.Logs) > 0 {
		snapshot1.Logs[0].Message = "MODIFIED"
		snapshot1.Logs = append(snapshot1.Logs, LogEntry{
			Timestamp: time.Now(),
			Level:     "test",
			Message:   "APPENDED",
		})
	}

	// Get second snapshot
	snapshot2, err := r.Get(run.ID)
	if err != nil {
		t.Fatalf("Failed to get run: %v", err)
	}

	// Verify snapshot2 is not affected by modifications to snapshot1
	if len(snapshot1.Logs) > 0 && len(snapshot2.Logs) > 0 {
		if snapshot2.Logs[0].Message == "MODIFIED" {
			t.Error("Snapshot2 was affected by modifications to snapshot1 (aliasing detected)")
		}
	}

	// Verify log count didn't change (or only increased naturally)
	if len(snapshot2.Logs) < originalLogCount {
		t.Errorf("Log count decreased: was %d, now %d", originalLogCount, len(snapshot2.Logs))
	}
}

// TestConcurrentCancelAndGet tests Cancel() and Get() happening simultaneously.
func TestConcurrentCancelAndGet(t *testing.T) {
	r := setupTestRunner(t)

	run, err := r.Submit(context.Background(), SubmitRequest{
		WorkflowYAML: testWorkflow,
	})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	var wg sync.WaitGroup

	// Start cancelling
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Cancel(run.ID)
		}()
	}

	// Start getting
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Get(run.ID)
		}()
	}

	// Start listing
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.List(ListFilter{})
		}()
	}

	wg.Wait()

	// Verify final state is valid
	finalRun, err := r.Get(run.ID)
	if err != nil {
		t.Fatalf("Failed to get final run: %v", err)
	}

	if finalRun.Status == RunStatusCancelled && finalRun.CompletedAt == nil {
		t.Error("Cancelled run has no CompletedAt timestamp")
	}
}

// TestStatusTransitionConsistency verifies that Status, CompletedAt, and Error are always consistent.
func TestStatusTransitionConsistency(t *testing.T) {
	r := setupTestRunner(t)

	run, err := r.Submit(context.Background(), SubmitRequest{
		WorkflowYAML: testWorkflow,
	})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	// Poll status repeatedly
	for i := 0; i < 50; i++ {
		snapshot, err := r.Get(run.ID)
		if err != nil {
			t.Fatalf("Failed to get run: %v", err)
		}

		// Verify consistency
		switch snapshot.Status {
		case RunStatusPending:
			if snapshot.StartedAt != nil {
				t.Error("Pending run has StartedAt")
			}
			if snapshot.CompletedAt != nil {
				t.Error("Pending run has CompletedAt")
			}
		case RunStatusRunning:
			if snapshot.StartedAt == nil {
				t.Error("Running run has no StartedAt")
			}
			if snapshot.CompletedAt != nil {
				t.Error("Running run has CompletedAt")
			}
		case RunStatusCompleted, RunStatusFailed, RunStatusCancelled:
			if snapshot.CompletedAt == nil {
				t.Errorf("Terminal status %s has no CompletedAt", snapshot.Status)
			}
		}

		time.Sleep(5 * time.Millisecond)
	}
}

// TestSnapshotMapImmutability verifies that modifying Input/Output maps doesn't affect internal state.
func TestSnapshotMapImmutability(t *testing.T) {
	r := setupTestRunner(t)

	run, err := r.Submit(context.Background(), SubmitRequest{
		WorkflowYAML: testWorkflow,
		Inputs:       map[string]any{"original": "value", "key2": "data"},
	})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	// Get first snapshot
	snapshot1, err := r.Get(run.ID)
	if err != nil {
		t.Fatalf("Failed to get run: %v", err)
	}

	// Modify snapshot's Input map
	if snapshot1.Inputs != nil {
		snapshot1.Inputs["malicious"] = "injection"
		snapshot1.Inputs["original"] = "corrupted"
	}

	// Get second snapshot
	snapshot2, err := r.Get(run.ID)
	if err != nil {
		t.Fatalf("Failed to get run: %v", err)
	}

	// Verify snapshot2 is not affected by modifications to snapshot1
	if snapshot2.Inputs != nil {
		if _, exists := snapshot2.Inputs["malicious"]; exists {
			t.Error("Map mutation in snapshot1 affected snapshot2 (aliasing detected)")
		}
		if snapshot2.Inputs["original"] != "value" {
			t.Errorf("Map mutation in snapshot1 corrupted original value: got %v, want 'value'", snapshot2.Inputs["original"])
		}
	}
}
