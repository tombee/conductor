# LLM Package

The `llm` package provides a provider-agnostic abstraction layer for Large Language Model (LLM) interactions. It is designed to be embeddable in other Go applications and supports multiple LLM providers with features like cost tracking, retry logic, and automatic failover.

## Overview

The package consists of several key components:

- **Provider Interface**: A unified interface that all LLM providers implement
- **Provider Registry**: A registry for managing and discovering LLM providers
- **Cost Tracking**: Per-request cost tracking with aggregation capabilities
- **Retry Logic**: Exponential backoff retry for transient failures
- **Failover**: Automatic failover between providers with circuit breaker pattern
- **Model Management**: Model tier abstraction (fast/balanced/strategic)

## Quick Start

### Basic Usage

```go
import (
    "context"
    "github.com/tombee/conductor/pkg/llm"
    "github.com/tombee/conductor/pkg/llm/providers"
)

func main() {
    // Create and register a provider
    provider, err := providers.NewAnthropicProvider("your-api-key")
    if err != nil {
        panic(err)
    }

    llm.Register(provider)
    llm.SetDefault("anthropic")

    // Make a completion request
    ctx := context.Background()
    resp, err := provider.Complete(ctx, llm.CompletionRequest{
        Model: "balanced", // Use model tier
        Messages: []llm.Message{
            {Role: llm.MessageRoleUser, Content: "Hello, Claude!"},
        },
    })

    if err != nil {
        panic(err)
    }

    println(resp.Content)
}
```

### Streaming Responses

```go
// Start a streaming request
chunks, err := provider.Stream(ctx, llm.CompletionRequest{
    Model: "balanced",
    Messages: []llm.Message{
        {Role: llm.MessageRoleUser, Content: "Tell me a story"},
    },
})

if err != nil {
    panic(err)
}

// Consume the stream
for chunk := range chunks {
    if chunk.Error != nil {
        log.Printf("Error: %v", chunk.Error)
        break
    }

    print(chunk.Delta.Content)

    if chunk.FinishReason != "" {
        println("\nDone:", chunk.FinishReason)
        if chunk.Usage != nil {
            fmt.Printf("Tokens used: %d\n", chunk.Usage.TotalTokens)
        }
    }
}
```

## Model Tiers

The package supports three model tiers that map to provider-specific models:

- **Fast** (`llm.ModelTierFast`): Quick, cost-effective models for simple tasks
- **Balanced** (`llm.ModelTierBalanced`): General-purpose models with good performance
- **Strategic** (`llm.ModelTierStrategic`): Most capable models for complex reasoning

Example mapping for Anthropic:
- Fast → Claude 3.5 Haiku
- Balanced → Claude 3.5 Sonnet
- Strategic → Claude 3 Opus

## Provider Registry

The registry manages multiple providers and supports default selection:

```go
registry := llm.NewRegistry()

// Register providers
anthropic, _ := providers.NewAnthropicProvider("key1")
openai, _ := providers.NewOpenAIProvider("key2")

registry.Register(anthropic)
registry.Register(openai)

// Set default provider
registry.SetDefault("anthropic")

// Get provider by name
provider, _ := registry.Get("anthropic")

// Get default provider
defaultProvider, _ := registry.GetDefault()

// List all providers
providerNames := registry.List()
```

## Cost Tracking

Track LLM usage costs per request:

```go
// Create cost tracker
tracker := llm.NewCostTracker()

// After a completion request
cost := modelInfo.CalculateCost(resp.Usage)
tracker.Track(llm.CostRecord{
    RequestID: resp.RequestID,
    Provider:  "anthropic",
    Model:     resp.Model,
    Timestamp: time.Now(),
    Usage:     resp.Usage,
    Cost:      cost,
    Metadata: map[string]string{
        "correlation_id": "my-request-123",
    },
})

// Query cost records
records := tracker.GetRecordsByProvider("anthropic")
records = tracker.GetRecordsByModel("claude-3-5-sonnet-20241022")
records = tracker.GetRecordsByTimeRange(startTime, endTime)

// Get aggregated statistics
byProvider := tracker.AggregateByProvider()
byModel := tracker.AggregateByModel()
timePeriod := tracker.AggregateByTimePeriod(startTime, endTime)

fmt.Printf("Total cost: $%.4f\n", byProvider["anthropic"].TotalCost)
fmt.Printf("Total requests: %d\n", byProvider["anthropic"].TotalRequests)
fmt.Printf("Total tokens: %d\n", byProvider["anthropic"].TotalTokens)
```

