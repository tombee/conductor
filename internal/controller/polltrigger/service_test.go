package polltrigger

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestService_Lifecycle(t *testing.T) {
	// This test takes 12+ seconds due to minimum poll interval enforcement
	if testing.Short() {
		t.Skip("skipping slow test in short mode")
	}

	// Create a test service
	firedCount := 0
	svc, err := NewService(ServiceConfig{
		Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
		WorkflowFirer: func(ctx context.Context, workflowPath string, triggerContext *PollTriggerContext) error {
			firedCount++
			return nil
		},
		StateDBPath: ":memory:",
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Start the service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Register a mock poller
	mockPoller := &mockPoller{
		name: "test",
		events: []map[string]interface{}{
			{"id": "event-1", "created_at": time.Now().Format(time.RFC3339), "title": "Test Event"},
		},
	}
	if err := svc.RegisterPoller(mockPoller); err != nil {
		t.Fatalf("failed to register poller: %v", err)
	}

	// Register a trigger
	reg := &PollTriggerRegistration{
		TriggerID:    "test-trigger",
		WorkflowPath: "/tmp/test-workflow.yaml",
		Integration:  "test",
		Query:        map[string]interface{}{"user_id": "test-user"},
		Interval:     1, // 1 second for fast test
		Startup:      "ignore_historical",
		InputMapping: map[string]string{},
	}
	if err := svc.RegisterTrigger(reg); err != nil {
		t.Fatalf("failed to register trigger: %v", err)
	}

	// Wait for the first poll to fire (minimum interval is 10s, plus jitter)
	// We need to wait at least 11 seconds to ensure the poll fires
	time.Sleep(12 * time.Second)

	// Stop the service
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := svc.Stop(shutdownCtx); err != nil {
		t.Fatalf("failed to stop service: %v", err)
	}

	// Verify workflow was fired at least once
	// Note: It should fire exactly once since the event is marked as seen
	if firedCount == 0 {
		t.Error("workflow was not fired")
	} else if firedCount > 1 {
		t.Logf("workflow fired %d times (expected 1, but multiple polls may have occurred)", firedCount)
	}
}

// mockPoller is a mock IntegrationPoller for testing.
type mockPoller struct {
	name   string
	events []map[string]interface{}
}

func (m *mockPoller) Name() string {
	return m.name
}

func (m *mockPoller) Poll(ctx context.Context, state *PollState, query map[string]interface{}) ([]map[string]interface{}, string, error) {
	return m.events, "", nil
}
