package cost

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tombee/conductor/pkg/llm"
)

// MemoryStore is an in-memory implementation of CostStore for CLI mode.
// It does not persist data between runs but provides fast, lock-free access.
type MemoryStore struct {
	mu      sync.RWMutex
	records []llm.CostRecord
	byID    map[string]*llm.CostRecord
}

// NewMemoryStore creates a new in-memory cost store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records: make([]llm.CostRecord, 0),
		byID:    make(map[string]*llm.CostRecord),
	}
}

// Store saves a cost record in memory.
func (s *MemoryStore) Store(ctx context.Context, record llm.CostRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID if not set
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	s.records = append(s.records, record)
	s.byID[record.ID] = &s.records[len(s.records)-1]

	return nil
}

// GetByID retrieves a cost record by its ID.
func (s *MemoryStore) GetByID(ctx context.Context, id string) (*llm.CostRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.byID[id]
	if !exists {
		return nil, fmt.Errorf("cost record not found: %s", id)
	}

	// Return a copy to prevent external modification
	recordCopy := *record
	return &recordCopy, nil
}

// GetByRequestID retrieves a cost record by request ID.
func (s *MemoryStore) GetByRequestID(ctx context.Context, requestID string) (*llm.CostRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.records {
		if s.records[i].RequestID == requestID {
			recordCopy := s.records[i]
			return &recordCopy, nil
		}
	}

	return nil, fmt.Errorf("cost record not found for request: %s", requestID)
}

// GetByRunID retrieves all cost records for a specific run.
func (s *MemoryStore) GetByRunID(ctx context.Context, runID string) ([]llm.CostRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []llm.CostRecord
	for _, record := range s.records {
		if record.RunID == runID {
			results = append(results, record)
		}
	}

	return results, nil
}

// GetByWorkflowID retrieves all cost records for a specific workflow.
func (s *MemoryStore) GetByWorkflowID(ctx context.Context, workflowID string) ([]llm.CostRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []llm.CostRecord
	for _, record := range s.records {
		if record.WorkflowID == workflowID {
			results = append(results, record)
		}
	}

	return results, nil
}

// GetByUserID retrieves all cost records for a specific user.
func (s *MemoryStore) GetByUserID(ctx context.Context, userID string) ([]llm.CostRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []llm.CostRecord
	for _, record := range s.records {
		if record.UserID == userID {
			results = append(results, record)
		}
	}

	return results, nil
}

// GetByProvider retrieves all cost records for a specific provider.
func (s *MemoryStore) GetByProvider(ctx context.Context, provider string) ([]llm.CostRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []llm.CostRecord
	for _, record := range s.records {
		if record.Provider == provider {
			results = append(results, record)
		}
	}

	return results, nil
}

// GetByModel retrieves all cost records for a specific model.
func (s *MemoryStore) GetByModel(ctx context.Context, model string) ([]llm.CostRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []llm.CostRecord
	for _, record := range s.records {
		if record.Model == model {
			results = append(results, record)
		}
	}

	return results, nil
}

// GetByTimeRange retrieves cost records within a time range.
func (s *MemoryStore) GetByTimeRange(ctx context.Context, start, end time.Time) ([]llm.CostRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []llm.CostRecord
	for _, record := range s.records {
		if (record.Timestamp.Equal(start) || record.Timestamp.After(start)) &&
			record.Timestamp.Before(end) {
			results = append(results, record)
		}
	}

	return results, nil
}

// Aggregate computes aggregated cost statistics based on filter options.
func (s *MemoryStore) Aggregate(ctx context.Context, opts AggregateOptions) (*llm.CostAggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := s.filterRecords(opts)
	return aggregateRecords(filtered), nil
}

// AggregateByProvider returns aggregates grouped by provider.
func (s *MemoryStore) AggregateByProvider(ctx context.Context, opts AggregateOptions) (map[string]llm.CostAggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := s.filterRecords(opts)
	result := make(map[string]llm.CostAggregate)

	// Group by provider
	byProvider := make(map[string][]llm.CostRecord)
	for _, record := range filtered {
		byProvider[record.Provider] = append(byProvider[record.Provider], record)
	}

	// Aggregate each group
	for provider, records := range byProvider {
		result[provider] = *aggregateRecords(records)
	}

	return result, nil
}

