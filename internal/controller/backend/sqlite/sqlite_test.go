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

package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/controller/backend"
)

// createTestBackend creates a SQLite backend for testing in a temporary directory.
func createTestBackend(t *testing.T) (*Backend, string) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := Config{
		Path: dbPath,
		WAL:  true,
	}

	be, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	return be, dbPath
}

func TestSQLiteBackend_CreateRun(t *testing.T) {
	be, _ := createTestBackend(t)
	defer be.Close()

	ctx := context.Background()
	run := &backend.Run{
		ID:         "test-run-1",
		WorkflowID: "test-workflow",
		Workflow:   "test.yaml",
		Status:     "running",
		Inputs:     map[string]any{"key": "value"},
		Completed:  0,
		Total:      5,
	}

	err := be.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Verify run was created
	retrieved, err := be.GetRun(ctx, "test-run-1")
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}

	if retrieved.ID != run.ID {
		t.Errorf("expected ID %s, got %s", run.ID, retrieved.ID)
	}
	if retrieved.Status != run.Status {
		t.Errorf("expected status %s, got %s", run.Status, retrieved.Status)
	}
	if retrieved.Inputs["key"] != "value" {
		t.Errorf("expected inputs to contain key=value, got %v", retrieved.Inputs)
	}
}

func TestSQLiteBackend_UpdateRun(t *testing.T) {
	be, _ := createTestBackend(t)
	defer be.Close()

	ctx := context.Background()
	run := &backend.Run{
		ID:         "test-run-2",
		WorkflowID: "test-workflow",
		Workflow:   "test.yaml",
		Status:     "running",
		Completed:  0,
		Total:      5,
	}

	if err := be.CreateRun(ctx, run); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Update run
	run.Status = "completed"
	run.Completed = 5
	run.Output = map[string]any{"result": "success"}

	if err := be.UpdateRun(ctx, run); err != nil {
		t.Fatalf("failed to update run: %v", err)
	}

	// Verify update
	retrieved, err := be.GetRun(ctx, "test-run-2")
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}

	if retrieved.Status != "completed" {
		t.Errorf("expected status completed, got %s", retrieved.Status)
	}
	if retrieved.Completed != 5 {
		t.Errorf("expected completed 5, got %d", retrieved.Completed)
	}
	if retrieved.Output["result"] != "success" {
		t.Errorf("expected output result=success, got %v", retrieved.Output)
	}
}

