package polltrigger

import (
	"context"
	"testing"
	"time"
)

func TestScheduler_Register(t *testing.T) {
	ctx := context.Background()

	scheduler := NewScheduler(nil)
	defer scheduler.Stop()

	// Register a trigger with 1 second interval (will be enforced to MinPollInterval)
	err := scheduler.Register(ctx, "test-trigger", 1)
	if err != nil {
		t.Fatalf("Failed to register trigger: %v", err)
	}

	// Check interval is registered (should be enforced to minimum)
	interval := scheduler.GetInterval("test-trigger")
	if interval != MinPollInterval {
		t.Errorf("Expected interval to be %d (minimum), got %d", MinPollInterval, interval)
	}

	// Check trigger is in list
	triggers := scheduler.ListTriggers()
	if len(triggers) != 1 {
		t.Errorf("Expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0] != "test-trigger" {
		t.Errorf("Expected trigger 'test-trigger', got %s", triggers[0])
	}
}

func TestScheduler_MinimumInterval(t *testing.T) {
	ctx := context.Background()

	scheduler := NewScheduler(nil)
	defer scheduler.Stop()

	// Try to register with interval below minimum
	err := scheduler.Register(ctx, "test-trigger", 5)
	if err != nil {
		t.Fatalf("Failed to register trigger: %v", err)
	}

	// Should be enforced to minimum
	interval := scheduler.GetInterval("test-trigger")
	if interval != MinPollInterval {
		t.Errorf("Expected interval to be enforced to %d, got %d", MinPollInterval, interval)
	}
}

func TestScheduler_Unregister(t *testing.T) {
	ctx := context.Background()

	scheduler := NewScheduler(nil)
	defer scheduler.Stop()

	// Register a trigger
	err := scheduler.Register(ctx, "test-trigger", 10)
	if err != nil {
		t.Fatalf("Failed to register trigger: %v", err)
	}

	// Verify it's registered
	if scheduler.GetInterval("test-trigger") == 0 {
		t.Fatal("Trigger not registered")
	}

	// Unregister
	scheduler.Unregister("test-trigger")

	// Interval should be 0
	interval := scheduler.GetInterval("test-trigger")
	if interval != 0 {
		t.Errorf("Expected interval to be 0 after unregister, got %d", interval)
	}

	// Should not be in list
	triggers := scheduler.ListTriggers()
	if len(triggers) != 0 {
		t.Errorf("Expected 0 triggers after unregister, got %d", len(triggers))
	}
}

func TestScheduler_UpdateInterval(t *testing.T) {
	ctx := context.Background()

	scheduler := NewScheduler(nil)
	defer scheduler.Stop()

	// Register with initial interval
	err := scheduler.Register(ctx, "test-trigger", 10)
	if err != nil {
		t.Fatalf("Failed to register trigger: %v", err)
	}

	interval1 := scheduler.GetInterval("test-trigger")
	if interval1 != 10 {
		t.Errorf("Expected interval 10, got %d", interval1)
	}

	// Update interval
	err = scheduler.Register(ctx, "test-trigger", 20)
	if err != nil {
		t.Fatalf("Failed to update trigger: %v", err)
	}

	interval2 := scheduler.GetInterval("test-trigger")
	if interval2 != 20 {
		t.Errorf("Expected interval 20, got %d", interval2)
	}
}

func TestScheduler_ListTriggers(t *testing.T) {
	ctx := context.Background()

	scheduler := NewScheduler(nil)
	defer scheduler.Stop()

	// Register multiple triggers
	scheduler.Register(ctx, "trigger1", 10)
	scheduler.Register(ctx, "trigger2", 15)
	scheduler.Register(ctx, "trigger3", 20)

	triggers := scheduler.ListTriggers()
	if len(triggers) != 3 {
		t.Errorf("Expected 3 triggers, got %d", len(triggers))
	}

	// Check all trigger IDs are present
	found := make(map[string]bool)
	for _, id := range triggers {
		found[id] = true
	}

	if !found["trigger1"] || !found["trigger2"] || !found["trigger3"] {
		t.Errorf("Missing expected trigger IDs: got %v", triggers)
	}
}

func TestScheduler_Stop(t *testing.T) {
	ctx := context.Background()

	scheduler := NewScheduler(nil)

	// Register triggers
	scheduler.Register(ctx, "trigger1", 10)
	scheduler.Register(ctx, "trigger2", 10)

	// Verify they're registered
	if len(scheduler.ListTriggers()) != 2 {
		t.Fatal("Triggers not registered")
	}

	// Stop scheduler
	scheduler.Stop()

	// ListTriggers should return empty
	triggers := scheduler.ListTriggers()
	if len(triggers) != 0 {
		t.Errorf("Expected 0 triggers after stop, got %d", len(triggers))
	}

	// Register should fail after stop
	err := scheduler.Register(ctx, "trigger3", 10)
	if err == nil {
		t.Error("Expected error when registering after stop, got nil")
	}
}

func TestScheduler_Jitter(t *testing.T) {
	// Test that jitter adds variation to intervals
	duration := 10 * time.Second
	results := make(map[time.Duration]bool)

	// Generate multiple jittered durations
	for i := 0; i < 100; i++ {
		jittered := addJitter(duration)
		results[jittered] = true

		// Verify jitter is within ±10%
		minDuration := time.Duration(float64(duration) * 0.9)
		maxDuration := time.Duration(float64(duration) * 1.1)

		if jittered < minDuration || jittered > maxDuration {
			t.Errorf("Jitter out of range: %v (expected between %v and %v)", jittered, minDuration, maxDuration)
		}
	}

	// Should have multiple different values (not all the same)
	if len(results) < 10 {
		t.Errorf("Jitter not adding enough variation: got %d unique values", len(results))
	}
}

func TestScheduler_NoHandler(t *testing.T) {
	ctx := context.Background()

	// Scheduler with nil handler should not panic
	scheduler := NewScheduler(nil)
	defer scheduler.Stop()

	err := scheduler.Register(ctx, "test-trigger", 10)
	if err != nil {
		t.Fatalf("Failed to register trigger: %v", err)
	}

	// Verify it's registered
	if scheduler.GetInterval("test-trigger") == 0 {
		t.Error("Trigger not registered")
	}
}
