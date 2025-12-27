# Embedding Conductor in Go Projects

:::note[Prerequisites]
This guide is for advanced users who want to embed Conductor as a Go library in their applications. If you're just getting started with Conductor, begin with the [Quick Start](../quick-start.md) guide.

**Required knowledge:**

- Go programming (intermediate level)
- Understanding of Go interfaces and dependency injection
- Familiarity with context-based cancellation
- Basic understanding of LLM concepts (tokens, prompts, completions)

**Before embedding:**

- Have worked with Conductor CLI to understand workflow concepts
- Read [Core Concepts](../learn/concepts/index.md) documentation
- Review [Architecture](../architecture/overview.md) to understand internal design
:::


Conductor is designed as an embeddable library that you can integrate into your Go applications. This guide shows you how to use Conductor packages to add AI-powered automation to your project.

:::note[When to Embed vs Use Daemon]
Most users should interact with Conductor through the CLI and daemon (`conductord`). Embedding is recommended only for:

- Serverless functions (where daemon overhead is prohibitive)
- Unit tests (where you need fine-grained control)
- Applications with custom provider implementations
- Scenarios where you need to bypass daemon features

**You lose these daemon features when embedding:**

- Automatic checkpointing and crash recovery
- Centralized provider credential management
- Consistent state management across restarts
- API access for community tools

See [Daemon Mode](../guides/daemon-mode.md) for more on the daemon architecture.
:::


## Installation

Add Conductor to your Go project:

```bash
go get github.com/tombee/conductor
```

## Core Concepts

Conduct's embeddable architecture consists of four main packages:

- **pkg/llm** - Provider abstraction for LLM interactions
- **pkg/workflow** - State machine-based workflow orchestration
- **pkg/agent** - ReAct-style agent loop for autonomous task execution
- **pkg/tools** - Extensible tool registry for agent capabilities

Each package is independent and can be used standalone or together.

## Quick Start: Simple LLM Integration

The simplest integration uses just the LLM package:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/tombee/conductor/pkg/llm"
    "github.com/tombee/conductor/pkg/llm/providers"
)

func main() {
    ctx := context.Background()

    // Create and register provider
    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    provider := providers.NewAnthropicProvider(apiKey)
    llm.Register(provider)
    llm.SetDefault("anthropic")

    // Make a completion request
    req := llm.CompletionRequest{
        Model: "fast",
        Messages: []llm.Message{
            {
                Role:    llm.MessageRoleSystem,
                Content: "You are a helpful assistant.",
            },
            {
                Role:    llm.MessageRoleUser,
                Content: "What is 2+2?",
            },
        },
    }

    resp, err := provider.Complete(ctx, req)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(resp.Content)
    fmt.Printf("Tokens: %d (cost: $%.6f)\n",
        resp.Usage.TotalTokens,
        calculateCost(resp))
}

func calculateCost(resp *llm.CompletionResponse) float64 {
    // Use cost tracker for accurate pricing
    tracker := llm.NewCostTracker()
    cost := tracker.TrackCompletion(
        "anthropic",
        resp.Model,
        resp.Usage,
        "example-request",
    )
    return cost.TotalCost
}
```

## Integration Pattern 1: Workflow Automation

Add workflow orchestration to your application:

```go
package automation

import (
    "context"
    "fmt"

    "github.com/tombee/conductor/pkg/llm"
    "github.com/tombee/conductor/pkg/workflow"
    "github.com/tombee/conductor/pkg/tools"
)

type WorkflowService struct {
    engine   *workflow.Executor
    store    workflow.Store
    eventBus *workflow.EventBus
}

func NewWorkflowService(llmProvider llm.Provider, dbPath string) (*WorkflowService, error) {
    // Create tool registry with builtin tools
    registry := tools.NewRegistry()
    // Register custom tools...

    // Create event bus for observability
    bus := workflow.NewEventBus()

    // Subscribe to events
    bus.Subscribe(workflow.EventTypeStateChanged, func(event workflow.Event) {
        e := event.(workflow.StateChangedEvent)
        fmt.Printf("Workflow %s: %s -> %s\n", e.WorkflowID, e.OldState, e.NewState)
    })

    // Create workflow executor
    engine := workflow.NewExecutor(
        workflow.WithLLMProvider(llmProvider),
        workflow.WithToolRegistry(registry),
        workflow.WithEventBus(bus),
    )

    // Create persistent store
    store := workflow.NewSQLiteStore(dbPath)

    return &WorkflowService{
        engine:   engine,
        store:    store,
        eventBus: bus,
    }, nil
}

