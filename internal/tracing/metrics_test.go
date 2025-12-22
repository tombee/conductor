package tracing

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric"
)

func TestNewMetricsCollector(t *testing.T) {
	provider := metric.NewMeterProvider()
	defer provider.Shutdown(context.Background())

	mc, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	if mc == nil {
		t.Fatal("Expected non-nil MetricsCollector")
	}

	if mc.meter == nil {
		t.Error("Expected meter to be set")
	}

	if mc.activeRuns == nil {
		t.Error("Expected activeRuns map to be initialized")
	}
}

func TestMetricsCollector_RecordRunStart(t *testing.T) {
	provider := metric.NewMeterProvider()
	defer provider.Shutdown(context.Background())

	mc, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	ctx := context.Background()
	mc.RecordRunStart(ctx, "run-123", "test-workflow")

	// Verify run is tracked as active
	mc.activeRunsMu.RLock()
	_, exists := mc.activeRuns["run-123"]
	mc.activeRunsMu.RUnlock()

	if !exists {
		t.Error("Expected run to be tracked as active")
	}
}

func TestMetricsCollector_RecordRunComplete(t *testing.T) {
	provider := metric.NewMeterProvider()
	defer provider.Shutdown(context.Background())

	mc, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	ctx := context.Background()
	runID := "run-456"

	// Start the run
	mc.RecordRunStart(ctx, runID, "test-workflow")

	// Verify it's tracked
	mc.activeRunsMu.RLock()
	_, exists := mc.activeRuns[runID]
	mc.activeRunsMu.RUnlock()
	if !exists {
		t.Fatal("Expected run to be tracked")
	}

	// Complete the run
	mc.RecordRunComplete(ctx, runID, "test-workflow", "completed", "api", 5*time.Second)

	// Verify it's removed from active runs
	mc.activeRunsMu.RLock()
	_, stillExists := mc.activeRuns[runID]
	mc.activeRunsMu.RUnlock()
	if stillExists {
		t.Error("Expected run to be removed from active runs after completion")
	}
}

func TestMetricsCollector_RecordStepComplete(t *testing.T) {
	provider := metric.NewMeterProvider()
	defer provider.Shutdown(context.Background())

	mc, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	ctx := context.Background()

	// Should not panic with valid inputs
	mc.RecordStepComplete(ctx, "workflow-1", "step-1", "success", 100*time.Millisecond)
	mc.RecordStepComplete(ctx, "workflow-1", "step-2", "failed", 50*time.Millisecond)
	mc.RecordStepComplete(ctx, "workflow-1", "step-3", "skipped", 0)
}

func TestMetricsCollector_RecordLLMRequest(t *testing.T) {
	provider := metric.NewMeterProvider()
	defer provider.Shutdown(context.Background())

	mc, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	ctx := context.Background()

	// Should not panic with valid inputs
	mc.RecordLLMRequest(ctx, "anthropic", "claude-3", "success", 100, 50, 0.05, 200*time.Millisecond)
	mc.RecordLLMRequest(ctx, "openai", "gpt-4", "error", 0, 0, 0, 100*time.Millisecond)
}

func TestMetricsCollector_QueueDepth(t *testing.T) {
	provider := metric.NewMeterProvider()
	defer provider.Shutdown(context.Background())

	mc, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	// Initial state
	mc.queueDepthMu.RLock()
	initial := mc.queueDepth
	mc.queueDepthMu.RUnlock()
	if initial != 0 {
		t.Errorf("Expected initial queue depth 0, got %d", initial)
	}

	// Increment
	mc.IncrementQueueDepth()
	mc.IncrementQueueDepth()

	mc.queueDepthMu.RLock()
	afterIncrement := mc.queueDepth
	mc.queueDepthMu.RUnlock()
	if afterIncrement != 2 {
		t.Errorf("Expected queue depth 2 after increments, got %d", afterIncrement)
	}

	// Decrement
	mc.DecrementQueueDepth()

	mc.queueDepthMu.RLock()
	afterDecrement := mc.queueDepth
	mc.queueDepthMu.RUnlock()
	if afterDecrement != 1 {
		t.Errorf("Expected queue depth 1 after decrement, got %d", afterDecrement)
	}
}

func TestMetricsCollector_QueueDepthNeverNegative(t *testing.T) {
	provider := metric.NewMeterProvider()
	defer provider.Shutdown(context.Background())

	mc, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	// Decrement when already 0
	mc.DecrementQueueDepth()

	mc.queueDepthMu.RLock()
	depth := mc.queueDepth
	mc.queueDepthMu.RUnlock()
	if depth != 0 {
		t.Errorf("Expected queue depth to stay at 0, got %d", depth)
	}
}

func TestMetricsCollector_ConcurrentAccess(t *testing.T) {
	provider := metric.NewMeterProvider()
	defer provider.Shutdown(context.Background())

	mc, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup

	// Run concurrent operations
	for i := 0; i < 100; i++ {
		wg.Add(4)

		go func(id int) {
			defer wg.Done()
			mc.IncrementQueueDepth()
		}(i)

		go func(id int) {
			defer wg.Done()
			mc.DecrementQueueDepth()
		}(i)

		go func(id int) {
			defer wg.Done()
			runID := "run-" + string(rune(id+'0'))
			mc.RecordRunStart(ctx, runID, "workflow")
			mc.RecordRunComplete(ctx, runID, "workflow", "completed", "test", time.Millisecond)
		}(i)

		go func(id int) {
			defer wg.Done()
			mc.RecordStepComplete(ctx, "workflow", "step", "success", time.Millisecond)
		}(i)
	}

	wg.Wait()

	// Should complete without panics or races
}

func TestMetricsCollector_CostTracking(t *testing.T) {
	provider := metric.NewMeterProvider()
	defer provider.Shutdown(context.Background())

	mc, err := NewMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	ctx := context.Background()

	// Record some costs
	mc.RecordLLMRequest(ctx, "anthropic", "claude-3", "success", 1000, 500, 0.05, time.Second)
	mc.RecordLLMRequest(ctx, "anthropic", "claude-3", "success", 2000, 1000, 0.10, time.Second)

	mc.totalCostMu.RLock()
	totalCost := mc.totalCostUSD
	mc.totalCostMu.RUnlock()

	expectedCost := 0.15
	if totalCost < expectedCost-0.001 || totalCost > expectedCost+0.001 {
		t.Errorf("Expected total cost ~%f, got %f", expectedCost, totalCost)
	}
}
