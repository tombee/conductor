// Package providers contains concrete implementations of LLM providers.
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tombee/conductor/pkg/errors"
	"github.com/tombee/conductor/pkg/httpclient"
	"github.com/tombee/conductor/pkg/llm"
)

// ConnectionPoolConfig configures the HTTP connection pool.
type ConnectionPoolConfig struct {
	// MaxIdleConns controls the maximum number of idle (keep-alive) connections.
	MaxIdleConns int

	// MaxIdleConnsPerHost controls max idle connections per host.
	MaxIdleConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle connection will remain idle.
	IdleConnTimeout time.Duration

	// ResponseHeaderTimeout is the timeout waiting for response headers.
	ResponseHeaderTimeout time.Duration
}

// DefaultConnectionPoolConfig returns default connection pool settings.
func DefaultConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
}

// ConnectionPoolMetrics tracks connection pool statistics.
type ConnectionPoolMetrics struct {
	mu               sync.RWMutex
	activeConns      int
	idleConns        int
	totalRequests    int64
	failedRequests   int64
}

// AnthropicProvider implements the Provider interface for Anthropic's Claude models.
// Includes connection pooling for improved performance and resource management.
type AnthropicProvider struct {
	apiKey      string
	httpClient  *http.Client
	poolConfig  ConnectionPoolConfig
	metrics     *ConnectionPoolMetrics
	lastUsage   *llm.TokenUsage
	usageMu     sync.RWMutex
	// TODO: Add actual Anthropic SDK client once API integration is working
}

// NewAnthropicProvider creates a new Anthropic provider instance with default connection pool.
// The apiKey should be retrieved from secure storage (keychain or encrypted config).
func NewAnthropicProvider(apiKey string) (*AnthropicProvider, error) {
	return NewAnthropicProviderWithPool(apiKey, DefaultConnectionPoolConfig())
}

// NewAnthropicProviderWithPool creates a new Anthropic provider with custom connection pool config.
func NewAnthropicProviderWithPool(apiKey string, poolConfig ConnectionPoolConfig) (*AnthropicProvider, error) {
	if apiKey == "" {
		return nil, &errors.ConfigError{
			Key:    "anthropic.api_key",
			Reason: "API key is required for Anthropic provider",
		}
	}

	// Create HTTP client using shared httpclient package
	cfg := httpclient.DefaultConfig()
	cfg.Timeout = 5 * time.Second
	cfg.UserAgent = "conductor-anthropic/1.0"
	// Note: Retry logic will be handled by the LLM retry wrapper (pkg/llm/retry.go)
	// which has Anthropic-specific error handling, so we disable retries here
	cfg.RetryAttempts = 0

	httpClient, err := httpclient.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Note: Connection pooling is handled by httpclient.New() with sensible defaults.
	// The poolConfig parameter is kept for backward compatibility but connection pool
	// settings are now managed by the shared httpclient package.
	// Correlation ID propagation is automatically handled by httpclient's logging transport.

	return &AnthropicProvider{
		apiKey:     apiKey,
		httpClient: httpClient,
		poolConfig: poolConfig,
		metrics:    &ConnectionPoolMetrics{},
	}, nil
}

// Name returns the provider identifier.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Capabilities returns the features supported by this provider.
func (p *AnthropicProvider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		Streaming: true,
		Tools:     true,
		Models:    anthropicModels,
	}
}

// Complete sends a synchronous completion request.
// This is a minimal implementation for Phase 1b. Full API integration in T012.
func (p *AnthropicProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	p.metrics.incrementTotalRequests()

	requestID := uuid.New().String()

	// Validate the request has messages
	if len(req.Messages) == 0 {
		p.metrics.incrementFailedRequests()
		return nil, &errors.ValidationError{
			Field:      "messages",
			Message:    "completion request must have at least one message",
			Suggestion: "Add at least one message to the completion request",
		}
	}

	// Validate and transform tools if provided
	var anthropicTools []anthropicTool
	if len(req.Tools) > 0 {
		var err error
		anthropicTools, err = transformTools(req.Tools)
		if err != nil {
			p.metrics.incrementFailedRequests()
			return nil, err
		}
	}

	// Stop sequences will be passed to API when implemented
	_ = req.StopSequences
	_ = anthropicTools

	// TODO: Implement actual Anthropic API call
	// For now, return an error indicating this needs real implementation
	p.metrics.incrementFailedRequests()
	return nil, &errors.ProviderError{
		Provider:   "anthropic",
		StatusCode: http.StatusNotImplemented,
		Message:    "Anthropic API integration not yet implemented",
		Suggestion: "Use a mock provider for testing or wait for full API implementation",
		RequestID:  requestID,
	}
}