func (s *WorkflowService) ExecuteWorkflow(ctx context.Context, defYAML []byte, inputs map[string]interface{}) (*workflow.Result, error) {
    // Parse workflow definition
    def, err := workflow.ParseDefinition(defYAML)
    if err != nil {
        return nil, fmt.Errorf("invalid workflow: %w", err)
    }

    // Validate
    if err := def.Validate(); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // Execute
    result, err := s.engine.Execute(ctx, def, inputs)
    if err != nil {
        return nil, fmt.Errorf("execution failed: %w", err)
    }

    return result, nil
}

func (s *WorkflowService) GetWorkflowHistory(ctx context.Context, limit int) ([]*workflow.Instance, error) {
    return s.store.List(ctx, workflow.ListOptions{
        Limit: limit,
    })
}
```

Usage in your application:

```go
func main() {
    // Initialize service
    provider := llm.GetDefault()
    svc, err := NewWorkflowService(provider, "./workflows.db")
    if err != nil {
        log.Fatal(err)
    }

    // Load workflow definition
    workflowYAML, _ := os.ReadFile("workflows/process-order.yaml")

    // Execute
    result, err := svc.ExecuteWorkflow(context.Background(), workflowYAML, map[string]interface{}{
        "order_id": "12345",
    })

    if err != nil {
        log.Printf("Workflow failed: %v", err)
        return
    }

    fmt.Printf("Order processed: %v\n", result.Outputs["status"])
}
```

## Integration Pattern 2: AI Agent for Support

Embed an agent for customer support automation:

```go
package support

import (
    "context"
    "fmt"

    "github.com/tombee/conductor/pkg/agent"
    "github.com/tombee/conductor/pkg/llm"
    "github.com/tombee/conductor/pkg/tools"
    "github.com/tombee/conductor/pkg/tools/builtin"
)

type SupportAgent struct {
    agent *agent.Agent
}

func NewSupportAgent(llmProvider llm.Provider, knowledgeBasePath string) *SupportAgent {
    // Create tool registry
    registry := tools.NewRegistry()

    // Add file tool for knowledge base access
    fileTool := builtin.NewFileTool(builtin.FileToolConfig{
        AllowedPaths: []string{knowledgeBasePath},
        MaxFileSize:  5 * 1024 * 1024, // 5MB
    })
    registry.Register(fileTool)

    // Add custom search tool
    registry.Register(&SearchKBTool{basePath: knowledgeBasePath})

    // Create agent adapter
    agentProvider := agent.NewLLMAdapter(llmProvider)

    // Create agent
    ag := agent.NewAgent(agentProvider, registry)
    ag = ag.WithMaxIterations(15)

    return &SupportAgent{agent: ag}
}

func (s *SupportAgent) HandleQuery(ctx context.Context, query string) (string, error) {
    systemPrompt := `You are a helpful customer support agent.
You have access to:
- file: Read documentation files
- search_kb: Search knowledge base

Answer user questions by:
1. Searching for relevant information
2. Reading necessary documents
3. Providing clear, helpful answers

If you cannot find the answer, say so clearly.`

    result, err := s.agent.Run(ctx, systemPrompt, query)
    if err != nil {
        return "", fmt.Errorf("agent failed: %w", err)
    }

    if !result.Success {
        return "", fmt.Errorf("agent error: %s", result.Error)
    }

    return result.FinalResponse, nil
}

// Custom tool for knowledge base search
type SearchKBTool struct {
    basePath string
}

func (t *SearchKBTool) Name() string {
    return "search_kb"
}

func (t *SearchKBTool) Description() string {
    return "Search the knowledge base for relevant articles"
}

