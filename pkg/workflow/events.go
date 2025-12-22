package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType represents the type of workflow event.
type EventType string

const (
	// EventStateChanged is emitted when a workflow changes state.
	EventStateChanged EventType = "state_changed"

	// EventStepCompleted is emitted when a workflow step completes.
	EventStepCompleted EventType = "step_completed"

	// EventError is emitted when an error occurs.
	EventError EventType = "error"
)

// Event represents a workflow event.
type Event struct {
	Type       EventType              `json:"type"`
	WorkflowID string                 `json:"workflow_id"`
	Timestamp  time.Time              `json:"timestamp"`
	Data       map[string]interface{} `json:"data"`
}

// StateChangedData contains data for state change events.
type StateChangedData struct {
	FromState State  `json:"from_state"`
	ToState   State  `json:"to_state"`
	Event     string `json:"event"`
}

// StepCompletedData contains data for step completion events.
type StepCompletedData struct {
	StepName string        `json:"step_name"`
	Duration time.Duration `json:"duration"`
	Result   interface{}   `json:"result,omitempty"`
}

// ErrorData contains data for error events.
type ErrorData struct {
	Error   string `json:"error"`
	Context string `json:"context,omitempty"`
}

// EventListener is a function that handles workflow events.
type EventListener func(ctx context.Context, event *Event) error

// EventEmitter manages event listeners and dispatches events.
type EventEmitter struct {
	mu        sync.RWMutex
	listeners map[EventType][]EventListener
	async     bool // If true, listeners are called asynchronously
}

// NewEventEmitter creates a new event emitter.
func NewEventEmitter(async bool) *EventEmitter {
	return &EventEmitter{
		listeners: make(map[EventType][]EventListener),
		async:     async,
	}
}

// On registers an event listener for the specified event type.
func (e *EventEmitter) On(eventType EventType, listener EventListener) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.listeners[eventType] = append(e.listeners[eventType], listener)
}

// Off removes an event listener.
// Note: This removes ALL listeners for the event type.
// For more granular control, consider using a listener ID system.
func (e *EventEmitter) Off(eventType EventType) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.listeners, eventType)
}

// Emit dispatches an event to all registered listeners.
func (e *EventEmitter) Emit(ctx context.Context, event *Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Set timestamp if not already set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	e.mu.RLock()
	listeners := make([]EventListener, len(e.listeners[event.Type]))
	copy(listeners, e.listeners[event.Type])
	e.mu.RUnlock()

	// Call listeners
	if e.async {
		return e.emitAsync(ctx, event, listeners)
	}
	return e.emitSync(ctx, event, listeners)
}

// emitSync calls listeners synchronously.
func (e *EventEmitter) emitSync(ctx context.Context, event *Event, listeners []EventListener) error {
	var lastError error

	for _, listener := range listeners {
		if err := listener(ctx, event); err != nil {
			// Continue calling other listeners even if one fails
			// Store the last error to return
			lastError = err
		}
	}

	return lastError
}

// emitAsync calls listeners asynchronously.
func (e *EventEmitter) emitAsync(ctx context.Context, event *Event, listeners []EventListener) error {
	// Use a wait group to track completion
	var wg sync.WaitGroup
	errChan := make(chan error, len(listeners))

	for _, listener := range listeners {
		wg.Add(1)
		go func(l EventListener) {
			defer wg.Done()
			if err := l(ctx, event); err != nil {
				errChan <- err
			}
		}(listener)
	}

	// Wait for all listeners to complete
	wg.Wait()
	close(errChan)

	// Collect any errors
	var lastError error
	for err := range errChan {
		lastError = err
	}

	return lastError
}

// EmitStateChanged emits a state change event.
func (e *EventEmitter) EmitStateChanged(ctx context.Context, workflowID string, fromState, toState State, eventName string) error {
	return e.Emit(ctx, &Event{
		Type:       EventStateChanged,
		WorkflowID: workflowID,
		Data: map[string]interface{}{
			"from_state": fromState,
			"to_state":   toState,
			"event":      eventName,
		},
	})
}

// EmitStepCompleted emits a step completion event.
func (e *EventEmitter) EmitStepCompleted(ctx context.Context, workflowID string, stepName string, duration time.Duration, result interface{}) error {
	data := map[string]interface{}{
		"step_name": stepName,
		"duration":  duration.Milliseconds(),
	}
	if result != nil {
		data["result"] = result
	}

	return e.Emit(ctx, &Event{
		Type:       EventStepCompleted,
		WorkflowID: workflowID,
		Data:       data,
	})
}

// EmitError emits an error event.
func (e *EventEmitter) EmitError(ctx context.Context, workflowID string, err error, contextInfo string) error {
	data := map[string]interface{}{
		"error": err.Error(),
	}
	if contextInfo != "" {
		data["context"] = contextInfo
	}

	return e.Emit(ctx, &Event{
		Type:       EventError,
		WorkflowID: workflowID,
		Data:       data,
	})
}

// ListenerCount returns the number of listeners for a given event type.
func (e *EventEmitter) ListenerCount(eventType EventType) int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return len(e.listeners[eventType])
}

// RemoveAllListeners removes all listeners for all event types.
func (e *EventEmitter) RemoveAllListeners() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.listeners = make(map[EventType][]EventListener)
}
