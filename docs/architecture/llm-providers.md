# LLM Provider System

## Overview

Conductor's LLM provider system offers a unified interface for interacting with multiple Large Language Model providers. The architecture prioritizes **provider portability**, **failover resilience**, and **cost optimization** while maintaining a simple developer experience.

## Architecture Principles

| Principle | Implementation | Benefit |
|-----------|----------------|---------|
| **Provider Agnostic** | Unified `Provider` interface | Switch providers without changing workflows |
| **Model Tiers** | Abstract tier system (fast/balanced/strategic) | Provider-independent workflow definitions |
| **Failover Support** | Automatic provider switching on error | High availability and resilience |
| **Cost Tracking** | Token usage monitoring per request | Budget management and optimization |
| **Streaming First** | Native streaming support | Better UX for interactive workflows |
| **Embeddable** | No external dependencies | Use in any Go application |

## Provider Interface

All LLM providers implement the same interface:

```go
type Provider interface {
    Name() string
    Capabilities() Capabilities
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}
```

### Core Methods

**Name()** - Returns unique provider identifier

```go
func (p *AnthropicProvider) Name() string {
    return "anthropic"
}
```

**Capabilities()** - Declares provider features

```go
type Capabilities struct {
    Streaming bool
    Tools     bool
    Models    []ModelInfo
}
```

**Complete()** - Synchronous completion request

Blocks until the LLM response is complete. Use for:

- Non-interactive workflows
- Batch processing
- When full response needed upfront

**Stream()** - Streaming completion request

Returns channel of incremental chunks. Use for:

- Interactive workflows
- Real-time feedback
- Long-running generations

### Optional Interfaces

Providers can implement additional interfaces for enhanced functionality:

**Detectable** - Auto-detection support

```go
type Detectable interface {
    Detect() (bool, error)
}
```

Enables automatic provider discovery (e.g., Claude Code detection).

**HealthCheckable** - Health verification

```go
type HealthCheckable interface {
    HealthCheck(ctx context.Context) HealthCheckResult
}
```

Supports three-step health validation:

1. **Installed** - Provider available
2. **Authenticated** - Credentials valid
3. **Working** - Can make requests

**UsageTrackable** - Post-request usage tracking

```go
type UsageTrackable interface {
    GetLastUsage() *TokenUsage
}
```

For providers that don't include usage in primary response.

## Supported Providers

### Anthropic (Claude)

**Models:** Claude Opus, Sonnet, Haiku families

**Features:**

- Streaming support ✓
- Tool calling ✓
- Prompt caching ✓
- Vision support ✓

**Configuration:**

```yaml
providers:
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
    default_model: claude-sonnet-4-5
```

**Model Tiers:**

- Fast: `claude-3-haiku-20240307`
- Balanced: `claude-sonnet-4-5`
- Strategic: `claude-opus-4-5`

### OpenAI (GPT)

**Models:** GPT, GPT Turbo

**Features:**

- Streaming support ✓
- Tool calling ✓
- Vision support ✓
- Fine-tuned models ✓

**Configuration:**

```yaml
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
    organization: ${OPENAI_ORG_ID}  # Optional
    default_model: gpt-4-turbo
```

**Model Tiers:**

- Fast: `gpt-3.5-turbo`
- Balanced: `gpt-4-turbo`
- Strategic: `gpt-4`

### Google (Gemini)

**Models:** Gemini Pro, Gemini Ultra

**Features:**

- Streaming support ✓
- Tool calling ✓
- Multimodal support ✓

**Configuration:**

```yaml
providers:
  google:
    api_key: ${GOOGLE_API_KEY}
    default_model: gemini-pro
```

**Model Tiers:**

- Fast: `gemini-pro`
- Balanced: `gemini-pro`
- Strategic: `gemini-ultra`

### Ollama (Local)

**Models:** Llama, Mistral, Mixtral, and custom models

**Features:**

- Streaming support ✓
- Tool calling ✓
- Fully local ✓
- No API costs ✓

**Configuration:**

```yaml
providers:
  ollama:
    base_url: http://localhost:11434
    default_model: llama3
```

**Model Tiers:**

- Fast: `llama3:8b`
- Balanced: `llama3:70b`
- Strategic: `mixtral:8x7b`

### Claude Code (Auto-detected)

**Special provider** that auto-detects when running inside Claude Code environment.

**Features:**

- Zero configuration ✓
- Automatic detection ✓
- Streaming support ✓
- Tool calling ✓

**Detection:**