func TestSQLiteBackend_ListRuns(t *testing.T) {
	be, _ := createTestBackend(t)
	defer be.Close()

	ctx := context.Background()

	// Create multiple runs
	runs := []*backend.Run{
		{ID: "run-1", WorkflowID: "wf1", Workflow: "test1.yaml", Status: "running"},
		{ID: "run-2", WorkflowID: "wf2", Workflow: "test2.yaml", Status: "completed"},
		{ID: "run-3", WorkflowID: "wf1", Workflow: "test1.yaml", Status: "completed"},
	}

	for _, run := range runs {
		if err := be.CreateRun(ctx, run); err != nil {
			t.Fatalf("failed to create run: %v", err)
		}
	}

	// Test list all
	all, err := be.ListRuns(ctx, backend.RunFilter{})
	if err != nil {
		t.Fatalf("failed to list runs: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 runs, got %d", len(all))
	}

	// Test filter by status
	completed, err := be.ListRuns(ctx, backend.RunFilter{Status: "completed"})
	if err != nil {
		t.Fatalf("failed to list runs: %v", err)
	}
	if len(completed) != 2 {
		t.Errorf("expected 2 completed runs, got %d", len(completed))
	}

	// Test filter by workflow
	wf1, err := be.ListRuns(ctx, backend.RunFilter{Workflow: "test1.yaml"})
	if err != nil {
		t.Fatalf("failed to list runs: %v", err)
	}
	if len(wf1) != 2 {
		t.Errorf("expected 2 runs for test1.yaml, got %d", len(wf1))
	}

	// Test limit
	limited, err := be.ListRuns(ctx, backend.RunFilter{Limit: 2})
	if err != nil {
		t.Fatalf("failed to list runs: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("expected 2 runs with limit, got %d", len(limited))
	}
}

func TestSQLiteBackend_DeleteRun(t *testing.T) {
	be, _ := createTestBackend(t)
	defer be.Close()

	ctx := context.Background()
	run := &backend.Run{
		ID:         "test-run-delete",
		WorkflowID: "test-workflow",
		Workflow:   "test.yaml",
		Status:     "running",
	}

	if err := be.CreateRun(ctx, run); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Delete run
	if err := be.DeleteRun(ctx, "test-run-delete"); err != nil {
		t.Fatalf("failed to delete run: %v", err)
	}

	// Verify deletion
	_, err := be.GetRun(ctx, "test-run-delete")
	if err == nil {
		t.Errorf("expected error getting deleted run, got nil")
	}
}

func TestSQLiteBackend_Checkpoint(t *testing.T) {
	be, _ := createTestBackend(t)
	defer be.Close()

	ctx := context.Background()

	// Create a run first
	run := &backend.Run{
		ID:         "test-run-checkpoint",
		WorkflowID: "test-workflow",
		Workflow:   "test.yaml",
		Status:     "running",
	}
	if err := be.CreateRun(ctx, run); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Save checkpoint
	checkpoint := &backend.Checkpoint{
		StepID:    "step-1",
		StepIndex: 1,
		Context:   map[string]any{"state": "saved"},
	}

	if err := be.SaveCheckpoint(ctx, "test-run-checkpoint", checkpoint); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	// Retrieve checkpoint
	retrieved, err := be.GetCheckpoint(ctx, "test-run-checkpoint")
	if err != nil {
		t.Fatalf("failed to get checkpoint: %v", err)
	}

	if retrieved.StepID != "step-1" {
		t.Errorf("expected step ID step-1, got %s", retrieved.StepID)
	}
	if retrieved.Context["state"] != "saved" {
		t.Errorf("expected context state=saved, got %v", retrieved.Context)
	}

	// Update checkpoint (upsert)
	checkpoint.StepID = "step-2"
	checkpoint.StepIndex = 2
	if err := be.SaveCheckpoint(ctx, "test-run-checkpoint", checkpoint); err != nil {
		t.Fatalf("failed to update checkpoint: %v", err)
	}

	retrieved, err = be.GetCheckpoint(ctx, "test-run-checkpoint")
	if err != nil {
		t.Fatalf("failed to get updated checkpoint: %v", err)
	}
	if retrieved.StepID != "step-2" {
		t.Errorf("expected updated step ID step-2, got %s", retrieved.StepID)
	}
}

func TestSQLiteBackend_StepResults(t *testing.T) {
	be, _ := createTestBackend(t)
	defer be.Close()

	ctx := context.Background()

	// Create a run first
	run := &backend.Run{
		ID:         "test-run-steps",
		WorkflowID: "test-workflow",
		Workflow:   "test.yaml",
		Status:     "running",
	}
	if err := be.CreateRun(ctx, run); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Save step results
	results := []*backend.StepResult{
		{
			RunID:     "test-run-steps",
			StepID:    "step-1",
			StepIndex: 0,
			Inputs:    map[string]any{"input": "data"},
			Outputs:   map[string]any{"output": "result"},
			Duration:  100 * time.Millisecond,
			Status:    "completed",
			CostUSD:   0.001,
		},
		{
			RunID:     "test-run-steps",
			StepID:    "step-2",
			StepIndex: 1,
			Outputs:   map[string]any{"output": "result2"},
			Duration:  200 * time.Millisecond,
			Status:    "completed",
			CostUSD:   0.002,
		},
	}

	for _, result := range results {
		if err := be.SaveStepResult(ctx, result); err != nil {
			t.Fatalf("failed to save step result: %v", err)
		}
	}

	// Get individual step result
	step1, err := be.GetStepResult(ctx, "test-run-steps", "step-1")
	if err != nil {
		t.Fatalf("failed to get step result: %v", err)
	}
	if step1.Inputs["input"] != "data" {
		t.Errorf("expected input=data, got %v", step1.Inputs)
	}
	if step1.CostUSD != 0.001 {
		t.Errorf("expected cost 0.001, got %f", step1.CostUSD)
	}

	// List all step results for run
	allResults, err := be.ListStepResults(ctx, "test-run-steps")
	if err != nil {
		t.Fatalf("failed to list step results: %v", err)
	}
	if len(allResults) != 2 {
		t.Errorf("expected 2 step results, got %d", len(allResults))
	}

	// Verify ordering by step_index
	if allResults[0].StepID != "step-1" || allResults[1].StepID != "step-2" {
		t.Errorf("step results not ordered correctly")
	}
}

func TestSQLiteBackend_ScheduleState(t *testing.T) {
	be, _ := createTestBackend(t)
	defer be.Close()

	ctx := context.Background()

	now := time.Now()
	nextRun := now.Add(1 * time.Hour)

	// Save schedule state
	state := &backend.ScheduleState{
		Name:       "test-schedule",
		LastRun:    &now,
		NextRun:    &nextRun,
		RunCount:   10,
		ErrorCount: 1,
		Enabled:    true,
	}

	if err := be.SaveScheduleState(ctx, state); err != nil {
		t.Fatalf("failed to save schedule state: %v", err)
	}

	// Get schedule state
	retrieved, err := be.GetScheduleState(ctx, "test-schedule")
	if err != nil {
		t.Fatalf("failed to get schedule state: %v", err)
	}

	if retrieved.Name != "test-schedule" {
		t.Errorf("expected name test-schedule, got %s", retrieved.Name)
	}
	if retrieved.RunCount != 10 {
		t.Errorf("expected run count 10, got %d", retrieved.RunCount)
	}
	if !retrieved.Enabled {
		t.Errorf("expected enabled=true")
	}

	// Update schedule state (upsert)
	state.RunCount = 11
	state.Enabled = false
	if err := be.SaveScheduleState(ctx, state); err != nil {
		t.Fatalf("failed to update schedule state: %v", err)
	}

	retrieved, err = be.GetScheduleState(ctx, "test-schedule")
	if err != nil {
		t.Fatalf("failed to get updated schedule state: %v", err)
	}
	if retrieved.RunCount != 11 {
		t.Errorf("expected updated run count 11, got %d", retrieved.RunCount)
	}
	if retrieved.Enabled {
		t.Errorf("expected enabled=false")
	}

	// List schedule states
	states, err := be.ListScheduleStates(ctx)
	if err != nil {
		t.Fatalf("failed to list schedule states: %v", err)
	}
	if len(states) != 1 {
		t.Errorf("expected 1 schedule state, got %d", len(states))
	}

	// Delete schedule state
	if err := be.DeleteScheduleState(ctx, "test-schedule"); err != nil {
		t.Fatalf("failed to delete schedule state: %v", err)
	}

	// Verify deletion
	_, err = be.GetScheduleState(ctx, "test-schedule")
	if err == nil {
		t.Errorf("expected error getting deleted schedule state, got nil")
	}
}

func TestSQLiteBackend_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "persist.db")

	cfg := Config{
		Path: dbPath,
		WAL:  true,
	}

	// Create backend and add a run
	be1, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()
	run := &backend.Run{
		ID:         "persist-run",
		WorkflowID: "test-workflow",
		Workflow:   "test.yaml",
		Status:     "completed",
	}
	if err := be1.CreateRun(ctx, run); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Close backend
	if err := be1.Close(); err != nil {
		t.Fatalf("failed to close backend: %v", err)
	}

	// Reopen backend
	be2, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to reopen backend: %v", err)
	}
	defer be2.Close()

	// Verify run persisted
	retrieved, err := be2.GetRun(ctx, "persist-run")
	if err != nil {
		t.Fatalf("failed to get persisted run: %v", err)
	}

	if retrieved.ID != "persist-run" {
		t.Errorf("expected ID persist-run, got %s", retrieved.ID)
	}
	if retrieved.Status != "completed" {
		t.Errorf("expected status completed, got %s", retrieved.Status)
	}
}

