# Chat Application with File Access

Build a chat assistant that can read and write files on your behalf.

## What You'll Build

A terminal-based chat that:
- Uses built-in file and shell actions as tools
- Streams LLM responses token-by-token
- Lets the AI perform file operations when asked

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
    "github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

func main() {
    s, err := newSDK()
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

func newSDK() (*sdk.SDK, error) {
    cc := claudecode.New()
    if found, _ := cc.Detect(); found {
        return sdk.New(
            sdk.WithProvider("claude-code", cc),
            sdk.WithBuiltinActions(), // Enable file, shell tools
        )
    }
    if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
        return sdk.New(
            sdk.WithAnthropicProvider(key),
            sdk.WithBuiltinActions(),
        )
    }
    return nil, fmt.Errorf("no provider available")
}
```

## Building the Agent Chat Loop

```go
func chat(s *sdk.SDK) {
    reader := bufio.NewReader(os.Stdin)
    ctx := context.Background()

    fmt.Println("File Assistant ready. Try: 'List files in the current directory'")
    fmt.Println("Type 'quit' to exit.\n")

    for {
        fmt.Print("You: ")
        input, _ := reader.ReadString('\n')
        input = strings.TrimSpace(input)

        if input == "quit" {
            break
        }

        fmt.Print("Assistant: ")

        // Run as an agent - the LLM can use file/shell tools
        result, err := s.RunAgent(ctx,
            `You are a helpful file assistant. You can:
            - List files using shell.run
            - Read file contents using file.read
            - Create files using file.write
            Be concise in responses.`,
            input,
        )

        if err != nil {
            fmt.Printf("\nError: %v\n", err)
            continue
        }

        fmt.Println() // newline after streamed response
        fmt.Printf("(Iterations: %d, Cost: $%.4f)\n\n",
            result.Iterations, result.Cost.Total)
    }
}
```

## Run It

```bash
go run main.go
```

```
File Assistant ready. Try: 'List files in the current directory'
Type 'quit' to exit.

You: List all Go files in the current directory
Assistant: I'll list the Go files for you.

Found these Go files:
- main.go
- sdk_test.go
- config.go
(Iterations: 2, Cost: $0.0023)

You: What's in main.go?
Assistant: Here's the content of main.go:

[displays file contents]
(Iterations: 2, Cost: $0.0018)

You: Create a file called notes.txt with "Hello from the SDK"
Assistant: I've created notes.txt with your message.
(Iterations: 2, Cost: $0.0015)
```

## Key Concepts

**Built-in Actions**: `WithBuiltinActions()` enables file, shell, http, and transform tools that the agent can use.

**RunAgent**: Unlike `Run()`, `RunAgent()` loops until the LLM stops calling tools, enabling autonomous multi-step operations.

**Iterations**: The agent may call multiple tools before responding. `result.Iterations` shows how many tool calls were made.

## Next Steps

- Add `sdk.WithCostLimit()` to prevent runaway spending
- Add more tools with `sdk.FuncTool()` for custom operations
- Try with `sdk.WithBuiltinIntegrations()` for GitHub/Slack access
