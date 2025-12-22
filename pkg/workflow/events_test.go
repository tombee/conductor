package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestEventEmitterOn(t *testing.T) {
	t.Run("register listener", func(t *testing.T) {
		emitter := NewEventEmitter(false)

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			return nil
		})

		if count := emitter.ListenerCount(EventStateChanged); count != 1 {
			t.Errorf("ListenerCount = %d, want 1", count)
		}
	})

	t.Run("register multiple listeners", func(t *testing.T) {
		emitter := NewEventEmitter(false)

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			return nil
		})
		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			return nil
		})

		if count := emitter.ListenerCount(EventStateChanged); count != 2 {
			t.Errorf("ListenerCount = %d, want 2", count)
		}
	})

	t.Run("register listeners for different events", func(t *testing.T) {
		emitter := NewEventEmitter(false)

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			return nil
		})
		emitter.On(EventStepCompleted, func(ctx context.Context, event *Event) error {
			return nil
		})

		if count := emitter.ListenerCount(EventStateChanged); count != 1 {
			t.Errorf("ListenerCount(EventStateChanged) = %d, want 1", count)
		}
		if count := emitter.ListenerCount(EventStepCompleted); count != 1 {
			t.Errorf("ListenerCount(EventStepCompleted) = %d, want 1", count)
		}
	})
}

func TestEventEmitterOff(t *testing.T) {
	t.Run("remove listeners", func(t *testing.T) {
		emitter := NewEventEmitter(false)

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			return nil
		})

		emitter.Off(EventStateChanged)

		if count := emitter.ListenerCount(EventStateChanged); count != 0 {
			t.Errorf("ListenerCount = %d, want 0", count)
		}
	})

	t.Run("remove non-existent event type", func(t *testing.T) {
		emitter := NewEventEmitter(false)

		// Should not panic
		emitter.Off(EventStateChanged)

		if count := emitter.ListenerCount(EventStateChanged); count != 0 {
			t.Errorf("ListenerCount = %d, want 0", count)
		}
	})
}

func TestEventEmitterEmitSync(t *testing.T) {
	ctx := context.Background()

	t.Run("emit to listener", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		called := false
		var capturedEvent *Event

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			called = true
			capturedEvent = event
			return nil
		})

		event := &Event{
			Type:       EventStateChanged,
			WorkflowID: "test-1",
			Data:       map[string]interface{}{"key": "value"},
		}

		err := emitter.Emit(ctx, event)
		if err != nil {
			t.Fatalf("Emit() error = %v", err)
		}

		if !called {
			t.Error("Listener was not called")
		}
		if capturedEvent.Type != EventStateChanged {
			t.Error("Event type not captured correctly")
		}
		if capturedEvent.Data["key"] != "value" {
			t.Error("Event data not captured correctly")
		}
	})

	t.Run("emit sets timestamp", func(t *testing.T) {
		emitter := NewEventEmitter(false)

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			return nil
		})

		event := &Event{
			Type:       EventStateChanged,
			WorkflowID: "test-1",
		}

		err := emitter.Emit(ctx, event)
		if err != nil {
			t.Fatalf("Emit() error = %v", err)
		}

		if event.Timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
	})

	t.Run("emit preserves existing timestamp", func(t *testing.T) {
		emitter := NewEventEmitter(false)

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			return nil
		})

		timestamp := time.Now().Add(-1 * time.Hour)
		event := &Event{
			Type:       EventStateChanged,
			WorkflowID: "test-1",
			Timestamp:  timestamp,
		}

		err := emitter.Emit(ctx, event)
		if err != nil {
			t.Fatalf("Emit() error = %v", err)
		}

		if !event.Timestamp.Equal(timestamp) {
			t.Error("Timestamp should be preserved")
		}
	})

	t.Run("emit with nil event", func(t *testing.T) {
		emitter := NewEventEmitter(false)

		err := emitter.Emit(ctx, nil)
		if err == nil {
			t.Fatal("Emit() should return error for nil event")
		}
	})

	t.Run("emit to multiple listeners", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		count := 0

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			count++
			return nil
		})
		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			count++
			return nil
		})

		event := &Event{
			Type:       EventStateChanged,
			WorkflowID: "test-1",
		}

		err := emitter.Emit(ctx, event)
		if err != nil {
			t.Fatalf("Emit() error = %v", err)
		}

		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
	})

	t.Run("listener error is returned", func(t *testing.T) {
		emitter := NewEventEmitter(false)

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			return errors.New("listener error")
		})

		event := &Event{
			Type:       EventStateChanged,
			WorkflowID: "test-1",
		}

		err := emitter.Emit(ctx, event)
		if err == nil {
			t.Fatal("Emit() should return listener error")
		}
	})

	t.Run("continues calling listeners after error", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		count := 0

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			count++
			return errors.New("first error")
		})
		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			count++
			return nil
		})

		event := &Event{
			Type:       EventStateChanged,
			WorkflowID: "test-1",
		}

		_ = emitter.Emit(ctx, event)

		if count != 2 {
			t.Errorf("count = %d, want 2 (should call all listeners)", count)
		}
	})

	t.Run("emit to non-matching event type", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		called := false

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			called = true
			return nil
		})

		event := &Event{
			Type:       EventStepCompleted,
			WorkflowID: "test-1",
		}

		err := emitter.Emit(ctx, event)
		if err != nil {
			t.Fatalf("Emit() error = %v", err)
		}

		if called {
			t.Error("Listener should not be called for non-matching event type")
		}
	})
}

