---
title: "Quick Start"
---

Get ConductorSDK running in 5 minutes.

## Prerequisites

- Go 1.22+
- An LLM API key:
  - **Anthropic:** Get a key at [console.anthropic.com](https://console.anthropic.com)
  - **OpenAI:** Get a key at [platform.openai.com](https://platform.openai.com)
  - **Ollama:** No key needed (runs locally)

## Install

```bash
go get github.com/tombee/conductor/sdk
```

## First Workflow

Create `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/tombee/conductor/sdk"
)

func main() {
    s, err := sdk.New(
        sdk.WithAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY")),
    )
    if err != nil {
        panic(err)
    }
    defer s.Close()

    wf, _ := s.NewWorkflow("hello").
        Input("topic", sdk.TypeString).
        Step("explain").LLM().
            Model("fast").
            Prompt("Explain {{.inputs.topic}} in one sentence.").
            Done().
        Build()

    result, _ := s.Run(context.Background(), wf, map[string]any{
        "topic": "recursion",
    })

    fmt.Println(result.Steps["explain"].Output)
    fmt.Printf("Cost: $%.4f\n", result.Cost.Total)
}
```

## Run

```bash
export ANTHROPIC_API_KEY=sk-ant-...
go run main.go
```

Output:
```
Recursion is when a function calls itself to solve smaller instances of the same problem until reaching a base case.
Cost: $0.0012
```

## Add Event Streaming

Track progress in real-time:

```go
s.OnEvent(sdk.EventStepCompleted, func(ctx context.Context, e *sdk.Event) {
    fmt.Printf("[%s] completed in %v\n", e.StepID, e.Duration)
})
```

## Load YAML Workflows

Use existing platform workflows:

```go
wf, err := s.LoadWorkflowFile("./workflow.yaml")
if err != nil {
    panic(err)
}

result, err := s.Run(ctx, wf, inputs)
```

## Next Steps

- **[Chat Application Tutorial](./tutorials/chat-application/)** — Build a streaming chat interface
- **[Code Review Bot Tutorial](./tutorials/code-review-bot/)** — Multi-step workflow with GitHub
- **[Recipes](./recipes/)** — Common patterns and solutions