```go
// Automatically registered if running in Claude Code
provider := &claudecode.Provider{}
if detected, _ := provider.Detect(); detected {
    registry.Register(provider)
}
```

## Model Tier System

Model tiers provide **provider-independent** workflow definitions. Instead of hardcoding specific models, workflows specify tiers based on task requirements.

### Tier Definitions

| Tier | Use Case | Characteristics | Example Tasks |
|------|----------|----------------|---------------|
| **fast** | Simple, high-volume tasks | Low latency, low cost, basic reasoning | Classification, extraction, formatting |
| **balanced** | General-purpose tasks | Good reasoning, moderate cost, versatile | Code review, summarization, analysis |
| **strategic** | Complex reasoning tasks | Best reasoning, highest cost, deep thinking | Architecture design, complex debugging, research |

### Tier Mapping

Each provider maps tiers to their models:

```go
type ModelInfo struct {
    ID           string
    Name         string
    Tier         ModelTier
    MaxTokens    int
    Pricing      ModelPricing
    Capabilities []string
}

// Example: Anthropic tier mapping
Models: []ModelInfo{
    {
        ID:        "claude-3-haiku-20240307",
        Tier:      TierFast,
        MaxTokens: 200000,
    },
    {
        ID:        "claude-sonnet-4-5",
        Tier:      TierBalanced,
        MaxTokens: 200000,
    },
    {
        ID:        "claude-opus-4-5",
        Tier:      TierStrategic,
        MaxTokens: 200000,
    },
}
```

### Using Tiers in Workflows

```yaml
steps:
  - id: classify
    type: llm
    model: fast  # Will use appropriate fast model for configured provider
    prompt: Classify this issue as bug, feature, or question

  - id: analyze
    type: llm
    model: balanced  # Provider-specific balanced model
    prompt: Analyze the code for potential improvements

  - id: architect
    type: llm
    model: strategic  # Best model for complex reasoning
    prompt: Design a scalable architecture for this system
```

### Tier Overrides

Override specific models for fine-grained control:

```yaml
providers:
  anthropic:
    tier_overrides:
      fast: claude-3-haiku-20240307
      balanced: claude-sonnet-3-5-20240229  # Use older Sonnet
      strategic: claude-opus-4-5
```

## Provider Registry

The registry manages provider lifecycle and selection.

### Registration

```go
import (
    "github.com/tombee/conductor/pkg/llm"
    "github.com/tombee/conductor/pkg/llm/providers"
)

// Create registry
registry := llm.NewRegistry()

// Register providers
anthropic := providers.NewAnthropic(apiKey)
registry.Register(anthropic)

openai := providers.NewOpenAI(apiKey)
registry.Register(openai)

// Set default provider
registry.SetDefault("anthropic")
```

### Provider Selection

Workflows can specify which provider to use:

```yaml
# Use default provider
steps:
  - id: step1
    type: llm
    model: balanced
    prompt: Analyze this code

# Specify provider explicitly
steps:
  - id: step2
    type: llm
    provider: openai
    model: balanced
    prompt: Generate documentation
```

### Global Registry

Conductor provides a global registry for simple use cases:

```go
import "github.com/tombee/conductor/pkg/llm"

// Register with global registry
llm.Register(provider)

// Set default
llm.SetDefault("anthropic")

// Get provider
provider, err := llm.Get("openai")

// Get default provider
provider, err := llm.GetDefault()
```

## Failover and Resilience

### Automatic Failover

Configure provider failover order for high availability:

```yaml
providers:
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}

  openai:
    api_key: ${OPENAI_API_KEY}

failover:
  enabled: true
  order:
    - anthropic
    - openai
```

```go
// Set failover programmatically
registry.SetFailoverOrder([]string{"anthropic", "openai", "ollama"})
```

When primary provider fails, Conductor automatically tries the next provider in order.

### Retry Logic

Built-in retry with exponential backoff:

```go
type RetryConfig struct {
    MaxAttempts     int
    InitialDelay    time.Duration
    MaxDelay        time.Duration
    BackoffMultiplier float64
}

// Configure retries
provider = llm.WithRetry(provider, llm.RetryConfig{
    MaxAttempts:     3,
    InitialDelay:    1 * time.Second,
    MaxDelay:        10 * time.Second,
    BackoffMultiplier: 2.0,
})
```

**Retry Triggers:**

- Rate limit errors (429)
- Server errors (500, 502, 503, 504)
- Timeout errors
- Network errors

**Non-Retriable Errors:**

- Authentication errors (401, 403)
- Invalid request errors (400)
- Content policy violations

