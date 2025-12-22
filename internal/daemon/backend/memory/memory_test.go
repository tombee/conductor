// Package memory provides an in-memory backend implementation.
package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/daemon/backend"
)

func TestNew(t *testing.T) {
	b := New()
	if b == nil {
		t.Fatal("New() returned nil")
	}
	if b.runs == nil {
		t.Error("runs map not initialized")
	}
	if b.checkpoints == nil {
		t.Error("checkpoints map not initialized")
	}
	if b.schedules == nil {
		t.Error("schedules map not initialized")
	}
}

func TestBackend_CreateRun(t *testing.T) {
	b := New()
	ctx := context.Background()

	t.Run("successful create", func(t *testing.T) {
		run := &backend.Run{
			ID:         "run-1",
			WorkflowID: "workflow-1",
			Workflow:   "test-workflow",
			Status:     "pending",
		}

		err := b.CreateRun(ctx, run)
		if err != nil {
			t.Fatalf("CreateRun() error = %v", err)
		}

		// Verify timestamps are set
		if run.CreatedAt.IsZero() {
			t.Error("CreatedAt not set")
		}
		if run.UpdatedAt.IsZero() {
			t.Error("UpdatedAt not set")
		}
	})

	t.Run("duplicate run fails", func(t *testing.T) {
		run := &backend.Run{
			ID:       "run-1", // Same ID as above
			Workflow: "test",
		}

		err := b.CreateRun(ctx, run)
		if err == nil {
			t.Error("CreateRun() should fail for duplicate ID")
		}
	})
}

func TestBackend_GetRun(t *testing.T) {
	b := New()
	ctx := context.Background()

	// Create a run first
	run := &backend.Run{
		ID:         "run-get",
		WorkflowID: "workflow-1",
		Workflow:   "test-workflow",
		Status:     "pending",
		Inputs:     map[string]any{"key": "value"},
	}
	_ = b.CreateRun(ctx, run)

	t.Run("existing run", func(t *testing.T) {
		got, err := b.GetRun(ctx, "run-get")
		if err != nil {
			t.Fatalf("GetRun() error = %v", err)
		}
		if got.ID != "run-get" {
			t.Errorf("GetRun() ID = %v, want %v", got.ID, "run-get")
		}
		if got.Workflow != "test-workflow" {
			t.Errorf("GetRun() Workflow = %v, want %v", got.Workflow, "test-workflow")
		}
	})

	t.Run("non-existent run", func(t *testing.T) {
		_, err := b.GetRun(ctx, "non-existent")
		if err == nil {
			t.Error("GetRun() should fail for non-existent run")
		}
	})
}

func TestBackend_UpdateRun(t *testing.T) {
	b := New()
	ctx := context.Background()

	// Create a run first
	run := &backend.Run{
		ID:       "run-update",
		Workflow: "test-workflow",
		Status:   "pending",
	}
	_ = b.CreateRun(ctx, run)
	originalCreatedAt := run.CreatedAt

	t.Run("successful update", func(t *testing.T) {
		run.Status = "running"
		run.CurrentStep = "step-1"

		err := b.UpdateRun(ctx, run)
		if err != nil {
			t.Fatalf("UpdateRun() error = %v", err)
		}

		// Verify update
		got, _ := b.GetRun(ctx, "run-update")
		if got.Status != "running" {
			t.Errorf("Status = %v, want %v", got.Status, "running")
		}
		if got.CurrentStep != "step-1" {
			t.Errorf("CurrentStep = %v, want %v", got.CurrentStep, "step-1")
		}
		if got.UpdatedAt.Before(originalCreatedAt) || got.UpdatedAt.Equal(originalCreatedAt) {
			t.Error("UpdatedAt should be after CreatedAt")
		}
	})

	t.Run("update non-existent run fails", func(t *testing.T) {
		nonExistent := &backend.Run{
			ID:       "non-existent",
			Workflow: "test",
		}

		err := b.UpdateRun(ctx, nonExistent)
		if err == nil {
			t.Error("UpdateRun() should fail for non-existent run")
		}
	})
}

