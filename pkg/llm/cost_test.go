package llm

import (
	"testing"
	"time"
)

// Helper to create a cost info for tests
func testCost(amount float64) *CostInfo {
	return &CostInfo{
		Amount:   amount,
		Currency: "USD",
		Accuracy: CostMeasured,
		Source:   SourceProvider,
	}
}

func TestCostTracker_Track(t *testing.T) {
	tracker := NewCostTracker()

	record := CostRecord{
		RequestID: "req-123",
		Provider:  "anthropic",
		Model:     "claude-3-5-sonnet-20241022",
		Timestamp: time.Now(),
		Usage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		Cost: testCost(0.0045),
		Metadata: map[string]string{
			"correlation_id": "corr-456",
		},
	}

	tracker.Track(record)

	records := tracker.GetRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].RequestID != "req-123" {
		t.Errorf("expected request ID req-123, got %s", records[0].RequestID)
	}
}

func TestCostTracker_GetRecordsByProvider(t *testing.T) {
	tracker := NewCostTracker()

	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Cost:      testCost(0.001),
	})

	tracker.Track(CostRecord{
		RequestID: "req-2",
		Provider:  "openai",
		Model:     "gpt-4",
		Timestamp: time.Now(),
		Cost:      testCost(0.002),
	})

	tracker.Track(CostRecord{
		RequestID: "req-3",
		Provider:  "anthropic",
		Model:     "claude-3-5-sonnet-20241022",
		Timestamp: time.Now(),
		Cost:      testCost(0.003),
	})

	anthropicRecords := tracker.GetRecordsByProvider("anthropic")
	if len(anthropicRecords) != 2 {
		t.Fatalf("expected 2 anthropic records, got %d", len(anthropicRecords))
	}

	openaiRecords := tracker.GetRecordsByProvider("openai")
	if len(openaiRecords) != 1 {
		t.Fatalf("expected 1 openai record, got %d", len(openaiRecords))
	}
}

func TestCostTracker_GetRecordsByModel(t *testing.T) {
	tracker := NewCostTracker()

	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Cost:      testCost(0.001),
	})

	tracker.Track(CostRecord{
		RequestID: "req-2",
		Provider:  "anthropic",
		Model:     "claude-3-5-sonnet-20241022",
		Timestamp: time.Now(),
		Cost:      testCost(0.002),
	})

	tracker.Track(CostRecord{
		RequestID: "req-3",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Cost:      testCost(0.003),
	})

	haikuRecords := tracker.GetRecordsByModel("claude-3-5-haiku-20241022")
	if len(haikuRecords) != 2 {
		t.Fatalf("expected 2 haiku records, got %d", len(haikuRecords))
	}

	sonnetRecords := tracker.GetRecordsByModel("claude-3-5-sonnet-20241022")
	if len(sonnetRecords) != 1 {
		t.Fatalf("expected 1 sonnet record, got %d", len(sonnetRecords))
	}
}

func TestCostTracker_GetRecordsByTimeRange(t *testing.T) {
	tracker := NewCostTracker()

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)

	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: twoHoursAgo,
		Cost:      testCost(0.001),
	})

	tracker.Track(CostRecord{
		RequestID: "req-2",
		Provider:  "anthropic",
		Model:     "claude-3-5-sonnet-20241022",
		Timestamp: oneHourAgo,
		Cost:      testCost(0.002),
	})

	tracker.Track(CostRecord{
		RequestID: "req-3",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: now,
		Cost:      testCost(0.003),
	})

	// Get records from last 90 minutes
	records := tracker.GetRecordsByTimeRange(now.Add(-90*time.Minute), now.Add(1*time.Minute))
	if len(records) != 2 {
		t.Fatalf("expected 2 records in time range, got %d", len(records))
	}
}

func TestCostTracker_GetRecordByRequestID(t *testing.T) {
	tracker := NewCostTracker()

	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Cost:      testCost(0.001),
	})

	tracker.Track(CostRecord{
		RequestID: "req-2",
		Provider:  "anthropic",
		Model:     "claude-3-5-sonnet-20241022",
		Timestamp: time.Now(),
		Cost:      testCost(0.002),
	})

	record := tracker.GetRecordByRequestID("req-2")
	if record == nil {
		t.Fatal("expected to find record, got nil")
	}

	if record.Cost == nil || record.Cost.Amount != 0.002 {
		t.Errorf("expected cost 0.002, got %v", record.Cost)
	}

	missing := tracker.GetRecordByRequestID("req-999")
	if missing != nil {
		t.Error("expected nil for missing record")
	}
}