## Retry Logic

Wrap providers with automatic retry on transient failures:

```go
// Create provider with retry
config := llm.DefaultRetryConfig()
config.MaxRetries = 3
config.InitialDelay = 100 * time.Millisecond

retryProvider := llm.NewRetryableProvider(provider, config)

// Requests will automatically retry on:
// - HTTP 5xx errors
// - HTTP 429 (rate limiting)
// - Network timeouts
// - Temporary network errors
```

### Custom Retry Logic

```go
config := llm.RetryConfig{
    MaxRetries:   5,
    InitialDelay: 50 * time.Millisecond,
    MaxDelay:     10 * time.Second,
    Multiplier:   2.0,
    Jitter:       0.1, // Add 10% randomness to prevent thundering herd
    RetryableErrors: func(err error) bool {
        // Custom logic to determine if error should trigger retry
        return shouldRetry(err)
    },
}

retryProvider := llm.NewRetryableProvider(provider, config)
```

## Provider Failover

Automatically failover between providers with circuit breaker:

```go
// Setup multiple providers
registry := llm.NewRegistry()
registry.Register(anthropic)
registry.Register(openai)
registry.Register(ollama)

// Configure failover
failoverConfig := llm.FailoverConfig{
    ProviderOrder: []string{"anthropic", "openai", "ollama"},
    CircuitBreakerThreshold: 5, // Open circuit after 5 consecutive failures
    CircuitBreakerTimeout: 30 * time.Second,
    OnFailover: func(from, to string, err error) {
        log.Printf("Failing over from %s to %s: %v", from, to, err)
    },
}

failoverProvider, _ := llm.NewFailoverProvider(registry, failoverConfig)

// Requests automatically failover on:
// - HTTP 5xx errors
// - HTTP 429 (rate limiting)
// - Request timeouts
// - Circuit breaker open

// Make requests through failover provider
resp, err := failoverProvider.Complete(ctx, req)
```

### Circuit Breaker

The circuit breaker prevents cascading failures:

```go
// Check circuit breaker status
status := failoverProvider.GetCircuitBreakerStatus()

for providerName, state := range status {
    fmt.Printf("Provider: %s\n", providerName)
    fmt.Printf("  Circuit Open: %v\n", state.Open)
    fmt.Printf("  Consecutive Failures: %d\n", state.ConsecutiveFailures)
    fmt.Printf("  Last Failure: %v\n", state.LastFailureTime)
}
```

## HTTP Connection Pooling

Providers use connection pooling for performance:

```go
// Configure connection pool
poolConfig := providers.ConnectionPoolConfig{
    MaxIdleConns:          10,
    MaxIdleConnsPerHost:   10,
    IdleConnTimeout:       30 * time.Second,
    ResponseHeaderTimeout: 10 * time.Second,
}

provider, _ := providers.NewAnthropicProviderWithPool("api-key", poolConfig)

// Get pool metrics
metrics := provider.GetPoolMetrics()
fmt.Printf("Total requests: %d\n", metrics.GetTotalRequests())
fmt.Printf("Failed requests: %d\n", metrics.GetFailedRequests())
```

## Tool/Function Calling

Support for LLM tool use:

```go
// Define a tool
tools := []llm.Tool{
    {
        Name:        "get_weather",
        Description: "Get current weather for a location",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{
                    "type":        "string",
                    "description": "City name",
                },
            },
            "required": []string{"location"},
        },
    },
}

// Make request with tools
resp, _ := provider.Complete(ctx, llm.CompletionRequest{
    Model:    "balanced",
    Messages: []llm.Message{
        {Role: llm.MessageRoleUser, Content: "What's the weather in SF?"},
    },
    Tools: tools,
})

// Handle tool calls
for _, toolCall := range resp.ToolCalls {
    fmt.Printf("Tool: %s\n", toolCall.Name)
    fmt.Printf("Arguments: %s\n", toolCall.Arguments)

    // Execute tool and send result back
    result := executeToolCall(toolCall)

    // Continue conversation with tool result
    nextResp, _ := provider.Complete(ctx, llm.CompletionRequest{
        Model: "balanced",
        Messages: []llm.Message{
            // ... previous messages ...
            {
                Role:       llm.MessageRoleTool,
                ToolCallID: toolCall.ID,
                Name:       toolCall.Name,
                Content:    result,
            },
        },
    })
}
```