func TestBackend_DeleteRun(t *testing.T) {
	b := New()
	ctx := context.Background()

	// Create a run and checkpoint
	run := &backend.Run{
		ID:       "run-delete",
		Workflow: "test",
	}
	_ = b.CreateRun(ctx, run)
	_ = b.SaveCheckpoint(ctx, "run-delete", &backend.Checkpoint{StepID: "step-1"})

	t.Run("delete existing run", func(t *testing.T) {
		err := b.DeleteRun(ctx, "run-delete")
		if err != nil {
			t.Fatalf("DeleteRun() error = %v", err)
		}

		// Verify deletion
		_, err = b.GetRun(ctx, "run-delete")
		if err == nil {
			t.Error("Run should be deleted")
		}

		// Verify checkpoint also deleted
		_, err = b.GetCheckpoint(ctx, "run-delete")
		if err == nil {
			t.Error("Checkpoint should be deleted with run")
		}
	})

	t.Run("delete non-existent run is idempotent", func(t *testing.T) {
		err := b.DeleteRun(ctx, "non-existent")
		if err != nil {
			t.Errorf("DeleteRun() should be idempotent, got error = %v", err)
		}
	})
}

func TestBackend_ListRuns(t *testing.T) {
	b := New()
	ctx := context.Background()

	// Create multiple runs
	runs := []*backend.Run{
		{ID: "run-1", Workflow: "workflow-a", Status: "pending"},
		{ID: "run-2", Workflow: "workflow-a", Status: "running"},
		{ID: "run-3", Workflow: "workflow-b", Status: "completed"},
		{ID: "run-4", Workflow: "workflow-b", Status: "pending"},
		{ID: "run-5", Workflow: "workflow-a", Status: "failed"},
	}
	for _, r := range runs {
		_ = b.CreateRun(ctx, r)
	}

	t.Run("list all", func(t *testing.T) {
		got, err := b.ListRuns(ctx, backend.RunFilter{})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		if len(got) != 5 {
			t.Errorf("ListRuns() returned %d runs, want 5", len(got))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		got, err := b.ListRuns(ctx, backend.RunFilter{Status: "pending"})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("ListRuns(status=pending) returned %d runs, want 2", len(got))
		}
		for _, r := range got {
			if r.Status != "pending" {
				t.Errorf("Run %s has status %s, want pending", r.ID, r.Status)
			}
		}
	})

	t.Run("filter by workflow", func(t *testing.T) {
		got, err := b.ListRuns(ctx, backend.RunFilter{Workflow: "workflow-b"})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("ListRuns(workflow=workflow-b) returned %d runs, want 2", len(got))
		}
	})

	t.Run("filter by both", func(t *testing.T) {
		got, err := b.ListRuns(ctx, backend.RunFilter{Status: "pending", Workflow: "workflow-b"})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("ListRuns(status=pending, workflow=workflow-b) returned %d runs, want 1", len(got))
		}
	})

	t.Run("with limit", func(t *testing.T) {
		got, err := b.ListRuns(ctx, backend.RunFilter{Limit: 3})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		if len(got) != 3 {
			t.Errorf("ListRuns(limit=3) returned %d runs, want 3", len(got))
		}
	})

	t.Run("empty result", func(t *testing.T) {
		got, err := b.ListRuns(ctx, backend.RunFilter{Status: "cancelled"})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("ListRuns(status=cancelled) returned %d runs, want 0", len(got))
		}
	})
}

func TestBackend_Checkpoint(t *testing.T) {
	b := New()
	ctx := context.Background()

	// Create a run first
	run := &backend.Run{ID: "run-cp", Workflow: "test"}
	_ = b.CreateRun(ctx, run)

	t.Run("save and get checkpoint", func(t *testing.T) {
		checkpoint := &backend.Checkpoint{
			StepID:    "step-1",
			StepIndex: 2,
			Context:   map[string]any{"key": "value"},
		}

		err := b.SaveCheckpoint(ctx, "run-cp", checkpoint)
		if err != nil {
			t.Fatalf("SaveCheckpoint() error = %v", err)
		}

		// Verify RunID and CreatedAt are set
		if checkpoint.RunID != "run-cp" {
			t.Errorf("RunID = %v, want run-cp", checkpoint.RunID)
		}
		if checkpoint.CreatedAt.IsZero() {
			t.Error("CreatedAt not set")
		}

		// Get checkpoint
		got, err := b.GetCheckpoint(ctx, "run-cp")
		if err != nil {
			t.Fatalf("GetCheckpoint() error = %v", err)
		}
		if got.StepID != "step-1" {
			t.Errorf("StepID = %v, want step-1", got.StepID)
		}
		if got.StepIndex != 2 {
			t.Errorf("StepIndex = %v, want 2", got.StepIndex)
		}
	})

	t.Run("overwrite checkpoint", func(t *testing.T) {
		checkpoint := &backend.Checkpoint{
			StepID:    "step-2",
			StepIndex: 3,
		}

		err := b.SaveCheckpoint(ctx, "run-cp", checkpoint)
		if err != nil {
			t.Fatalf("SaveCheckpoint() error = %v", err)
		}

		got, _ := b.GetCheckpoint(ctx, "run-cp")
		if got.StepID != "step-2" {
			t.Errorf("StepID = %v, want step-2", got.StepID)
		}
	})

	t.Run("get non-existent checkpoint", func(t *testing.T) {
		_, err := b.GetCheckpoint(ctx, "non-existent")
		if err == nil {
			t.Error("GetCheckpoint() should fail for non-existent checkpoint")
		}
	})
}

