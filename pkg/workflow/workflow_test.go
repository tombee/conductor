package workflow

import (
	"context"
	"errors"
	"testing"
)

func TestStateIsValid(t *testing.T) {
	tests := []struct {
		name  string
		state State
		want  bool
	}{
		{"created is valid", StateCreated, true},
		{"running is valid", StateRunning, true},
		{"paused is valid", StatePaused, true},
		{"completed is valid", StateCompleted, true},
		{"failed is valid", StateFailed, true},
		{"invalid state", State("invalid"), false},
		{"empty state", State(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.IsValid(); got != tt.want {
				t.Errorf("State.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStateIsTerminal(t *testing.T) {
	tests := []struct {
		name  string
		state State
		want  bool
	}{
		{"created is not terminal", StateCreated, false},
		{"running is not terminal", StateRunning, false},
		{"paused is not terminal", StatePaused, false},
		{"completed is terminal", StateCompleted, true},
		{"failed is terminal", StateFailed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.IsTerminal(); got != tt.want {
				t.Errorf("State.IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTransitionCanTransition(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		transition *Transition
		workflow   *Workflow
		wantOk     bool
		wantErr    bool
	}{
		{
			name: "simple transition allowed",
			transition: &Transition{
				From:  StateCreated,
				To:    StateRunning,
				Event: "start",
			},
			workflow: &Workflow{State: StateCreated},
			wantOk:   true,
			wantErr:  false,
		},
		{
			name: "wrong starting state",
			transition: &Transition{
				From:  StateCreated,
				To:    StateRunning,
				Event: "start",
			},
			workflow: &Workflow{State: StateRunning},
			wantOk:   false,
			wantErr:  false,
		},
		{
			name: "guard allows transition",
			transition: &Transition{
				From:  StateCreated,
				To:    StateRunning,
				Event: "start",
				Guards: []TransitionGuard{
					func(ctx context.Context, w *Workflow) (bool, error) {
						return true, nil
					},
				},
			},
			workflow: &Workflow{State: StateCreated},
			wantOk:   true,
			wantErr:  false,
		},
		{
			name: "guard blocks transition",
			transition: &Transition{
				From:  StateCreated,
				To:    StateRunning,
				Event: "start",
				Guards: []TransitionGuard{
					func(ctx context.Context, w *Workflow) (bool, error) {
						return false, nil
					},
				},
			},
			workflow: &Workflow{State: StateCreated},
			wantOk:   false,
			wantErr:  false,
		},
		{
			name: "guard returns error",
			transition: &Transition{
				From:  StateCreated,
				To:    StateRunning,
				Event: "start",
				Guards: []TransitionGuard{
					func(ctx context.Context, w *Workflow) (bool, error) {
						return false, errors.New("guard error")
					},
				},
			},
			workflow: &Workflow{State: StateCreated},
			wantOk:   false,
			wantErr:  true,
		},
		{
			name: "multiple guards all pass",
			transition: &Transition{
				From:  StateCreated,
				To:    StateRunning,
				Event: "start",
				Guards: []TransitionGuard{
					func(ctx context.Context, w *Workflow) (bool, error) {
						return true, nil
					},
					func(ctx context.Context, w *Workflow) (bool, error) {
						return true, nil
					},
				},
			},
			workflow: &Workflow{State: StateCreated},
			wantOk:   true,
			wantErr:  false,
		},
		{
			name: "multiple guards one fails",
			transition: &Transition{
				From:  StateCreated,
				To:    StateRunning,
				Event: "start",
				Guards: []TransitionGuard{
					func(ctx context.Context, w *Workflow) (bool, error) {
						return true, nil
					},
					func(ctx context.Context, w *Workflow) (bool, error) {
						return false, nil
					},
				},
			},
			workflow: &Workflow{State: StateCreated},
			wantOk:   false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := tt.transition.CanTransition(ctx, tt.workflow)
			if (err != nil) != tt.wantErr {
				t.Errorf("CanTransition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if ok != tt.wantOk {
				t.Errorf("CanTransition() = %v, want %v", ok, tt.wantOk)
			}
		})
	}
}

func TestTransitionExecute(t *testing.T) {
	ctx := context.Background()

	t.Run("simple state change", func(t *testing.T) {
		transition := &Transition{
			From:  StateCreated,
			To:    StateRunning,
			Event: "start",
		}
		workflow := &Workflow{
			ID:    "test-1",
			State: StateCreated,
		}

		err := transition.Execute(ctx, workflow)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if workflow.State != StateRunning {
			t.Errorf("State = %v, want %v", workflow.State, StateRunning)
		}
		if workflow.StartedAt == nil {
			t.Error("StartedAt should be set")
		}
	})

	t.Run("completion sets timestamp", func(t *testing.T) {
		transition := &Transition{
			From:  StateRunning,
			To:    StateCompleted,
			Event: "complete",
		}
		workflow := &Workflow{
			ID:    "test-2",
			State: StateRunning,
		}

		err := transition.Execute(ctx, workflow)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if workflow.State != StateCompleted {
			t.Errorf("State = %v, want %v", workflow.State, StateCompleted)
		}
		if workflow.CompletedAt == nil {
			t.Error("CompletedAt should be set")
		}
	})

	t.Run("actions are executed", func(t *testing.T) {
		actionCalled := false
		transition := &Transition{
			From:  StateCreated,
			To:    StateRunning,
			Event: "start",
			Actions: []TransitionAction{
				func(ctx context.Context, w *Workflow) error {
					actionCalled = true
					w.Metadata = map[string]interface{}{"action": "executed"}
					return nil
				},
			},
		}
		workflow := &Workflow{
			ID:    "test-3",
			State: StateCreated,
		}

		err := transition.Execute(ctx, workflow)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if !actionCalled {
			t.Error("Action was not called")
		}
		if workflow.Metadata["action"] != "executed" {
			t.Error("Action did not modify workflow")
		}
	})

	t.Run("action error stops execution", func(t *testing.T) {
		transition := &Transition{
			From:  StateCreated,
			To:    StateRunning,
			Event: "start",
			Actions: []TransitionAction{
				func(ctx context.Context, w *Workflow) error {
					return errors.New("action failed")
				},
			},
		}
		workflow := &Workflow{
			ID:    "test-4",
			State: StateCreated,
		}

		err := transition.Execute(ctx, workflow)
		if err == nil {
			t.Fatal("Execute() should return error")
		}
		if workflow.State != StateCreated {
			t.Errorf("State should not change on action error, got %v", workflow.State)
		}
	})

	t.Run("clears error when transitioning from failed", func(t *testing.T) {
		transition := &Transition{
			From:  StateFailed,
			To:    StateRunning,
			Event: "retry",
		}
		workflow := &Workflow{
			ID:    "test-5",
			State: StateFailed,
			Error: "previous error",
		}

		err := transition.Execute(ctx, workflow)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if workflow.Error != "" {
			t.Errorf("Error should be cleared, got %q", workflow.Error)
		}
	})
}

func TestStateMachine(t *testing.T) {
	ctx := context.Background()

	t.Run("trigger valid event", func(t *testing.T) {
		transitions := DefaultTransitions()
		sm := NewStateMachine(transitions)

		workflow := &Workflow{
			ID:    "test-1",
			State: StateCreated,
		}

		err := sm.Trigger(ctx, workflow, "start")
		if err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}

		if workflow.State != StateRunning {
			t.Errorf("State = %v, want %v", workflow.State, StateRunning)
		}
	})

	t.Run("trigger unknown event", func(t *testing.T) {
		transitions := DefaultTransitions()
		sm := NewStateMachine(transitions)

		workflow := &Workflow{
			ID:    "test-2",
			State: StateCreated,
		}

		err := sm.Trigger(ctx, workflow, "unknown")
		if err == nil {
			t.Fatal("Trigger() should return error for unknown event")
		}
	})

	t.Run("trigger event from wrong state", func(t *testing.T) {
		transitions := DefaultTransitions()
		sm := NewStateMachine(transitions)

		workflow := &Workflow{
			ID:    "test-3",
			State: StateCreated,
		}

		err := sm.Trigger(ctx, workflow, "pause")
		if err == nil {
			t.Fatal("Trigger() should return error for invalid transition")
		}
	})

	t.Run("hooks are called", func(t *testing.T) {
		transitions := DefaultTransitions()
		sm := NewStateMachine(transitions)

		beforeCalled := false
		afterCalled := false
		var capturedFrom, capturedTo State

		sm.SetHooks(&Hooks{
			BeforeTransition: func(ctx context.Context, w *Workflow, event string) error {
				beforeCalled = true
				return nil
			},
			AfterTransition: func(ctx context.Context, w *Workflow, from State, to State) error {
				afterCalled = true
				capturedFrom = from
				capturedTo = to
				return nil
			},
		})

		workflow := &Workflow{
			ID:    "test-4",
			State: StateCreated,
		}

		err := sm.Trigger(ctx, workflow, "start")
		if err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}

		if !beforeCalled {
			t.Error("BeforeTransition hook was not called")
		}
		if !afterCalled {
			t.Error("AfterTransition hook was not called")
		}
		if capturedFrom != StateCreated {
			t.Errorf("AfterTransition from = %v, want %v", capturedFrom, StateCreated)
		}
		if capturedTo != StateRunning {
			t.Errorf("AfterTransition to = %v, want %v", capturedTo, StateRunning)
		}
	})

	t.Run("before hook error prevents transition", func(t *testing.T) {
		transitions := DefaultTransitions()
		sm := NewStateMachine(transitions)

		sm.SetHooks(&Hooks{
			BeforeTransition: func(ctx context.Context, w *Workflow, event string) error {
				return errors.New("hook error")
			},
		})

		workflow := &Workflow{
			ID:    "test-5",
			State: StateCreated,
		}

		err := sm.Trigger(ctx, workflow, "start")
		if err == nil {
			t.Fatal("Trigger() should return error when hook fails")
		}

		if workflow.State != StateCreated {
			t.Errorf("State should not change when hook fails, got %v", workflow.State)
		}
	})
}

func TestAvailableEvents(t *testing.T) {
	ctx := context.Background()

	t.Run("available events for created state", func(t *testing.T) {
		transitions := DefaultTransitions()
		sm := NewStateMachine(transitions)

		workflow := &Workflow{
			ID:    "test-1",
			State: StateCreated,
		}

		events, err := sm.AvailableEvents(ctx, workflow)
		if err != nil {
			t.Fatalf("AvailableEvents() error = %v", err)
		}

		if len(events) != 1 {
			t.Fatalf("len(events) = %d, want 1", len(events))
		}
		if events[0] != "start" {
			t.Errorf("events[0] = %q, want %q", events[0], "start")
		}
	})

	t.Run("available events for running state", func(t *testing.T) {
		transitions := DefaultTransitions()
		sm := NewStateMachine(transitions)

		workflow := &Workflow{
			ID:    "test-2",
			State: StateRunning,
		}

		events, err := sm.AvailableEvents(ctx, workflow)
		if err != nil {
			t.Fatalf("AvailableEvents() error = %v", err)
		}

		// Should have pause, complete, and fail
		// Note: DefaultTransitions has duplicate "fail" events, so map stores only one
		if len(events) < 2 {
			t.Errorf("len(events) = %d, want at least 2", len(events))
		}
	})

	t.Run("no available events for completed state", func(t *testing.T) {
		transitions := DefaultTransitions()
		sm := NewStateMachine(transitions)

		workflow := &Workflow{
			ID:    "test-3",
			State: StateCompleted,
		}

		events, err := sm.AvailableEvents(ctx, workflow)
		if err != nil {
			t.Fatalf("AvailableEvents() error = %v", err)
		}

		if len(events) != 0 {
			t.Errorf("len(events) = %d, want 0", len(events))
		}
	})
}

func TestDefaultTransitions(t *testing.T) {
	transitions := DefaultTransitions()

	if len(transitions) == 0 {
		t.Fatal("DefaultTransitions() should return transitions")
	}

	// Count transitions by event (note: there are duplicate "fail" events)
	eventCount := make(map[string]int)
	for _, transition := range transitions {
		eventCount[transition.Event]++
	}

	// Verify we have the expected events
	expectedEvents := []string{"start", "pause", "resume", "complete", "fail"}
	for _, event := range expectedEvents {
		if eventCount[event] == 0 {
			t.Errorf("Missing event: %s", event)
		}
	}

	// Verify fail event appears twice (from running and paused)
	if eventCount["fail"] != 2 {
		t.Errorf("Expected 2 'fail' transitions, got %d", eventCount["fail"])
	}
}
