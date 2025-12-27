package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tombee/conductor/pkg/errors"
)

// Store defines the interface for workflow persistence.
type Store interface {
	// Create creates a new workflow.
	Create(ctx context.Context, workflow *Workflow) error

	// Get retrieves a workflow by ID.
	Get(ctx context.Context, id string) (*Workflow, error)

	// Update updates an existing workflow.
	Update(ctx context.Context, workflow *Workflow) error

	// Delete deletes a workflow by ID.
	Delete(ctx context.Context, id string) error

	// List returns all workflows matching the query.
	List(ctx context.Context, query *Query) ([]*Workflow, error)
}

// Query defines query parameters for listing workflows.
type Query struct {
	State    *State                 // Filter by state
	Metadata map[string]interface{} // Filter by metadata fields
	Limit    int                    // Maximum number of results (0 = no limit)
	Offset   int                    // Number of results to skip
}

// MemoryStore is an in-memory implementation of Store.
// It is thread-safe and suitable for testing or single-instance deployments.
type MemoryStore struct {
	mu        sync.RWMutex
	workflows map[string]*Workflow
}

// NewMemoryStore creates a new in-memory workflow store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		workflows: make(map[string]*Workflow),
	}
}

// Create creates a new workflow.
func (s *MemoryStore) Create(ctx context.Context, workflow *Workflow) error {
	if workflow == nil {
		return &errors.ValidationError{
			Field:   "workflow",
			Message: "workflow cannot be nil",
		}
	}
	if workflow.ID == "" {
		return &errors.ValidationError{
			Field:   "id",
			Message: "workflow ID cannot be empty",
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if workflow already exists
	if _, exists := s.workflows[workflow.ID]; exists {
		return &errors.ValidationError{
			Field:      "id",
			Message:    fmt.Sprintf("workflow with ID %s already exists", workflow.ID),
			Suggestion: "use a unique workflow ID or call Update instead",
		}
	}

	// Set timestamps
	now := time.Now()
	if workflow.CreatedAt.IsZero() {
		workflow.CreatedAt = now
	}
	if workflow.UpdatedAt.IsZero() {
		workflow.UpdatedAt = now
	}

	// Set default state
	if workflow.State == "" {
		workflow.State = StateCreated
	}

	// Initialize metadata if nil
	if workflow.Metadata == nil {
		workflow.Metadata = make(map[string]interface{})
	}

	// Store a copy to prevent external modifications
	s.workflows[workflow.ID] = copyWorkflow(workflow)

	return nil
}

// Get retrieves a workflow by ID.
func (s *MemoryStore) Get(ctx context.Context, id string) (*Workflow, error) {
	if id == "" {
		return nil, &errors.ValidationError{
			Field:   "id",
			Message: "workflow ID cannot be empty",
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	workflow, exists := s.workflows[id]
	if !exists {
		return nil, &errors.NotFoundError{
			Resource: "workflow",
			ID:       id,
		}
	}

	// Return a copy to prevent external modifications
	return copyWorkflow(workflow), nil
}

// Update updates an existing workflow.
func (s *MemoryStore) Update(ctx context.Context, workflow *Workflow) error {
	if workflow == nil {
		return &errors.ValidationError{
			Field:   "workflow",
			Message: "workflow cannot be nil",
		}
	}
	if workflow.ID == "" {
		return &errors.ValidationError{
			Field:   "id",
			Message: "workflow ID cannot be empty",
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if workflow exists
	if _, exists := s.workflows[workflow.ID]; !exists {
		return &errors.NotFoundError{
			Resource: "workflow",
			ID:       workflow.ID,
		}
	}

	// Update timestamp
	workflow.UpdatedAt = time.Now()

	// Store a copy
	s.workflows[workflow.ID] = copyWorkflow(workflow)

	return nil
}

// Delete deletes a workflow by ID.
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return &errors.ValidationError{
			Field:   "id",
			Message: "workflow ID cannot be empty",
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if workflow exists
	if _, exists := s.workflows[id]; !exists {
		return &errors.NotFoundError{
			Resource: "workflow",
			ID:       id,
		}
	}

	delete(s.workflows, id)

	return nil
}

// List returns all workflows matching the query.
func (s *MemoryStore) List(ctx context.Context, query *Query) ([]*Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Workflow

	// Collect all matching workflows
	for _, workflow := range s.workflows {
		if matchesQuery(workflow, query) {
			results = append(results, copyWorkflow(workflow))
		}
	}

	// Apply offset and limit
	if query != nil {
		if query.Offset > 0 {
			if query.Offset >= len(results) {
				return []*Workflow{}, nil
			}
			results = results[query.Offset:]
		}
		if query.Limit > 0 && len(results) > query.Limit {
			results = results[:query.Limit]
		}
	}

	return results, nil
}

// matchesQuery checks if a workflow matches the query criteria.
func matchesQuery(workflow *Workflow, query *Query) bool {
	if query == nil {
		return true
	}

	// Filter by state
	if query.State != nil && workflow.State != *query.State {
		return false
	}

	// Filter by metadata
	if query.Metadata != nil {
		for key, value := range query.Metadata {
			workflowValue, exists := workflow.Metadata[key]
			if !exists || workflowValue != value {
				return false
			}
		}
	}

	return true
}

// copyWorkflow creates a deep copy of a workflow.
func copyWorkflow(w *Workflow) *Workflow {
	if w == nil {
		return nil
	}

	copy := &Workflow{
		ID:        w.ID,
		Name:      w.Name,
		State:     w.State,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
		Error:     w.Error,
	}

	// Copy pointers
	if w.StartedAt != nil {
		startedAt := *w.StartedAt
		copy.StartedAt = &startedAt
	}
	if w.CompletedAt != nil {
		completedAt := *w.CompletedAt
		copy.CompletedAt = &completedAt
	}

	// Copy metadata map
	if w.Metadata != nil {
		copy.Metadata = make(map[string]interface{}, len(w.Metadata))
		for k, v := range w.Metadata {
			copy.Metadata[k] = v
		}
	}

	return copy
}
