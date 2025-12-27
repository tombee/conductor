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
	"sync"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/daemon/backend"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestNewStateManager(t *testing.T) {
	sm := NewStateManager(nil)
	if sm == nil {
		t.Fatal("expected non-nil StateManager")
	}
	if sm.runs == nil {
		t.Error("expected non-nil runs map")
	}
}

func TestStateManager_CreateRun(t *testing.T) {
	sm := NewStateManager(nil)

	def := &workflow.Definition{
		Name: "test-workflow",
		Steps: []workflow.StepDefinition{
			{ID: "step1"},
			{ID: "step2"},
		},
	}
	inputs := map[string]any{"key": "value"}

	run, err := sm.CreateRun(context.Background(), def, inputs, "http://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if run.ID == "" {
		t.Error("expected non-empty run ID")
	}
	if run.WorkflowID != "test-workflow" {
		t.Errorf("expected workflow ID 'test-workflow', got %s", run.WorkflowID)
	}
	if run.Status != RunStatusPending {
		t.Errorf("expected status pending, got %s", run.Status)
	}
	if run.Inputs["key"] != "value" {
		t.Error("expected inputs to be preserved")
	}
	if run.SourceURL != "http://example.com" {
		t.Errorf("expected source URL to be preserved")
	}
	if run.Progress == nil || run.Progress.Total != 2 {
		t.Error("expected progress with total=2")
	}
	if run.ctx == nil || run.cancel == nil {
		t.Error("expected context and cancel to be set")
	}
	if run.stopped == nil {
		t.Error("expected stopped channel to be set")
	}
}

func TestStateManager_GetRun(t *testing.T) {
	sm := NewStateManager(nil)

	def := &workflow.Definition{Name: "test-workflow"}
	run, _ := sm.CreateRun(context.Background(), def, nil, "")

	// Test successful get
	snapshot, err := sm.GetRun(run.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.ID != run.ID {
		t.Error("expected matching run ID")
	}

	// Test not found
	_, err = sm.GetRun("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent run")
	}
}

func TestStateManager_GetRunInternal(t *testing.T) {
	sm := NewStateManager(nil)

	def := &workflow.Definition{Name: "test-workflow"}
	run, _ := sm.CreateRun(context.Background(), def, nil, "")

	// Test successful get
	internalRun, exists := sm.GetRunInternal(run.ID)
	if !exists {
		t.Error("expected run to exist")
	}
	if internalRun != run {
		t.Error("expected same run pointer")
	}

	// Test not found
	_, exists = sm.GetRunInternal("nonexistent")
	if exists {
		t.Error("expected run to not exist")
	}
}

func TestStateManager_ListRuns(t *testing.T) {
	sm := NewStateManager(nil)

	def1 := &workflow.Definition{Name: "workflow-1"}
	def2 := &workflow.Definition{Name: "workflow-2"}

	run1, _ := sm.CreateRun(context.Background(), def1, nil, "")
	run2, _ := sm.CreateRun(context.Background(), def2, nil, "")

	// Update status to test filtering
	run2.Status = RunStatusCompleted

	// Test list all
	all := sm.ListRuns(ListFilter{})
	if len(all) != 2 {
		t.Errorf("expected 2 runs, got %d", len(all))
	}

	// Test filter by status
	pending := sm.ListRuns(ListFilter{Status: RunStatusPending})
	if len(pending) != 1 || pending[0].ID != run1.ID {
		t.Error("expected 1 pending run")
	}

	completed := sm.ListRuns(ListFilter{Status: RunStatusCompleted})
	if len(completed) != 1 || completed[0].ID != run2.ID {
		t.Error("expected 1 completed run")
	}

	// Test filter by workflow
	wf1Runs := sm.ListRuns(ListFilter{Workflow: "workflow-1"})
	if len(wf1Runs) != 1 || wf1Runs[0].WorkflowID != "workflow-1" {
		t.Error("expected 1 run for workflow-1")
	}
}