## Error Handling

The package defines several error types:

```go
// Provider errors
llm.ErrProviderNotFound        // Provider not registered
llm.ErrProviderAlreadyRegistered  // Duplicate registration
llm.ErrNoDefaultProvider       // No default set
llm.ErrInvalidProvider         // Invalid provider

// Retry errors
llm.ErrMaxRetriesExceeded      // All retries exhausted

// Failover errors
llm.ErrAllProvidersFailed      // All providers in chain failed
llm.ErrCircuitOpen             // Circuit breaker is open

// HTTP errors
httpErr := &llm.HTTPError{
    StatusCode: 503,
    Message:    "Service unavailable",
}
```

## Advanced Configuration

### Request Metadata

Add correlation IDs and metadata for tracking:

```go
resp, _ := provider.Complete(ctx, llm.CompletionRequest{
    Model: "balanced",
    Messages: []llm.Message{
        {Role: llm.MessageRoleUser, Content: "Hello"},
    },
    Metadata: map[string]string{
        "correlation_id": "req-12345",
        "user_id":        "user-789",
        "session_id":     "session-abc",
    },
})
```

### Temperature and Max Tokens

Control generation parameters:

```go
temperature := 0.7
maxTokens := 1000

resp, _ := provider.Complete(ctx, llm.CompletionRequest{
    Model:       "balanced",
    Messages:    messages,
    Temperature: &temperature,
    MaxTokens:   &maxTokens,
})
```

### Stop Sequences

Specify stop sequences for generation:

```go
resp, _ := provider.Complete(ctx, llm.CompletionRequest{
    Model:    "balanced",
    Messages: messages,
    StopSequences: []string{"\n\n", "END"},
})
```

## Testing

The package includes mock providers for testing:

```go
import "github.com/tombee/conductor/pkg/llm/providers"

func TestMyFunction(t *testing.T) {
    // Create mock provider
    mock := providers.NewMockAnthropicProvider()

    // Customize mock behavior
    mock.MockComplete = func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
        return &llm.CompletionResponse{
            Content:      "Custom mock response",
            FinishReason: llm.FinishReasonStop,
            RequestID:    "test-123",
        }, nil
    }

    // Use mock in tests
    resp, _ := mock.Complete(ctx, llm.CompletionRequest{
        Model: "balanced",
        Messages: []llm.Message{
            {Role: llm.MessageRoleUser, Content: "test"},
        },
    })

    if resp.Content != "Custom mock response" {
        t.Errorf("unexpected response: %s", resp.Content)
    }
}
```

## Performance Considerations

1. **Connection Pooling**: Reuses HTTP connections for better performance
2. **Context Timeouts**: Always pass a context with timeout to avoid hanging
3. **Stream Processing**: Process stream chunks incrementally to reduce latency
4. **Model Selection**: Use appropriate model tier for your use case
5. **Retry Configuration**: Tune retry delays and max attempts based on your needs

## Supported Providers

### Phase 1 (Current)

- **Anthropic**: Full implementation with all Claude models
- **OpenAI**: Interface placeholder (Phase 2)
- **Ollama**: Interface placeholder (Phase 2)

### Adding Custom Providers

Implement the `Provider` interface:

```go
type CustomProvider struct {
    // Your provider state
}

func (p *CustomProvider) Name() string {
    return "custom"
}

func (p *CustomProvider) Capabilities() llm.Capabilities {
    return llm.Capabilities{
        Streaming: true,
        Tools:     true,
        Models: []llm.ModelInfo{
            // Your models
        },
    }
}

func (p *CustomProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
    // Your implementation
}

func (p *CustomProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
    // Your streaming implementation
}

// Register your provider
llm.Register(&CustomProvider{})
```

## API Reference

For detailed API documentation, see the inline GoDoc comments or run:

```bash
go doc github.com/tombee/conductor/pkg/llm
```

## License

Apache 2.0 - See LICENSE file for details.
