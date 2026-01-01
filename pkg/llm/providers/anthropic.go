// Package providers contains concrete implementations of LLM providers.
package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tombee/conductor/pkg/errors"
	"github.com/tombee/conductor/pkg/httpclient"
	"github.com/tombee/conductor/pkg/llm"
)

// ConnectionPoolMetrics tracks connection pool statistics.
type ConnectionPoolMetrics struct {
	mu             sync.RWMutex
	activeConns    int
	idleConns      int
	totalRequests  int64
	failedRequests int64
}

const (
	// anthropicAPIBaseURL is the base URL for the Anthropic API
	anthropicAPIBaseURL = "https://api.anthropic.com/v1"

	// anthropicAPIVersion is the API version to use
	anthropicAPIVersion = "2023-06-01"
)

// AnthropicProvider implements the Provider interface for Anthropic's Claude models.
type AnthropicProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	metrics    *ConnectionPoolMetrics
	lastUsage  *llm.TokenUsage
	usageMu    sync.RWMutex
}

// NewAnthropicProvider creates a new Anthropic provider instance.
// The apiKey should be retrieved from secure storage (keychain or encrypted config).
func NewAnthropicProvider(apiKey string) (*AnthropicProvider, error) {
	if apiKey == "" {
		return nil, &errors.ConfigError{
			Key:    "anthropic.api_key",
			Reason: "API key is required for Anthropic provider",
		}
	}

	// Create HTTP client using shared httpclient package
	cfg := httpclient.DefaultConfig()
	cfg.Timeout = 120 * time.Second // LLM requests can take a while
	cfg.UserAgent = "conductor-anthropic/1.0"
	// Retry logic is handled by the LLM retry wrapper (pkg/llm/retry.go)
	// which has Anthropic-specific error handling
	cfg.RetryAttempts = 0

	httpClient, err := httpclient.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return &AnthropicProvider{
		apiKey:     apiKey,
		baseURL:    anthropicAPIBaseURL,
		httpClient: httpClient,
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

// Complete sends a synchronous completion request to the Anthropic Messages API.
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

	// Build the API request
	apiReq, err := p.buildAPIRequest(req, anthropicTools, false)
	if err != nil {
		p.metrics.incrementFailedRequests()
		return nil, err
	}

	// Make the API call
	resp, err := p.doRequest(ctx, apiReq, requestID)
	if err != nil {
		p.metrics.incrementFailedRequests()
		return nil, err
	}

	// Parse the response
	completionResp, err := p.parseResponse(resp, requestID)
	if err != nil {
		p.metrics.incrementFailedRequests()
		return nil, err
	}

	return completionResp, nil
}

// buildAPIRequest constructs an anthropicRequest from a CompletionRequest.
func (p *AnthropicProvider) buildAPIRequest(req llm.CompletionRequest, tools []anthropicTool, stream bool) (*anthropicRequest, error) {
	// Resolve model
	model := p.resolveModel(req.Model)

	// Convert messages to Anthropic format
	var systemPrompt string
	var apiMessages []anthropicMessage

	for _, msg := range req.Messages {
		switch msg.Role {
		case llm.MessageRoleSystem:
			// Anthropic uses a separate system field
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += msg.Content

		case llm.MessageRoleUser:
			content := []interface{}{
				anthropicTextContent{Type: "text", Text: msg.Content},
			}
			apiMessages = append(apiMessages, anthropicMessage{
				Role:    "user",
				Content: content,
			})

		case llm.MessageRoleAssistant:
			var content []interface{}
			if msg.Content != "" {
				content = append(content, anthropicTextContent{Type: "text", Text: msg.Content})
			}
			// Include tool calls if present
			for _, tc := range msg.ToolCalls {
				var input map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Arguments), &input); err != nil {
					input = map[string]interface{}{}
				}
				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": input,
				})
			}
			if len(content) > 0 {
				apiMessages = append(apiMessages, anthropicMessage{
					Role:    "assistant",
					Content: content,
				})
			}

		case llm.MessageRoleTool:
			content := []interface{}{
				anthropicToolResultContent{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				},
			}
			apiMessages = append(apiMessages, anthropicMessage{
				Role:    "user",
				Content: content,
			})
		}
	}

	// Determine max tokens (use default if not specified)
	maxTokens := 4096
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	apiReq := &anthropicRequest{
		Model:         model,
		Messages:      apiMessages,
		MaxTokens:     maxTokens,
		System:        systemPrompt,
		Temperature:   req.Temperature,
		Tools:         tools,
		StopSequences: req.StopSequences,
		Stream:        stream,
	}

	return apiReq, nil
}

