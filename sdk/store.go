package sdk

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Store defines the interface for workflow state persistence.
// Implement a custom Store to persist run history, build audit logs,
// or export execution data to monitoring systems.
type Store interface {
	// SaveRun persists workflow execution state
	SaveRun(ctx context.Context, run *Run) error

	// GetRun retrieves workflow execution state
	GetRun(ctx context.Context, runID string) (*Run, error)

	// ListRuns returns runs matching the filter with pagination
	ListRuns(ctx context.Context, filter RunFilter) ([]*Run, error)
}

// RunFilter specifies criteria for listing runs.
type RunFilter struct {
	WorkflowID   string     // Filter by workflow ID (optional)
	Status       RunStatus  // Filter by status (optional)
	StartedAfter *time.Time // Filter by start time (optional)
	Limit        int        // Max results (default 100, max 10000)
	Offset       int        // Pagination offset (default 0)
}

// Run represents a workflow execution instance.
type Run struct {
	ID          string
	WorkflowID  string
	Status      RunStatus
	Steps       map[string]*StepResult
	StartedAt   time.Time
	CompletedAt *time.Time
}

// RunStatus indicates the current state of a workflow run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// inMemoryStore implements Store with in-memory storage.
// It limits history to 1000 most recent runs.
type inMemoryStore struct {
	mu       sync.RWMutex
	runs     map[string]*Run
	runOrder []string // Ordered list for LRU eviction
	maxRuns  int
}

// newInMemoryStore creates a new in-memory store.
func newInMemoryStore() *inMemoryStore {
	return &inMemoryStore{
		runs:     make(map[string]*Run),
		runOrder: make([]string, 0, 1000),
		maxRuns:  1000,
	}
}

func (s *inMemoryStore) SaveRun(ctx context.Context, run *Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if run already exists
	if _, exists := s.runs[run.ID]; !exists {
		// New run - add to order
		s.runOrder = append(s.runOrder, run.ID)

		// Evict oldest run if we've hit the limit
		if len(s.runOrder) > s.maxRuns {
			oldestID := s.runOrder[0]
			delete(s.runs, oldestID)
			s.runOrder = s.runOrder[1:]
		}
	}

	// Save or update run
	s.runs[run.ID] = run
	return nil
}

func (s *inMemoryStore) GetRun(ctx context.Context, runID string) (*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, exists := s.runs[runID]
	if !exists {
		return nil, fmt.Errorf("run not found: %s", runID)
	}

	return run, nil
}

func (s *inMemoryStore) ListRuns(ctx context.Context, filter RunFilter) ([]*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Apply defaults
	limit := filter.Limit
	if limit == 0 {
		limit = 100
	}
	if limit > 10000 {
		limit = 10000
	}

	// Collect matching runs
	var matches []*Run
	for _, run := range s.runs {
		// Apply filters
		if filter.WorkflowID != "" && run.WorkflowID != filter.WorkflowID {
			continue
		}
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		if filter.StartedAfter != nil && run.StartedAt.Before(*filter.StartedAfter) {
			continue
		}

		matches = append(matches, run)
	}

	// Apply offset and limit
	start := filter.Offset
	if start >= len(matches) {
		return []*Run{}, nil
	}

	end := start + limit
	if end > len(matches) {
		end = len(matches)
	}

	return matches[start:end], nil
}
