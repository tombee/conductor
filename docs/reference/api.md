# API Reference

Go package documentation for embedding Conductor in applications.

## Overview

Conductor's packages are designed to be embedded in Go applications. This reference documents the public APIs for all core packages.

For complete API reference including all methods and types, see the [GoDoc documentation](https://pkg.go.dev/github.com/tombee/conductor).

## Package Index

- [pkg/llm](#pkgllm) - LLM provider abstraction
- [pkg/workflow](#pkgworkflow) - Workflow engine
- [pkg/agent](#pkgagent) - Agent execution loop
- [pkg/tools](#pkgtools) - Tool registry and execution

---

## pkg/llm

Provider-agnostic interface for Large Language Model interactions.

### Provider Interface

The core interface all LLM providers must implement:

```go
type Provider interface {
    // Name returns the unique identifier for this provider
    Name() string

    // Capabilities returns the provider's supported features
    Capabilities() Capabilities

    // Complete sends a synchronous completion request
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

    // Stream sends a streaming completion request
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}
```

### Registry

Manage multiple LLM providers:

```go
// Create a new registry
registry := llm.NewRegistry()

// Register providers
anthropic := providers.NewAnthropicProvider(apiKey)
registry.Register(anthropic)

// Set default provider
registry.SetDefault("anthropic")

// Get a provider
provider, err := registry.Get("anthropic")

// Get default provider
provider, err := registry.GetDefault()

// List all providers
names := registry.List() // []string{"anthropic", "openai"}

// Configure failover
registry.SetFailoverOrder([]string{"anthropic", "openai", "ollama"})
```

Global registry (convenience functions):

```go
llm.Register(provider)
llm.SetDefault("anthropic")
provider, _ := llm.Get("anthropic")
provider, _ := llm.GetDefault()
```

### Completion Requests

Make LLM completion requests:

```go
req := llm.CompletionRequest{
    Messages: []llm.Message{
        {Role: llm.MessageRoleSystem, Content: "You are a helpful assistant."},
        {Role: llm.MessageRoleUser, Content: "What is the capital of France?"},
    },
    Model: "fast", // or "balanced", "strategic", or specific model ID
    Temperature: &temperature, // optional
    MaxTokens: &maxTokens,     // optional
    Tools: []llm.Tool{...},    // optional
    Metadata: map[string]string{"correlation_id": "req-123"},
}

// Synchronous completion
resp, err := provider.Complete(ctx, req)
if err != nil {
    log.Fatal(err)
}
fmt.Println(resp.Content)
fmt.Printf("Tokens used: %d\n", resp.Usage.TotalTokens)

// Streaming completion
stream, err := provider.Stream(ctx, req)
if err != nil {
    log.Fatal(err)
}

for chunk := range stream {
    if chunk.Error != nil {
        log.Printf("Stream error: %v", chunk.Error)
        break
    }
    fmt.Print(chunk.Delta.Content)

    if chunk.FinishReason != "" {
        fmt.Printf("\nFinish reason: %s\n", chunk.FinishReason)
        fmt.Printf("Total tokens: %d\n", chunk.Usage.TotalTokens)
    }
}
```

### Tool Calling

Define tools the LLM can use:

```go
tool := llm.Tool{
    Name: "get_weather",
    Description: "Get current weather for a location",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{
                "type": "string",
                "description": "City name",
            },
            "units": map[string]interface{}{
                "type": "string",
                "enum": []string{"celsius", "fahrenheit"},
            },
        },
        "required": []string{"location"},
    },
}

req := llm.CompletionRequest{
    Messages: messages,
    Model: "balanced",
    Tools: []llm.Tool{tool},
}

resp, _ := provider.Complete(ctx, req)

// Handle tool calls
if len(resp.ToolCalls) > 0 {
    for _, toolCall := range resp.ToolCalls {
        fmt.Printf("Tool: %s\n", toolCall.Name)
        fmt.Printf("Arguments: %s\n", toolCall.Arguments)

        // Execute tool and add result to conversation
        result := executeWeatherTool(toolCall.Arguments)
        messages = append(messages, llm.Message{
            Role: llm.MessageRoleTool,
            Content: result,
            ToolCallID: toolCall.ID,
            Name: toolCall.Name,
        })
    }

    // Continue conversation with tool results
    resp, _ = provider.Complete(ctx, llm.CompletionRequest{
        Messages: messages,
        Model: "balanced",
    })
}
```

### Cost Tracking

Track token usage and costs:

```go
import "github.com/tombee/conductor/pkg/llm"

// Create cost tracker
tracker := llm.NewCostTracker()

// Track a completion
cost := tracker.TrackCompletion(
    "anthropic",
    "claude-3-5-sonnet-20241022",
    resp.Usage,
    "correlation-id-123",
)

fmt.Printf("Cost: $%.6f\n", cost.TotalCost)
fmt.Printf("Input tokens: %d ($%.6f)\n", cost.InputTokens, cost.InputCost)
fmt.Printf("Output tokens: %d ($%.6f)\n", cost.OutputTokens, cost.OutputCost)

// Get total costs
totalCost := tracker.GetTotalCost()
fmt.Printf("Total cost across all calls: $%.4f\n", totalCost)

// Get costs by provider
costs := tracker.GetCostsByProvider()
for provider, cost := range costs {
    fmt.Printf("%s: $%.4f\n", provider, cost)
}

// Get costs by correlation ID
sessionCost := tracker.GetCostsByCorrelationID("session-456")
```

### Model Tiers

Abstract model selection using tiers:

```go
// Get model for tier
model := llm.GetModelForTier("fast", "anthropic")
// Returns: "claude-3-5-haiku-20241022"

model = llm.GetModelForTier("balanced", "anthropic")
// Returns: "claude-3-5-sonnet-20241022"

model = llm.GetModelForTier("strategic", "anthropic")
// Returns: "claude-3-opus-20240229"

// List available models for a provider
models := llm.ListModelsForProvider("anthropic")
for _, m := range models {
    fmt.Printf("%s: %s (tier: %s)\n", m.ID, m.Name, m.Tier)
}
```

### Retry and Failover

Wrap providers with retry and failover logic:

```go
// Add retry wrapper
retriableProvider := llm.WithRetry(provider, llm.RetryConfig{
    MaxAttempts:        5,
    InitialBackoff:     100 * time.Millisecond,
    MaxBackoff:         5 * time.Second,
    BackoffMultiplier:  2.0,
})

// Add failover wrapper
primaryProvider := anthropic
secondaryProvider := openai

failoverProvider := llm.WithFailover(
    primaryProvider,
    []llm.Provider{secondaryProvider},
    llm.FailoverConfig{
        CircuitBreakerThreshold: 5,
        CircuitBreakerTimeout:   30 * time.Second,
    },
)
```

---

## pkg/workflow

State machine-based workflow orchestration.

### Definition

Define workflows in YAML or programmatically:

```go
import "github.com/tombee/conductor/pkg/workflow"

// Parse YAML definition
data, _ := os.ReadFile("workflow.yaml")
def, err := workflow.ParseDefinition(data)
if err != nil {
    log.Fatal(err)
}

// Or create programmatically
def := &workflow.Definition{
    Name:        "my-workflow",
    Description: "Example workflow",
    Version:     "1.0",
    Inputs: []workflow.Input{
        {
            Name:        "user_input",
            Type:        "string",
            Required:    true,
            Description: "User input text",
        },
    },
    Steps: []workflow.Step{
        {
            ID:      "process",
            Name:    "Process Input",
            Type:    workflow.StepTypeLLM,
            Prompt:  "{{.user_input}}",
            Model:   "fast",
            System:  "You are a helpful assistant.",
            Timeout: 30,
        },
    },
    Outputs: []workflow.Output{
        {
            Name:        "result",
            Type:        "string",
            Value:       "$.process.content",
            Description: "Processing result",
        },
    },
}

// Validate definition
if err := def.Validate(); err != nil {
    log.Fatal(err)
}
```

### Execution

Execute workflows:

```go
// Create engine
engine := workflow.NewEngine(
    workflow.WithLLMProvider(llmProvider),
    workflow.WithToolRegistry(toolRegistry),
)

// Execute workflow
result, err := engine.Execute(ctx, def, map[string]interface{}{
    "user_input": "Hello, world!",
})

if err != nil {
    log.Printf("Workflow failed: %v", err)
    return
}

// Access outputs
fmt.Printf("Result: %s\n", result.Outputs["result"])

// Check execution details
fmt.Printf("Duration: %s\n", result.Duration)
fmt.Printf("Steps completed: %d\n", len(result.Steps))
for _, step := range result.Steps {
    fmt.Printf("  %s: %s\n", step.ID, step.Status)
}
```

### State Management

Store and retrieve workflow state:

```go
// Create store
store := workflow.NewSQLiteStore("conductor.db")

// Save workflow instance
instance := &workflow.Instance{
    ID:         "wf-123",
    Definition: def,
    State:      workflow.StateRunning,
    Inputs:     inputs,
    CreatedAt:  time.Now(),
}
err := store.Save(ctx, instance)

// Get workflow instance
instance, err := store.Get(ctx, "wf-123")

// List workflows
instances, err := store.List(ctx, workflow.ListOptions{
    State: workflow.StateCompleted,
    Limit: 10,
})

// Update state
instance.State = workflow.StateCompleted
instance.Outputs = outputs
instance.CompletedAt = time.Now()
err = store.Update(ctx, instance)
```

### Events

Subscribe to workflow events:

```go
// Create event bus
bus := workflow.NewEventBus()

// Subscribe to state changes
bus.Subscribe(workflow.EventTypeStateChanged, func(event workflow.Event) {
    stateEvent := event.(workflow.StateChangedEvent)
    fmt.Printf("Workflow %s: %s -> %s\n",
        stateEvent.WorkflowID,
        stateEvent.OldState,
        stateEvent.NewState,
    )
})

// Subscribe to step completions
bus.Subscribe(workflow.EventTypeStepCompleted, func(event workflow.Event) {
    stepEvent := event.(workflow.StepCompletedEvent)
    fmt.Printf("Step %s completed in %s\n",
        stepEvent.StepID,
        stepEvent.Duration,
    )
})

// Subscribe to errors
bus.Subscribe(workflow.EventTypeError, func(event workflow.Event) {
    errorEvent := event.(workflow.ErrorEvent)
    log.Printf("Error in workflow %s: %v", errorEvent.WorkflowID, errorEvent.Error)
})

// Create engine with event bus
engine := workflow.NewEngine(
    workflow.WithEventBus(bus),
)
```

### Step Types

Different step types available:

```go
// LLM step
llmStep := workflow.Step{
    ID:     "llm_call",
    Type:   workflow.StepTypeLLM,
    Prompt: "Analyze this text: {{.input}}",
    Model:  "balanced",
}

// Builtin connector step (file operations)
builtinStep := workflow.Step{
    ID:               "read_file",
    Type:             workflow.StepTypeBuiltin,
    BuiltinConnector: "file",
    BuiltinOperation: "read",
    Inputs: map[string]interface{}{
        "path": "{{.file_path}}",
    },
}

// Connector step (HTTP connector)
connectorStep := workflow.Step{
    ID:        "create_issue",
    Type:      workflow.StepTypeConnector,
    Connector: "github.create_issue",
    Action: "my_custom_action",
    Inputs: map[string]interface{}{
        "param": "{{.value}}",
    },
}

// Condition step
conditionStep := workflow.Step{
    ID:   "check_success",
    Type: workflow.StepTypeCondition,
    Condition: &workflow.Condition{
        Expression: "$.llm_call.success == true",
        ThenSteps:  []string{"success_handler"},
        ElseSteps:  []string{"error_handler"},
    },
}

// Parallel step (Phase 2)
parallelStep := workflow.Step{
    ID:   "parallel_processing",
    Type: workflow.StepTypeParallel,
    Steps: []workflow.Step{
        // ... nested steps that run in parallel
    },
}
```

---

## pkg/agent

ReAct-style agent that uses tools to accomplish tasks.

### Agent Creation

Create and configure agents:

```go
import "github.com/tombee/conductor/pkg/agent"

// Create agent
ag := agent.NewAgent(llmProvider, toolRegistry)

// Configure max iterations
ag = ag.WithMaxIterations(30) // default: 20

// Add streaming handler
ag = ag.WithStreamHandler(func(event agent.StreamEvent) {
    fmt.Printf("[%s] %v\n", event.Type, event.Content)
})
```

### Agent Execution

Run agents with tasks:

```go
result, err := ag.Run(
    ctx,
    "You are a helpful coding assistant.",  // system prompt
    "Read main.go and count the functions", // user prompt
)

if err != nil {
    log.Printf("Agent failed: %v", err)
}

if result.Success {
    fmt.Printf("Result: %s\n", result.FinalResponse)
} else {
    fmt.Printf("Agent failed: %s\n", result.Error)
}

// Examine execution details
fmt.Printf("Iterations: %d\n", result.Iterations)
fmt.Printf("Duration: %s\n", result.Duration)
fmt.Printf("Tokens used: %d\n", result.TokensUsed.TotalTokens)

// Review tool executions
for _, exec := range result.ToolExecutions {
    fmt.Printf("Tool: %s\n", exec.ToolName)
    fmt.Printf("  Success: %v\n", exec.Success)
    fmt.Printf("  Duration: %s\n", exec.Duration)
    if !exec.Success {
        fmt.Printf("  Error: %s\n", exec.Error)
    }
}
```

### Context Management

The agent automatically manages context window:

```go
// Context manager tracks token usage
contextMgr := agent.NewContextManager(100000) // 100k token limit

// Check if pruning is needed
messages := []agent.Message{...}
if contextMgr.ShouldPrune(messages) {
    // Prune oldest messages while keeping system prompt
    messages = contextMgr.Prune(messages)
}

// Context manager is automatic when using agent.Run()
// Manual usage is for advanced scenarios
```

### LLM Provider Adapter

Adapt Conductor's LLM provider to agent's interface:

```go
import (
    "github.com/tombee/conductor/pkg/llm"
    "github.com/tombee/conductor/pkg/agent"
)

// Conductor's LLM provider
conductorProvider := llm.GetDefault()

// Create adapter for agent
agentProvider := agent.NewLLMAdapter(conductorProvider)

// Use with agent
ag := agent.NewAgent(agentProvider, toolRegistry)
```

---

## pkg/tools

Registry and execution framework for agent tools.

### Tool Interface

Implement custom tools:

```go
import "github.com/tombee/conductor/pkg/tools"

type WeatherTool struct{}

func (t *WeatherTool) Name() string {
    return "get_weather"
}

func (t *WeatherTool) Description() string {
    return "Get current weather for a location"
}

func (t *WeatherTool) Schema() *tools.Schema {
    return &tools.Schema{
        Inputs: &tools.ParameterSchema{
            Type: "object",
            Properties: map[string]*tools.Property{
                "location": {
                    Type:        "string",
                    Description: "City name",
                },
                "units": {
                    Type:        "string",
                    Description: "Temperature units",
                    Enum:        []interface{}{"celsius", "fahrenheit"},
                    Default:     "celsius",
                },
            },
            Required: []string{"location"},
        },
        Outputs: &tools.ParameterSchema{
            Type: "object",
            Properties: map[string]*tools.Property{
                "temperature": {
                    Type:        "number",
                    Description: "Current temperature",
                },
                "conditions": {
                    Type:        "string",
                    Description: "Weather conditions",
                },
            },
        },
    }
}

func (t *WeatherTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
    location := inputs["location"].(string)
    units := inputs["units"].(string)

    // Call weather API...
    temp, conditions := getWeather(location, units)

    return map[string]interface{}{
        "temperature": temp,
        "conditions":  conditions,
    }, nil
}
```

### Registry Operations

Manage tool registry:

```go
// Create registry
registry := tools.NewRegistry()

// Register tools
registry.Register(&WeatherTool{})
registry.Register(tools.NewFileTool("/allowed/path"))
registry.Register(tools.NewShellTool([]string{"ls", "cat", "grep"}))
registry.Register(tools.NewHTTPTool([]string{"api.example.com"}))

// Get a tool
tool, err := registry.Get("get_weather")

// Check if tool exists
if registry.Has("get_weather") {
    // ...
}

// List all tools
toolNames := registry.List()
fmt.Println(toolNames) // ["get_weather", "file", "shell", "http"]

// Execute a tool
outputs, err := registry.Execute(ctx, "get_weather", map[string]interface{}{
    "location": "Paris",
    "units":    "celsius",
})

// Get tool schemas for LLM
descriptors := registry.GetToolDescriptors()
// Use descriptors when calling LLM with function calling
```

### Builtin Tools

Use provided builtin tools:

**File Tool:**

```go
import "github.com/tombee/conductor/pkg/tools/builtin"

fileTool := builtin.NewFileTool(builtin.FileToolConfig{
    AllowedPaths: []string{"/workspace", "/tmp"},
    MaxFileSize:  10 * 1024 * 1024, // 10MB
})

registry.Register(fileTool)

// Read file
outputs, _ := registry.Execute(ctx, "file", map[string]interface{}{
    "operation": "read",
    "path":      "/workspace/main.go",
})
content := outputs["content"].(string)

// Write file
outputs, _ = registry.Execute(ctx, "file", map[string]interface{}{
    "operation": "write",
    "path":      "/workspace/output.txt",
    "content":   "Hello, world!",
})
```

**Shell Tool:**

```go
shellTool := builtin.NewShellTool(builtin.ShellToolConfig{
    AllowedCommands: []string{"git", "ls", "cat", "grep"},
    Timeout:         30 * time.Second,
})

registry.Register(shellTool)

// Execute command
outputs, _ := registry.Execute(ctx, "shell", map[string]interface{}{
    "command": "git",
    "args":    []string{"status"},
})
stdout := outputs["stdout"].(string)
exitCode := outputs["exit_code"].(int)
```

**HTTP Tool:**

```go
httpTool := builtin.NewHTTPTool(builtin.HTTPToolConfig{
    AllowedHosts: []string{"api.github.com", "api.example.com"},
    Timeout:      15 * time.Second,
})

registry.Register(httpTool)

// Make HTTP request
outputs, _ := registry.Execute(ctx, "http", map[string]interface{}{
    "method": "GET",
    "url":    "https://api.github.com/repos/tombee/conductor",
    "headers": map[string]string{
        "Accept": "application/json",
    },
})
statusCode := outputs["status_code"].(int)
body := outputs["body"].(string)
```

---

## Error Handling

All packages follow consistent error handling patterns:

```go
// Check for specific errors
if errors.Is(err, llm.ErrProviderNotFound) {
    // Handle missing provider
}

if errors.Is(err, workflow.ErrInvalidState) {
    // Handle invalid state transition
}

// Wrap errors with context
err = fmt.Errorf("failed to execute workflow: %w", err)

// Extract underlying errors
var execErr *workflow.ExecutionError
if errors.As(err, &execErr) {
    fmt.Printf("Step %s failed: %v\n", execErr.StepID, execErr.Cause)
}
```

## Concurrency

All public APIs are safe for concurrent use:

```go
// Multiple goroutines can use the same registry
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        provider, _ := llm.Get("anthropic")
        provider.Complete(ctx, req)
    }()
}
wg.Wait()

// Same for workflow engine and tool registry
```

## Context Cancellation

Respect context cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// All operations respect context
resp, err := provider.Complete(ctx, req)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Request timed out")
}

result, err := engine.Execute(ctx, def, inputs)
if errors.Is(err, context.Canceled) {
    log.Println("Workflow canceled")
}
```

---

## Next Steps

- [Embedding Guide](../advanced/embedding.md) - Detailed guide for embedding Conductor
- [Workflow Schema Reference](workflow-schema.md) - YAML workflow reference
- [CLI Reference](cli.md) - Command-line interface
- [Examples](../examples/) - Example workflows and integrations
- [GoDoc](https://pkg.go.dev/github.com/tombee/conductor) - Complete API documentation
