package connector

import (
	"fmt"
	"sync"
	"time"
)

// Metrics tracks connector execution metrics for observability.
type Metrics struct {
	// Request counters by connector, operation, and status
	RequestsByConnector map[string]int64
	RequestsByOperation map[string]map[string]int64 // connector -> operation -> count
	RequestsByStatus    map[string]map[int]int64    // connector -> status code -> count

	// Duration tracking
	DurationsByConnector map[string][]time.Duration // For calculating percentiles
	DurationsByOperation map[string]map[string][]time.Duration

	// Rate limit waits
	RateLimitWaitsByConnector map[string]int64
	RateLimitWaitDuration     map[string]time.Duration

	// Last event timestamp
	LastEventTime time.Time
}

// MetricsCollector collects and exports connector metrics.
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics *Metrics
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: &Metrics{
			RequestsByConnector:       make(map[string]int64),
			RequestsByOperation:       make(map[string]map[string]int64),
			RequestsByStatus:          make(map[string]map[int]int64),
			DurationsByConnector:      make(map[string][]time.Duration),
			DurationsByOperation:      make(map[string]map[string][]time.Duration),
			RateLimitWaitsByConnector: make(map[string]int64),
			RateLimitWaitDuration:     make(map[string]time.Duration),
			LastEventTime:             time.Now(),
		},
	}
}

// RecordRequest records a connector request execution.
func (m *MetricsCollector) RecordRequest(connector, operation string, statusCode int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.LastEventTime = time.Now()

	// Increment request counters
	m.metrics.RequestsByConnector[connector]++

	if m.metrics.RequestsByOperation[connector] == nil {
		m.metrics.RequestsByOperation[connector] = make(map[string]int64)
	}
	m.metrics.RequestsByOperation[connector][operation]++

	if m.metrics.RequestsByStatus[connector] == nil {
		m.metrics.RequestsByStatus[connector] = make(map[int]int64)
	}
	m.metrics.RequestsByStatus[connector][statusCode]++

	// Track durations (keep last 1000 for percentile calculation)
	m.metrics.DurationsByConnector[connector] = append(m.metrics.DurationsByConnector[connector], duration)
	if len(m.metrics.DurationsByConnector[connector]) > 1000 {
		m.metrics.DurationsByConnector[connector] = m.metrics.DurationsByConnector[connector][1:]
	}

	if m.metrics.DurationsByOperation[connector] == nil {
		m.metrics.DurationsByOperation[connector] = make(map[string][]time.Duration)
	}
	m.metrics.DurationsByOperation[connector][operation] = append(m.metrics.DurationsByOperation[connector][operation], duration)
	if len(m.metrics.DurationsByOperation[connector][operation]) > 1000 {
		m.metrics.DurationsByOperation[connector][operation] = m.metrics.DurationsByOperation[connector][operation][1:]
	}
}

// RecordRateLimitWait records a rate limit wait event.
func (m *MetricsCollector) RecordRateLimitWait(connector string, waitDuration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.LastEventTime = time.Now()
	m.metrics.RateLimitWaitsByConnector[connector]++
	m.metrics.RateLimitWaitDuration[connector] += waitDuration
}

// GetMetrics returns a snapshot of current metrics.
func (m *MetricsCollector) GetMetrics() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deep copy to avoid race conditions
	snapshot := Metrics{
		RequestsByConnector:       make(map[string]int64),
		RequestsByOperation:       make(map[string]map[string]int64),
		RequestsByStatus:          make(map[string]map[int]int64),
		DurationsByConnector:      make(map[string][]time.Duration),
		DurationsByOperation:      make(map[string]map[string][]time.Duration),
		RateLimitWaitsByConnector: make(map[string]int64),
		RateLimitWaitDuration:     make(map[string]time.Duration),
		LastEventTime:             m.metrics.LastEventTime,
	}

	for k, v := range m.metrics.RequestsByConnector {
		snapshot.RequestsByConnector[k] = v
	}

	for k, v := range m.metrics.RequestsByOperation {
		snapshot.RequestsByOperation[k] = make(map[string]int64)
		for k2, v2 := range v {
			snapshot.RequestsByOperation[k][k2] = v2
		}
	}

	for k, v := range m.metrics.RequestsByStatus {
		snapshot.RequestsByStatus[k] = make(map[int]int64)
		for k2, v2 := range v {
			snapshot.RequestsByStatus[k][k2] = v2
		}
	}

	for k, v := range m.metrics.RateLimitWaitsByConnector {
		snapshot.RateLimitWaitsByConnector[k] = v
	}

	for k, v := range m.metrics.RateLimitWaitDuration {
		snapshot.RateLimitWaitDuration[k] = v
	}

	// Copy duration slices
	for k, v := range m.metrics.DurationsByConnector {
		snapshot.DurationsByConnector[k] = append([]time.Duration{}, v...)
	}

	for k, v := range m.metrics.DurationsByOperation {
		snapshot.DurationsByOperation[k] = make(map[string][]time.Duration)
		for k2, v2 := range v {
			snapshot.DurationsByOperation[k][k2] = append([]time.Duration{}, v2...)
		}
	}

	return snapshot
}