func TestBackend_ScheduleState(t *testing.T) {
	b := New()
	ctx := context.Background()

	t.Run("save and get schedule state", func(t *testing.T) {
		now := time.Now()
		state := &backend.ScheduleState{
			Name:       "daily-backup",
			LastRun:    &now,
			RunCount:   10,
			ErrorCount: 1,
			Enabled:    true,
		}

		err := b.SaveScheduleState(ctx, state)
		if err != nil {
			t.Fatalf("SaveScheduleState() error = %v", err)
		}

		if state.UpdatedAt.IsZero() {
			t.Error("UpdatedAt not set")
		}

		got, err := b.GetScheduleState(ctx, "daily-backup")
		if err != nil {
			t.Fatalf("GetScheduleState() error = %v", err)
		}
		if got.RunCount != 10 {
			t.Errorf("RunCount = %v, want 10", got.RunCount)
		}
		if !got.Enabled {
			t.Error("Enabled should be true")
		}
	})

	t.Run("update schedule state", func(t *testing.T) {
		state := &backend.ScheduleState{
			Name:     "daily-backup",
			RunCount: 11,
			Enabled:  false,
		}

		err := b.SaveScheduleState(ctx, state)
		if err != nil {
			t.Fatalf("SaveScheduleState() error = %v", err)
		}

		got, _ := b.GetScheduleState(ctx, "daily-backup")
		if got.RunCount != 11 {
			t.Errorf("RunCount = %v, want 11", got.RunCount)
		}
		if got.Enabled {
			t.Error("Enabled should be false")
		}
	})

	t.Run("get non-existent state", func(t *testing.T) {
		_, err := b.GetScheduleState(ctx, "non-existent")
		if err == nil {
			t.Error("GetScheduleState() should fail for non-existent state")
		}
	})

	t.Run("list schedule states", func(t *testing.T) {
		// Add more states
		_ = b.SaveScheduleState(ctx, &backend.ScheduleState{Name: "weekly-report", Enabled: true})
		_ = b.SaveScheduleState(ctx, &backend.ScheduleState{Name: "hourly-check", Enabled: true})

		got, err := b.ListScheduleStates(ctx)
		if err != nil {
			t.Fatalf("ListScheduleStates() error = %v", err)
		}
		if len(got) != 3 {
			t.Errorf("ListScheduleStates() returned %d states, want 3", len(got))
		}
	})

	t.Run("delete schedule state", func(t *testing.T) {
		err := b.DeleteScheduleState(ctx, "hourly-check")
		if err != nil {
			t.Fatalf("DeleteScheduleState() error = %v", err)
		}

		_, err = b.GetScheduleState(ctx, "hourly-check")
		if err == nil {
			t.Error("State should be deleted")
		}

		// List should have 2 now
		got, _ := b.ListScheduleStates(ctx)
		if len(got) != 2 {
			t.Errorf("ListScheduleStates() returned %d states, want 2", len(got))
		}
	})
}

func TestBackend_Close(t *testing.T) {
	b := New()
	err := b.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestBackend_Concurrency(t *testing.T) {
	b := New()
	ctx := context.Background()

	t.Run("concurrent creates", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				run := &backend.Run{
					ID:       fmt.Sprintf("concurrent-run-%d", id),
					Workflow: "test",
					Status:   "pending",
				}
				if err := b.CreateRun(ctx, run); err != nil {
					errs <- err
				}
			}(i)
		}

		wg.Wait()
		close(errs)

		for err := range errs {
			t.Errorf("CreateRun() error = %v", err)
		}

		// Verify all runs created
		runs, _ := b.ListRuns(ctx, backend.RunFilter{})
		if len(runs) < 100 {
			t.Errorf("Expected at least 100 runs, got %d", len(runs))
		}
	})

	t.Run("concurrent reads and writes", func(t *testing.T) {
		// Create a run
		run := &backend.Run{ID: "rw-test", Workflow: "test", Status: "pending"}
		_ = b.CreateRun(ctx, run)

		var wg sync.WaitGroup

		// Concurrent reads
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = b.GetRun(ctx, "rw-test")
			}()
		}

		// Concurrent updates
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				r := &backend.Run{
					ID:          "rw-test",
					Workflow:    "test",
					Status:      "running",
					CurrentStep: fmt.Sprintf("step-%d", i),
				}
				_ = b.UpdateRun(ctx, r)
			}(i)
		}

		wg.Wait()

		// Verify run still exists and is valid
		got, err := b.GetRun(ctx, "rw-test")
		if err != nil {
			t.Fatalf("GetRun() error = %v", err)
		}
		if got.Status != "running" {
			t.Errorf("Status = %v, want running", got.Status)
		}
	})
}