func TestCostTracker_AggregateByProvider(t *testing.T) {
	tracker := NewCostTracker()

	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Usage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		Cost: testCost(0.001),
	})

	tracker.Track(CostRecord{
		RequestID: "req-2",
		Provider:  "openai",
		Model:     "gpt-4",
		Timestamp: time.Now(),
		Usage: TokenUsage{
			PromptTokens:     200,
			CompletionTokens: 100,
			TotalTokens:      300,
		},
		Cost: testCost(0.002),
	})

	tracker.Track(CostRecord{
		RequestID: "req-3",
		Provider:  "anthropic",
		Model:     "claude-3-5-sonnet-20241022",
		Timestamp: time.Now(),
		Usage: TokenUsage{
			PromptTokens:     150,
			CompletionTokens: 75,
			TotalTokens:      225,
		},
		Cost: testCost(0.003),
	})

	aggregates := tracker.AggregateByProvider()

	anthropicAgg := aggregates["anthropic"]
	if anthropicAgg.TotalCost != 0.004 {
		t.Errorf("expected anthropic total cost 0.004, got %f", anthropicAgg.TotalCost)
	}
	if anthropicAgg.TotalRequests != 2 {
		t.Errorf("expected anthropic 2 requests, got %d", anthropicAgg.TotalRequests)
	}
	if anthropicAgg.TotalTokens != 375 {
		t.Errorf("expected anthropic 375 tokens, got %d", anthropicAgg.TotalTokens)
	}

	openaiAgg := aggregates["openai"]
	if openaiAgg.TotalCost != 0.002 {
		t.Errorf("expected openai total cost 0.002, got %f", openaiAgg.TotalCost)
	}
	if openaiAgg.TotalRequests != 1 {
		t.Errorf("expected openai 1 request, got %d", openaiAgg.TotalRequests)
	}
}

func TestCostTracker_AggregateByModel(t *testing.T) {
	tracker := NewCostTracker()

	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Usage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		Cost: testCost(0.001),
	})

	tracker.Track(CostRecord{
		RequestID: "req-2",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Usage: TokenUsage{
			PromptTokens:     200,
			CompletionTokens: 100,
			TotalTokens:      300,
		},
		Cost: testCost(0.002),
	})

	aggregates := tracker.AggregateByModel()

	haikuAgg := aggregates["claude-3-5-haiku-20241022"]
	if haikuAgg.TotalCost != 0.003 {
		t.Errorf("expected haiku total cost 0.003, got %f", haikuAgg.TotalCost)
	}
	if haikuAgg.TotalRequests != 2 {
		t.Errorf("expected haiku 2 requests, got %d", haikuAgg.TotalRequests)
	}
	if haikuAgg.TotalTokens != 450 {
		t.Errorf("expected haiku 450 tokens, got %d", haikuAgg.TotalTokens)
	}
}

func TestCostTracker_AggregateByTimePeriod(t *testing.T) {
	tracker := NewCostTracker()

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)

	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: twoHoursAgo,
		Usage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		Cost: testCost(0.001),
	})

	tracker.Track(CostRecord{
		RequestID: "req-2",
		Provider:  "anthropic",
		Model:     "claude-3-5-sonnet-20241022",
		Timestamp: oneHourAgo,
		Usage: TokenUsage{
			PromptTokens:     200,
			CompletionTokens: 100,
			TotalTokens:      300,
		},
		Cost: testCost(0.002),
	})

	tracker.Track(CostRecord{
		RequestID: "req-3",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: now,
		Usage: TokenUsage{
			PromptTokens:     150,
			CompletionTokens: 75,
			TotalTokens:      225,
		},
		Cost: testCost(0.003),
	})

	// Aggregate last 90 minutes
	agg := tracker.AggregateByTimePeriod(now.Add(-90*time.Minute), now.Add(1*time.Minute))

	if agg.TotalCost != 0.005 {
		t.Errorf("expected total cost 0.005, got %f", agg.TotalCost)
	}
	if agg.TotalRequests != 2 {
		t.Errorf("expected 2 requests, got %d", agg.TotalRequests)
	}
	if agg.TotalTokens != 525 {
		t.Errorf("expected 525 tokens, got %d", agg.TotalTokens)
	}
}

