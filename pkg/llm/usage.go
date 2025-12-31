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

package llm

import (
	"fmt"
	"sync"
	"time"
)

// UsageRecord tracks token usage for a single LLM request.
type UsageRecord struct {
	// RequestID uniquely identifies the provider request.
	RequestID string

	// RunID is the conductor run ID.
	RunID string

	// StepName is the step that made this request.
	StepName string

	// WorkflowID is the workflow definition ID.
	WorkflowID string

	// Provider is the name of the provider that handled the request.
	Provider string

	// Model is the model ID used for the request.
	Model string

	// Timestamp is when the request was made.
	Timestamp time.Time

	// Duration is how long the request took.
	Duration time.Duration

	// Usage contains token consumption information.
	Usage TokenUsage
}

// UsageTracker tracks LLM token usage.
type UsageTracker struct {
	mu      sync.RWMutex
	records []UsageRecord
}

// NewUsageTracker creates a new usage tracker.
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		records: make([]UsageRecord, 0),
	}
}

// Track records token usage for an LLM request.
func (t *UsageTracker) Track(record UsageRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, record)
}

// GetRecords returns all usage records.
func (t *UsageTracker) GetRecords() []UsageRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	records := make([]UsageRecord, len(t.records))
	copy(records, t.records)
	return records
}

// AggregateByProvider calculates total usage by provider.
func (t *UsageTracker) AggregateByProvider() map[string]UsageAggregate {
	t.mu.RLock()
	defer t.mu.RUnlock()

	aggregates := make(map[string]UsageAggregate)
	for _, record := range t.records {
		agg := aggregates[record.Provider]
		agg.TotalRequests++
		agg.TotalTokens += record.Usage.TotalTokens
		agg.TotalPromptTokens += record.Usage.PromptTokens
		agg.TotalCompletionTokens += record.Usage.CompletionTokens
		agg.TotalCacheCreationTokens += record.Usage.CacheCreationTokens
		agg.TotalCacheReadTokens += record.Usage.CacheReadTokens
		aggregates[record.Provider] = agg
	}
	return aggregates
}

// AggregateByModel calculates total usage by model.
func (t *UsageTracker) AggregateByModel() map[string]UsageAggregate {
	t.mu.RLock()
	defer t.mu.RUnlock()

	aggregates := make(map[string]UsageAggregate)
	for _, record := range t.records {
		agg := aggregates[record.Model]
		agg.TotalRequests++
		agg.TotalTokens += record.Usage.TotalTokens
		agg.TotalPromptTokens += record.Usage.PromptTokens
		agg.TotalCompletionTokens += record.Usage.CompletionTokens
		agg.TotalCacheCreationTokens += record.Usage.CacheCreationTokens
		agg.TotalCacheReadTokens += record.Usage.CacheReadTokens
		aggregates[record.Model] = agg
	}
	return aggregates
}

// Clear removes all usage records.
func (t *UsageTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = make([]UsageRecord, 0)
}

// UsageAggregate contains aggregated token usage statistics.
type UsageAggregate struct {
	// TotalRequests is the number of requests.
	TotalRequests int

	// TotalTokens is the sum of all tokens used.
	TotalTokens int

	// TotalPromptTokens is the sum of all prompt tokens.
	TotalPromptTokens int

	// TotalCompletionTokens is the sum of all completion tokens.
	TotalCompletionTokens int

	// TotalCacheCreationTokens is the sum of all cache creation tokens.
	TotalCacheCreationTokens int

	// TotalCacheReadTokens is the sum of all cache read tokens.
	TotalCacheReadTokens int
}

// FormatTokens formats a token count for display.
func FormatTokens(tokens int) string {
	if tokens >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	}
	if tokens >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1_000)
	}
	return fmt.Sprintf("%d", tokens)
}

// globalUsageTracker is the default global usage tracker instance.
var globalUsageTracker = NewUsageTracker()

// TrackUsage records usage in the global tracker.
func TrackUsage(record UsageRecord) {
	globalUsageTracker.Track(record)
}

// GetUsageRecords returns all records from the global tracker.
func GetUsageRecords() []UsageRecord {
	return globalUsageTracker.GetRecords()
}

// AggregateUsageByProvider returns aggregated usage by provider.
func AggregateUsageByProvider() map[string]UsageAggregate {
	return globalUsageTracker.AggregateByProvider()
}

// AggregateUsageByModel returns aggregated usage by model.
func AggregateUsageByModel() map[string]UsageAggregate {
	return globalUsageTracker.AggregateByModel()
}

// ClearUsageRecords clears all records from the global tracker.
func ClearUsageRecords() {
	globalUsageTracker.Clear()
}
