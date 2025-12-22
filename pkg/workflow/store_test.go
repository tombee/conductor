package workflow

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("create workflow successfully", func(t *testing.T) {
		store := NewMemoryStore()
		workflow := &Workflow{
			ID:   "test-1",
			Name: "Test Workflow",
		}

		err := store.Create(ctx, workflow)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		// Verify timestamps were set
		if workflow.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set")
		}
		if workflow.UpdatedAt.IsZero() {
			t.Error("UpdatedAt should be set")
		}
		if workflow.State != StateCreated {
			t.Errorf("State = %v, want %v", workflow.State, StateCreated)
		}
		if workflow.Metadata == nil {
			t.Error("Metadata should be initialized")
		}
	})

	t.Run("create with nil workflow", func(t *testing.T) {
		store := NewMemoryStore()

		err := store.Create(ctx, nil)
		if err == nil {
			t.Fatal("Create() should return error for nil workflow")
		}
	})

	t.Run("create with empty ID", func(t *testing.T) {
		store := NewMemoryStore()
		workflow := &Workflow{
			Name: "Test Workflow",
		}

		err := store.Create(ctx, workflow)
		if err == nil {
			t.Fatal("Create() should return error for empty ID")
		}
	})

	t.Run("create duplicate ID", func(t *testing.T) {
		store := NewMemoryStore()
		workflow1 := &Workflow{
			ID:   "test-1",
			Name: "Workflow 1",
		}
		workflow2 := &Workflow{
			ID:   "test-1",
			Name: "Workflow 2",
		}

		err := store.Create(ctx, workflow1)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		err = store.Create(ctx, workflow2)
		if err == nil {
			t.Fatal("Create() should return error for duplicate ID")
		}
	})

	t.Run("create preserves existing timestamps", func(t *testing.T) {
		store := NewMemoryStore()
		createdAt := time.Now().Add(-1 * time.Hour)
		updatedAt := time.Now().Add(-30 * time.Minute)

		workflow := &Workflow{
			ID:        "test-1",
			Name:      "Test Workflow",
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}

		err := store.Create(ctx, workflow)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if !workflow.CreatedAt.Equal(createdAt) {
			t.Error("CreatedAt should be preserved")
		}
		if !workflow.UpdatedAt.Equal(updatedAt) {
			t.Error("UpdatedAt should be preserved")
		}
	})
}

func TestMemoryStoreGet(t *testing.T) {
	ctx := context.Background()

	t.Run("get existing workflow", func(t *testing.T) {
		store := NewMemoryStore()
		workflow := &Workflow{
			ID:   "test-1",
			Name: "Test Workflow",
			Metadata: map[string]interface{}{
				"key": "value",
			},
		}

		err := store.Create(ctx, workflow)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		retrieved, err := store.Get(ctx, "test-1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if retrieved.ID != workflow.ID {
			t.Errorf("ID = %v, want %v", retrieved.ID, workflow.ID)
		}
		if retrieved.Name != workflow.Name {
			t.Errorf("Name = %v, want %v", retrieved.Name, workflow.Name)
		}
		if retrieved.Metadata["key"] != "value" {
			t.Error("Metadata not preserved")
		}
	})

	t.Run("get non-existent workflow", func(t *testing.T) {
		store := NewMemoryStore()

		_, err := store.Get(ctx, "non-existent")
		if err == nil {
			t.Fatal("Get() should return error for non-existent workflow")
		}
	})

	t.Run("get with empty ID", func(t *testing.T) {
		store := NewMemoryStore()

		_, err := store.Get(ctx, "")
		if err == nil {
			t.Fatal("Get() should return error for empty ID")
		}
	})

	t.Run("get returns copy", func(t *testing.T) {
		store := NewMemoryStore()
		workflow := &Workflow{
			ID:   "test-1",
			Name: "Test Workflow",
		}

		err := store.Create(ctx, workflow)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		retrieved, err := store.Get(ctx, "test-1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		// Modify retrieved workflow
		retrieved.Name = "Modified"

		// Get again and verify original is unchanged
		retrieved2, err := store.Get(ctx, "test-1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if retrieved2.Name != "Test Workflow" {
			t.Error("Store should return copies, not references")
		}
	})
}

func TestMemoryStoreUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("update existing workflow", func(t *testing.T) {
		store := NewMemoryStore()
		workflow := &Workflow{
			ID:   "test-1",
			Name: "Original Name",
		}

		err := store.Create(ctx, workflow)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		workflow.Name = "Updated Name"
		workflow.State = StateRunning

		err = store.Update(ctx, workflow)
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		retrieved, err := store.Get(ctx, "test-1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if retrieved.Name != "Updated Name" {
			t.Errorf("Name = %v, want %v", retrieved.Name, "Updated Name")
		}
		if retrieved.State != StateRunning {
			t.Errorf("State = %v, want %v", retrieved.State, StateRunning)
		}
	})

	t.Run("update updates timestamp", func(t *testing.T) {
		store := NewMemoryStore()
		workflow := &Workflow{
			ID:   "test-1",
			Name: "Test Workflow",
		}

		err := store.Create(ctx, workflow)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		originalUpdatedAt := workflow.UpdatedAt
		time.Sleep(1 * time.Millisecond)

		workflow.Name = "Updated"
		err = store.Update(ctx, workflow)
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		retrieved, err := store.Get(ctx, "test-1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if !retrieved.UpdatedAt.After(originalUpdatedAt) {
			t.Error("UpdatedAt should be updated")
		}
	})

	t.Run("update non-existent workflow", func(t *testing.T) {
		store := NewMemoryStore()
		workflow := &Workflow{
			ID:   "non-existent",
			Name: "Test",
		}

		err := store.Update(ctx, workflow)
		if err == nil {
			t.Fatal("Update() should return error for non-existent workflow")
		}
	})

	t.Run("update with nil workflow", func(t *testing.T) {
		store := NewMemoryStore()

		err := store.Update(ctx, nil)
		if err == nil {
			t.Fatal("Update() should return error for nil workflow")
		}
	})

	t.Run("update with empty ID", func(t *testing.T) {
		store := NewMemoryStore()
		workflow := &Workflow{
			Name: "Test",
		}

		err := store.Update(ctx, workflow)
		if err == nil {
			t.Fatal("Update() should return error for empty ID")
		}
	})
}

func TestMemoryStoreDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("delete existing workflow", func(t *testing.T) {
		store := NewMemoryStore()
		workflow := &Workflow{
			ID:   "test-1",
			Name: "Test Workflow",
		}

		err := store.Create(ctx, workflow)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		err = store.Delete(ctx, "test-1")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err = store.Get(ctx, "test-1")
		if err == nil {
			t.Fatal("Get() should return error after delete")
		}
	})

	t.Run("delete non-existent workflow", func(t *testing.T) {
		store := NewMemoryStore()

		err := store.Delete(ctx, "non-existent")
		if err == nil {
			t.Fatal("Delete() should return error for non-existent workflow")
		}
	})

	t.Run("delete with empty ID", func(t *testing.T) {
		store := NewMemoryStore()

		err := store.Delete(ctx, "")
		if err == nil {
			t.Fatal("Delete() should return error for empty ID")
		}
	})
}

func TestMemoryStoreList(t *testing.T) {
	ctx := context.Background()

	t.Run("list all workflows", func(t *testing.T) {
		store := NewMemoryStore()

		// Create test workflows
		for i := 1; i <= 3; i++ {
			workflow := &Workflow{
				ID:   string(rune('0' + i)),
				Name: "Workflow",
			}
			err := store.Create(ctx, workflow)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
		}

		workflows, err := store.List(ctx, nil)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(workflows) != 3 {
			t.Errorf("len(workflows) = %d, want 3", len(workflows))
		}
	})

	t.Run("list with state filter", func(t *testing.T) {
		store := NewMemoryStore()

		// Create workflows with different states
		workflow1 := &Workflow{ID: "1", State: StateCreated}
		workflow2 := &Workflow{ID: "2", State: StateRunning}
		workflow3 := &Workflow{ID: "3", State: StateCreated}

		for _, w := range []*Workflow{workflow1, workflow2, workflow3} {
			err := store.Create(ctx, w)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
		}

		state := StateCreated
		workflows, err := store.List(ctx, &Query{State: &state})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(workflows) != 2 {
			t.Errorf("len(workflows) = %d, want 2", len(workflows))
		}
		for _, w := range workflows {
			if w.State != StateCreated {
				t.Errorf("State = %v, want %v", w.State, StateCreated)
			}
		}
	})

	t.Run("list with metadata filter", func(t *testing.T) {
		store := NewMemoryStore()

		// Create workflows with different metadata
		workflow1 := &Workflow{
			ID:       "1",
			Metadata: map[string]interface{}{"type": "test", "priority": "high"},
		}
		workflow2 := &Workflow{
			ID:       "2",
			Metadata: map[string]interface{}{"type": "prod", "priority": "low"},
		}
		workflow3 := &Workflow{
			ID:       "3",
			Metadata: map[string]interface{}{"type": "test", "priority": "low"},
		}

		for _, w := range []*Workflow{workflow1, workflow2, workflow3} {
			err := store.Create(ctx, w)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
		}

		workflows, err := store.List(ctx, &Query{
			Metadata: map[string]interface{}{"type": "test"},
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(workflows) != 2 {
			t.Errorf("len(workflows) = %d, want 2", len(workflows))
		}
	})

	t.Run("list with limit", func(t *testing.T) {
		store := NewMemoryStore()

		// Create 5 workflows
		for i := 1; i <= 5; i++ {
			workflow := &Workflow{ID: string(rune('0' + i))}
			err := store.Create(ctx, workflow)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
		}

		workflows, err := store.List(ctx, &Query{Limit: 3})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(workflows) != 3 {
			t.Errorf("len(workflows) = %d, want 3", len(workflows))
		}
	})

	t.Run("list with offset", func(t *testing.T) {
		store := NewMemoryStore()

		// Create 5 workflows
		for i := 1; i <= 5; i++ {
			workflow := &Workflow{ID: string(rune('0' + i))}
			err := store.Create(ctx, workflow)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
		}

		workflows, err := store.List(ctx, &Query{Offset: 2})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(workflows) != 3 {
			t.Errorf("len(workflows) = %d, want 3", len(workflows))
		}
	})

	t.Run("list with offset and limit", func(t *testing.T) {
		store := NewMemoryStore()

		// Create 10 workflows
		for i := 1; i <= 10; i++ {
			workflow := &Workflow{ID: string(rune('0' + i))}
			err := store.Create(ctx, workflow)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
		}

		workflows, err := store.List(ctx, &Query{Offset: 3, Limit: 4})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(workflows) != 4 {
			t.Errorf("len(workflows) = %d, want 4", len(workflows))
		}
	})

	t.Run("list with offset beyond results", func(t *testing.T) {
		store := NewMemoryStore()

		workflow := &Workflow{ID: "1"}
		err := store.Create(ctx, workflow)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		workflows, err := store.List(ctx, &Query{Offset: 10})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(workflows) != 0 {
			t.Errorf("len(workflows) = %d, want 0", len(workflows))
		}
	})

	t.Run("list empty store", func(t *testing.T) {
		store := NewMemoryStore()

		workflows, err := store.List(ctx, nil)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(workflows) != 0 {
			t.Errorf("len(workflows) = %d, want 0", len(workflows))
		}
	})
}

func TestCopyWorkflow(t *testing.T) {
	t.Run("copy with all fields", func(t *testing.T) {
		now := time.Now()
		startedAt := now.Add(-1 * time.Hour)
		completedAt := now

		original := &Workflow{
			ID:          "test-1",
			Name:        "Test",
			State:       StateCompleted,
			Metadata:    map[string]interface{}{"key": "value"},
			CreatedAt:   now.Add(-2 * time.Hour),
			UpdatedAt:   now,
			StartedAt:   &startedAt,
			CompletedAt: &completedAt,
			Error:       "test error",
		}

		copy := copyWorkflow(original)

		if copy.ID != original.ID {
			t.Error("ID not copied")
		}
		if copy.Name != original.Name {
			t.Error("Name not copied")
		}
		if copy.State != original.State {
			t.Error("State not copied")
		}
		if copy.Error != original.Error {
			t.Error("Error not copied")
		}

		// Verify metadata is copied
		if copy.Metadata["key"] != "value" {
			t.Error("Metadata not copied")
		}

		// Verify it's a deep copy by modifying original
		original.Metadata["key"] = "modified"
		if copy.Metadata["key"] != "value" {
			t.Error("Metadata should be deep copied")
		}

		// Verify pointer fields are copied
		if copy.StartedAt == nil || !copy.StartedAt.Equal(*original.StartedAt) {
			t.Error("StartedAt not copied correctly")
		}
		if copy.CompletedAt == nil || !copy.CompletedAt.Equal(*original.CompletedAt) {
			t.Error("CompletedAt not copied correctly")
		}
	})

	t.Run("copy nil workflow", func(t *testing.T) {
		copy := copyWorkflow(nil)
		if copy != nil {
			t.Error("Copy of nil should be nil")
		}
	})

	t.Run("copy with nil pointers", func(t *testing.T) {
		original := &Workflow{
			ID:    "test-1",
			State: StateCreated,
		}

		copy := copyWorkflow(original)

		if copy.StartedAt != nil {
			t.Error("StartedAt should be nil")
		}
		if copy.CompletedAt != nil {
			t.Error("CompletedAt should be nil")
		}
	})
}