// Stream sends a streaming completion request.
// This is a minimal implementation for Phase 1b. Full API integration in T013.
func (p *AnthropicProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	p.metrics.incrementTotalRequests()

	requestID := uuid.New().String()

	// Resolve model
	model := p.resolveModel(req.Model)

	// Validate the request
	if len(req.Messages) == 0 {
		p.metrics.incrementFailedRequests()
		return nil, &errors.ValidationError{
			Field:      "messages",
			Message:    "completion request must have at least one message",
			Suggestion: "Add at least one message to the completion request",
		}
	}

	// Validate and transform tools if provided
	var anthropicTools []anthropicTool
	if len(req.Tools) > 0 {
		var err error
		anthropicTools, err = transformTools(req.Tools)
		if err != nil {
			p.metrics.incrementFailedRequests()
			return nil, err
		}
	}

	// Stop sequences will be passed to API when implemented
	_ = req.StopSequences
	_ = anthropicTools

	// Create output channel
	chunks := make(chan llm.StreamChunk, 10)

	// For now, return an error chunk indicating this needs real implementation
	go func() {
		defer close(chunks)
		p.metrics.incrementFailedRequests()
		chunks <- llm.StreamChunk{
			RequestID: requestID,
			Error: &errors.ProviderError{
				Provider:   "anthropic",
				StatusCode: http.StatusNotImplemented,
				Message:    fmt.Sprintf("Anthropic streaming not yet implemented (model: %s)", model),
				Suggestion: "Use a mock provider for testing or wait for full API implementation",
				RequestID:  requestID,
			},
			FinishReason: llm.FinishReasonError,
		}
	}()

	return chunks, nil
}

// GetLastUsage returns the token usage from the most recent request.
// Implements the UsageTrackable interface for cost tracking.
func (p *AnthropicProvider) GetLastUsage() *llm.TokenUsage {
	p.usageMu.RLock()
	defer p.usageMu.RUnlock()

	if p.lastUsage == nil {
		return nil
	}

	// Return a copy to prevent mutation
	usage := *p.lastUsage
	return &usage
}

// setLastUsage updates the cached usage from a response.
func (p *AnthropicProvider) setLastUsage(usage llm.TokenUsage) {
	p.usageMu.Lock()
	defer p.usageMu.Unlock()
	p.lastUsage = &usage
}

// resolveModel converts a tier or model ID to an Anthropic model ID.
func (p *AnthropicProvider) resolveModel(modelOrTier string) string {
	// Check if it's a tier
	switch llm.ModelTier(modelOrTier) {
	case llm.ModelTierFast:
		return "claude-3-5-haiku-20241022"
	case llm.ModelTierBalanced:
		return "claude-3-5-sonnet-20241022"
	case llm.ModelTierStrategic:
		return "claude-3-opus-20240229"
	}

	// Otherwise assume it's a specific model ID
	return modelOrTier
}

// GetModelInfo returns the ModelInfo for a given model ID.
func (p *AnthropicProvider) GetModelInfo(modelID string) (*llm.ModelInfo, error) {
	for i := range anthropicModels {
		if anthropicModels[i].ID == modelID {
			return &anthropicModels[i], nil
		}
	}
	return nil, &errors.NotFoundError{
		Resource: "model",
		ID:       modelID,
	}
}

// anthropicModels contains metadata for all Claude models.
var anthropicModels = []llm.ModelInfo{
	{
		ID:              "claude-3-5-haiku-20241022",
		Name:            "Claude 3.5 Haiku",
		Tier:            llm.ModelTierFast,
		MaxTokens:       200000,
		MaxOutputTokens: 8192,
		SupportsTools:   true,
		SupportsVision:  true,
		Description:     "Fast and cost-effective for simple tasks and high-volume requests.",
	},
	{
		ID:              "claude-3-5-sonnet-20241022",
		Name:            "Claude 3.5 Sonnet",
		Tier:            llm.ModelTierBalanced,
		MaxTokens:       200000,
		MaxOutputTokens: 8192,
		SupportsTools:   true,
		SupportsVision:  true,
		Description:     "Balanced capability and cost for most general-purpose tasks.",
	},
	{
		ID:              "claude-3-opus-20240229",
		Name:            "Claude 3 Opus",
		Tier:            llm.ModelTierStrategic,
		MaxTokens:       200000,
		MaxOutputTokens: 4096,
		SupportsTools:   true,
		SupportsVision:  true,
		Description:     "Maximum capability for complex reasoning and expert tasks.",
	},
}

