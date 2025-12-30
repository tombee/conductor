# Quick Start

Get ConductorSDK running in 5 minutes.

## Prerequisites

- Go 1.22+
- One of these LLM options:
  - **Claude Code CLI** (recommended) — Install from [claude.ai/code](https://claude.ai/code)
  - **Anthropic API key** — Get from [console.anthropic.com](https://console.anthropic.com)
  - **OpenAI API key** — Get from [platform.openai.com](https://platform.openai.com)
  - **Ollama** — Runs locally, no key needed

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
    "github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

func main() {
    s, err := newSDK()
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

// newSDK creates an SDK with the best available provider.
func newSDK() (*sdk.SDK, error) {
    // Try Claude Code CLI first (zero-config if installed)
    cc := claudecode.New()
    if found, _ := cc.Detect(); found {
        return sdk.New(sdk.WithProvider("claude-code", cc))
    }

    // Fall back to API keys
    if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
        return sdk.New(sdk.WithAnthropicProvider(key))
    }
    if key := os.Getenv("OPENAI_API_KEY"); key != "" {
        return sdk.New(sdk.WithOpenAIProvider(key))
    }

    return nil, fmt.Errorf("no provider: install Claude Code CLI or set ANTHROPIC_API_KEY")
}
```

## Run

```bash
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

- **[Chat with File Access](./tutorials/chat-application/)** — Build a chat assistant with built-in actions
- **[Code Review Bot Tutorial](./tutorials/code-review-bot/)** — Multi-step workflow with structured output
- **[Recipes](./recipes/)** — Common patterns and solutions
