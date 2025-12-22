# Workflow Package

The `workflow` package provides workflow orchestration primitives for LLM-based automation.

## Components

### Workflow Definition Parser (`definition.go`)

YAML-based workflow definitions with support for:
- **Inputs**: Type-validated input parameters (string, number, boolean, object, array)
- **Steps**: Action, LLM, Condition, and Parallel step types
- **Error Handling**: Fail, Ignore, Retry, and Fallback strategies
- **Retry Configuration**: Exponential backoff with configurable attempts
- **Outputs**: Computed outputs from step results

Example workflow definition:

```yaml
name: example-workflow
version: "1.0"
description: A sample workflow

inputs:
  - name: input_file
    type: string
    required: true

steps:
  - id: read_file
    name: Read Input File
    file.read: ${input_file}

  - id: process
    name: Process with LLM
    type: llm
    prompt: "Analyze this content: ${read_file.content}"

outputs:
  - name: result
    type: string
    value: $.process.response
```

### Step Executor (`executor.go`)

Executes individual workflow steps with:
- **Tool Integration**: Execute registered tools for action steps
- **LLM Integration**: Make LLM calls for llm steps
- **Timeout Management**: Per-step timeout with context cancellation
- **Retry Logic**: Exponential backoff retry for failed steps
- **Error Handling**: Configurable error strategies (fail, ignore, retry, fallback)

### State Machine (`workflow.go`)

State-based workflow execution with:
- **States**: Created, Running, Paused, Completed, Failed
- **Transitions**: Event-driven state changes with guards and actions
- **Hooks**: BeforeTransition, AfterTransition, OnError lifecycle hooks
- **Lifecycle Tracking**: StartedAt, CompletedAt timestamps

### Event System (`events.go`)

Pub/sub event system for workflow observability:
- **Event Types**: StateChanged, StepCompleted, Error
- **Synchronous/Asynchronous**: Configurable listener execution
- **Multiple Listeners**: Support for multiple listeners per event type

### Storage (`store.go`)

In-memory and persistent workflow storage:
- **CRUD Operations**: Create, Read, Update, Delete workflows
- **List/Query**: Find workflows by state, date range
- **Thread-Safe**: RWMutex-protected concurrent access

## API Examples

### Loading and Executing a Workflow

```go
import (
    "context"
    "github.com/tombee/conductor/pkg/workflow"
    "github.com/tombee/conductor/pkg/llm"
    "github.com/tombee/conductor/pkg/tools"
)

func main() {
    // Parse workflow from YAML
    def, err := workflow.ParseDefinition("workflow.yaml")
    if err != nil {
        panic(err)
    }

    // Create workflow instance
    wf := workflow.NewWorkflow(def)

    // Set up dependencies
    llmProvider, _ := llm.GetDefault()
    toolRegistry := tools.NewRegistry()

    // Create executor
    executor := workflow.NewExecutor(llmProvider, toolRegistry)

    // Execute workflow
    ctx := context.Background()
    result, err := executor.Execute(ctx, wf, map[string]interface{}{
        "input_file": "/path/to/file.txt",
    })

    if err != nil {
        panic(err)
    }

    println("Workflow completed:", result.Outputs["result"])
}
```

### Creating a Workflow Programmatically

```go
// Create workflow definition
def := &workflow.Definition{
    Name:        "my-workflow",
    Version:     "1.0",
    Description: "Example workflow",
    Inputs: []workflow.Input{
        {
            Name:     "message",
            Type:     "string",
            Required: true,
        },
    },
    Steps: []workflow.Step{
        {
            ID:     "process",
            Name:   "Process Message",
            Type:   workflow.StepTypeLLM,
            Action: "complete",
            Inputs: map[string]interface{}{
                "prompt": "Analyze this: ${message}",
            },
        },
    },
    Outputs: []workflow.Output{
        {
            Name:  "result",
            Type:  "string",
            Value: "$.process.response",
        },
    },
}

// Create and run workflow
wf := workflow.NewWorkflow(def)
executor := workflow.NewExecutor(llmProvider, toolRegistry)
result, _ := executor.Execute(ctx, wf, map[string]interface{}{
    "message": "Hello, world!",
})
```