// AggregateByModel returns aggregates grouped by model.
func (s *MemoryStore) AggregateByModel(ctx context.Context, opts AggregateOptions) (map[string]llm.CostAggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := s.filterRecords(opts)
	result := make(map[string]llm.CostAggregate)

	// Group by model
	byModel := make(map[string][]llm.CostRecord)
	for _, record := range filtered {
		byModel[record.Model] = append(byModel[record.Model], record)
	}

	// Aggregate each group
	for model, records := range byModel {
		result[model] = *aggregateRecords(records)
	}

	return result, nil
}

// AggregateByWorkflow returns aggregates grouped by workflow.
func (s *MemoryStore) AggregateByWorkflow(ctx context.Context, opts AggregateOptions) (map[string]llm.CostAggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := s.filterRecords(opts)
	result := make(map[string]llm.CostAggregate)

	// Group by workflow
	byWorkflow := make(map[string][]llm.CostRecord)
	for _, record := range filtered {
		if record.WorkflowID != "" {
			byWorkflow[record.WorkflowID] = append(byWorkflow[record.WorkflowID], record)
		}
	}

	// Aggregate each group
	for workflow, records := range byWorkflow {
		result[workflow] = *aggregateRecords(records)
	}

	return result, nil
}

// DeleteOlderThan removes records older than the specified duration.
func (s *MemoryStore) DeleteOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-age)
	var kept []llm.CostRecord
	deleted := int64(0)

	for _, record := range s.records {
		if record.Timestamp.After(cutoff) {
			kept = append(kept, record)
		} else {
			delete(s.byID, record.ID)
			deleted++
		}
	}

	s.records = kept
	return deleted, nil
}

// Close closes the memory store (no-op for in-memory).
func (s *MemoryStore) Close() error {
	return nil
}

// filterRecords applies AggregateOptions filters to records.
// Must be called with read lock held.
func (s *MemoryStore) filterRecords(opts AggregateOptions) []llm.CostRecord {
	var filtered []llm.CostRecord

	for _, record := range s.records {
		// Time range filter
		if opts.StartTime != nil && record.Timestamp.Before(*opts.StartTime) {
			continue
		}
		if opts.EndTime != nil && !record.Timestamp.Before(*opts.EndTime) {
			continue
		}

		// Provider filter
		if opts.Provider != "" && record.Provider != opts.Provider {
			continue
		}

		// Model filter
		if opts.Model != "" && record.Model != opts.Model {
			continue
		}

		// Workflow filter
		if opts.WorkflowID != "" && record.WorkflowID != opts.WorkflowID {
			continue
		}

		// User filter
		if opts.UserID != "" && record.UserID != opts.UserID {
			continue
		}

		// Run filter
		if opts.RunID != "" && record.RunID != opts.RunID {
			continue
		}

		filtered = append(filtered, record)
	}

	return filtered
}

// aggregateRecords computes a CostAggregate from a slice of records.
func aggregateRecords(records []llm.CostRecord) *llm.CostAggregate {
	agg := &llm.CostAggregate{}

	for _, record := range records {
		// Sum cost if available
		if record.Cost != nil {
			agg.TotalCost += record.Cost.Amount

			// Track accuracy breakdown
			switch record.Cost.Accuracy {
			case llm.CostMeasured:
				agg.AccuracyBreakdown.Measured++
			case llm.CostEstimated:
				agg.AccuracyBreakdown.Estimated++
			case llm.CostUnavailable:
				agg.AccuracyBreakdown.Unavailable++
			}
		} else {
			agg.AccuracyBreakdown.Unavailable++
		}

		agg.TotalRequests++
		agg.TotalTokens += record.Usage.TotalTokens
		agg.TotalPromptTokens += record.Usage.PromptTokens
		agg.TotalCompletionTokens += record.Usage.CompletionTokens
		agg.TotalCacheCreationTokens += record.Usage.CacheCreationTokens
		agg.TotalCacheReadTokens += record.Usage.CacheReadTokens
	}

	// Determine overall accuracy
	agg.Accuracy = determineAccuracy(agg.AccuracyBreakdown)

	return agg
}

// determineAccuracy calculates overall accuracy from breakdown.
func determineAccuracy(breakdown llm.AccuracyBreakdown) llm.CostAccuracy {
	total := breakdown.Measured + breakdown.Estimated + breakdown.Unavailable

	if total == 0 {
		return llm.CostUnavailable
	}

	// All one type
	if breakdown.Measured == total {
		return llm.CostMeasured
	}
	if breakdown.Estimated == total {
		return llm.CostEstimated
	}
	if breakdown.Unavailable == total {
		return llm.CostUnavailable
	}

	// Mixed types - use "estimated" as conservative choice
	return llm.CostEstimated
}