### Circuit Breaker

Prevent cascading failures with circuit breaker pattern:

```yaml
resilience:
  circuit_breaker:
    enabled: true
    failure_threshold: 5  # Open after 5 failures
    success_threshold: 2  # Close after 2 successes
    timeout: 60s          # Stay open for 60s
```

## Cost Tracking

### Token Usage

Every request tracks token consumption:

```go
type TokenUsage struct {
    PromptTokens         int  // Input tokens
    CompletionTokens     int  // Output tokens
    TotalTokens          int  // Sum of above
    CacheCreationTokens  int  // Tokens written to cache
    CacheReadTokens      int  // Tokens read from cache
}
```

### Cost Calculation

Providers include pricing information:

```go
type ModelPricing struct {
    InputCostPerMToken  float64  // Cost per million input tokens
    OutputCostPerMToken float64  // Cost per million output tokens
    CacheCostPerMToken  float64  // Cost per million cached tokens
}

// Calculate cost
cost := pricing.InputCostPerMToken * (float64(usage.PromptTokens) / 1_000_000)
cost += pricing.OutputCostPerMToken * (float64(usage.CompletionTokens) / 1_000_000)
```

### Cost Tracking

```go
import "github.com/tombee/conductor/pkg/llm/cost"

// Create cost tracker
tracker := cost.NewMemoryStore()

// Record usage
tracker.Record(ctx, cost.Entry{
    Provider:         "anthropic",
    Model:            "claude-sonnet-4-5",
    PromptTokens:     1500,
    CompletionTokens: 300,
    TotalTokens:      1800,
    Cost:             0.0045,
    Timestamp:        time.Now(),
})

// Get total cost
total, err := tracker.GetTotalCost(ctx)

// Get cost by provider
costs, err := tracker.GetCostByProvider(ctx)
```

### Cost Budgets

Set spending limits:

```yaml
cost:
  budget:
    daily_limit: 10.00    # USD
    monthly_limit: 200.00
  alerts:
    threshold: 0.8  # Alert at 80% of budget
```

## Streaming

### Streaming Architecture

Streaming responses return chunks as they're generated:

```go
chunks, err := provider.Stream(ctx, req)
if err != nil {
    return err
}

var fullResponse strings.Builder

for chunk := range chunks {
    if chunk.Error != nil {
        return chunk.Error
    }

    // Process incremental content
    fmt.Print(chunk.Delta.Content)
    fullResponse.WriteString(chunk.Delta.Content)

    // Final chunk contains usage
    if chunk.Usage != nil {
        fmt.Printf("\nTokens used: %d\n", chunk.Usage.TotalTokens)
    }
}
```

### Tool Calls in Streaming

Tool calls may arrive over multiple chunks:

```go
toolCalls := make(map[int]*ToolCallBuilder)

for chunk := range chunks {
    if delta := chunk.Delta.ToolCallDelta; delta != nil {
        // Build up tool call incrementally
        builder := toolCalls[delta.Index]
        if builder == nil {
            builder = &ToolCallBuilder{}
            toolCalls[delta.Index] = builder
        }

        builder.Add(delta)
    }
}

// Extract completed tool calls
for _, builder := range toolCalls {
    toolCall := builder.Build()
    // Execute tool...
}
```

## Health Checks

### Provider Health

Verify provider status:

```go
if healthCheckable, ok := provider.(llm.HealthCheckable); ok {
    result := healthCheckable.HealthCheck(ctx)

    if !result.Healthy() {
        fmt.Printf("Provider unhealthy: %s\n", result.Message)
        fmt.Printf("Failed at step: %s\n", result.ErrorStep)
    }
}
```

### Health Check Steps

1. **Installed** - Provider available/reachable
2. **Authenticated** - Credentials valid
3. **Working** - Can make successful requests

```go
type HealthCheckResult struct {
    Installed     bool
    Authenticated bool
    Working       bool
    Error         error
    ErrorStep     HealthCheckStep
    Message       string
    Version       string
}
```

### CLI Health Check

```bash
$ conductor health

Provider Health:
  anthropic:
    ✓ Installed
    ✓ Authenticated
    ✓ Working
    Version: API v1

  openai:
    ✓ Installed
    ✗ Authenticated (API key invalid)
    - Working (not checked)
    Error: authentication failed

  ollama:
    ✗ Installed (server not reachable)
    - Authenticated (not checked)
    - Working (not checked)
    Error: connection refused at http://localhost:11434
```

## Creating Custom Providers

### Implement Provider Interface

