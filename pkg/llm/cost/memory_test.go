package cost

import (
	"context"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/llm"
)

func TestMemoryStore_StoreAndGet(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	record := llm.CostRecord{
		RequestID: "req-123",
		RunID:     "run-456",
		Provider:  "anthropic",
		Model:     "claude-3-opus-20240229",
		Timestamp: time.Now(),
		Usage: llm.TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
		},
		Cost: &llm.CostInfo{
			Amount:   0.0525,
			Currency: "USD",
			Accuracy: llm.CostMeasured,
			Source:   llm.SourcePricingTable,
		},
	}

	// Store record
	err := store.Store(ctx, record)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Retrieve by request ID to get the stored record with ID
	retrieved, err := store.GetByRequestID(ctx, "req-123")
	if err != nil {
		t.Fatalf("GetByRequestID() error = %v", err)
	}

	// Record should have generated ID
	if retrieved.ID == "" {
		t.Error("expected ID to be generated")
	}

	// Retrieve by ID
	byID, err := store.GetByID(ctx, retrieved.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if byID.RequestID != record.RequestID {
		t.Errorf("RequestID = %v, want %v", byID.RequestID, record.RequestID)
	}

	// Update retrieved for final check
	retrieved = byID

	if retrieved.RequestID != record.RequestID {
		t.Errorf("RequestID = %v, want %v", retrieved.RequestID, record.RequestID)
	}
}

func TestMemoryStore_GetByRequestID(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	record := llm.CostRecord{
		RequestID: "req-unique-123",
		Provider:  "openai",
		Model:     "gpt-4o",
		Timestamp: time.Now(),
	}

	err := store.Store(ctx, record)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Retrieve by request ID
	retrieved, err := store.GetByRequestID(ctx, "req-unique-123")
	if err != nil {
		t.Fatalf("GetByRequestID() error = %v", err)
	}

	if retrieved.Provider != "openai" {
		t.Errorf("Provider = %v, want openai", retrieved.Provider)
	}

	// Non-existent request ID
	_, err = store.GetByRequestID(ctx, "non-existent")
	if err == nil {
		t.Error("expected error for non-existent request ID")
	}
}

