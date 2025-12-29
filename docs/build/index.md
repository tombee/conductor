---
title: "Build with ConductorSDK"
---

ConductorSDK is an embeddable Go library that brings Conductor's workflow executor and multi-provider LLM abstraction directly into your application.

## When to Use ConductorSDK

| Scenario | Use |
|----------|-----|
| Desktop apps with AI features | **SDK** |
| CLI tools with LLM capabilities | **SDK** |
| Serverless functions | **SDK** |
| Unit tests with mock providers | **SDK** |
| Long-running automation server | **Platform** |
| Scheduled/webhook-triggered workflows | **Platform** |
| Team workflows with shared credentials | **Platform** |

## Quick Example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/tombee/conductor/sdk"
)

func main() {
    // Create SDK with an LLM provider
    // Get a key from console.anthropic.com (or use OpenAI, Ollama)
    s, err := sdk.New(
        sdk.WithAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY")),
        sdk.WithCostLimit(1.0), // $1 max
    )
    if err != nil {
        panic(err)
    }
    defer s.Close()

    // Define workflow
    wf, err := s.NewWorkflow("greet").
        Input("name", sdk.TypeString).
        Step("greet").LLM().
            Model("claude-sonnet-4-20250514").
            Prompt("Say hello to {{.inputs.name}}").
            Done().
        Build()
    if err != nil {
        panic(err)
    }

    // Run workflow
    result, err := s.Run(context.Background(), wf, map[string]any{
        "name": "World",
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(result.Steps["greet"].Output)
}
```

## Key Features

**Fluent Workflow Builder** — Type-safe, IDE-friendly workflow definition with compile-time validation.

**Multi-Provider LLM** — Anthropic, OpenAI, Google, and local models through a unified interface.

**Event Streaming** — React to step completions in real-time for progress updates and logging.

**Cost Tracking** — Per-step cost breakdown with configurable limits to prevent runaway spending.

**YAML Loading** — Load platform workflows and extend them programmatically.

**Custom Tools** — Register functions as tools for LLM steps to use.

**Agent Loops** — Run autonomous ReAct-style agents with `RunAgent()` for open-ended tasks.

## What You Lose vs Platform

The SDK is designed for embedded use cases. These platform features are **not available**:

- Webhook/schedule triggers (use your app's own triggers)
- Credential management (bring your own API keys)
- Run history persistence (use `WithStore()` for custom storage)
- Controller API access

## Get Started

1. **[Quick Start](./quickstart/)** — Install and run your first workflow in 5 minutes
2. **[Tutorials](./tutorials/)** — Build real applications step-by-step
3. **[Recipes](./recipes/)** — Copy-paste patterns for common tasks
4. **[API Reference](https://pkg.go.dev/github.com/tombee/conductor/sdk)** — Full package documentation