func TestBackend_EdgeCases(t *testing.T) {
	b := New()
	ctx := context.Background()

	t.Run("run with unicode", func(t *testing.T) {
		run := &backend.Run{
			ID:       "unicode-run",
			Workflow: "workflow-тест-日本語",
			Status:   "pending",
			Inputs:   map[string]any{"message": "Hello 世界 Привет"},
		}

		err := b.CreateRun(ctx, run)
		if err != nil {
			t.Fatalf("CreateRun() error = %v", err)
		}

		got, _ := b.GetRun(ctx, "unicode-run")
		if got.Workflow != "workflow-тест-日本語" {
			t.Errorf("Workflow = %v, want workflow-тест-日本語", got.Workflow)
		}
	})

	t.Run("run with empty inputs", func(t *testing.T) {
		run := &backend.Run{
			ID:       "empty-inputs",
			Workflow: "test",
			Status:   "pending",
			Inputs:   nil,
		}

		err := b.CreateRun(ctx, run)
		if err != nil {
			t.Fatalf("CreateRun() error = %v", err)
		}

		got, _ := b.GetRun(ctx, "empty-inputs")
		if got.Inputs != nil {
			t.Errorf("Inputs should be nil")
		}
	})

	t.Run("run with complex nested data", func(t *testing.T) {
		run := &backend.Run{
			ID:       "complex-run",
			Workflow: "test",
			Status:   "completed",
			Inputs: map[string]any{
				"nested": map[string]any{
					"deep": map[string]any{
						"value": 123,
					},
				},
				"array": []any{1, 2, 3},
			},
			Output: map[string]any{
				"result": "success",
			},
		}

		err := b.CreateRun(ctx, run)
		if err != nil {
			t.Fatalf("CreateRun() error = %v", err)
		}

		got, _ := b.GetRun(ctx, "complex-run")
		if got.Output["result"] != "success" {
			t.Error("Output not preserved")
		}
	})

	t.Run("empty workflow filter returns all", func(t *testing.T) {
		// Clear existing runs
		b2 := New()
		ctx2 := context.Background()

		_ = b2.CreateRun(ctx2, &backend.Run{ID: "r1", Workflow: "w1", Status: "pending"})
		_ = b2.CreateRun(ctx2, &backend.Run{ID: "r2", Workflow: "w2", Status: "pending"})

		runs, _ := b2.ListRuns(ctx2, backend.RunFilter{Workflow: ""})
		if len(runs) != 2 {
			t.Errorf("Expected 2 runs, got %d", len(runs))
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		// Memory backend doesn't actually check context, but the interface requires it
		// This tests that operations don't panic with cancelled context
		_, err := b.GetRun(cancelCtx, "any")
		// We expect an error but not a panic
		_ = err
	})
}

func BenchmarkBackend_CreateRun(b *testing.B) {
	bk := New()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run := &backend.Run{
			ID:       fmt.Sprintf("bench-run-%d", i),
			Workflow: "benchmark",
			Status:   "pending",
		}
		_ = bk.CreateRun(ctx, run)
	}
}

func BenchmarkBackend_GetRun(b *testing.B) {
	bk := New()
	ctx := context.Background()

	// Pre-create runs
	for i := 0; i < 1000; i++ {
		run := &backend.Run{
			ID:       fmt.Sprintf("bench-run-%d", i),
			Workflow: "benchmark",
			Status:   "pending",
		}
		_ = bk.CreateRun(ctx, run)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bk.GetRun(ctx, fmt.Sprintf("bench-run-%d", i%1000))
	}
}

func BenchmarkBackend_ListRuns(b *testing.B) {
	bk := New()
	ctx := context.Background()

	// Pre-create runs
	for i := 0; i < 100; i++ {
		run := &backend.Run{
			ID:       fmt.Sprintf("bench-run-%d", i),
			Workflow: "benchmark",
			Status:   "pending",
		}
		_ = bk.CreateRun(ctx, run)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bk.ListRuns(ctx, backend.RunFilter{})
	}
}
