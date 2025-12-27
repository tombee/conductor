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

package backend_test

import (
	"context"
	"io"
	"testing"

	"github.com/tombee/conductor/internal/daemon/backend"
	"github.com/tombee/conductor/internal/daemon/backend/memory"
)

// minimalRunStore is a test implementation that only implements RunStore.
// This demonstrates that minimal backends can be created with just 3 methods.
type minimalRunStore struct {
	runs map[string]*backend.Run
}

func newMinimalRunStore() *minimalRunStore {
	return &minimalRunStore{runs: make(map[string]*backend.Run)}
}

func (m *minimalRunStore) CreateRun(ctx context.Context, run *backend.Run) error {
	m.runs[run.ID] = run
	return nil
}

func (m *minimalRunStore) GetRun(ctx context.Context, id string) (*backend.Run, error) {
	if run, ok := m.runs[id]; ok {
		return run, nil
	}
	return nil, nil
}

func (m *minimalRunStore) UpdateRun(ctx context.Context, run *backend.Run) error {
	m.runs[run.ID] = run
	return nil
}

// Compile-time assertion that minimalRunStore implements RunStore
var _ backend.RunStore = (*minimalRunStore)(nil)

func TestMinimalRunStore(t *testing.T) {
	store := newMinimalRunStore()

	// Test that a minimal implementation works
	ctx := context.Background()
	run := &backend.Run{
		ID:       "test-run",
		Workflow: "test-workflow",
		Status:   "pending",
	}

	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	got, err := store.GetRun(ctx, "test-run")
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if got.ID != "test-run" {
		t.Errorf("got ID %q, want %q", got.ID, "test-run")
	}

	run.Status = "running"
	if err := store.UpdateRun(ctx, run); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}
}

func TestFeatureDetection(t *testing.T) {
	t.Run("MemoryBackendHasAllCapabilities", func(t *testing.T) {
		store := memory.New()

		// Memory backend should implement all interfaces
		if _, ok := interface{}(store).(backend.RunStore); !ok {
			t.Error("memory backend does not implement RunStore")
		}
		if _, ok := interface{}(store).(backend.RunLister); !ok {
			t.Error("memory backend does not implement RunLister")
		}
		if _, ok := interface{}(store).(backend.CheckpointStore); !ok {
			t.Error("memory backend does not implement CheckpointStore")
		}
		if _, ok := interface{}(store).(io.Closer); !ok {
			t.Error("memory backend does not implement io.Closer")
		}
		if _, ok := interface{}(store).(backend.Backend); !ok {
			t.Error("memory backend does not implement Backend")
		}
		if _, ok := interface{}(store).(backend.ScheduleBackend); !ok {
			t.Error("memory backend does not implement ScheduleBackend")
		}
	})

	t.Run("MinimalStoreOnlyHasRunStore", func(t *testing.T) {
		store := newMinimalRunStore()

		// Minimal store should only implement RunStore
		if _, ok := interface{}(store).(backend.RunStore); !ok {
			t.Error("minimal store does not implement RunStore")
		}
		if _, ok := interface{}(store).(backend.RunLister); ok {
			t.Error("minimal store unexpectedly implements RunLister")
		}
		if _, ok := interface{}(store).(backend.CheckpointStore); ok {
			t.Error("minimal store unexpectedly implements CheckpointStore")
		}
		if _, ok := interface{}(store).(io.Closer); ok {
			t.Error("minimal store unexpectedly implements io.Closer")
		}
	})
}

func TestBackendComposite(t *testing.T) {
	// Verify that Backend is the composition of all interfaces
	var be backend.Backend = memory.New()

	// Should be able to use it as any of the component interfaces
	var _ backend.RunStore = be
	var _ backend.RunLister = be
	var _ backend.CheckpointStore = be
	var _ io.Closer = be
}