func (t *SearchKBTool) Schema() *tools.Schema {
    return &tools.Schema{
        Inputs: &tools.ParameterSchema{
            Type: "object",
            Properties: map[string]*tools.Property{
                "query": {
                    Type:        "string",
                    Description: "Search query",
                },
            },
            Required: []string{"query"},
        },
        Outputs: &tools.ParameterSchema{
            Type: "object",
            Properties: map[string]*tools.Property{
                "results": {
                    Type:        "array",
                    Description: "List of relevant article paths",
                },
            },
        },
    }
}

func (t *SearchKBTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
    query := inputs["query"].(string)

    // Search implementation...
    results := searchKnowledgeBase(t.basePath, query)

    return map[string]interface{}{
        "results": results,
    }, nil
}
```

Usage:

```go
func handleSupportTicket(ticket Ticket) {
    // Create agent
    provider := llm.GetDefault()
    agent := NewSupportAgent(provider, "/kb")

    // Process query
    response, err := agent.HandleQuery(context.Background(), ticket.Question)
    if err != nil {
        log.Printf("Agent error: %v", err)
        // Fallback to human support
        escalateToHuman(ticket)
        return
    }

    // Send response
    ticket.Reply(response)
}
```

## Integration Pattern 3: Custom Provider

Implement a custom LLM provider:

```go
package myprovider

import (
    "context"
    "github.com/tombee/conductor/pkg/llm"
)

type MyProvider struct {
    apiKey string
    client *http.Client
}