// doRequest sends the API request and returns the response body.
func (p *AnthropicProvider) doRequest(ctx context.Context, apiReq *anthropicRequest, requestID string) (*anthropicResponse, error) {
	// Marshal request body
	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, &errors.ProviderError{
			Provider:  "anthropic",
			Message:   fmt.Sprintf("failed to marshal request: %v", err),
			RequestID: requestID,
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, &errors.ProviderError{
			Provider:  "anthropic",
			Message:   fmt.Sprintf("failed to create request: %v", err),
			RequestID: requestID,
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, &errors.ProviderError{
			Provider:  "anthropic",
			Message:   fmt.Sprintf("request failed: %v", err),
			RequestID: requestID,
		}
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &errors.ProviderError{
			Provider:   "anthropic",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to read response: %v", err),
			RequestID:  requestID,
		}
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		var errResp anthropicErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, &errors.ProviderError{
				Provider:   "anthropic",
				StatusCode: resp.StatusCode,
				Message:    errResp.Error.Message,
				Suggestion: p.getSuggestionForError(resp.StatusCode, errResp.Error.Type),
				RequestID:  requestID,
			}
		}
		return nil, &errors.ProviderError{
			Provider:   "anthropic",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(respBody)),
			RequestID:  requestID,
		}
	}

	// Parse successful response
	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &errors.ProviderError{
			Provider:  "anthropic",
			Message:   fmt.Sprintf("failed to parse response: %v", err),
			RequestID: requestID,
		}
	}

	return &apiResp, nil
}

// getSuggestionForError returns a helpful suggestion based on the error type.
func (p *AnthropicProvider) getSuggestionForError(statusCode int, errorType string) string {
	switch statusCode {
	case http.StatusUnauthorized:
		return "Check that your API key is valid and correctly configured"
	case http.StatusForbidden:
		return "Your API key may not have access to this model or feature"
	case http.StatusTooManyRequests:
		return "Rate limit exceeded. Consider implementing backoff or reducing request frequency"
	case http.StatusBadRequest:
		if errorType == "invalid_request_error" {
			return "Check the request parameters for errors"
		}
		return "Review the request format and parameters"
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return "Anthropic API is experiencing issues. Retry after a short delay"
	default:
		return "Check the Anthropic API documentation for more details"
	}
}

// parseResponse converts an anthropicResponse to a CompletionResponse.
func (p *AnthropicProvider) parseResponse(resp *anthropicResponse, requestID string) (*llm.CompletionResponse, error) {
	// Extract text content and tool calls
	var textContent strings.Builder
	var toolCalls []llm.ToolCall

	for _, block := range resp.Content {
		blockType, _ := block["type"].(string)

		switch blockType {
		case "text":
			if text, ok := block["text"].(string); ok {
				if textContent.Len() > 0 {
					textContent.WriteString("\n")
				}
				textContent.WriteString(text)
			}
		case "tool_use":
			id, _ := block["id"].(string)
			name, _ := block["name"].(string)
			input := block["input"]

			inputJSON, err := json.Marshal(input)
			if err != nil {
				inputJSON = []byte("{}")
			}

			toolCalls = append(toolCalls, llm.ToolCall{
				ID:        id,
				Name:      name,
				Arguments: string(inputJSON),
			})
		}
	}

	// Map stop reason to finish reason
	finishReason := p.mapStopReason(resp.StopReason)

	// Build usage stats
	usage := llm.TokenUsage{
		PromptTokens:        resp.Usage.InputTokens,
		CompletionTokens:    resp.Usage.OutputTokens,
		TotalTokens:         resp.Usage.InputTokens + resp.Usage.OutputTokens,
		CacheCreationTokens: resp.Usage.CacheCreationTokens,
		CacheReadTokens:     resp.Usage.CacheReadTokens,
	}

	// Cache usage for cost tracking
	p.setLastUsage(usage)

	return &llm.CompletionResponse{
		Content:      textContent.String(),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
		Model:        resp.Model,
		RequestID:    requestID,
		Created:      time.Now(),
	}, nil
}

