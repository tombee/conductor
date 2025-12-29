---
title: "Recipes"
---

Copy-paste patterns for common SDK tasks.

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

## Cost Budgeting

Limit spending per workflow run:

```go
s, err := sdk.New(
    sdk.WithAnthropicProvider(apiKey),
    sdk.WithCostLimit(5.0), // $5 max per run
)

result, err := s.Run(ctx, wf, inputs)
if err != nil {
    var costErr *sdk.CostLimitExceededError
    if errors.As(err, &costErr) {
        fmt.Printf("Budget exceeded: spent $%.2f of $%.2f limit\n",
            costErr.Spent, costErr.Limit)
    }
    return err
}

// Check actual cost
fmt.Printf("Total cost: $%.4f\n", result.Cost.Total)
for stepID, cost := range result.Cost.ByStep {
    fmt.Printf("  %s: $%.4f\n", stepID, cost)
}
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
    s, _ := sdk.New(sdk.WithAnthropicProvider(apiKey))
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