func NewMyProvider(apiKey string) *MyProvider {
    return &MyProvider{
        apiKey: apiKey,
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (p *MyProvider) Name() string {
    return "myprovider"
}

func (p *MyProvider) Capabilities() llm.Capabilities {
    return llm.Capabilities{
        Streaming: true,
        Tools:     true,
        Models: []llm.ModelInfo{
            {
                ID:          "my-model-v1",
                Name:        "My Model v1",
                Tier:        llm.TierBalanced,
                ContextWindow: 128000,
            },
        },
    }
}

func (p *MyProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
    // Transform request to provider format
    providerReq := p.buildRequest(req)

    // Make API call
    resp, err := p.client.Do(providerReq)
    if err != nil {
        return nil, fmt.Errorf("API call failed: %w", err)
    }
    defer resp.Body.Close()

    // Parse response
    var providerResp MyProviderResponse
    if err := json.NewDecoder(resp.Body).Decode(&providerResp); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    // Transform to Conductor format
    return &llm.CompletionResponse{
        Content:      providerResp.Text,
        FinishReason: llm.FinishReasonStop,
        Usage: llm.TokenUsage{
            PromptTokens:     providerResp.InputTokens,
            CompletionTokens: providerResp.OutputTokens,
            TotalTokens:      providerResp.InputTokens + providerResp.OutputTokens,
        },
        Model:     req.Model,
        RequestID: providerResp.RequestID,
        Created:   time.Now(),
    }, nil
}

func (p *MyProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
    chunks := make(chan llm.StreamChunk)

    go func() {
        defer close(chunks)

        // Implementation...
        // Send chunks as they arrive from the API
    }()

    return chunks, nil
}

// Register your provider
func init() {
    provider := NewMyProvider(os.Getenv("MY_API_KEY"))
    llm.Register(provider)
}
```

## Integration Pattern 4: Streaming Responses

Handle streaming for real-time updates:

```go
func streamingCompletion(ctx context.Context, provider llm.Provider, prompt string) error {
    req := llm.CompletionRequest{
        Model: "balanced",
        Messages: []llm.Message{
            {Role: llm.MessageRoleUser, Content: prompt},
        },
    }

    stream, err := provider.Stream(ctx, req)
    if err != nil {
        return fmt.Errorf("stream failed: %w", err)
    }

    var fullResponse string

    for chunk := range stream {
        if chunk.Error != nil {
            return fmt.Errorf("stream error: %w", chunk.Error)
        }

        // Send chunk to client (websocket, SSE, etc.)
        fmt.Print(chunk.Delta.Content)
        fullResponse += chunk.Delta.Content

        // Check for completion
        if chunk.FinishReason != "" {
            fmt.Println()
            fmt.Printf("Completed: %s (tokens: %d)\n",
                chunk.FinishReason,
                chunk.Usage.TotalTokens)
            break
        }
    }

    return nil
}
```

## Integration Pattern 5: Multi-Provider Setup

Use multiple providers with failover:

```go
package llmconfig

import (
    "github.com/tombee/conductor/pkg/llm"
    "github.com/tombee/conductor/pkg/llm/providers"
)

func SetupProviders() error {
    // Register multiple providers
    anthropic := providers.NewAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY"))
    openai := providers.NewOpenAIProvider(os.Getenv("OPENAI_API_KEY"))
    ollama := providers.NewOllamaProvider("http://localhost:11434")

    if err := llm.Register(anthropic); err != nil {
        return err
    }
    if err := llm.Register(openai); err != nil {
        return err
    }
    if err := llm.Register(ollama); err != nil {
        return err
    }

    // Set primary provider
    if err := llm.SetDefault("anthropic"); err != nil {
        return err
    }

    // Configure failover order
    if err := llm.SetFailoverOrder([]string{
        "anthropic",
        "openai",
        "ollama", // Local fallback
    }); err != nil {
        return err
    }

    return nil
}

func GetProviderWithFailover() llm.Provider {
    primary, _ := llm.Get("anthropic")
    secondary, _ := llm.Get("openai")
    tertiary, _ := llm.Get("ollama")

    // Wrap with failover
    return llm.WithFailover(
        primary,
        []llm.Provider{secondary, tertiary},
        llm.FailoverConfig{
            CircuitBreakerThreshold: 5,
            CircuitBreakerTimeout:   30 * time.Second,
        },
    )
}
```

## Configuration Management

Structure your configuration:

```go
package config

import (
    "os"
    "time"
)

type Config struct {
    LLM      LLMConfig      `yaml:"llm"`
    Workflow WorkflowConfig `yaml:"workflow"`
    Agent    AgentConfig    `yaml:"agent"`
}

type LLMConfig struct {
    DefaultProvider string            `yaml:"default_provider"`
    Providers       map[string]string `yaml:"providers"` // name -> api_key
    Failover        []string          `yaml:"failover"`
    RetryAttempts   int               `yaml:"retry_attempts"`
}

type WorkflowConfig struct {
    DatabasePath string        `yaml:"database_path"`
    Retention    time.Duration `yaml:"retention"`
}

type AgentConfig struct {
    MaxIterations   int           `yaml:"max_iterations"`
    ContextWindow   int           `yaml:"context_window"`
    DefaultTimeout  time.Duration `yaml:"default_timeout"`
    AllowedToolDirs []string      `yaml:"allowed_tool_dirs"`
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}
```

Example config.yaml:

```yaml
llm:
  default_provider: anthropic
  providers:
    anthropic: ${ANTHROPIC_API_KEY}
    openai: ${OPENAI_API_KEY}
  failover:
    - anthropic
    - openai
  retry_attempts: 3

workflow:
  database_path: ./data/workflows.db
  retention: 168h # 7 days

agent:
  max_iterations: 20
  context_window: 100000
  default_timeout: 5m
  allowed_tool_dirs:
    - /workspace
    - /tmp
```

## Testing

Test your Conductor integration:

```go
package myapp_test

import (
    "context"
    "testing"

    "github.com/tombee/conductor/pkg/llm"
    "github.com/tombee/conductor/pkg/workflow"
)

// Mock LLM provider for tests
type MockProvider struct {
    responses []llm.CompletionResponse
    callCount int
}

func (m *MockProvider) Name() string {
    return "mock"
}

func (m *MockProvider) Capabilities() llm.Capabilities {
    return llm.Capabilities{Streaming: false, Tools: false}
}

func (m *MockProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
    if m.callCount >= len(m.responses) {
        return nil, fmt.Errorf("no more mock responses")
    }
    resp := m.responses[m.callCount]
    m.callCount++
    return &resp, nil
}

func (m *MockProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
    return nil, fmt.Errorf("streaming not supported in mock")
}

func TestWorkflowExecution(t *testing.T) {
    // Setup mock provider
    mock := &MockProvider{
        responses: []llm.CompletionResponse{
            {
                Content: "Test response",
                Usage:   llm.TokenUsage{TotalTokens: 100},
            },
        },
    }

    // Register mock
    llm.Register(mock)
    defer llm.Unregister("mock")

    // Create engine with mock
    engine := workflow.NewExecutor(
        workflow.WithLLMProvider(mock),
    )

    // Test workflow
    def := &workflow.Definition{
        Name: "test",
        Steps: []workflow.Step{
            {
                ID:     "test_step",
                Type:   workflow.StepTypeLLM,
                Action: "mock.complete",
            },
        },
    }

    result, err := engine.Execute(context.Background(), def, nil)
    if err != nil {
        t.Fatalf("execution failed: %v", err)
    }

    if !result.Success {
        t.Errorf("expected success, got failure")
    }
}
```

## Best Practices

### 1. Dependency Injection

Pass dependencies explicitly:

```go
// Good: Dependency injection
func NewService(llmProvider llm.Provider, db *sql.DB) *Service {
    return &Service{
        llm: llmProvider,
        db:  db,
    }
}

// Avoid: Global state
func NewService() *Service {
    return &Service{
        llm: llm.GetDefault(), // Tight coupling
    }
}
```

### 2. Context Propagation

Always propagate context:

```go
func (s *Service) Process(ctx context.Context, input string) error {
    // Pass context through
    result, err := s.agent.Run(ctx, systemPrompt, input)
    if err != nil {
        return err
    }
    // ...
}
```

### 3. Error Handling

Wrap errors with context:

```go
result, err := engine.Execute(ctx, def, inputs)
if err != nil {
    return fmt.Errorf("failed to execute workflow %s: %w", def.Name, err)
}
```

### 4. Resource Cleanup

Clean up resources properly:

```go
func (s *Service) Shutdown(ctx context.Context) error {
    // Close store connections
    if err := s.store.Close(); err != nil {
        return fmt.Errorf("failed to close store: %w", err)
    }

    // Unregister providers
    for _, name := range llm.List() {
        llm.Unregister(name)
    }

    return nil
}
```

### 5. Cost Monitoring

Track costs in production:

```go
type CostMonitor struct {
    tracker *llm.CostTracker
}

func (m *CostMonitor) WrapProvider(provider llm.Provider) llm.Provider {
    return &trackedProvider{
        Provider: provider,
        monitor:  m,
    }
}

type trackedProvider struct {
    llm.Provider
    monitor *CostMonitor
}

func (p *trackedProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
    resp, err := p.Provider.Complete(ctx, req)
    if err != nil {
        return nil, err
    }

    // Track cost
    correlationID := req.Metadata["correlation_id"]
    cost := p.monitor.tracker.TrackCompletion(
        p.Name(),
        resp.Model,
        resp.Usage,
        correlationID,
    )

    log.Printf("Request %s cost: $%.6f", correlationID, cost.TotalCost)

    return resp, nil
}
```

## Performance Considerations

### Connection Pooling

Conduct's HTTP clients automatically pool connections. For optimal performance:

```go
// Reuse providers across requests
var globalProvider llm.Provider

func init() {
    globalProvider = providers.NewAnthropicProvider(apiKey)
    llm.Register(globalProvider)
}

// Don't create new providers per request
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Good: Reuse global provider
    resp, _ := globalProvider.Complete(ctx, req)

    // Bad: Creates new HTTP client each time
    // provider := providers.NewAnthropicProvider(apiKey)
}
```

### Concurrent Execution

All Conductor APIs are thread-safe:

```go
// Safe: Concurrent workflow execution
var wg sync.WaitGroup
for _, workflowDef := range workflows {
    wg.Add(1)
    go func(def *workflow.Definition) {
        defer wg.Done()
        result, err := engine.Execute(ctx, def, inputs)
        // Handle result...
    }(workflowDef)
}
wg.Wait()
```

## Next Steps

- **API Reference**: [API Reference](../reference/api.md) - Detailed API documentation
- **Architecture**: [Architecture](../architecture/overview.md) - How Conductor works internally
- **Examples**: [Examples Gallery](../examples/index.md) - Real-world usage examples
- **Contributing**: [Contributing Guide](contributing.md) - How to contribute to Conductor

---
*Last updated: 2025-12-23*