// mapStopReason converts Anthropic's stop_reason to our FinishReason.
func (p *AnthropicProvider) mapStopReason(stopReason string) llm.FinishReason {
	switch stopReason {
	case "end_turn", "stop_sequence":
		return llm.FinishReasonStop
	case "max_tokens":
		return llm.FinishReasonLength
	case "tool_use":
		return llm.FinishReasonToolCalls
	case "content_filtered":
		return llm.FinishReasonContentFilter
	default:
		return llm.FinishReasonStop
	}
}

// Stream sends a streaming completion request to the Anthropic Messages API.
func (p *AnthropicProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	p.metrics.incrementTotalRequests()

	requestID := uuid.New().String()

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

	// Build the API request with streaming enabled
	apiReq, err := p.buildAPIRequest(req, anthropicTools, true)
	if err != nil {
		p.metrics.incrementFailedRequests()
		return nil, err
	}

	// Marshal request body
	body, err := json.Marshal(apiReq)
	if err != nil {
		p.metrics.incrementFailedRequests()
		return nil, &errors.ProviderError{
			Provider:  "anthropic",
			Message:   fmt.Sprintf("failed to marshal request: %v", err),
			RequestID: requestID,
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		p.metrics.incrementFailedRequests()
		return nil, &errors.ProviderError{
			Provider:  "anthropic",
			Message:   fmt.Sprintf("failed to create request: %v", err),
			RequestID: requestID,
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		p.metrics.incrementFailedRequests()
		return nil, &errors.ProviderError{
			Provider:  "anthropic",
			Message:   fmt.Sprintf("request failed: %v", err),
			RequestID: requestID,
		}
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		p.metrics.incrementFailedRequests()

		var errResp anthropicErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, &errors.ProviderError{
				Provider:   "anthropic",
				StatusCode: resp.StatusCode,
				Message:    errResp.Error.Message,
				Suggestion: p.getSuggestionForError(resp.StatusCode, errResp.Error.Type),
				RequestID:  requestID,
			}
		}
		return nil, &errors.ProviderError{
			Provider:   "anthropic",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(respBody)),
			RequestID:  requestID,
		}
	}

	// Create output channel
	chunks := make(chan llm.StreamChunk, 10)

	// Start goroutine to process SSE stream
	go p.processStream(ctx, resp, chunks, requestID)

	return chunks, nil
}