// MockAnthropicProvider is a mock implementation for testing.
type MockAnthropicProvider struct {
	*AnthropicProvider
	MockComplete func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error)
	MockStream   func(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error)
}

// NewMockAnthropicProvider creates a mock provider for testing.
func NewMockAnthropicProvider() *MockAnthropicProvider {
	base, _ := NewAnthropicProvider("mock-api-key")
	return &MockAnthropicProvider{
		AnthropicProvider: base,
	}
}

// Complete delegates to the mock function if set, otherwise uses the base implementation.
func (m *MockAnthropicProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	if m.MockComplete != nil {
		return m.MockComplete(ctx, req)
	}

	// Default mock response
	requestID := uuid.New().String()
	model := m.resolveModel(req.Model)

	// Create a simple mock response
	content := "This is a mock response from Claude."
	promptTokens := estimateTokens(req.Messages)
	completionTokens := estimateTokens([]llm.Message{{Content: content}})

	usage := llm.TokenUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}

	resp := &llm.CompletionResponse{
		Content:      content,
		FinishReason: llm.FinishReasonStop,
		Usage:        usage,
		Model:        model,
		RequestID:    requestID,
		Created:      time.Now(),
	}

	// Track usage for cost tracking
	m.setLastUsage(usage)

	return resp, nil
}

// Stream delegates to the mock function if set, otherwise provides a simple mock stream.
func (m *MockAnthropicProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	if m.MockStream != nil {
		return m.MockStream(ctx, req)
	}

	// Default mock streaming response
	requestID := uuid.New().String()
	chunks := make(chan llm.StreamChunk, 10)

	go func() {
		defer close(chunks)

		// Send a few text chunks
		words := []string{"This", " is", " a", " mock", " streaming", " response", "."}
		for _, word := range words {
			chunks <- llm.StreamChunk{
				RequestID: requestID,
				Delta: llm.StreamDelta{
					Content: word,
				},
			}
		}

		// Send final chunk with usage
		promptTokens := estimateTokens(req.Messages)
		completionTokens := 10

		usage := llm.TokenUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}

		// Track usage for cost tracking
		m.setLastUsage(usage)

		chunks <- llm.StreamChunk{
			RequestID:    requestID,
			FinishReason: llm.FinishReasonStop,
			Usage:        &usage,
		}
	}()

	return chunks, nil
}

// estimateTokens provides a rough token count estimate (4 chars ≈ 1 token).
func estimateTokens(messages []llm.Message) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
	}
	return totalChars / 4
}

// Helper to create a mock response with specific content.
func CreateMockResponse(content string, model string) *llm.CompletionResponse {
	return &llm.CompletionResponse{
		Content:      content,
		FinishReason: llm.FinishReasonStop,
		Usage: llm.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: len(content) / 4,
			TotalTokens:      10 + len(content)/4,
		},
		Model:     model,
		RequestID: uuid.New().String(),
		Created:   time.Now(),
	}
}

// MarshalToolInput converts a tool input struct to a JSON string for Arguments.
func MarshalToolInput(input interface{}) (string, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshaling tool input: %w", err)
	}
	return string(data), nil
}

// UnmarshalToolInput parses a tool Arguments JSON string into a struct.
func UnmarshalToolInput(arguments string, output interface{}) error {
	if err := json.Unmarshal([]byte(arguments), output); err != nil {
		return &errors.ValidationError{
			Field:      "tool_arguments",
			Message:    fmt.Sprintf("failed to parse tool arguments: %v", err),
			Suggestion: "Ensure tool arguments are valid JSON matching the expected schema",
		}
	}
	return nil
}

// GetPoolMetrics returns the current connection pool metrics.
// Note: These are application-level metrics. Actual HTTP transport metrics
// are not directly exposed by Go's http.Transport.
func (p *AnthropicProvider) GetPoolMetrics() ConnectionPoolMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	return ConnectionPoolMetrics{
		activeConns:    p.metrics.activeConns,
		idleConns:      p.metrics.idleConns,
		totalRequests:  p.metrics.totalRequests,
		failedRequests: p.metrics.failedRequests,
	}
}

