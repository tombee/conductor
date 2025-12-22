package operation

import (
	"strings"
	"testing"
	"time"
)

func TestMetricsCollector_RecordRequest(t *testing.T) {
	collector := NewMetricsCollector()

	// Record some requests
	collector.RecordRequest("github", "create_issue", 201, 150*time.Millisecond)
	collector.RecordRequest("github", "create_issue", 201, 200*time.Millisecond)
	collector.RecordRequest("github", "list_repos", 200, 100*time.Millisecond)
	collector.RecordRequest("slack", "post_message", 200, 50*time.Millisecond)

	metrics := collector.GetMetrics()

	// Check operation-level counts
	if metrics.RequestsByOperation["github"] != 3 {
		t.Errorf("Expected 3 github requests, got %d", metrics.RequestsByOperation["github"])
	}
	if metrics.RequestsByOperation["slack"] != 1 {
		t.Errorf("Expected 1 slack request, got %d", metrics.RequestsByOperation["slack"])
	}

	// Check operation type counts
	if metrics.RequestsByOperationType["github"]["create_issue"] != 2 {
		t.Errorf("Expected 2 create_issue operations, got %d", metrics.RequestsByOperationType["github"]["create_issue"])
	}
	if metrics.RequestsByOperationType["github"]["list_repos"] != 1 {
		t.Errorf("Expected 1 list_repos operation, got %d", metrics.RequestsByOperationType["github"]["list_repos"])
	}

	// Check status codes
	if metrics.RequestsByStatus["github"][201] != 2 {
		t.Errorf("Expected 2 requests with status 201, got %d", metrics.RequestsByStatus["github"][201])
	}
	if metrics.RequestsByStatus["github"][200] != 1 {
		t.Errorf("Expected 1 request with status 200, got %d", metrics.RequestsByStatus["github"][200])
	}

	// Check durations are tracked
	if len(metrics.DurationsByOperation["github"]) != 3 {
		t.Errorf("Expected 3 duration entries for github, got %d", len(metrics.DurationsByOperation["github"]))
	}
	if len(metrics.DurationsByOperationType["github"]["create_issue"]) != 2 {
		t.Errorf("Expected 2 duration entries for create_issue, got %d", len(metrics.DurationsByOperationType["github"]["create_issue"]))
	}
}

func TestMetricsCollector_RecordRateLimitWait(t *testing.T) {
	collector := NewMetricsCollector()

	// Record rate limit waits
	collector.RecordRateLimitWait("github", 1*time.Second)
	collector.RecordRateLimitWait("github", 2*time.Second)
	collector.RecordRateLimitWait("slack", 500*time.Millisecond)

	metrics := collector.GetMetrics()

	// Check wait counts
	if metrics.RateLimitWaitsByOperation["github"] != 2 {
		t.Errorf("Expected 2 rate limit waits for github, got %d", metrics.RateLimitWaitsByOperation["github"])
	}
	if metrics.RateLimitWaitsByOperation["slack"] != 1 {
		t.Errorf("Expected 1 rate limit wait for slack, got %d", metrics.RateLimitWaitsByOperation["slack"])
	}

	// Check wait durations
	expectedGithubDuration := 3 * time.Second
	if metrics.RateLimitWaitDuration["github"] != expectedGithubDuration {
		t.Errorf("Expected github wait duration %v, got %v", expectedGithubDuration, metrics.RateLimitWaitDuration["github"])
	}
}

func TestMetricsCollector_Reset(t *testing.T) {
	collector := NewMetricsCollector()

	// Record some data
	collector.RecordRequest("github", "create_issue", 201, 150*time.Millisecond)
	collector.RecordRateLimitWait("github", 1*time.Second)

	// Reset
	collector.Reset()

	metrics := collector.GetMetrics()

	// Verify everything is reset
	if len(metrics.RequestsByOperation) != 0 {
		t.Errorf("Expected empty RequestsByOperation after reset, got %d entries", len(metrics.RequestsByOperation))
	}
	if len(metrics.RateLimitWaitsByOperation) != 0 {
		t.Errorf("Expected empty RateLimitWaitsByOperation after reset, got %d entries", len(metrics.RateLimitWaitsByOperation))
	}
}

