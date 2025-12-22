package llm

import (
	"sync"
	"time"
)

// CostAccuracy indicates reliability of cost value.
type CostAccuracy string

const (
	// CostMeasured indicates provider reported exact token count.
	CostMeasured CostAccuracy = "measured"

	// CostEstimated indicates cost calculated from published pricing.
	CostEstimated CostAccuracy = "estimated"

	// CostUnavailable indicates insufficient data for cost calculation.
	CostUnavailable CostAccuracy = "unavailable"
)

// CostInfo contains cost details with accuracy tracking.
type CostInfo struct {
	// Amount is the cost in the specified currency.
	Amount float64

	// Currency is the currency code (always "USD" for now).
	Currency string

	// Accuracy indicates how reliable this cost value is.
	Accuracy CostAccuracy

	// Source indicates where this cost came from.
	Source string
}

// Common cost sources.
const (
	// SourceProvider indicates cost from provider API usage data.
	SourceProvider = "provider"

	// SourcePricingTable indicates cost calculated from local pricing config.
	SourcePricingTable = "pricing_table"

	// SourceEstimated indicates cost approximated via tokenizer.
	SourceEstimated = "estimated"
)

// CostRecord tracks the cost of a single LLM request.
type CostRecord struct {
	// ID is a unique record identifier.
	ID string

	// RequestID uniquely identifies the provider request.
	RequestID string

	// RunID is the conductor run ID.
	RunID string

	// StepName is the step that made this request.
	StepName string

	// WorkflowID is the workflow definition ID.
	WorkflowID string

	// UserID is the user who triggered this cost.
	UserID string

	// Provider is the name of the provider that handled the request.
	Provider string

	// Model is the model ID used for the request.
	Model string

	// ActualProvider tracks the provider that actually handled the request (for failover).
	ActualProvider string

	// Timestamp is when the request was made.
	Timestamp time.Time

	// Duration is how long the request took.
	Duration time.Duration

	// Usage contains token consumption information.
	Usage TokenUsage

	// Cost contains cost information with accuracy tracking.
	// nil if cost unavailable.
	Cost *CostInfo

	// Classification indicates data sensitivity level.
	Classification string

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

		// Sum cost if available
		if record.Cost != nil {
			agg.TotalCost += record.Cost.Amount

			// Track accuracy breakdown
			switch record.Cost.Accuracy {
			case CostMeasured:
				agg.AccuracyBreakdown.Measured++
			case CostEstimated:
				agg.AccuracyBreakdown.Estimated++
			case CostUnavailable:
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

		// Determine overall accuracy
		agg.Accuracy = determineAccuracy(agg.AccuracyBreakdown)

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

		// Sum cost if available
		if record.Cost != nil {
			agg.TotalCost += record.Cost.Amount

			// Track accuracy breakdown
			switch record.Cost.Accuracy {
			case CostMeasured:
				agg.AccuracyBreakdown.Measured++
			case CostEstimated:
				agg.AccuracyBreakdown.Estimated++
			case CostUnavailable:
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

		// Determine overall accuracy
		agg.Accuracy = determineAccuracy(agg.AccuracyBreakdown)

		aggregates[record.Model] = agg
	}
	return aggregates
}

// AggregateByTimePeriod calculates total cost and usage for a time period.
func (t *CostTracker) AggregateByTimePeriod(start, end time.Time) CostAggregate {
	records := t.GetRecordsByTimeRange(start, end)

	var agg CostAggregate
	for _, record := range records {
		// Sum cost if available
		if record.Cost != nil {
			agg.TotalCost += record.Cost.Amount

			// Track accuracy breakdown
			switch record.Cost.Accuracy {
			case CostMeasured:
				agg.AccuracyBreakdown.Measured++
			case CostEstimated:
				agg.AccuracyBreakdown.Estimated++
			case CostUnavailable:
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
// Returns "measured" if all are measured, "unavailable" if all are unavailable,
// "mixed" for combinations, or specific type if only one type present.
func determineAccuracy(breakdown AccuracyBreakdown) CostAccuracy {
	total := breakdown.Measured + breakdown.Estimated + breakdown.Unavailable

	// No records
	if total == 0 {
		return CostUnavailable
	}

	// All one type
	if breakdown.Measured == total {
		return CostMeasured
	}
	if breakdown.Estimated == total {
		return CostEstimated
	}
	if breakdown.Unavailable == total {
		return CostUnavailable
	}

	// Mixed types - use "estimated" as the conservative choice
	// (treating mixed as estimated since not all values are measured)
	return CostEstimated
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

	// TotalCacheCreationTokens is the sum of all cache creation tokens.
	TotalCacheCreationTokens int

	// TotalCacheReadTokens is the sum of all cache read tokens.
	TotalCacheReadTokens int

	// Accuracy indicates the overall accuracy of aggregated costs.
	// "measured" if all costs are measured, "mixed" if combination, "unavailable" if none.
	Accuracy CostAccuracy

	// AccuracyBreakdown shows count of requests by accuracy level.
	AccuracyBreakdown AccuracyBreakdown
}

// AccuracyBreakdown tracks count of requests by accuracy level.
type AccuracyBreakdown struct {
	// Measured is count of requests with measured costs.
	Measured int

	// Estimated is count of requests with estimated costs.
	Estimated int

	// Unavailable is count of requests with unavailable costs.
	Unavailable int
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
