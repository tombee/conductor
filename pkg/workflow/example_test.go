package workflow_test

import (
	"context"
	"fmt"
	"log"

	"github.com/tombee/conductor/pkg/workflow"
)

// Example demonstrates a complete workflow lifecycle with state machine,
// event emitter, and persistent storage.
func Example() {
	ctx := context.Background()

	// Create a workflow store
	store := workflow.NewMemoryStore()

	// Create an event emitter (async mode)
	emitter := workflow.NewEventEmitter(true)

	// Register event listeners
	emitter.On(workflow.EventStateChanged, func(ctx context.Context, event *workflow.Event) error {
		fmt.Printf("State changed: %v -> %v\n",
			event.Data["from_state"],
			event.Data["to_state"])
		return nil
	})

	// Create a state machine with default transitions
	sm := workflow.NewStateMachine(workflow.DefaultTransitions())

	// Set up hooks to emit events and persist state
	sm.SetHooks(&workflow.Hooks{
		AfterTransition: func(ctx context.Context, w *workflow.Workflow, from workflow.State, to workflow.State) error {
			// Emit state change event
			if err := emitter.EmitStateChanged(ctx, w.ID, from, to, "transition"); err != nil {
				return err
			}
			// Persist the workflow
			return store.Update(ctx, w)
		},
	})

	// Create a new workflow
	wf := &workflow.Workflow{
		ID:   "example-workflow-1",
		Name: "Example Workflow",
		Metadata: map[string]interface{}{
			"project": "demo",
			"priority": "high",
		},
	}

	// Save it to the store
	if err := store.Create(ctx, wf); err != nil {
		log.Fatal(err)
	}

	// Trigger state transitions
	if err := sm.Trigger(ctx, wf, "start"); err != nil {
		log.Fatal(err)
	}

	if err := sm.Trigger(ctx, wf, "complete"); err != nil {
		log.Fatal(err)
	}

	// Retrieve the workflow from storage
	retrieved, err := store.Get(ctx, "example-workflow-1")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Final state: %s\n", retrieved.State)
	fmt.Printf("Is terminal: %v\n", retrieved.State.IsTerminal())

	// Output:
	// State changed: created -> running
	// State changed: running -> completed
	// Final state: completed
	// Is terminal: true
}

// ExampleQuery demonstrates querying workflows by state and metadata.
func Example_query() {
	ctx := context.Background()
	store := workflow.NewMemoryStore()

	// Create multiple workflows
	workflows := []*workflow.Workflow{
		{ID: "wf-1", State: workflow.StateRunning, Metadata: map[string]interface{}{"priority": "high"}},
		{ID: "wf-2", State: workflow.StateCompleted, Metadata: map[string]interface{}{"priority": "low"}},
		{ID: "wf-3", State: workflow.StateRunning, Metadata: map[string]interface{}{"priority": "high"}},
		{ID: "wf-4", State: workflow.StateFailed, Metadata: map[string]interface{}{"priority": "high"}},
	}

	for _, wf := range workflows {
		if err := store.Create(ctx, wf); err != nil {
			log.Fatal(err)
		}
	}

	// Query: Find all running workflows with high priority
	state := workflow.StateRunning
	results, err := store.List(ctx, &workflow.Query{
		State:    &state,
		Metadata: map[string]interface{}{"priority": "high"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d running high-priority workflows\n", len(results))

	// Output:
	// Found 2 running high-priority workflows
}
