package polltrigger

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsCollector collects Prometheus-compatible metrics for poll triggers.
type MetricsCollector struct {
	meter metric.Meter

	// Counters
	pollsTotal  metric.Int64Counter
	eventsTotal metric.Int64Counter
	errorsTotal metric.Int64Counter

	// Histograms
	pollLatency metric.Float64Histogram

	// Gauges (using observable gauges)
	activeTriggers   int64
	activeTriggersMu sync.RWMutex
}

// NewMetricsCollector creates a new poll trigger metrics collector.
func NewMetricsCollector(meterProvider metric.MeterProvider) (*MetricsCollector, error) {
	meter := meterProvider.Meter("conductor")

	mc := &MetricsCollector{
		meter: meter,
	}

	var err error

	// Initialize counters
	mc.pollsTotal, err = meter.Int64Counter(
		"conductor_poll_trigger_polls_total",
		metric.WithDescription("Total number of poll trigger executions"),
		metric.WithUnit("{poll}"),
	)
	if err != nil {
		return nil, err
	}

	mc.eventsTotal, err = meter.Int64Counter(
		"conductor_poll_trigger_events_total",
		metric.WithDescription("Total number of events detected by poll triggers"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return nil, err
	}

	mc.errorsTotal, err = meter.Int64Counter(
		"conductor_poll_trigger_errors_total",
		metric.WithDescription("Total number of poll trigger errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize histograms
	mc.pollLatency, err = meter.Float64Histogram(
		"conductor_poll_trigger_latency_seconds",
		metric.WithDescription("Poll trigger execution latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize observable gauges
	_, err = meter.Int64ObservableGauge(
		"conductor_poll_trigger_active",
		metric.WithDescription("Number of active poll triggers"),
		metric.WithUnit("{trigger}"),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			mc.activeTriggersMu.RLock()
			count := mc.activeTriggers
			mc.activeTriggersMu.RUnlock()
			observer.Observe(count)
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	return mc, nil
}

// RecordPollStart records the start of a poll execution.
func (mc *MetricsCollector) RecordPollStart(ctx context.Context, integration string) {
	// Poll starts are tracked implicitly via RecordPollComplete
}

// RecordPollComplete records the completion of a poll execution.
func (mc *MetricsCollector) RecordPollComplete(ctx context.Context, integration string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}

	attrs := []attribute.KeyValue{
		attribute.String("integration", integration),
		attribute.String("status", status),
	}

	mc.pollsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	mc.pollLatency.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordEvents records the number of events detected in a poll.
func (mc *MetricsCollector) RecordEvents(ctx context.Context, integration string, eventType string, count int) {
	attrs := []attribute.KeyValue{
		attribute.String("integration", integration),
		attribute.String("event_type", eventType),
	}

	mc.eventsTotal.Add(ctx, int64(count), metric.WithAttributes(attrs...))
}

// RecordError records a poll trigger error.
func (mc *MetricsCollector) RecordError(ctx context.Context, integration string, errorType string) {
	attrs := []attribute.KeyValue{
		attribute.String("integration", integration),
		attribute.String("error_type", errorType),
	}

	mc.errorsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// SetActiveTriggers sets the count of active poll triggers.
func (mc *MetricsCollector) SetActiveTriggers(count int) {
	mc.activeTriggersMu.Lock()
	mc.activeTriggers = int64(count)
	mc.activeTriggersMu.Unlock()
}
