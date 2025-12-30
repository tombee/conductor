# Recipes

Copy-paste patterns for common SDK tasks.

## Provider Auto-Detection

Use the best available provider automatically:

```go
import (
    "os"
    "github.com/tombee/conductor/sdk"
    "github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

func newSDK() (*sdk.SDK, error) {
    // Try Claude Code CLI first (zero-config if installed)
    cc := claudecode.New()
    if found, _ := cc.Detect(); found {
        return sdk.New(sdk.WithProvider("claude-code", cc))
    }

    // Fall back to API keys from environment
    if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
        return sdk.New(sdk.WithAnthropicProvider(key))
    }
    if key := os.Getenv("OPENAI_API_KEY"); key != "" {
        return sdk.New(sdk.WithOpenAIProvider(key))
    }

    return nil, fmt.Errorf("no provider: install Claude Code CLI or set API key")
}
```

## Custom Tool Registration

Register a function as a tool for LLM agents:

```go
tool := sdk.FuncTool(
    "get_weather",
    "Get current weather for a location",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "location": map[string]any{
                "type":        "string",
                "description": "City name",
            },
        },
        "required": []string{"location"},
    },
    func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
        location := inputs["location"].(string)
        // Fetch weather data...
        return map[string]any{
            "temperature": 72,
            "conditions":  "sunny",
        }, nil
    },
)

if err := s.RegisterTool(tool); err != nil {
    return err
}
```

## Token Usage Tracking

Track token consumption per step (requires API providers):

```go
// Token tracking requires API providers (Anthropic, OpenAI)
// Note: Claude Code CLI doesn't report token usage
s, err := sdk.New(
    sdk.WithAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY")),
)

result, err := s.Run(ctx, wf, inputs)
if err != nil {
    return err
}

fmt.Printf("Tokens: %d input, %d output, %d total\n",
    result.Cost.InputTokens,
    result.Cost.OutputTokens,
    result.Cost.TotalTokens)
```

## Error Handling

Handle errors with context:

```go
result, err := s.Run(ctx, wf, inputs)
if err != nil {
    var validErr *sdk.ValidationError
    var stepErr *sdk.StepExecutionError
    var costErr *sdk.CostLimitExceededError

    switch {
    case errors.As(err, &validErr):
        fmt.Printf("Invalid input %s: %s\n", validErr.Field, validErr.Message)
    case errors.As(err, &stepErr):
        fmt.Printf("Step %s failed: %s\n", stepErr.StepID, stepErr.Cause)
    case errors.As(err, &costErr):
        fmt.Printf("Cost limit: spent $%.2f\n", costErr.Spent)
    default:
        return fmt.Errorf("workflow failed: %w", err)
    }
}
```

## Embedding Workflows with go:embed

Compile YAML workflows into your binary:

```go
import (
    "context"
    _ "embed"
    "github.com/tombee/conductor/sdk"
)

//go:embed workflows/review.yaml
var reviewWorkflow []byte

func main() {
    s, _ := newSDK()
    defer s.Close()

    wf, _ := s.LoadWorkflow(reviewWorkflow)
    result, _ := s.Run(context.Background(), wf, inputs)
}
```

## Event Progress Tracking

Show real-time progress:

```go
s.OnEvent(sdk.EventStepStarted, func(ctx context.Context, e *sdk.Event) {
    fmt.Printf("[%s] starting...\n", e.StepID)
})

s.OnEvent(sdk.EventStepCompleted, func(ctx context.Context, e *sdk.Event) {
    fmt.Printf("[%s] done (%v, $%.4f)\n", e.StepID, e.Duration, e.Cost)
})

s.OnEvent(sdk.EventToken, func(ctx context.Context, e *sdk.Event) {
    fmt.Print(e.Token) // Stream tokens to stdout
})
```
