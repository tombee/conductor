package operation

import (
	"fmt"
	"sync"
	"time"
)

// Metrics tracks operation execution metrics for observability.
type Metrics struct {
	// Request counters by operation name, operation type, and status
	RequestsByOperation     map[string]int64
	RequestsByOperationType map[string]map[string]int64 // operation -> operation type -> count
	RequestsByStatus        map[string]map[int]int64    // operation -> status code -> count

	// Duration tracking
	DurationsByOperation     map[string][]time.Duration // For calculating percentiles
	DurationsByOperationType map[string]map[string][]time.Duration

	// Rate limit waits
	RateLimitWaitsByOperation map[string]int64
	RateLimitWaitDuration     map[string]time.Duration

	// Last event timestamp
	LastEventTime time.Time
}

// MetricsCollector collects and exports operation metrics.
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics *Metrics
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: &Metrics{
			RequestsByOperation:       make(map[string]int64),
			RequestsByOperationType:   make(map[string]map[string]int64),
			RequestsByStatus:          make(map[string]map[int]int64),
			DurationsByOperation:      make(map[string][]time.Duration),
			DurationsByOperationType:  make(map[string]map[string][]time.Duration),
			RateLimitWaitsByOperation: make(map[string]int64),
			RateLimitWaitDuration:     make(map[string]time.Duration),
			LastEventTime:             time.Now(),
		},
	}
}

// RecordRequest records an operation request execution.
func (m *MetricsCollector) RecordRequest(operationName, operationType string, statusCode int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.LastEventTime = time.Now()

	// Increment request counters
	m.metrics.RequestsByOperation[operationName]++

	if m.metrics.RequestsByOperationType[operationName] == nil {
		m.metrics.RequestsByOperationType[operationName] = make(map[string]int64)
	}
	m.metrics.RequestsByOperationType[operationName][operationType]++

	if m.metrics.RequestsByStatus[operationName] == nil {
		m.metrics.RequestsByStatus[operationName] = make(map[int]int64)
	}
	m.metrics.RequestsByStatus[operationName][statusCode]++

	// Track durations (keep last 1000 for percentile calculation)
	m.metrics.DurationsByOperation[operationName] = append(m.metrics.DurationsByOperation[operationName], duration)
	if len(m.metrics.DurationsByOperation[operationName]) > 1000 {
		m.metrics.DurationsByOperation[operationName] = m.metrics.DurationsByOperation[operationName][1:]
	}

	if m.metrics.DurationsByOperationType[operationName] == nil {
		m.metrics.DurationsByOperationType[operationName] = make(map[string][]time.Duration)
	}
	m.metrics.DurationsByOperationType[operationName][operationType] = append(m.metrics.DurationsByOperationType[operationName][operationType], duration)
	if len(m.metrics.DurationsByOperationType[operationName][operationType]) > 1000 {
		m.metrics.DurationsByOperationType[operationName][operationType] = m.metrics.DurationsByOperationType[operationName][operationType][1:]
	}
}

// RecordRateLimitWait records a rate limit wait event.
func (m *MetricsCollector) RecordRateLimitWait(operationName string, waitDuration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.LastEventTime = time.Now()
	m.metrics.RateLimitWaitsByOperation[operationName]++
	m.metrics.RateLimitWaitDuration[operationName] += waitDuration
}

// GetMetrics returns a snapshot of current metrics.
func (m *MetricsCollector) GetMetrics() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deep copy to avoid race conditions
	snapshot := Metrics{
		RequestsByOperation:       make(map[string]int64),
		RequestsByOperationType:   make(map[string]map[string]int64),
		RequestsByStatus:          make(map[string]map[int]int64),
		DurationsByOperation:      make(map[string][]time.Duration),
		DurationsByOperationType:  make(map[string]map[string][]time.Duration),
		RateLimitWaitsByOperation: make(map[string]int64),
		RateLimitWaitDuration:     make(map[string]time.Duration),
		LastEventTime:             m.metrics.LastEventTime,
	}

	for k, v := range m.metrics.RequestsByOperation {
		snapshot.RequestsByOperation[k] = v
	}

	for k, v := range m.metrics.RequestsByOperationType {
		snapshot.RequestsByOperationType[k] = make(map[string]int64)
		for k2, v2 := range v {
			snapshot.RequestsByOperationType[k][k2] = v2
		}
	}

	for k, v := range m.metrics.RequestsByStatus {
		snapshot.RequestsByStatus[k] = make(map[int]int64)
		for k2, v2 := range v {
			snapshot.RequestsByStatus[k][k2] = v2
		}
	}

	for k, v := range m.metrics.RateLimitWaitsByOperation {
		snapshot.RateLimitWaitsByOperation[k] = v
	}

	for k, v := range m.metrics.RateLimitWaitDuration {
		snapshot.RateLimitWaitDuration[k] = v
	}

	// Copy duration slices
	for k, v := range m.metrics.DurationsByOperation {
		snapshot.DurationsByOperation[k] = append([]time.Duration{}, v...)
	}

	for k, v := range m.metrics.DurationsByOperationType {
		snapshot.DurationsByOperationType[k] = make(map[string][]time.Duration)
		for k2, v2 := range v {
			snapshot.DurationsByOperationType[k][k2] = append([]time.Duration{}, v2...)
		}
	}

	return snapshot
}