func TestEventEmitterEmitAsync(t *testing.T) {
	ctx := context.Background()

	t.Run("emit asynchronously", func(t *testing.T) {
		emitter := NewEventEmitter(true)
		var mu sync.Mutex
		count := 0

		for i := 0; i < 5; i++ {
			emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				count++
				mu.Unlock()
				return nil
			})
		}

		event := &Event{
			Type:       EventStateChanged,
			WorkflowID: "test-1",
		}

		start := time.Now()
		err := emitter.Emit(ctx, event)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Emit() error = %v", err)
		}

		// With async, all listeners run in parallel, so total time should be ~10ms
		// With sync, total time would be ~50ms
		if duration > 30*time.Millisecond {
			t.Errorf("Async emit took too long: %v", duration)
		}

		mu.Lock()
		defer mu.Unlock()
		if count != 5 {
			t.Errorf("count = %d, want 5", count)
		}
	})

	t.Run("async collects errors", func(t *testing.T) {
		emitter := NewEventEmitter(true)

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			return errors.New("async error")
		})

		event := &Event{
			Type:       EventStateChanged,
			WorkflowID: "test-1",
		}

		err := emitter.Emit(ctx, event)
		if err == nil {
			t.Fatal("Emit() should return error from async listener")
		}
	})
}

func TestEventEmitterStateChanged(t *testing.T) {
	ctx := context.Background()

	t.Run("emit state changed event", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		var capturedEvent *Event

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			capturedEvent = event
			return nil
		})

		err := emitter.EmitStateChanged(ctx, "workflow-1", StateCreated, StateRunning, "start")
		if err != nil {
			t.Fatalf("EmitStateChanged() error = %v", err)
		}

		if capturedEvent == nil {
			t.Fatal("Event was not captured")
		}
		if capturedEvent.Type != EventStateChanged {
			t.Errorf("Type = %v, want %v", capturedEvent.Type, EventStateChanged)
		}
		if capturedEvent.WorkflowID != "workflow-1" {
			t.Errorf("WorkflowID = %v, want %v", capturedEvent.WorkflowID, "workflow-1")
		}
		if capturedEvent.Data["from_state"] != StateCreated {
			t.Error("from_state not set correctly")
		}
		if capturedEvent.Data["to_state"] != StateRunning {
			t.Error("to_state not set correctly")
		}
		if capturedEvent.Data["event"] != "start" {
			t.Error("event not set correctly")
		}
	})
}