### State Machine Management

```go
// Create workflow with state tracking
wf := workflow.NewWorkflow(def)

// Check current state
println("State:", wf.State) // "created"

// Transition to running
err := wf.Transition(workflow.StateRunning)
if err != nil {
    panic(err) // Transition not allowed
}

// Add lifecycle hooks
wf.OnBeforeTransition(func(from, to workflow.State) error {
    println("Transitioning from", from, "to", to)
    return nil // Return error to prevent transition
})

wf.OnAfterTransition(func(from, to workflow.State) {
    println("Transitioned to", to)
})

wf.OnError(func(err error) {
    println("Workflow error:", err)
})
```

### Event Subscription

```go
// Create event bus
bus := workflow.NewEventBus()

// Subscribe to workflow events
bus.Subscribe(workflow.EventTypeStateChanged, func(event workflow.Event) {
    println("State changed:", event.Data["old_state"], "→", event.Data["new_state"])
})

bus.Subscribe(workflow.EventTypeStepCompleted, func(event workflow.Event) {
    println("Step completed:", event.Data["step_id"])
})

bus.Subscribe(workflow.EventTypeError, func(event workflow.Event) {
    println("Error:", event.Data["error"])
})

// Emit events during workflow execution
executor := workflow.NewExecutorWithBus(llmProvider, toolRegistry, bus)
```

### Workflow Storage

```go
// Create in-memory store
store := workflow.NewStore()

// Save workflow
err := store.Create(wf)

// Retrieve workflow
wf, err := store.Get(wf.ID)

// List workflows
workflows, err := store.List()

// Find by state
running := store.FindByState(workflow.StateRunning)

// Update workflow
wf.State = workflow.StateCompleted
err = store.Update(wf)

// Delete workflow
err = store.Delete(wf.ID)
```

### Error Handling Configuration

```go
// Configure retry for a step
step := workflow.Step{
    ID:               "flaky_step",
    Type:             workflow.StepTypeBuiltin,
    BuiltinConnector: "shell",
    BuiltinOperation: "run",
    OnError: workflow.ErrorHandlerRetry,
    RetryConfig: &workflow.RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 1 * time.Second,
        MaxDelay:     10 * time.Second,
        Multiplier:   2.0,
    },
}

// Configure fallback
step := workflow.Step{
    ID:               "primary_step",
    Type:             workflow.StepTypeBuiltin,
    BuiltinConnector: "file",
    BuiltinOperation: "read",
    OnError: workflow.ErrorHandlerFallback,
    FallbackStep: &workflow.Step{
        ID:               "fallback_step",
        Type:             workflow.StepTypeBuiltin,
        BuiltinConnector: "file",
        BuiltinOperation: "read",
    },
}

// Ignore errors
step := workflow.Step{
    ID:               "optional_step",
    Type:             workflow.StepTypeBuiltin,
    BuiltinConnector: "file",
    BuiltinOperation: "read",
    OnError: workflow.ErrorHandlerIgnore,
}
```

## Phase 1 Status

**Implemented:**
- ✅ YAML workflow definition parser with validation
- ✅ Step executor with tool and LLM integration
- ✅ Basic state machine with transition guards
- ✅ Event emission and subscription
- ✅ In-memory workflow storage

**Phase 1 Limitations:**
- Expression evaluation is placeholder (returns true)
- Variable substitution is simple copy (no $.path.to.value resolution)
- Parallel step execution is placeholder
- Fallback steps are not automatically executed

**Future Enhancements:**
- Expression language (CEL) for conditions
- JSONPath variable substitution
- Concurrent parallel step execution
- Workflow composition (sub-workflows)
- PostgreSQL storage backend