func TestMemoryStore_GetByRunID(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	runID := "run-789"

	// Store multiple records for same run
	for i := 0; i < 3; i++ {
		record := llm.CostRecord{
			RequestID: string(rune('a' + i)),
			RunID:     runID,
			Provider:  "anthropic",
			Timestamp: time.Now(),
		}
		if err := store.Store(ctx, record); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	// Store record for different run
	otherRecord := llm.CostRecord{
		RequestID: "other",
		RunID:     "run-999",
		Provider:  "anthropic",
		Timestamp: time.Now(),
	}
	if err := store.Store(ctx, otherRecord); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Get records for specific run
	records, err := store.GetByRunID(ctx, runID)
	if err != nil {
		t.Fatalf("GetByRunID() error = %v", err)
	}

	if len(records) != 3 {
		t.Errorf("got %d records, want 3", len(records))
	}

	for _, r := range records {
		if r.RunID != runID {
			t.Errorf("RunID = %v, want %v", r.RunID, runID)
		}
	}
}

func TestMemoryStore_GetByTimeRange(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)

	records := []llm.CostRecord{
		{RequestID: "1", Timestamp: lastWeek},
		{RequestID: "2", Timestamp: yesterday},
		{RequestID: "3", Timestamp: now},
	}

	for _, r := range records {
		if err := store.Store(ctx, r); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	// Get records from yesterday to now
	start := yesterday.Add(-time.Hour) // Slightly before yesterday
	end := now.Add(time.Hour)         // Slightly after now

	results, err := store.GetByTimeRange(ctx, start, end)
	if err != nil {
		t.Fatalf("GetByTimeRange() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("got %d records, want 2 (yesterday and now)", len(results))
	}
}

func TestMemoryStore_Aggregate(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Store test records
	records := []llm.CostRecord{
		{
			RequestID: "1",
			Provider:  "anthropic",
			Model:     "claude-3-opus-20240229",
			Timestamp: time.Now(),
			Usage:     llm.TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
			Cost:      &llm.CostInfo{Amount: 0.01, Accuracy: llm.CostMeasured},
		},
		{
			RequestID: "2",
			Provider:  "anthropic",
			Model:     "claude-3-opus-20240229",
			Timestamp: time.Now(),
			Usage:     llm.TokenUsage{PromptTokens: 200, CompletionTokens: 100, TotalTokens: 300},
			Cost:      &llm.CostInfo{Amount: 0.02, Accuracy: llm.CostMeasured},
		},
		{
			RequestID: "3",
			Provider:  "openai",
			Model:     "gpt-4o",
			Timestamp: time.Now(),
			Usage:     llm.TokenUsage{PromptTokens: 150, CompletionTokens: 75, TotalTokens: 225},
			Cost:      &llm.CostInfo{Amount: 0.015, Accuracy: llm.CostEstimated},
		},
	}

	for _, r := range records {
		if err := store.Store(ctx, r); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	// Aggregate all records
	agg, err := store.Aggregate(ctx, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}

	if agg.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", agg.TotalRequests)
	}

	expectedCost := 0.045 // 0.01 + 0.02 + 0.015
	if agg.TotalCost != expectedCost {
		t.Errorf("TotalCost = %f, want %f", agg.TotalCost, expectedCost)
	}

	expectedTokens := 675 // 150 + 300 + 225
	if agg.TotalTokens != expectedTokens {
		t.Errorf("TotalTokens = %d, want %d", agg.TotalTokens, expectedTokens)
	}

	// Check accuracy breakdown
	if agg.AccuracyBreakdown.Measured != 2 {
		t.Errorf("Measured count = %d, want 2", agg.AccuracyBreakdown.Measured)
	}
	if agg.AccuracyBreakdown.Estimated != 1 {
		t.Errorf("Estimated count = %d, want 1", agg.AccuracyBreakdown.Estimated)
	}

	// Overall accuracy should be estimated (mixed)
	if agg.Accuracy != llm.CostEstimated {
		t.Errorf("Accuracy = %v, want %v", agg.Accuracy, llm.CostEstimated)
	}
}

func TestMemoryStore_AggregateByProvider(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Store records for different providers
	records := []llm.CostRecord{
		{
			RequestID: "1",
			Provider:  "anthropic",
			Usage:     llm.TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
			Cost:      &llm.CostInfo{Amount: 0.01, Accuracy: llm.CostMeasured},
		},
		{
			RequestID: "2",
			Provider:  "anthropic",
			Usage:     llm.TokenUsage{PromptTokens: 200, CompletionTokens: 100, TotalTokens: 300},
			Cost:      &llm.CostInfo{Amount: 0.02, Accuracy: llm.CostMeasured},
		},
		{
			RequestID: "3",
			Provider:  "openai",
			Usage:     llm.TokenUsage{PromptTokens: 150, CompletionTokens: 75, TotalTokens: 225},
			Cost:      &llm.CostInfo{Amount: 0.015, Accuracy: llm.CostMeasured},
		},
	}

	for _, r := range records {
		if err := store.Store(ctx, r); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	// Aggregate by provider
	aggs, err := store.AggregateByProvider(ctx, AggregateOptions{})
	if err != nil {
		t.Fatalf("AggregateByProvider() error = %v", err)
	}

	if len(aggs) != 2 {
		t.Errorf("got %d providers, want 2", len(aggs))
	}

	// Check Anthropic aggregate
	anthAgg, exists := aggs["anthropic"]
	if !exists {
		t.Fatal("expected anthropic in aggregates")
	}
	if anthAgg.TotalRequests != 2 {
		t.Errorf("Anthropic TotalRequests = %d, want 2", anthAgg.TotalRequests)
	}
	if anthAgg.TotalCost != 0.03 {
		t.Errorf("Anthropic TotalCost = %f, want 0.03", anthAgg.TotalCost)
	}

	// Check OpenAI aggregate
	openAgg, exists := aggs["openai"]
	if !exists {
		t.Fatal("expected openai in aggregates")
	}
	if openAgg.TotalRequests != 1 {
		t.Errorf("OpenAI TotalRequests = %d, want 1", openAgg.TotalRequests)
	}
}

func TestMemoryStore_DeleteOlderThan(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	now := time.Now()
	old := now.Add(-48 * time.Hour)
	recent := now.Add(-1 * time.Hour)

	records := []llm.CostRecord{
		{RequestID: "old-1", Timestamp: old},
		{RequestID: "old-2", Timestamp: old},
		{RequestID: "recent", Timestamp: recent},
	}

	for _, r := range records {
		if err := store.Store(ctx, r); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	// Delete records older than 24 hours
	deleted, err := store.DeleteOlderThan(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("DeleteOlderThan() error = %v", err)
	}

	if deleted != 2 {
		t.Errorf("deleted %d records, want 2", deleted)
	}

	// Verify only recent record remains
	agg, err := store.Aggregate(ctx, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}

	if agg.TotalRequests != 1 {
		t.Errorf("TotalRequests after deletion = %d, want 1", agg.TotalRequests)
	}
}

func TestMemoryStore_FilterRecords(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	records := []llm.CostRecord{
		{
			RequestID:  "1",
			Provider:   "anthropic",
			Model:      "claude-3-opus-20240229",
			WorkflowID: "workflow-1",
			UserID:     "user-1",
			RunID:      "run-1",
			Timestamp:  yesterday,
			Cost:       &llm.CostInfo{Amount: 0.01, Accuracy: llm.CostMeasured},
		},
		{
			RequestID:  "2",
			Provider:   "openai",
			Model:      "gpt-4o",
			WorkflowID: "workflow-2",
			UserID:     "user-2",
			RunID:      "run-2",
			Timestamp:  now,
			Cost:       &llm.CostInfo{Amount: 0.02, Accuracy: llm.CostMeasured},
		},
	}

	for _, r := range records {
		if err := store.Store(ctx, r); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	tests := []struct {
		name       string
		opts       AggregateOptions
		wantCount  int
	}{
		{
			name:      "filter by provider",
			opts:      AggregateOptions{Provider: "anthropic"},
			wantCount: 1,
		},
		{
			name:      "filter by model",
			opts:      AggregateOptions{Model: "gpt-4o"},
			wantCount: 1,
		},
		{
			name:      "filter by workflow",
			opts:      AggregateOptions{WorkflowID: "workflow-1"},
			wantCount: 1,
		},
		{
			name:      "filter by user",
			opts:      AggregateOptions{UserID: "user-2"},
			wantCount: 1,
		},
		{
			name:      "filter by run",
			opts:      AggregateOptions{RunID: "run-1"},
			wantCount: 1,
		},
		{
			name: "filter by time range",
			opts: AggregateOptions{
				StartTime: &yesterday,
				EndTime:   &now,
			},
			wantCount: 1, // Only yesterday record
		},
		{
			name:      "no filters",
			opts:      AggregateOptions{},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg, err := store.Aggregate(ctx, tt.opts)
			if err != nil {
				t.Fatalf("Aggregate() error = %v", err)
			}

			if agg.TotalRequests != tt.wantCount {
				t.Errorf("TotalRequests = %d, want %d", agg.TotalRequests, tt.wantCount)
			}
		})
	}
}