// Reset resets all metrics (useful for testing).
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = &Metrics{
		RequestsByOperation:       make(map[string]int64),
		RequestsByOperationType:   make(map[string]map[string]int64),
		RequestsByStatus:          make(map[string]map[int]int64),
		DurationsByOperation:      make(map[string][]time.Duration),
		DurationsByOperationType:  make(map[string]map[string][]time.Duration),
		RateLimitWaitsByOperation: make(map[string]int64),
		RateLimitWaitDuration:     make(map[string]time.Duration),
		LastEventTime:             time.Now(),
	}
}

// PrometheusExporter exports operation metrics in Prometheus format.
type PrometheusExporter struct {
	collector *MetricsCollector
}

// NewPrometheusExporter creates a Prometheus metrics exporter.
func NewPrometheusExporter(collector *MetricsCollector) *PrometheusExporter {
	return &PrometheusExporter{
		collector: collector,
	}
}

// Export returns metrics in Prometheus text format.
func (e *PrometheusExporter) Export() string {
	metrics := e.collector.GetMetrics()

	var output string

	// conductor_operation_requests_total{operation="github",operation_type="create_issue",status="200"}
	output += "# HELP conductor_operation_requests_total Total number of operation requests\n"
	output += "# TYPE conductor_operation_requests_total counter\n"
	for operationName, operationTypes := range metrics.RequestsByOperationType {
		for operationType, count := range operationTypes {
			// Get status breakdown for this operation/operation_type
			if statuses, ok := metrics.RequestsByStatus[operationName]; ok {
				for status, statusCount := range statuses {
					output += fmt.Sprintf("conductor_operation_requests_total{operation=%q,operation_type=%q,status=\"%d\"} %d\n",
						operationName, operationType, status, statusCount)
				}
			}
			// Also emit total for this operation/operation_type
			output += fmt.Sprintf("conductor_operation_requests_total{operation=%q,operation_type=%q} %d\n",
				operationName, operationType, count)
		}
	}
	output += "\n"

	// conductor_operation_request_duration_seconds{operation="github",operation_type="create_issue"}
	output += "# HELP conductor_operation_request_duration_seconds Operation request duration in seconds\n"
	output += "# TYPE conductor_operation_request_duration_seconds histogram\n"
	for operationName, operationTypes := range metrics.DurationsByOperationType {
		for operationType, durations := range operationTypes {
			if len(durations) > 0 {
				// Calculate summary statistics
				sum, count := calculateDurationStats(durations)
				output += fmt.Sprintf("conductor_operation_request_duration_seconds_sum{operation=%q,operation_type=%q} %.6f\n",
					operationName, operationType, sum)
				output += fmt.Sprintf("conductor_operation_request_duration_seconds_count{operation=%q,operation_type=%q} %d\n",
					operationName, operationType, count)
			}
		}
	}
	output += "\n"

	// conductor_operation_rate_limit_waits_total{operation="github"}
	output += "# HELP conductor_operation_rate_limit_waits_total Total number of rate limit waits\n"
	output += "# TYPE conductor_operation_rate_limit_waits_total counter\n"
	for operationName, count := range metrics.RateLimitWaitsByOperation {
		output += fmt.Sprintf("conductor_operation_rate_limit_waits_total{operation=%q} %d\n",
			operationName, count)
	}
	output += "\n"

	// conductor_operation_rate_limit_wait_duration_seconds{operation="github"}
	output += "# HELP conductor_operation_rate_limit_wait_duration_seconds Total duration spent waiting for rate limits\n"
	output += "# TYPE conductor_operation_rate_limit_wait_duration_seconds counter\n"
	for operationName, duration := range metrics.RateLimitWaitDuration {
		output += fmt.Sprintf("conductor_operation_rate_limit_wait_duration_seconds{operation=%q} %.6f\n",
			operationName, duration.Seconds())
	}
	output += "\n"

	// Last event timestamp
	output += "# HELP conductor_operation_last_event_timestamp_seconds Timestamp of last operation event\n"
	output += "# TYPE conductor_operation_last_event_timestamp_seconds gauge\n"
	output += fmt.Sprintf("conductor_operation_last_event_timestamp_seconds %d\n", metrics.LastEventTime.Unix())

	return output
}

// calculateDurationStats calculates sum and count for durations.
func calculateDurationStats(durations []time.Duration) (float64, int64) {
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum.Seconds(), int64(len(durations))
}