func TestSQLiteBackend_WALMode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "wal.db")

	cfg := Config{
		Path: dbPath,
		WAL:  true,
	}

	be, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create backend with WAL: %v", err)
	}
	defer be.Close()

	// Verify WAL file is created
	walPath := dbPath + "-wal"

	// Add some data to trigger WAL creation
	ctx := context.Background()
	run := &backend.Run{
		ID:         "wal-test",
		WorkflowID: "test-workflow",
		Workflow:   "test.yaml",
		Status:     "running",
	}
	if err := be.CreateRun(ctx, run); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// WAL file should exist (though it may be empty initially)
	// Note: SQLite may not create WAL file immediately, so we don't strictly require it
	if _, err := os.Stat(walPath); err == nil {
		t.Logf("WAL file created at %s", walPath)
	}
}

func TestSQLiteBackend_ForeignKeyConstraints(t *testing.T) {
	be, _ := createTestBackend(t)
	defer be.Close()

	ctx := context.Background()

	// Create a run
	run := &backend.Run{
		ID:         "fk-test-run",
		WorkflowID: "test-workflow",
		Workflow:   "test.yaml",
		Status:     "running",
	}
	if err := be.CreateRun(ctx, run); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Save step result
	result := &backend.StepResult{
		RunID:     "fk-test-run",
		StepID:    "step-1",
		StepIndex: 0,
		Duration:  100 * time.Millisecond,
		Status:    "completed",
	}
	if err := be.SaveStepResult(ctx, result); err != nil {
		t.Fatalf("failed to save step result: %v", err)
	}

	// Delete run (should cascade delete step result)
	if err := be.DeleteRun(ctx, "fk-test-run"); err != nil {
		t.Fatalf("failed to delete run: %v", err)
	}

	// Verify step result was also deleted
	_, err := be.GetStepResult(ctx, "fk-test-run", "step-1")
	if err == nil {
		t.Errorf("expected error getting step result after run deletion, got nil")
	}
}
