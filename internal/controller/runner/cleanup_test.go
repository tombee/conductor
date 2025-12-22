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
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

func TestStateManager_DeleteRun(t *testing.T) {
	sm := NewStateManager(nil)

	// Create a test run
	ctx := context.Background()
	def := &workflow.Definition{Name: "test"}
	run, err := sm.CreateRun(ctx, def, nil, "", "", "", nil, nil)
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Verify run exists
	if sm.RunCount() != 1 {
		t.Errorf("expected 1 run, got %d", sm.RunCount())
	}

	// Delete the run
	sm.DeleteRun(run.ID)

	// Verify run is gone
	if sm.RunCount() != 0 {
		t.Errorf("expected 0 runs after delete, got %d", sm.RunCount())
	}

	// Verify GetRun returns error
	_, err = sm.GetRun(run.ID)
	if err == nil {
		t.Error("expected error getting deleted run")
	}
}

func TestStateManager_RunCount(t *testing.T) {
	sm := NewStateManager(nil)

	if sm.RunCount() != 0 {
		t.Errorf("expected 0 runs initially, got %d", sm.RunCount())
	}

	ctx := context.Background()
	def := &workflow.Definition{Name: "test"}

	// Create 3 runs
	for i := 0; i < 3; i++ {
		_, err := sm.CreateRun(ctx, def, nil, "", "", "", nil, nil)
		if err != nil {
			t.Fatalf("failed to create run: %v", err)
		}
	}

	if sm.RunCount() != 3 {
		t.Errorf("expected 3 runs, got %d", sm.RunCount())
	}
}

func TestStateManager_CleanupCompletedRuns(t *testing.T) {
	sm := NewStateManager(nil)
	ctx := context.Background()
	def := &workflow.Definition{Name: "test"}

	// Create some completed runs with different ages
	oldRun, err := sm.CreateRun(ctx, def, nil, "", "", "", nil, nil)
	if err != nil {
		t.Fatalf("failed to create old run: %v", err)
	}
	oldRun.Status = RunStatusCompleted
	oldTime := time.Now().Add(-25 * time.Hour)
	oldRun.mu.Lock()
	oldRun.CompletedAt = &oldTime
	oldRun.mu.Unlock()

	recentRun, err := sm.CreateRun(ctx, def, nil, "", "", "", nil, nil)
	if err != nil {
		t.Fatalf("failed to create recent run: %v", err)
	}
	recentRun.Status = RunStatusCompleted
	recentTime := time.Now().Add(-1 * time.Hour)
	recentRun.mu.Lock()
	recentRun.CompletedAt = &recentTime
	recentRun.mu.Unlock()

	// Create an active run (should not be deleted)
	activeRun, err := sm.CreateRun(ctx, def, nil, "", "", "", nil, nil)
	if err != nil {
		t.Fatalf("failed to create active run: %v", err)
	}
	activeRun.Status = RunStatusRunning

	// Verify we have 3 runs
	if sm.RunCount() != 3 {
		t.Errorf("expected 3 runs, got %d", sm.RunCount())
	}

	// Cleanup with 24h retention
	deleted := sm.CleanupCompletedRuns(24 * time.Hour)

	// Verify only old completed run was deleted
	if deleted != 1 {
		t.Errorf("expected 1 run deleted, got %d", deleted)
	}

	if sm.RunCount() != 2 {
		t.Errorf("expected 2 runs remaining, got %d", sm.RunCount())
	}

	// Verify old run is gone
	_, err = sm.GetRun(oldRun.ID)
	if err == nil {
		t.Error("expected old run to be deleted")
	}

	// Verify recent run still exists
	_, err = sm.GetRun(recentRun.ID)
	if err != nil {
		t.Error("expected recent run to still exist")
	}

	// Verify active run still exists
	_, err = sm.GetRun(activeRun.ID)
	if err != nil {
		t.Error("expected active run to still exist")
	}
}

func TestStateManager_CleanupCompletedRuns_OnlyCompletedStatuses(t *testing.T) {
	sm := NewStateManager(nil)
	ctx := context.Background()
	def := &workflow.Definition{Name: "test"}

	oldTime := time.Now().Add(-25 * time.Hour)

	// Create runs with different statuses, all old
	statuses := []RunStatus{
		RunStatusPending,
		RunStatusRunning,
		RunStatusCompleted,
		RunStatusFailed,
		RunStatusCancelled,
	}

	expectedDeleted := 0
	for _, status := range statuses {
		run, err := sm.CreateRun(ctx, def, nil, "", "", "", nil, nil)
		if err != nil {
			t.Fatalf("failed to create run: %v", err)
		}
		run.Status = status
		run.mu.Lock()
		run.CompletedAt = &oldTime
		run.mu.Unlock()

		// Only succeeded, failed, and cancelled should be deleted
		if status == RunStatusCompleted || status == RunStatusFailed || status == RunStatusCancelled {
			expectedDeleted++
		}
	}

	// Cleanup with 24h retention
	deleted := sm.CleanupCompletedRuns(24 * time.Hour)

	if deleted != expectedDeleted {
		t.Errorf("expected %d runs deleted, got %d", expectedDeleted, deleted)
	}

	// Should have 2 runs left (pending and running)
	if sm.RunCount() != 2 {
		t.Errorf("expected 2 runs remaining (pending and running), got %d", sm.RunCount())
	}
}

func TestStateManager_CleanupCompletedRuns_NoCompletedAt(t *testing.T) {
	sm := NewStateManager(nil)
	ctx := context.Background()
	def := &workflow.Definition{Name: "test"}

	// Create a completed run without CompletedAt (edge case)
	run, err := sm.CreateRun(ctx, def, nil, "", "", "", nil, nil)
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}
	run.Status = RunStatusCompleted
	// Don't set CompletedAt

	// Cleanup should not delete it (no CompletedAt means we can't determine age)
	deleted := sm.CleanupCompletedRuns(24 * time.Hour)

	if deleted != 0 {
		t.Errorf("expected 0 runs deleted, got %d", deleted)
	}

	if sm.RunCount() != 1 {
		t.Errorf("expected 1 run remaining, got %d", sm.RunCount())
	}
}

func TestStateManager_StartCleanupLoop(t *testing.T) {
	// This is a basic integration test for the cleanup loop
	// We use a short interval for testing
	sm := NewStateManager(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create an old run
	def := &workflow.Definition{Name: "test"}
	run, err := sm.CreateRun(ctx, def, nil, "", "", "", nil, nil)
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}
	run.Status = RunStatusCompleted
	oldTime := time.Now().Add(-25 * time.Hour)
	run.mu.Lock()
	run.CompletedAt = &oldTime
	run.mu.Unlock()

	// Note: We don't actually test the ticker in this test because it would take 60 minutes
	// Instead, we just verify the loop can be started and stopped cleanly
	go sm.StartCleanupLoop(ctx, 24*time.Hour, logger)

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel the context
	cancel()

	// Give it a moment to stop
	time.Sleep(10 * time.Millisecond)

	// The test passes if we get here without hanging
}