func TestStateManager_ActiveRunCount(t *testing.T) {
	sm := NewStateManager(nil)

	def := &workflow.Definition{Name: "test-workflow"}

	// No runs
	if count := sm.ActiveRunCount(); count != 0 {
		t.Errorf("expected 0 active runs, got %d", count)
	}

	// One pending run
	run1, _ := sm.CreateRun(context.Background(), def, nil, "")
	if count := sm.ActiveRunCount(); count != 1 {
		t.Errorf("expected 1 active run, got %d", count)
	}

	// Add running run
	run2, _ := sm.CreateRun(context.Background(), def, nil, "")
	run2.Status = RunStatusRunning
	if count := sm.ActiveRunCount(); count != 2 {
		t.Errorf("expected 2 active runs, got %d", count)
	}

	// Complete a run
	run1.Status = RunStatusCompleted
	if count := sm.ActiveRunCount(); count != 1 {
		t.Errorf("expected 1 active run, got %d", count)
	}

	// Fail a run
	run2.Status = RunStatusFailed
	if count := sm.ActiveRunCount(); count != 0 {
		t.Errorf("expected 0 active runs, got %d", count)
	}
}

func TestStateManager_Snapshot(t *testing.T) {
	sm := NewStateManager(nil)

	def := &workflow.Definition{Name: "test-workflow"}
	run, _ := sm.CreateRun(context.Background(), def, map[string]any{"key": "value"}, "")
	run.Output = map[string]any{"result": "success"}
	run.Logs = []LogEntry{{Message: "test log"}}

	snapshot := sm.Snapshot(run)

	// Verify deep copy
	if &snapshot.Inputs == &run.Inputs {
		t.Error("expected Inputs to be deep copied")
	}
	if &snapshot.Output == &run.Output {
		t.Error("expected Output to be deep copied")
	}
	if &snapshot.Logs == &run.Logs {
		t.Error("expected Logs to be deep copied")
	}

	// Modify original, verify snapshot unchanged
	run.Inputs["key"] = "modified"
	if snapshot.Inputs["key"] != "value" {
		t.Error("snapshot should be immutable")
	}
}

func TestStateManager_snapshotRun_DeepCopy(t *testing.T) {
	sm := NewStateManager(nil)

	now := time.Now()
	run := &Run{
		ID:         "test-run",
		WorkflowID: "test-workflow",
		Workflow:   "test-workflow",
		Status:     RunStatusRunning,
		Inputs:     map[string]any{"input": "data"},
		Output:     map[string]any{"output": "result"},
		Progress: &Progress{
			CurrentStep: "step1",
			Completed:   1,
			Total:       3,
		},
		StartedAt: &now,
		Logs: []LogEntry{
			{Message: "log1"},
			{Message: "log2"},
		},
	}

	sm.runs[run.ID] = run
	snapshot := sm.snapshotRun(run)

	// Verify all fields are copied
	if snapshot.ID != run.ID {
		t.Error("ID not copied")
	}
	if snapshot.Progress.CurrentStep != "step1" {
		t.Error("Progress not copied")
	}

	// Modify original progress
	run.Progress.CurrentStep = "step2"
	if snapshot.Progress.CurrentStep != "step1" {
		t.Error("Progress should be deep copied")
	}

	// Verify logs are copied
	run.Logs[0].Message = "modified"
	if snapshot.Logs[0].Message != "log1" {
		t.Error("Logs should be deep copied")
	}
}

func TestStateManager_toBackendRun(t *testing.T) {
	sm := NewStateManager(nil)

	now := time.Now()
	run := &Run{
		ID:            "test-run",
		WorkflowID:    "test-workflow",
		Workflow:      "test-workflow",
		Status:        RunStatusRunning,
		CorrelationID: "corr-123",
		Inputs:        map[string]any{"key": "value"},
		Output:        map[string]any{"result": "success"},
		Error:         "test error",
		Progress: &Progress{
			CurrentStep: "step1",
			Completed:   1,
			Total:       3,
		},
		StartedAt:   &now,
		CompletedAt: &now,
		CreatedAt:   now,
	}

	beRun := sm.toBackendRun(run)

	if beRun.ID != "test-run" {
		t.Error("ID not converted")
	}
	if beRun.Status != "running" {
		t.Error("Status not converted")
	}
	if beRun.CorrelationID != "corr-123" {
		t.Error("CorrelationID not converted")
	}
	if beRun.CurrentStep != "step1" {
		t.Error("CurrentStep not converted")
	}
	if beRun.Completed != 1 {
		t.Error("Completed not converted")
	}
	if beRun.Total != 3 {
		t.Error("Total not converted")
	}
	if beRun.StartedAt == nil {
		t.Error("StartedAt not converted")
	}
	if beRun.CompletedAt == nil {
		t.Error("CompletedAt not converted")
	}
}