// Reset resets all metrics (useful for testing).
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = &Metrics{
		RequestsByConnector:       make(map[string]int64),
		RequestsByOperation:       make(map[string]map[string]int64),
		RequestsByStatus:          make(map[string]map[int]int64),
		DurationsByConnector:      make(map[string][]time.Duration),
		DurationsByOperation:      make(map[string]map[string][]time.Duration),
		RateLimitWaitsByConnector: make(map[string]int64),
		RateLimitWaitDuration:     make(map[string]time.Duration),
		LastEventTime:             time.Now(),
	}
}

// PrometheusExporter exports connector metrics in Prometheus format.
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

	// conductor_connector_requests_total{connector="github",operation="create_issue",status="200"}
	output += "# HELP conductor_connector_requests_total Total number of connector requests\n"
	output += "# TYPE conductor_connector_requests_total counter\n"
	for connector, operations := range metrics.RequestsByOperation {
		for operation, count := range operations {
			// Get status breakdown for this connector/operation
			if statuses, ok := metrics.RequestsByStatus[connector]; ok {
				for status, statusCount := range statuses {
					output += fmt.Sprintf("conductor_connector_requests_total{connector=%q,operation=%q,status=\"%d\"} %d\n",
						connector, operation, status, statusCount)
				}
			}
			// Also emit total for this connector/operation
			output += fmt.Sprintf("conductor_connector_requests_total{connector=%q,operation=%q} %d\n",
				connector, operation, count)
		}
	}
	output += "\n"

	// conductor_connector_request_duration_seconds{connector="github",operation="create_issue"}
	output += "# HELP conductor_connector_request_duration_seconds Connector request duration in seconds\n"
	output += "# TYPE conductor_connector_request_duration_seconds histogram\n"
	for connector, operations := range metrics.DurationsByOperation {
		for operation, durations := range operations {
			if len(durations) > 0 {
				// Calculate summary statistics
				sum, count := calculateDurationStats(durations)
				output += fmt.Sprintf("conductor_connector_request_duration_seconds_sum{connector=%q,operation=%q} %.6f\n",
					connector, operation, sum)
				output += fmt.Sprintf("conductor_connector_request_duration_seconds_count{connector=%q,operation=%q} %d\n",
					connector, operation, count)
			}
		}
	}
	output += "\n"

	// conductor_connector_rate_limit_waits_total{connector="github"}
	output += "# HELP conductor_connector_rate_limit_waits_total Total number of rate limit waits\n"
	output += "# TYPE conductor_connector_rate_limit_waits_total counter\n"
	for connector, count := range metrics.RateLimitWaitsByConnector {
		output += fmt.Sprintf("conductor_connector_rate_limit_waits_total{connector=%q} %d\n",
			connector, count)
	}
	output += "\n"

	// conductor_connector_rate_limit_wait_duration_seconds{connector="github"}
	output += "# HELP conductor_connector_rate_limit_wait_duration_seconds Total duration spent waiting for rate limits\n"
	output += "# TYPE conductor_connector_rate_limit_wait_duration_seconds counter\n"
	for connector, duration := range metrics.RateLimitWaitDuration {
		output += fmt.Sprintf("conductor_connector_rate_limit_wait_duration_seconds{connector=%q} %.6f\n",
			connector, duration.Seconds())
	}
	output += "\n"

	// Last event timestamp
	output += "# HELP conductor_connector_last_event_timestamp_seconds Timestamp of last connector event\n"
	output += "# TYPE conductor_connector_last_event_timestamp_seconds gauge\n"
	output += fmt.Sprintf("conductor_connector_last_event_timestamp_seconds %d\n", metrics.LastEventTime.Unix())

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
