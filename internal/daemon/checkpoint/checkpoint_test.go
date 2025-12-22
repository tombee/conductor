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

package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestManager_SaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	m, err := NewManager(ManagerConfig{
		Dir: tmpDir,
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	ctx := context.Background()

	// Save a checkpoint
	cp := &Checkpoint{
		RunID:      "run-123",
		WorkflowID: "test-workflow",
		StepID:     "step-1",
		StepIndex:  0,
		Context: map[string]any{
			"inputs": map[string]any{"foo": "bar"},
		},
		StepOutputs: map[string]any{},
	}

	err = m.Save(ctx, cp)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	cpPath := filepath.Join(tmpDir, "run-123.json")
	if _, err := os.Stat(cpPath); os.IsNotExist(err) {
		t.Error("Checkpoint file was not created")
	}

	// Load the checkpoint
	loaded, err := m.Load(ctx, "run-123")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.RunID != cp.RunID {
		t.Errorf("Expected RunID %s, got %s", cp.RunID, loaded.RunID)
	}
	if loaded.WorkflowID != cp.WorkflowID {
		t.Errorf("Expected WorkflowID %s, got %s", cp.WorkflowID, loaded.WorkflowID)
	}
	if loaded.StepID != cp.StepID {
		t.Errorf("Expected StepID %s, got %s", cp.StepID, loaded.StepID)
	}
	if loaded.StepIndex != cp.StepIndex {
		t.Errorf("Expected StepIndex %d, got %d", cp.StepIndex, loaded.StepIndex)
	}
}

func TestManager_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	m, err := NewManager(ManagerConfig{
		Dir: tmpDir,
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	ctx := context.Background()

	// Load non-existent checkpoint should return nil, nil
	loaded, err := m.Load(ctx, "non-existent")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded != nil {
		t.Errorf("Expected nil for non-existent checkpoint, got %v", loaded)
	}
}

func TestManager_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	m, err := NewManager(ManagerConfig{
		Dir: tmpDir,
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	ctx := context.Background()

	// Save a checkpoint
	cp := &Checkpoint{
		RunID:      "run-456",
		WorkflowID: "test-workflow",
		StepID:     "step-1",
	}
	m.Save(ctx, cp)

	// Delete it
	err = m.Delete(ctx, "run-456")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	loaded, _ := m.Load(ctx, "run-456")
	if loaded != nil {
		t.Error("Checkpoint should have been deleted")
	}
}

func TestManager_ListInterrupted(t *testing.T) {
	tmpDir := t.TempDir()

	m, err := NewManager(ManagerConfig{
		Dir: tmpDir,
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	ctx := context.Background()

	// Save multiple checkpoints
	for _, runID := range []string{"run-1", "run-2", "run-3"} {
		cp := &Checkpoint{
			RunID:      runID,
			WorkflowID: "test-workflow",
		}
		m.Save(ctx, cp)
	}

	// List interrupted runs
	runs, err := m.ListInterrupted(ctx)
	if err != nil {
		t.Fatalf("ListInterrupted failed: %v", err)
	}

	if len(runs) != 3 {
		t.Errorf("Expected 3 interrupted runs, got %d", len(runs))
	}
}

func TestManager_Disabled(t *testing.T) {
	// Create manager with empty dir (disabled)
	m, err := NewManager(ManagerConfig{
		Dir: "",
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if m.Enabled() {
		t.Error("Manager should be disabled with empty dir")
	}

	ctx := context.Background()

	// Operations should be no-ops
	err = m.Save(ctx, &Checkpoint{RunID: "test"})
	if err != nil {
		t.Errorf("Save should be no-op when disabled, got error: %v", err)
	}

	loaded, err := m.Load(ctx, "test")
	if err != nil {
		t.Errorf("Load should be no-op when disabled, got error: %v", err)
	}
	if loaded != nil {
		t.Error("Load should return nil when disabled")
	}

	runs, err := m.ListInterrupted(ctx)
	if err != nil {
		t.Errorf("ListInterrupted should be no-op when disabled, got error: %v", err)
	}
	if runs != nil {
		t.Error("ListInterrupted should return nil when disabled")
	}
}