func TestStateManager_Concurrent(t *testing.T) {
	sm := NewStateManager(nil)
	def := &workflow.Definition{Name: "test-workflow"}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sm.CreateRun(context.Background(), def, nil, "")
		}()
	}
	wg.Wait()

	if len(sm.runs) != 10 {
		t.Errorf("expected 10 runs, got %d", len(sm.runs))
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sm.ListRuns(ListFilter{})
			_ = sm.ActiveRunCount()
		}()
	}
	wg.Wait()
}

func TestStepOutputToMap(t *testing.T) {
	tests := []struct {
		name   string
		output workflow.StepOutput
		want   map[string]any
	}{
		{
			name:   "empty output",
			output: workflow.StepOutput{},
			want:   map[string]any{},
		},
		{
			name:   "text only",
			output: workflow.StepOutput{Text: "hello"},
			want:   map[string]any{"response": "hello"},
		},
		{
			name:   "data map",
			output: workflow.StepOutput{Data: map[string]any{"key": "value"}},
			want:   map[string]any{"key": "value"},
		},
		{
			name:   "data non-map",
			output: workflow.StepOutput{Data: "raw data"},
			want:   map[string]any{"data": "raw data"},
		},
		{
			name:   "error",
			output: workflow.StepOutput{Error: "something failed"},
			want:   map[string]any{"error": "something failed"},
		},
		{
			name: "combined",
			output: workflow.StepOutput{
				Text:  "response text",
				Data:  map[string]any{"extra": "data"},
				Error: "warning",
			},
			want: map[string]any{
				"response": "response text",
				"extra":    "data",
				"error":    "warning",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stepOutputToMap(tt.output)
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("expected %s=%v, got %v", k, v, got[k])
				}
			}
		})
	}
}

func TestStateManager_UpdateRun(t *testing.T) {
	sm := NewStateManager(nil)
	def := &workflow.Definition{Name: "test-workflow"}
	run, _ := sm.CreateRun(context.Background(), def, nil, "")

	// Update run state
	run.Status = RunStatusRunning

	// UpdateRun should not error with nil backend
	err := sm.UpdateRun(context.Background(), run)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStateManager_UpdateRun_WithBackend(t *testing.T) {
	// Test that UpdateRun works with a backend - uses mock
	mockBE := &mockBackend{}
	sm := NewStateManager(mockBE)
	def := &workflow.Definition{Name: "test-workflow"}
	run, _ := sm.CreateRun(context.Background(), def, nil, "")

	run.Status = RunStatusCompleted
	err := sm.UpdateRun(context.Background(), run)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mockBE.updateCalled {
		t.Error("expected UpdateRun to call backend.UpdateRun")
	}
}

// mockBackend is a minimal mock for testing backend interactions.
type mockBackend struct {
	createCalled bool
	updateCalled bool
}

func (m *mockBackend) CreateRun(ctx context.Context, run *backend.Run) error {
	m.createCalled = true
	return nil
}

func (m *mockBackend) GetRun(ctx context.Context, id string) (*backend.Run, error) {
	return nil, nil
}

func (m *mockBackend) UpdateRun(ctx context.Context, run *backend.Run) error {
	m.updateCalled = true
	return nil
}

func (m *mockBackend) DeleteRun(ctx context.Context, id string) error {
	return nil
}

func (m *mockBackend) ListRuns(ctx context.Context, filter backend.RunFilter) ([]*backend.Run, error) {
	return nil, nil
}

func (m *mockBackend) SaveCheckpoint(ctx context.Context, runID string, cp *backend.Checkpoint) error {
	return nil
}

func (m *mockBackend) GetCheckpoint(ctx context.Context, runID string) (*backend.Checkpoint, error) {
	return nil, nil
}

func (m *mockBackend) SaveScheduleState(ctx context.Context, state *backend.ScheduleState) error {
	return nil
}

func (m *mockBackend) GetScheduleState(ctx context.Context, scheduleID string) (*backend.ScheduleState, error) {
	return nil, nil
}

func (m *mockBackend) Close() error {
	return nil
}
