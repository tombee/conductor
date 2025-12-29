---
title: "Chat Application"
---

Build a streaming chat interface that displays tokens as they arrive.

## What You'll Build

A terminal-based chat application that:
- Streams LLM responses token-by-token
- Maintains conversation history
- Shows cost per message

## Setup

```go
package main

import (
    "bufio"
    "context"
    "fmt"
    "os"
    "strings"

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

    // Stream tokens to stdout
    s.OnEvent(sdk.EventToken, func(ctx context.Context, e *sdk.Event) {
        fmt.Print(e.Token)
    })

    chat(s)
}
```

## Building the Chat Loop

```go
func chat(s *sdk.SDK) {
    var history []sdk.Message
    reader := bufio.NewReader(os.Stdin)

    fmt.Println("Chat started. Type 'quit' to exit.")

    for {
        fmt.Print("\nYou: ")
        input, _ := reader.ReadString('\n')
        input = strings.TrimSpace(input)

        if input == "quit" {
            break
        }

        // Add user message to history
        history = append(history, sdk.Message{
            Role:    "user",
            Content: input,
        })

        // Build workflow with conversation history
        wf, _ := s.NewWorkflow("chat").
            Step("respond").LLM().
                Model("fast").
                System("You are a helpful assistant.").
                Messages(history).
                Done().
            Build()

        fmt.Print("Assistant: ")
        result, err := s.Run(context.Background(), wf, nil)
        if err != nil {
            fmt.Printf("\nError: %v\n", err)
            continue
        }
        fmt.Println() // newline after streamed response

        // Add assistant response to history
        response := result.Steps["respond"].Output.(string)
        history = append(history, sdk.Message{
            Role:    "assistant",
            Content: response,
        })

        fmt.Printf("(Cost: $%.4f)\n", result.Cost.Total)
    }
}
```

## Run It

```bash
export ANTHROPIC_API_KEY=sk-ant-...
go run main.go
```

```
Chat started. Type 'quit' to exit.

You: What's the capital of France?
Assistant: The capital of France is Paris.
(Cost: $0.0008)

You: What's its population?
Assistant: Paris has a population of approximately 2.1 million in the city proper...
(Cost: $0.0012)
```

## Key Concepts

**Event Streaming**: The `OnEvent(sdk.EventToken, ...)` callback fires for each token, enabling real-time display.

**Conversation History**: The `Messages()` builder method accepts prior messages, giving the LLM context for follow-up questions.

**Cost Tracking**: Each `result.Cost.Total` shows the actual API cost for that exchange.

## Next Steps

- Add system prompts for different personas
- Implement `/clear` command to reset history
- Add cost budgeting with `sdk.WithCostLimit()`
