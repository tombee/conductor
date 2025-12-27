// Package workflow provides workflow orchestration primitives for LLM-based automation.
//
// This package defines a state machine-based workflow system with support for:
//   - State transitions with guard conditions and actions
//   - Event-driven state changes
//   - Persistence hooks for workflow state
//   - Pluggable storage backends
//
// The workflow system is designed to be embedded in other applications and supports
// the typical lifecycle: created -> running -> (paused) -> completed/failed.
package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/tombee/conductor/pkg/errors"
)

// State represents a workflow state.
type State string

// Workflow states
const (
	StateCreated   State = "created"
	StateRunning   State = "running"
	StatePaused    State = "paused"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
)

// Valid states for validation
var validStates = map[State]bool{
	StateCreated:   true,
	StateRunning:   true,
	StatePaused:    true,
	StateCompleted: true,
	StateFailed:    true,
}

// IsValid checks if a state is valid.
func (s State) IsValid() bool {
	return validStates[s]
}

// IsTerminal returns true if the state is terminal (no further transitions).
func (s State) IsTerminal() bool {
	return s == StateCompleted || s == StateFailed
}

// Workflow represents a workflow instance with its current state and metadata.
type Workflow struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	State       State                  `json:"state"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// TransitionGuard is a function that determines if a transition is allowed.
// It receives the current workflow and returns true if the transition should proceed.
type TransitionGuard func(ctx context.Context, w *Workflow) (bool, error)

// TransitionAction is a function that is executed during a state transition.
// It receives the current workflow and can modify it or perform side effects.
type TransitionAction func(ctx context.Context, w *Workflow) error

// Transition defines a state transition with guards and actions.
type Transition struct {
	From    State
	To      State
	Event   string
	Guards  []TransitionGuard
	Actions []TransitionAction
}

// CanTransition checks if the transition is allowed based on current state and guards.
func (t *Transition) CanTransition(ctx context.Context, w *Workflow) (bool, error) {
	// Check if we're in the correct starting state
	if w.State != t.From {
		return false, nil
	}

	// Run all guards
	for _, guard := range t.Guards {
		allowed, err := guard(ctx, w)
		if err != nil {
			return false, fmt.Errorf("guard error: %w", err)
		}
		if !allowed {
			return false, nil
		}
	}

	return true, nil
}

// Execute performs the transition and runs all actions.
func (t *Transition) Execute(ctx context.Context, w *Workflow) error {
	// Execute all actions
	for _, action := range t.Actions {
		if err := action(ctx, w); err != nil {
			return fmt.Errorf("action error: %w", err)
		}
	}

	// Update state
	oldState := w.State
	w.State = t.To
	w.UpdatedAt = time.Now()

	// Update lifecycle timestamps
	switch t.To {
	case StateRunning:
		if w.StartedAt == nil {
			now := time.Now()
			w.StartedAt = &now
		}
	case StateCompleted, StateFailed:
		if w.CompletedAt == nil {
			now := time.Now()
			w.CompletedAt = &now
		}
	}

	// Clear error if transitioning away from failed
	if oldState == StateFailed && t.To != StateFailed {
		w.Error = ""
	}

	return nil
}

// StateMachine manages workflow state transitions.
type StateMachine struct {
	transitions map[string]*Transition // key: event name
	hooks       *Hooks
}

// Hooks defines lifecycle hooks for the state machine.
type Hooks struct {
	BeforeTransition func(ctx context.Context, w *Workflow, event string) error
	AfterTransition  func(ctx context.Context, w *Workflow, from State, to State) error
	OnError          func(ctx context.Context, w *Workflow, err error) error
}

// NewStateMachine creates a new state machine with the given transitions.
func NewStateMachine(transitions []*Transition) *StateMachine {
	sm := &StateMachine{
		transitions: make(map[string]*Transition),
		hooks:       &Hooks{},
	}

	for _, t := range transitions {
		sm.transitions[t.Event] = t
	}

	return sm
}

// SetHooks configures lifecycle hooks for the state machine.
func (sm *StateMachine) SetHooks(hooks *Hooks) {
	if hooks != nil {
		sm.hooks = hooks
	}
}

// Trigger attempts to trigger an event and transition the workflow.
func (sm *StateMachine) Trigger(ctx context.Context, w *Workflow, event string) error {
	// Find transition for event
	transition, ok := sm.transitions[event]
	if !ok {
		return &errors.ValidationError{
			Field:      "event",
			Message:    fmt.Sprintf("unknown event: %s", event),
			Suggestion: "use one of the valid events for the current state",
		}
	}

	// Check if transition is allowed
	allowed, err := transition.CanTransition(ctx, w)
	if err != nil {
		if sm.hooks.OnError != nil {
			if hookErr := sm.hooks.OnError(ctx, w, err); hookErr != nil {
				return fmt.Errorf("transition guard error: %w (hook error: %v)", err, hookErr)
			}
		}
		return fmt.Errorf("transition guard error: %w", err)
	}
	if !allowed {
		return &errors.ValidationError{
			Field:      "state",
			Message:    fmt.Sprintf("transition not allowed: from %s on event %s", w.State, event),
			Suggestion: fmt.Sprintf("workflow must be in correct state to trigger event %s", event),
		}
	}

	// Store old state for hook
	oldState := w.State

	// Call before transition hook
	if sm.hooks.BeforeTransition != nil {
		if err := sm.hooks.BeforeTransition(ctx, w, event); err != nil {
			if sm.hooks.OnError != nil {
				if hookErr := sm.hooks.OnError(ctx, w, err); hookErr != nil {
					return fmt.Errorf("before transition hook error: %w (error hook error: %v)", err, hookErr)
				}
			}
			return fmt.Errorf("before transition hook error: %w", err)
		}
	}

	// Execute transition
	if err := transition.Execute(ctx, w); err != nil {
		if sm.hooks.OnError != nil {
			if hookErr := sm.hooks.OnError(ctx, w, err); hookErr != nil {
				return fmt.Errorf("transition execution error: %w (hook error: %v)", err, hookErr)
			}
		}
		return fmt.Errorf("transition execution error: %w", err)
	}

	// Call after transition hook
	if sm.hooks.AfterTransition != nil {
		if err := sm.hooks.AfterTransition(ctx, w, oldState, w.State); err != nil {
			if sm.hooks.OnError != nil {
				if hookErr := sm.hooks.OnError(ctx, w, err); hookErr != nil {
					return fmt.Errorf("after transition hook error: %w (error hook error: %v)", err, hookErr)
				}
			}
			return fmt.Errorf("after transition hook error: %w", err)
		}
	}

	return nil
}

// AvailableEvents returns the list of events that can be triggered from the current state.
func (sm *StateMachine) AvailableEvents(ctx context.Context, w *Workflow) ([]string, error) {
	var events []string

	for event, transition := range sm.transitions {
		allowed, err := transition.CanTransition(ctx, w)
		if err != nil {
			return nil, fmt.Errorf("error checking transition for event %s: %w", event, err)
		}
		if allowed {
			events = append(events, event)
		}
	}

	return events, nil
}

// DefaultTransitions returns a standard set of workflow transitions.
func DefaultTransitions() []*Transition {
	return []*Transition{
		{
			From:  StateCreated,
			To:    StateRunning,
			Event: "start",
		},
		{
			From:  StateRunning,
			To:    StatePaused,
			Event: "pause",
		},
		{
			From:  StatePaused,
			To:    StateRunning,
			Event: "resume",
		},
		{
			From:  StateRunning,
			To:    StateCompleted,
			Event: "complete",
		},
		{
			From:  StateRunning,
			To:    StateFailed,
			Event: "fail",
		},
		{
			From:  StatePaused,
			To:    StateFailed,
			Event: "fail",
		},
	}
}