func TestEventEmitterStepCompleted(t *testing.T) {
	ctx := context.Background()

	t.Run("emit step completed event", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		var capturedEvent *Event

		emitter.On(EventStepCompleted, func(ctx context.Context, event *Event) error {
			capturedEvent = event
			return nil
		})

		duration := 100 * time.Millisecond
		result := map[string]interface{}{"output": "success"}

		err := emitter.EmitStepCompleted(ctx, "workflow-1", "step-1", duration, result)
		if err != nil {
			t.Fatalf("EmitStepCompleted() error = %v", err)
		}

		if capturedEvent == nil {
			t.Fatal("Event was not captured")
		}
		if capturedEvent.Type != EventStepCompleted {
			t.Errorf("Type = %v, want %v", capturedEvent.Type, EventStepCompleted)
		}
		if capturedEvent.Data["step_name"] != "step-1" {
			t.Error("step_name not set correctly")
		}
		if capturedEvent.Data["duration"] != int64(100) {
			t.Error("duration not set correctly")
		}
		if capturedEvent.Data["result"] == nil {
			t.Error("result not set")
		}
	})

	t.Run("emit step completed without result", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		var capturedEvent *Event

		emitter.On(EventStepCompleted, func(ctx context.Context, event *Event) error {
			capturedEvent = event
			return nil
		})

		err := emitter.EmitStepCompleted(ctx, "workflow-1", "step-1", 100*time.Millisecond, nil)
		if err != nil {
			t.Fatalf("EmitStepCompleted() error = %v", err)
		}

		if capturedEvent == nil {
			t.Fatal("Event was not captured")
		}
		if _, hasResult := capturedEvent.Data["result"]; hasResult {
			t.Error("result should not be set when nil")
		}
	})
}

func TestEventEmitterError(t *testing.T) {
	ctx := context.Background()

	t.Run("emit error event", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		var capturedEvent *Event

		emitter.On(EventError, func(ctx context.Context, event *Event) error {
			capturedEvent = event
			return nil
		})

		err := emitter.EmitError(ctx, "workflow-1", errors.New("test error"), "test context")
		if err != nil {
			t.Fatalf("EmitError() error = %v", err)
		}

		if capturedEvent == nil {
			t.Fatal("Event was not captured")
		}
		if capturedEvent.Type != EventError {
			t.Errorf("Type = %v, want %v", capturedEvent.Type, EventError)
		}
		if capturedEvent.Data["error"] != "test error" {
			t.Error("error not set correctly")
		}
		if capturedEvent.Data["context"] != "test context" {
			t.Error("context not set correctly")
		}
	})

	t.Run("emit error without context", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		var capturedEvent *Event

		emitter.On(EventError, func(ctx context.Context, event *Event) error {
			capturedEvent = event
			return nil
		})

		err := emitter.EmitError(ctx, "workflow-1", errors.New("test error"), "")
		if err != nil {
			t.Fatalf("EmitError() error = %v", err)
		}

		if capturedEvent == nil {
			t.Fatal("Event was not captured")
		}
		if _, hasContext := capturedEvent.Data["context"]; hasContext {
			t.Error("context should not be set when empty")
		}
	})
}

func TestEventEmitterRemoveAllListeners(t *testing.T) {
	t.Run("remove all listeners", func(t *testing.T) {
		emitter := NewEventEmitter(false)

		emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
			return nil
		})
		emitter.On(EventStepCompleted, func(ctx context.Context, event *Event) error {
			return nil
		})

		emitter.RemoveAllListeners()

		if count := emitter.ListenerCount(EventStateChanged); count != 0 {
			t.Errorf("ListenerCount(EventStateChanged) = %d, want 0", count)
		}
		if count := emitter.ListenerCount(EventStepCompleted); count != 0 {
			t.Errorf("ListenerCount(EventStepCompleted) = %d, want 0", count)
		}
	})
}

func TestEventEmitterConcurrency(t *testing.T) {
	ctx := context.Background()

	t.Run("concurrent listener registration", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		var wg sync.WaitGroup

		// Register listeners concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
					return nil
				})
			}()
		}

		wg.Wait()

		if count := emitter.ListenerCount(EventStateChanged); count != 10 {
			t.Errorf("ListenerCount = %d, want 10", count)
		}
	})

	t.Run("concurrent emit and register", func(t *testing.T) {
		emitter := NewEventEmitter(false)
		var wg sync.WaitGroup

		// Emit events while registering listeners
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				emitter.On(EventStateChanged, func(ctx context.Context, event *Event) error {
					time.Sleep(1 * time.Millisecond)
					return nil
				})
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				event := &Event{
					Type:       EventStateChanged,
					WorkflowID: "test",
				}
				_ = emitter.Emit(ctx, event)
			}()
		}

		wg.Wait()
		// Should not panic or deadlock
	})
}
