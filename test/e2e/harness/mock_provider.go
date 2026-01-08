// Package harness provides testing utilities for E2E workflow tests.
package harness

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tombee/conductor/pkg/llm"
)

// MockLLMProvider implements llm.Provider for testing.
// It returns pre-configured responses and records all requests for assertions.
type MockLLMProvider struct {
	responses    []MockResponse
	currentIndex int
	requests     []llm.CompletionRequest
	mu           sync.Mutex
}

// MockResponse defines a pre-configured response for the mock provider.
type MockResponse struct {
	// Content is the text response to return.
	Content string

	// ToolCalls contains tool invocations to return.
	ToolCalls []llm.ToolCall

	// FinishReason indicates why generation stopped.
	FinishReason llm.FinishReason

	// TokenUsage contains token counts for cost tracking.
	TokenUsage llm.TokenUsage

	// Error is returned instead of a successful response.
	Error error

	// Model is the model ID to report (defaults to "mock-model").
	Model string
}

// NewMockProvider creates a mock LLM provider with pre-configured responses.
// Responses are returned in order for each Complete/Stream call.
// If more requests are made than responses provided, an error is returned.
func NewMockProvider(responses ...MockResponse) *MockLLMProvider {
	return &MockLLMProvider{
		responses: responses,
		requests:  make([]llm.CompletionRequest, 0),
	}
}

// Name returns the provider identifier.
func (m *MockLLMProvider) Name() string {
	return "mock"
}

// Capabilities returns the mock provider's capabilities.
func (m *MockLLMProvider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		Streaming: true,
		Tools:     true,
		Models: []llm.ModelInfo{
			{
				ID:                           "mock-model",
				Name:                         "Mock Model",
				Tier:                         llm.ModelTierBalanced,
				MaxTokens:                    100000,
				MaxOutputTokens:              4096,
				InputPricePerMillion:         0,
				OutputPricePerMillion:        0,
				CacheCreationPricePerMillion: 0,
				CacheReadPricePerMillion:     0,
				SupportsTools:                true,
				SupportsVision:               false,
				Description:                  "Mock model for testing",
			},
		},
	}
}

// Complete sends a synchronous completion request and returns a pre-configured response.
func (m *MockLLMProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the request
	m.requests = append(m.requests, req)

	// Check if we have a response to return
	if m.currentIndex >= len(m.responses) {
		return nil, fmt.Errorf("mock provider: no more responses configured (requested %d, configured %d)", m.currentIndex+1, len(m.responses))
	}

	mockResp := m.responses[m.currentIndex]
	m.currentIndex++

	// Return error if configured
	if mockResp.Error != nil {
		return nil, mockResp.Error
	}

	// Set defaults
	model := mockResp.Model
	if model == "" {
		model = "mock-model"
	}

	finishReason := mockResp.FinishReason
	if finishReason == "" {
		if len(mockResp.ToolCalls) > 0 {
			finishReason = llm.FinishReasonToolCalls
		} else {
			finishReason = llm.FinishReasonStop
		}
	}

	// Calculate total tokens if not set
	usage := mockResp.TokenUsage
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	return &llm.CompletionResponse{
		Content:      mockResp.Content,
		ToolCalls:    mockResp.ToolCalls,
		FinishReason: finishReason,
		Usage:        usage,
		Cost:         0,
		Model:        model,
		RequestID:    uuid.New().String(),
		Created:      time.Now(),
	}, nil
}

// Stream sends a streaming completion request and returns a channel of chunks.
func (m *MockLLMProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the request
	m.requests = append(m.requests, req)

	// Check if we have a response to return
	if m.currentIndex >= len(m.responses) {
		return nil, fmt.Errorf("mock provider: no more responses configured (requested %d, configured %d)", m.currentIndex+1, len(m.responses))
	}

	mockResp := m.responses[m.currentIndex]
	m.currentIndex++

	// Create channel for streaming chunks
	chunks := make(chan llm.StreamChunk, 10)

	// Stream the response asynchronously
	go func() {
		defer close(chunks)

		// Return error if configured
		if mockResp.Error != nil {
			chunks <- llm.StreamChunk{
				Error:     mockResp.Error,
				RequestID: uuid.New().String(),
			}
			return
		}

		requestID := uuid.New().String()

		// Stream content in chunks (split into words for realistic behavior)
		if mockResp.Content != "" {
			// Split by spaces to simulate word-by-word streaming
			words := []string{}
			current := ""
			for _, char := range mockResp.Content {
				if char == ' ' || char == '\n' {
					if current != "" {
						words = append(words, current)
						current = ""
					}
					words = append(words, string(char))
				} else {
					current += string(char)
				}
			}
			if current != "" {
				words = append(words, current)
			}

			for _, word := range words {
				select {
				case <-ctx.Done():
					chunks <- llm.StreamChunk{
						Error:     ctx.Err(),
						RequestID: requestID,
					}
					return
				case chunks <- llm.StreamChunk{
					Delta: llm.StreamDelta{
						Content: word,
					},
					RequestID: requestID,
				}:
				}
			}
		}

		// Stream tool calls if present
		if len(mockResp.ToolCalls) > 0 {
			for i, toolCall := range mockResp.ToolCalls {
				select {
				case <-ctx.Done():
					chunks <- llm.StreamChunk{
						Error:     ctx.Err(),
						RequestID: requestID,
					}
					return
				case chunks <- llm.StreamChunk{
					Delta: llm.StreamDelta{
						ToolCallDelta: &llm.ToolCallDelta{
							Index:          i,
							ID:             toolCall.ID,
							Name:           toolCall.Name,
							ArgumentsDelta: toolCall.Arguments,
						},
					},
					RequestID: requestID,
				}:
				}
			}
		}

		// Set defaults for final chunk
		finishReason := mockResp.FinishReason
		if finishReason == "" {
			if len(mockResp.ToolCalls) > 0 {
				finishReason = llm.FinishReasonToolCalls
			} else {
				finishReason = llm.FinishReasonStop
			}
		}

		usage := mockResp.TokenUsage
		if usage.TotalTokens == 0 {
			usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		}

		// Send final chunk with finish reason and usage
		select {
		case <-ctx.Done():
			chunks <- llm.StreamChunk{
				Error:     ctx.Err(),
				RequestID: requestID,
			}
		case chunks <- llm.StreamChunk{
			FinishReason: finishReason,
			Usage:        &usage,
			RequestID:    requestID,
		}:
		}
	}()

	return chunks, nil
}

// GetRequests returns all recorded completion requests.
// This allows tests to assert on the requests made to the provider.
func (m *MockLLMProvider) GetRequests() []llm.CompletionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy to prevent external modifications
	requestsCopy := make([]llm.CompletionRequest, len(m.requests))
	copy(requestsCopy, m.requests)
	return requestsCopy
}

// Reset clears all recorded requests and resets the response index.
// This allows reusing the same mock provider across multiple tests.
func (m *MockLLMProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = make([]llm.CompletionRequest, 0)
	m.currentIndex = 0
}