// GetPoolConfig returns the connection pool configuration.
func (p *AnthropicProvider) GetPoolConfig() ConnectionPoolConfig {
	return p.poolConfig
}

// incrementTotalRequests increments the total request counter.
func (m *ConnectionPoolMetrics) incrementTotalRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRequests++
}

// incrementFailedRequests increments the failed request counter.
func (m *ConnectionPoolMetrics) incrementFailedRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedRequests++
}

// GetTotalRequests returns the total number of requests made.
func (m *ConnectionPoolMetrics) GetTotalRequests() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalRequests
}

// GetFailedRequests returns the total number of failed requests.
func (m *ConnectionPoolMetrics) GetFailedRequests() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.failedRequests
}

// anthropicTool represents a tool in Anthropic's API format.
type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// anthropicToolUse represents a tool_use content block in Anthropic's response.
type anthropicToolUse struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// transformTools converts generic Tool definitions to Anthropic's API format.
// Validates that InputSchema depth does not exceed 10 levels per FR1.
func transformTools(tools []llm.Tool) ([]anthropicTool, error) {
	result := make([]anthropicTool, len(tools))

	for i, tool := range tools {
		// Validate schema depth (starting from depth 0)
		if err := validateSchemaDepth(tool.InputSchema, 0, 10); err != nil {
			return nil, &errors.ValidationError{
				Field:      fmt.Sprintf("tools[%d].input_schema", i),
				Message:    err.Error(),
				Suggestion: "Simplify the tool schema to have maximum 10 levels of nesting",
			}
		}

		result[i] = anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	return result, nil
}

// validateSchemaDepth checks that a JSON schema doesn't exceed the maximum nesting depth.
// The depth limit applies to the number of nested "properties" or "items" objects.
func validateSchemaDepth(schema map[string]interface{}, currentDepth, maxDepth int) error {
	if currentDepth >= maxDepth {
		return fmt.Errorf("schema nesting depth exceeds maximum of %d levels", maxDepth)
	}

	for key, value := range schema {
		switch v := value.(type) {
		case map[string]interface{}:
			// Only count depth for "properties" and "items" keys to measure actual nesting
			if key == "properties" || key == "items" {
				if err := validateSchemaDepth(v, currentDepth+1, maxDepth); err != nil {
					return err
				}
			} else {
				// Other nested maps don't increase depth (e.g., individual property definitions)
				if err := validateSchemaDepth(v, currentDepth, maxDepth); err != nil {
					return err
				}
			}
		case []interface{}:
			// Check array elements
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if err := validateSchemaDepth(itemMap, currentDepth+1, maxDepth); err != nil {
						return err
					}
				}
			}
		default:
			// Primitive values don't increase depth
			_ = key
		}
	}

	return nil
}

// parseToolUses extracts tool calls from Anthropic's response content blocks.
// The response may contain multiple content blocks including text and tool_use blocks.
func parseToolUses(contentBlocks []interface{}) ([]llm.ToolCall, error) {
	var toolCalls []llm.ToolCall

	for _, block := range contentBlocks {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}

		blockType, ok := blockMap["type"].(string)
		if !ok || blockType != "tool_use" {
			continue
		}

		// Extract tool use fields
		id, _ := blockMap["id"].(string)
		name, _ := blockMap["name"].(string)
		input := blockMap["input"]

		// Marshal input to JSON string for Arguments field
		inputJSON, err := json.Marshal(input)
		if err != nil {
			return nil, &errors.ProviderError{
				Provider:   "anthropic",
				Message:    fmt.Sprintf("failed to marshal tool input: %v", err),
				Suggestion: "Check that tool response contains valid JSON input",
			}
		}

		// Validate arguments against schema (basic validation)
		if err := validateToolArguments(string(inputJSON)); err != nil {
			return nil, err
		}

		toolCalls = append(toolCalls, llm.ToolCall{
			ID:        id,
			Name:      name,
			Arguments: string(inputJSON),
		})
	}

	return toolCalls, nil
}

// validateToolArguments performs basic validation on tool arguments JSON.
func validateToolArguments(arguments string) error {
	var temp interface{}
	if err := json.Unmarshal([]byte(arguments), &temp); err != nil {
		return &errors.ValidationError{
			Field:      "tool_arguments",
			Message:    fmt.Sprintf("invalid tool arguments JSON: %v", err),
			Suggestion: "Ensure tool arguments are valid JSON",
		}
	}
	return nil
}
