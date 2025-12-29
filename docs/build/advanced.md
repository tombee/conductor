---
title: "Advanced Usage"
---

Deep integration patterns for complex use cases.

## Architecture Overview

ConductorSDK wraps these internal packages:

- **pkg/llm** — Provider abstraction for LLM interactions
- **pkg/workflow** — Workflow orchestration and execution
- **pkg/agent** — ReAct-style agent loops
- **pkg/tools** — Tool registry and execution

## Multi-Provider Configuration

Register and select between multiple providers:

```go
s, err := sdk.New(
    sdk.WithAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY")),
    sdk.WithOpenAIProvider(os.Getenv("OPENAI_API_KEY")),
)
```

Use model tiers for automatic model selection:

| Tier | Anthropic | OpenAI |
|------|-----------|--------|
| `fast` | Claude 3.5 Haiku | GPT-4o mini |
| `balanced` | Claude 3.5 Sonnet | GPT-4o |
| `powerful` | Claude 3 Opus | GPT-4 |

## Agent Loop Integration

Run autonomous agents with tool access:

```go
s, err := sdk.New(
    sdk.WithAnthropicProvider(apiKey),
    sdk.WithBuiltinActions(), // file, shell, http tools
)

// Register custom tools
tool := sdk.FuncTool("search_docs", "Search documentation", schema, fn)
s.RegisterTool(tool)

// Run agent
result, err := s.RunAgent(ctx,
    "You are a helpful coding assistant.",
    "Find and fix the bug in main.go",
)

fmt.Printf("Agent completed in %d iterations\n", result.Iterations)
fmt.Println(result.FinalResponse)
```

## Testing with Mock Providers

Test workflows without API calls:

```go
func TestWorkflow(t *testing.T) {
    mock := &MockProvider{responses: []string{"Hello, World!"}}
    s, _ := sdk.New(sdk.WithProvider("mock", mock))
    defer s.Close()

    wf, _ := s.NewWorkflow("test").
        Step("greet").LLM().Prompt("Say hello").Done().
        Build()

    result, err := s.Run(context.Background(), wf, nil)
    assert.NoError(t, err)
    assert.Equal(t, "Hello, World!", result.Steps["greet"].Output)
}
```

## Resource Cleanup

Always close the SDK to release resources:

```go
s, err := sdk.New(...)
if err != nil {
    return err
}
defer s.Close() // Disconnects MCP servers, zeros credentials
```

The `Close()` method:
- Disconnects all MCP server connections
- Zeros API keys from memory (security)
- Is safe to call multiple times

## API Reference

Full API documentation: [pkg.go.dev/github.com/tombee/conductor/sdk](https://pkg.go.dev/github.com/tombee/conductor/sdk)

Key types:
- `SDK` — Main entry point
- `Workflow` — Workflow definition
- `WorkflowBuilder` — Fluent workflow construction
- `Result` — Execution result with cost breakdown
- `Event` — Execution events for streaming