func TestPrometheusExporter_Export(t *testing.T) {
	collector := NewMetricsCollector()

	// Record some requests
	collector.RecordRequest("github", "create_issue", 201, 150*time.Millisecond)
	collector.RecordRequest("github", "create_issue", 201, 200*time.Millisecond)
	collector.RecordRequest("github", "list_repos", 200, 100*time.Millisecond)
	collector.RecordRateLimitWait("github", 1*time.Second)

	exporter := NewPrometheusExporter(collector)
	output := exporter.Export()

	// Verify output contains expected metrics
	expectedMetrics := []string{
		"conductor_operation_requests_total",
		"conductor_operation_request_duration_seconds",
		"conductor_operation_rate_limit_waits_total",
		"conductor_operation_rate_limit_wait_duration_seconds",
		"conductor_operation_last_event_timestamp_seconds",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(output, metric) {
			t.Errorf("Expected output to contain %q", metric)
		}
	}

	// Verify labels are present
	if !strings.Contains(output, `operation="github"`) {
		t.Error("Expected output to contain github operation label")
	}
	if !strings.Contains(output, `operation_type="create_issue"`) {
		t.Error("Expected output to contain create_issue operation_type label")
	}
	if !strings.Contains(output, `status="201"`) {
		t.Error("Expected output to contain status label")
	}

	// Verify counter values
	if !strings.Contains(output, `conductor_operation_requests_total{operation="github",operation_type="create_issue"} 2`) {
		t.Error("Expected create_issue count of 2")
	}
	if !strings.Contains(output, `conductor_operation_rate_limit_waits_total{operation="github"} 1`) {
		t.Error("Expected 1 rate limit wait")
	}
}

func TestPrometheusExporter_DurationMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	// Record requests with known durations
	collector.RecordRequest("github", "create_issue", 201, 100*time.Millisecond)
	collector.RecordRequest("github", "create_issue", 201, 200*time.Millisecond)

	exporter := NewPrometheusExporter(collector)
	output := exporter.Export()

	// Check for histogram metrics
	if !strings.Contains(output, "conductor_operation_request_duration_seconds_sum") {
		t.Error("Expected duration sum metric")
	}
	if !strings.Contains(output, "conductor_operation_request_duration_seconds_count") {
		t.Error("Expected duration count metric")
	}

	// Verify count
	if !strings.Contains(output, `conductor_operation_request_duration_seconds_count{operation="github",operation_type="create_issue"} 2`) {
		t.Error("Expected count of 2 for duration metric")
	}

	// Verify sum is approximately 0.3 seconds (100ms + 200ms)
	if !strings.Contains(output, `conductor_operation_request_duration_seconds_sum{operation="github",operation_type="create_issue"} 0.3`) {
		t.Error("Expected sum of approximately 0.3 seconds")
	}
}

func TestMetricsCollector_DurationBufferLimit(t *testing.T) {
	collector := NewMetricsCollector()

	// Record more than 1000 requests to test buffer limit
	for i := 0; i < 1100; i++ {
		collector.RecordRequest("github", "test", 200, time.Millisecond)
	}

	metrics := collector.GetMetrics()

	// Verify buffer is limited to 1000 entries
	if len(metrics.DurationsByOperation["github"]) > 1000 {
		t.Errorf("Expected max 1000 duration entries, got %d", len(metrics.DurationsByOperation["github"]))
	}
	if len(metrics.DurationsByOperationType["github"]["test"]) > 1000 {
		t.Errorf("Expected max 1000 operation duration entries, got %d", len(metrics.DurationsByOperationType["github"]["test"]))
	}
}

func TestMetricsCollector_Concurrent(t *testing.T) {
	collector := NewMetricsCollector()

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				collector.RecordRequest("github", "test", 200, time.Millisecond)
				collector.RecordRateLimitWait("github", time.Millisecond)
				collector.GetMetrics()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	metrics := collector.GetMetrics()

	// Verify we got all 1000 requests (10 goroutines * 100 requests)
	if metrics.RequestsByOperation["github"] != 1000 {
		t.Errorf("Expected 1000 requests, got %d", metrics.RequestsByOperation["github"])
	}
	if metrics.RateLimitWaitsByOperation["github"] != 1000 {
		t.Errorf("Expected 1000 rate limit waits, got %d", metrics.RateLimitWaitsByOperation["github"])
	}
}
