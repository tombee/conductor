package tracing

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsCollector collects Prometheus-compatible metrics for workflow execution
type MetricsCollector struct {
	meter metric.Meter

	// Counters
	runsTotal          metric.Int64Counter
	stepsTotal         metric.Int64Counter
	llmRequestsTotal   metric.Int64Counter
	tokensTotal        metric.Int64Counter
	replayTotal        metric.Int64Counter
	replayCostSavedUSD metric.Float64Counter
	debugEventsTotal   metric.Int64Counter

	// Histograms
	runDuration  metric.Float64Histogram
	stepDuration metric.Float64Histogram
	llmLatency   metric.Float64Histogram

	// Gauges (using observable gauges)
	activeRuns          map[string]bool // Track active run IDs
	activeRunsMu        sync.RWMutex
	queueDepth          int64 // Track pending runs in queue
	queueDepthMu        sync.RWMutex
	totalCostUSD        float64
	totalCostMu         sync.RWMutex
	debugSessionsActive int64 // Track active debug sessions
	debugSessionsMu     sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector using the given meter provider
func NewMetricsCollector(meterProvider metric.MeterProvider) (*MetricsCollector, error) {
	meter := meterProvider.Meter("conductor")

	mc := &MetricsCollector{
		meter:      meter,
		activeRuns: make(map[string]bool),
	}

	var err error

	// Initialize counters
	mc.runsTotal, err = meter.Int64Counter(
		"conductor_runs_total",
		metric.WithDescription("Total number of workflow runs"),
		metric.WithUnit("{run}"),
	)
	if err != nil {
		return nil, err
	}

	mc.stepsTotal, err = meter.Int64Counter(
		"conductor_steps_total",
		metric.WithDescription("Total number of workflow steps executed"),
		metric.WithUnit("{step}"),
	)
	if err != nil {
		return nil, err
	}

	mc.llmRequestsTotal, err = meter.Int64Counter(
		"conductor_llm_requests_total",
		metric.WithDescription("Total number of LLM requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	mc.tokensTotal, err = meter.Int64Counter(
		"conductor_tokens_total",
		metric.WithDescription("Total number of tokens processed"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return nil, err
	}

	mc.replayTotal, err = meter.Int64Counter(
		"conductor_replay_total",
		metric.WithDescription("Total number of workflow replays"),
		metric.WithUnit("{replay}"),
	)
	if err != nil {
		return nil, err
	}

	mc.replayCostSavedUSD, err = meter.Float64Counter(
		"conductor_replay_cost_saved_usd",
		metric.WithDescription("Total cost saved through replay (USD)"),
		metric.WithUnit("USD"),
	)
	if err != nil {
		return nil, err
	}

	mc.debugEventsTotal, err = meter.Int64Counter(
		"conductor_debug_events_total",
		metric.WithDescription("Total number of debug events emitted"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize histograms
	mc.runDuration, err = meter.Float64Histogram(
		"conductor_run_duration_seconds",
		metric.WithDescription("Workflow run duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	mc.stepDuration, err = meter.Float64Histogram(
		"conductor_step_duration_seconds",
		metric.WithDescription("Step execution duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	mc.llmLatency, err = meter.Float64Histogram(
		"conductor_llm_latency_seconds",
		metric.WithDescription("LLM request latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize observable gauges
	_, err = meter.Int64ObservableGauge(
		"conductor_active_runs",
		metric.WithDescription("Number of currently active workflow runs"),
		metric.WithUnit("{run}"),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			mc.activeRunsMu.RLock()
			count := len(mc.activeRuns)
			mc.activeRunsMu.RUnlock()
			observer.Observe(int64(count))
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	_, err = meter.Float64ObservableGauge(
		"conductor_cost_usd",
		metric.WithDescription("Total cost in USD"),
		metric.WithUnit("USD"),
		metric.WithFloat64Callback(func(ctx context.Context, observer metric.Float64Observer) error {
			mc.totalCostMu.RLock()
			cost := mc.totalCostUSD
			mc.totalCostMu.RUnlock()
			observer.Observe(cost)
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	_, err = meter.Int64ObservableGauge(
		"conductor_queue_depth",
		metric.WithDescription("Number of pending workflow runs in queue"),
		metric.WithUnit("{run}"),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			mc.queueDepthMu.RLock()
			depth := mc.queueDepth
			mc.queueDepthMu.RUnlock()
			observer.Observe(depth)
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	_, err = meter.Int64ObservableGauge(
		"conductor_debug_sessions_active",
		metric.WithDescription("Number of active debug sessions"),
		metric.WithUnit("{session}"),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			mc.debugSessionsMu.RLock()
			count := mc.debugSessionsActive
			mc.debugSessionsMu.RUnlock()
			observer.Observe(count)
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	return mc, nil
}

// RecordRunStart records the start of a workflow run
func (mc *MetricsCollector) RecordRunStart(ctx context.Context, runID, workflowID string) {
	mc.activeRunsMu.Lock()
	mc.activeRuns[runID] = true
	mc.activeRunsMu.Unlock()
}

// RecordRunComplete records the completion of a workflow run
func (mc *MetricsCollector) RecordRunComplete(ctx context.Context, runID, workflowID, status, trigger string, duration time.Duration) {
	mc.activeRunsMu.Lock()
	delete(mc.activeRuns, runID)
	mc.activeRunsMu.Unlock()

	attrs := []attribute.KeyValue{
		attribute.String("workflow", workflowID),
		attribute.String("status", status),
		attribute.String("trigger", trigger),
	}

	mc.runsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	mc.runDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordStepComplete records the completion of a workflow step
func (mc *MetricsCollector) RecordStepComplete(ctx context.Context, workflowID, stepName, status string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("workflow", workflowID),
		attribute.String("step", stepName),
		attribute.String("status", status),
	}

	mc.stepsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	mc.stepDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordLLMRequest records an LLM request completion
func (mc *MetricsCollector) RecordLLMRequest(ctx context.Context, provider, model, status string, promptTokens, completionTokens int, costUSD float64, latency time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("provider", provider),
		attribute.String("model", model),
		attribute.String("status", status),
	}

	mc.llmRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	mc.llmLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attrs...))

	// Record tokens
	if promptTokens > 0 {
		tokenAttrs := append(attrs, attribute.String("type", "prompt"))
		mc.tokensTotal.Add(ctx, int64(promptTokens), metric.WithAttributes(tokenAttrs...))
	}
	if completionTokens > 0 {
		tokenAttrs := append(attrs, attribute.String("type", "completion"))
		mc.tokensTotal.Add(ctx, int64(completionTokens), metric.WithAttributes(tokenAttrs...))
	}

	// Update total cost
	if costUSD > 0 {
		mc.totalCostMu.Lock()
		mc.totalCostUSD += costUSD
		mc.totalCostMu.Unlock()
	}
}

// IncrementQueueDepth increments the pending run queue depth
func (mc *MetricsCollector) IncrementQueueDepth() {
	mc.queueDepthMu.Lock()
	mc.queueDepth++
	mc.queueDepthMu.Unlock()
}

// DecrementQueueDepth decrements the pending run queue depth
func (mc *MetricsCollector) DecrementQueueDepth() {
	mc.queueDepthMu.Lock()
	if mc.queueDepth > 0 {
		mc.queueDepth--
	}
	mc.queueDepthMu.Unlock()
}

// RecordReplay records a workflow replay completion
func (mc *MetricsCollector) RecordReplay(ctx context.Context, workflowID, status string, costSavedUSD float64) {
	attrs := []attribute.KeyValue{
		attribute.String("workflow", workflowID),
		attribute.String("status", status),
	}

	mc.replayTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

	if costSavedUSD > 0 {
		mc.replayCostSavedUSD.Add(ctx, costSavedUSD, metric.WithAttributes(attrs...))
	}
}

// RecordDebugSessionStart increments the active debug sessions gauge
func (mc *MetricsCollector) RecordDebugSessionStart() {
	mc.debugSessionsMu.Lock()
	mc.debugSessionsActive++
	mc.debugSessionsMu.Unlock()
}

// RecordDebugSessionEnd decrements the active debug sessions gauge
func (mc *MetricsCollector) RecordDebugSessionEnd() {
	mc.debugSessionsMu.Lock()
	if mc.debugSessionsActive > 0 {
		mc.debugSessionsActive--
	}
	mc.debugSessionsMu.Unlock()
}

// RecordDebugEvent records a debug event emission
func (mc *MetricsCollector) RecordDebugEvent(ctx context.Context, eventType string) {
	attrs := []attribute.KeyValue{
		attribute.String("event_type", eventType),
	}

	mc.debugEventsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// SubscriberCounter is an interface for counting subscribers (used by LogAggregator)
type SubscriberCounter interface {
	TotalSubscriberCount() int
}

// RunCounter is an interface for counting runs (used by StateManager)
type RunCounter interface {
	RunCount() int
}

// SetSubscriberCounter stores a reference for memory metrics.
// This is called during controller initialization to wire up metrics.
func (mc *MetricsCollector) SetSubscriberCounter(counter SubscriberCounter) {
	// Observable gauges already set up in NewMetricsCollector
	// This method stores a reference that could be used for subscriber-specific metrics
	// For now, it's a no-op placeholder for future memory tracking
}

// SetRunCounter stores a reference for memory metrics.
// This is called during controller initialization to wire up metrics.
func (mc *MetricsCollector) SetRunCounter(counter RunCounter) {
	// Observable gauges already set up in NewMetricsCollector
	// This method stores a reference that could be used for run-specific metrics
	// For now, it's a no-op placeholder for future memory tracking
}