// processStream reads the SSE stream and sends chunks to the channel.
func (p *AnthropicProvider) processStream(ctx context.Context, resp *http.Response, chunks chan<- llm.StreamChunk, requestID string) {
	defer close(chunks)
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var currentToolCall *llm.ToolCallDelta
	var toolCallIndex int
	var totalUsage *llm.TokenUsage

	for {
		select {
		case <-ctx.Done():
			chunks <- llm.StreamChunk{
				RequestID:    requestID,
				Error:        ctx.Err(),
				FinishReason: llm.FinishReasonError,
			}
			p.metrics.incrementFailedRequests()
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Stream ended normally
				if totalUsage != nil {
					p.setLastUsage(*totalUsage)
				}
				return
			}
			chunks <- llm.StreamChunk{
				RequestID:    requestID,
				Error:        fmt.Errorf("stream read error: %w", err),
				FinishReason: llm.FinishReasonError,
			}
			p.metrics.incrementFailedRequests()
			return
		}

		// Parse SSE format: "event: <type>\ndata: <json>\n\n"
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip event type lines
		if strings.HasPrefix(line, "event:") {
			continue
		}

		// Parse data lines
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)

		if data == "" || data == "[DONE]" {
			continue
		}

		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue // Skip malformed events
		}

		switch event.Type {
		case "message_start":
			// Initial message with metadata
			if event.Message != nil {
				chunks <- llm.StreamChunk{
					RequestID: requestID,
				}
			}

		case "content_block_start":
			// New content block starting
			if event.ContentBlock != nil {
				blockType, _ := event.ContentBlock["type"].(string)
				if blockType == "tool_use" {
					// Starting a new tool call
					id, _ := event.ContentBlock["id"].(string)
					name, _ := event.ContentBlock["name"].(string)
					currentToolCall = &llm.ToolCallDelta{
						Index: toolCallIndex,
						ID:    id,
						Name:  name,
					}
					toolCallIndex++

					chunks <- llm.StreamChunk{
						RequestID: requestID,
						Delta: llm.StreamDelta{
							ToolCallDelta: currentToolCall,
						},
					}
				}
			}

		case "content_block_delta":
			// Incremental content
			if event.Delta != nil {
				deltaType, _ := event.Delta["type"].(string)

				switch deltaType {
				case "text_delta":
					text, _ := event.Delta["text"].(string)
					if text != "" {
						chunks <- llm.StreamChunk{
							RequestID: requestID,
							Delta: llm.StreamDelta{
								Content: text,
							},
						}
					}

				case "input_json_delta":
					partialJSON, _ := event.Delta["partial_json"].(string)
					if partialJSON != "" && currentToolCall != nil {
						chunks <- llm.StreamChunk{
							RequestID: requestID,
							Delta: llm.StreamDelta{
								ToolCallDelta: &llm.ToolCallDelta{
									Index:          currentToolCall.Index,
									ArgumentsDelta: partialJSON,
								},
							},
						}
					}
				}
			}

		case "content_block_stop":
			// Content block finished
			currentToolCall = nil

		case "message_delta":
			// Message-level updates (stop reason, usage)
			if event.Delta != nil {
				stopReason, _ := event.Delta["stop_reason"].(string)
				if stopReason != "" {
					finishReason := p.mapStopReason(stopReason)
					chunks <- llm.StreamChunk{
						RequestID:    requestID,
						FinishReason: finishReason,
					}
				}
			}
			if event.Usage != nil {
				totalUsage = &llm.TokenUsage{
					PromptTokens:        event.Usage.InputTokens,
					CompletionTokens:    event.Usage.OutputTokens,
					TotalTokens:         event.Usage.InputTokens + event.Usage.OutputTokens,
					CacheCreationTokens: event.Usage.CacheCreationTokens,
					CacheReadTokens:     event.Usage.CacheReadTokens,
				}
				chunks <- llm.StreamChunk{
					RequestID: requestID,
					Usage:     totalUsage,
				}
			}

		case "message_stop":
			// Message complete
			if totalUsage != nil {
				p.setLastUsage(*totalUsage)
			}
			return

		case "error":
			// Error event
			errMsg := "unknown streaming error"
			if event.Delta != nil {
				if msg, ok := event.Delta["message"].(string); ok {
					errMsg = msg
				}
			}
			chunks <- llm.StreamChunk{
				RequestID: requestID,
				Error: &errors.ProviderError{
					Provider:  "anthropic",
					Message:   errMsg,
					RequestID: requestID,
				},
				FinishReason: llm.FinishReasonError,
			}
			p.metrics.incrementFailedRequests()
			return
		}
	}
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

// CreateMockResponse is a helper to create a mock response with specific content.
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

// anthropicRequest represents the request body for the Anthropic Messages API.
type anthropicRequest struct {
	Model         string             `json:"model"`
	Messages      []anthropicMessage `json:"messages"`
	MaxTokens     int                `json:"max_tokens"`
	System        string             `json:"system,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	Tools         []anthropicTool    `json:"tools,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
}

// anthropicMessage represents a message in the Anthropic API format.
type anthropicMessage struct {
	Role    string        `json:"role"`
	Content []interface{} `json:"content"`
}

// anthropicTextContent represents a text content block.
type anthropicTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicToolResultContent represents a tool result content block.
type anthropicToolResultContent struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
}

// anthropicResponse represents the response from the Anthropic Messages API.
type anthropicResponse struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []map[string]interface{} `json:"content"`
	Model        string                   `json:"model"`
	StopReason   string                   `json:"stop_reason"`
	StopSequence *string                  `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage           `json:"usage"`
}

// anthropicUsage represents token usage in the Anthropic API response.
type anthropicUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// anthropicErrorResponse represents an error response from the Anthropic API.
type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// anthropicStreamEvent represents a streaming event from the Anthropic API.
type anthropicStreamEvent struct {
	Type         string                 `json:"type"`
	Index        int                    `json:"index,omitempty"`
	ContentBlock map[string]interface{} `json:"content_block,omitempty"`
	Delta        map[string]interface{} `json:"delta,omitempty"`
	Message      *anthropicResponse     `json:"message,omitempty"`
	Usage        *anthropicUsage        `json:"usage,omitempty"`
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
