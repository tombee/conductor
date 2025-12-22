package llm

import (
	"sync"
	"time"
)

// CostRecord tracks the cost of a single LLM request.
type CostRecord struct {
	// RequestID uniquely identifies the request.
	RequestID string

	// Provider is the name of the provider that handled the request.
	Provider string

	// Model is the model ID used for the request.
	Model string

	// Timestamp is when the request was made.
	Timestamp time.Time

	// Usage contains token consumption information.
	Usage TokenUsage

	// Cost is the calculated cost in USD.
	Cost float64

	// Metadata contains additional tracking information (correlation IDs, etc).
	Metadata map[string]string
}

// CostTracker tracks LLM request costs with correlation IDs.
// It supports aggregation by provider, model, and time period.
type CostTracker struct {
	mu      sync.RWMutex
	records []CostRecord
}

// NewCostTracker creates a new cost tracker.
func NewCostTracker() *CostTracker {
	return &CostTracker{
		records: make([]CostRecord, 0),
	}
}

// Track records a cost for an LLM request.
func (t *CostTracker) Track(record CostRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, record)
}

// GetRecords returns all cost records.
func (t *CostTracker) GetRecords() []CostRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	records := make([]CostRecord, len(t.records))
	copy(records, t.records)
	return records
}

// GetRecordsByProvider returns all records for a specific provider.
func (t *CostTracker) GetRecordsByProvider(provider string) []CostRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var filtered []CostRecord
	for _, record := range t.records {
		if record.Provider == provider {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

// GetRecordsByModel returns all records for a specific model.
func (t *CostTracker) GetRecordsByModel(model string) []CostRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var filtered []CostRecord
	for _, record := range t.records {
		if record.Model == model {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

// GetRecordsByTimeRange returns all records within a time range.
func (t *CostTracker) GetRecordsByTimeRange(start, end time.Time) []CostRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var filtered []CostRecord
	for _, record := range t.records {
		if record.Timestamp.After(start) && record.Timestamp.Before(end) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

// GetRecordByRequestID returns a specific record by request ID.
func (t *CostTracker) GetRecordByRequestID(requestID string) *CostRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, record := range t.records {
		if record.RequestID == requestID {
			return &record
		}
	}
	return nil
}

// AggregateByProvider calculates total cost and usage by provider.
func (t *CostTracker) AggregateByProvider() map[string]CostAggregate {
	t.mu.RLock()
	defer t.mu.RUnlock()

	aggregates := make(map[string]CostAggregate)
	for _, record := range t.records {
		agg := aggregates[record.Provider]
		agg.TotalCost += record.Cost
		agg.TotalRequests++
		agg.TotalTokens += record.Usage.TotalTokens
		agg.TotalPromptTokens += record.Usage.PromptTokens
		agg.TotalCompletionTokens += record.Usage.CompletionTokens
		aggregates[record.Provider] = agg
	}
	return aggregates
}

// AggregateByModel calculates total cost and usage by model.
func (t *CostTracker) AggregateByModel() map[string]CostAggregate {
	t.mu.RLock()
	defer t.mu.RUnlock()

	aggregates := make(map[string]CostAggregate)
	for _, record := range t.records {
		agg := aggregates[record.Model]
		agg.TotalCost += record.Cost
		agg.TotalRequests++
		agg.TotalTokens += record.Usage.TotalTokens
		agg.TotalPromptTokens += record.Usage.PromptTokens
		agg.TotalCompletionTokens += record.Usage.CompletionTokens
		aggregates[record.Model] = agg
	}
	return aggregates
}

// AggregateByTimePeriod calculates total cost and usage for a time period.
func (t *CostTracker) AggregateByTimePeriod(start, end time.Time) CostAggregate {
	records := t.GetRecordsByTimeRange(start, end)

	var agg CostAggregate
	for _, record := range records {
		agg.TotalCost += record.Cost
		agg.TotalRequests++
		agg.TotalTokens += record.Usage.TotalTokens
		agg.TotalPromptTokens += record.Usage.PromptTokens
		agg.TotalCompletionTokens += record.Usage.CompletionTokens
	}
	return agg
}

// Clear removes all cost records.
func (t *CostTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = make([]CostRecord, 0)
}

// CostAggregate contains aggregated cost and usage statistics.
type CostAggregate struct {
	// TotalCost is the sum of all costs in USD.
	TotalCost float64

	// TotalRequests is the number of requests.
	TotalRequests int

	// TotalTokens is the sum of all tokens used.
	TotalTokens int

	// TotalPromptTokens is the sum of all prompt tokens.
	TotalPromptTokens int

	// TotalCompletionTokens is the sum of all completion tokens.
	TotalCompletionTokens int
}

// globalCostTracker is the default global cost tracker instance.
var globalCostTracker = NewCostTracker()

// TrackCost records a cost in the global tracker.
func TrackCost(record CostRecord) {
	globalCostTracker.Track(record)
}

// GetCostRecords returns all records from the global tracker.
func GetCostRecords() []CostRecord {
	return globalCostTracker.GetRecords()
}

// GetCostRecordsByProvider returns records for a provider from the global tracker.
func GetCostRecordsByProvider(provider string) []CostRecord {
	return globalCostTracker.GetRecordsByProvider(provider)
}

// GetCostRecordsByModel returns records for a model from the global tracker.
func GetCostRecordsByModel(model string) []CostRecord {
	return globalCostTracker.GetRecordsByModel(model)
}

// GetCostRecordsByTimeRange returns records in a time range from the global tracker.
func GetCostRecordsByTimeRange(start, end time.Time) []CostRecord {
	return globalCostTracker.GetRecordsByTimeRange(start, end)
}

// GetCostRecordByRequestID returns a specific record from the global tracker.
func GetCostRecordByRequestID(requestID string) *CostRecord {
	return globalCostTracker.GetRecordByRequestID(requestID)
}

// AggregateCostByProvider returns aggregated costs by provider from the global tracker.
func AggregateCostByProvider() map[string]CostAggregate {
	return globalCostTracker.AggregateByProvider()
}

// AggregateCostByModel returns aggregated costs by model from the global tracker.
func AggregateCostByModel() map[string]CostAggregate {
	return globalCostTracker.AggregateByModel()
}

// AggregateCostByTimePeriod returns aggregated costs for a time period from the global tracker.
func AggregateCostByTimePeriod(start, end time.Time) CostAggregate {
	return globalCostTracker.AggregateByTimePeriod(start, end)
}

// ClearCostRecords clears all records from the global tracker.
func ClearCostRecords() {
	globalCostTracker.Clear()
}