```go
package myprovider

import (
    "context"
    "github.com/tombee/conductor/pkg/llm"
)

type CustomProvider struct {
    apiKey    string
    baseURL   string
    client    *http.Client
}

func NewCustomProvider(apiKey string) *CustomProvider {
    return &CustomProvider{
        apiKey:  apiKey,
        baseURL: "https://api.example.com",
        client:  &http.Client{Timeout: 30 * time.Second},
    }
}

func (p *CustomProvider) Name() string {
    return "custom"
}

func (p *CustomProvider) Capabilities() llm.Capabilities {
    return llm.Capabilities{
        Streaming: true,
        Tools:     false,
        Models: []llm.ModelInfo{
            {
                ID:        "custom-fast",
                Tier:      llm.TierFast,
                MaxTokens: 4096,
            },
        },
    }
}

func (p *CustomProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
    // Build API request
    apiReq := p.buildRequest(req)

    // Make HTTP request
    resp, err := p.client.Do(apiReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // Parse response
    apiResp, err := p.parseResponse(resp)
    if err != nil {
        return nil, err
    }

    // Convert to Conductor format
    return &llm.CompletionResponse{
        Content:      apiResp.Text,
        FinishReason: llm.FinishReasonStop,
        Usage: llm.TokenUsage{
            PromptTokens:     apiResp.InputTokens,
            CompletionTokens: apiResp.OutputTokens,
            TotalTokens:      apiResp.InputTokens + apiResp.OutputTokens,
        },
        Model:     req.Model,
        RequestID: apiResp.ID,
        Created:   time.Now(),
    }, nil
}

func (p *CustomProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
    chunks := make(chan llm.StreamChunk)

    go func() {
        defer close(chunks)

        // Implement streaming logic
        // Send chunks as they arrive
        chunks <- llm.StreamChunk{
            Delta: llm.StreamDelta{
                Content: "partial response",
            },
        }

        // Send final chunk with usage
        chunks <- llm.StreamChunk{
            FinishReason: llm.FinishReasonStop,
            Usage: &llm.TokenUsage{
                TotalTokens: 100,
            },
        }
    }()

    return chunks, nil
}
```

### Optional: Health Check Support

```go
func (p *CustomProvider) HealthCheck(ctx context.Context) llm.HealthCheckResult {
    result := llm.HealthCheckResult{
        Installed: true,
    }

    // Check authentication
    if err := p.validateAuth(ctx); err != nil {
        result.Error = err
        result.ErrorStep = llm.HealthCheckStepAuthenticated
        result.Message = "API key is invalid or expired"
        return result
    }
    result.Authenticated = true

    // Check if provider is working
    if err := p.testRequest(ctx); err != nil {
        result.Error = err
        result.ErrorStep = llm.HealthCheckStepWorking
        result.Message = "Provider is not responding to requests"
        return result
    }
    result.Working = true

    return result
}
```

### Register Custom Provider

```go
import "github.com/tombee/conductor/pkg/llm"

func main() {
    provider := myprovider.NewCustomProvider(apiKey)

    // Register with global registry
    if err := llm.Register(provider); err != nil {
        log.Fatal(err)
    }

    llm.SetDefault("custom")
}
```

## Best Practices

### Provider Selection

1. **Use tiers, not models** - Define workflows with tiers for portability
2. **Set up failover** - Configure backup providers for resilience
3. **Monitor costs** - Track spending and set budgets
4. **Health check regularly** - Verify provider status before critical operations

### Performance Optimization

1. **Use streaming for interactive UX** - Better perceived latency
2. **Enable prompt caching** - Reduce costs for repeated prompts (Anthropic)
3. **Choose appropriate tiers** - Don't use strategic tier for simple tasks
4. **Implement request batching** - Combine similar requests when possible

### Error Handling

1. **Always check context cancellation** - Respect timeouts
2. **Log provider errors** - Track failure patterns
3. **Implement graceful degradation** - Fall back to simpler models
4. **Surface actionable errors** - Help users fix authentication/config issues

### Security

1. **Store credentials securely** - Use environment variables or secret managers
2. **Rotate API keys regularly** - Limit exposure window
3. **Audit provider usage** - Track which workflows use which providers
4. **Validate responses** - Don't trust LLM outputs without validation

## See Also

- [Execution Flow](execution-flow.md) - How workflows execute LLM steps
- [Embedding Guide](../extending/embedding.md) - Using providers in Go applications
- [Configuration Reference](../reference/configuration.md) - Provider configuration options