func TestCostTracker_Clear(t *testing.T) {
	tracker := NewCostTracker()

	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Cost:      testCost(0.001),
	})

	records := tracker.GetRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 record before clear, got %d", len(records))
	}

	tracker.Clear()

	records = tracker.GetRecords()
	if len(records) != 0 {
		t.Fatalf("expected 0 records after clear, got %d", len(records))
	}
}

func TestGlobalCostTracker(t *testing.T) {
	// Clear global tracker before test
	ClearCostRecords()

	TrackCost(CostRecord{
		RequestID: "req-global-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Cost:      testCost(0.001),
	})

	records := GetCostRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 global record, got %d", len(records))
	}

	if records[0].RequestID != "req-global-1" {
		t.Errorf("expected request ID req-global-1, got %s", records[0].RequestID)
	}

	// Clean up
	ClearCostRecords()
}

func TestCostAccuracy_Aggregation(t *testing.T) {
	tracker := NewCostTracker()

	// Add measured cost
	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Cost: &CostInfo{
			Amount:   0.001,
			Currency: "USD",
			Accuracy: CostMeasured,
			Source:   SourceProvider,
		},
	})

	// Add estimated cost
	tracker.Track(CostRecord{
		RequestID: "req-2",
		Provider:  "anthropic",
		Model:     "claude-3-5-haiku-20241022",
		Timestamp: time.Now(),
		Cost: &CostInfo{
			Amount:   0.002,
			Currency: "USD",
			Accuracy: CostEstimated,
			Source:   SourcePricingTable,
		},
	})

	agg := tracker.AggregateByProvider()["anthropic"]

	// Check accuracy breakdown
	if agg.AccuracyBreakdown.Measured != 1 {
		t.Errorf("expected 1 measured, got %d", agg.AccuracyBreakdown.Measured)
	}
	if agg.AccuracyBreakdown.Estimated != 1 {
		t.Errorf("expected 1 estimated, got %d", agg.AccuracyBreakdown.Estimated)
	}

	// Mixed accuracy should return estimated
	if agg.Accuracy != CostEstimated {
		t.Errorf("expected accuracy to be estimated (mixed), got %s", agg.Accuracy)
	}
}

func TestCostAccuracy_AllMeasured(t *testing.T) {
	tracker := NewCostTracker()

	// Add all measured costs
	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "test",
		Timestamp: time.Now(),
		Cost: &CostInfo{
			Amount:   0.001,
			Currency: "USD",
			Accuracy: CostMeasured,
			Source:   SourceProvider,
		},
	})

	tracker.Track(CostRecord{
		RequestID: "req-2",
		Provider:  "anthropic",
		Model:     "test",
		Timestamp: time.Now(),
		Cost: &CostInfo{
			Amount:   0.002,
			Currency: "USD",
			Accuracy: CostMeasured,
			Source:   SourceProvider,
		},
	})

	agg := tracker.AggregateByProvider()["anthropic"]

	if agg.Accuracy != CostMeasured {
		t.Errorf("expected accuracy to be measured, got %s", agg.Accuracy)
	}
}

func TestCacheTokens_Aggregation(t *testing.T) {
	tracker := NewCostTracker()

	tracker.Track(CostRecord{
		RequestID: "req-1",
		Provider:  "anthropic",
		Model:     "claude-3-5-sonnet-20241022",
		Timestamp: time.Now(),
		Usage: TokenUsage{
			PromptTokens:        100,
			CompletionTokens:    50,
			TotalTokens:         150,
			CacheCreationTokens: 20,
			CacheReadTokens:     30,
		},
		Cost: testCost(0.001),
	})

	agg := tracker.AggregateByProvider()["anthropic"]

	if agg.TotalCacheCreationTokens != 20 {
		t.Errorf("expected 20 cache creation tokens, got %d", agg.TotalCacheCreationTokens)
	}
	if agg.TotalCacheReadTokens != 30 {
		t.Errorf("expected 30 cache read tokens, got %d", agg.TotalCacheReadTokens)
	}
}
