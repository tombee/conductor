package harness

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tombee/conductor/pkg/llm"
)

func TestNewMockProvider(t *testing.T) {
	tests := []struct {
		name          string
		responses     []MockResponse
		wantResponses int
	}{
		{
			name:          "no responses",
			responses:     nil,
			wantResponses: 0,
		},
		{
			name: "single response",
			responses: []MockResponse{
				{Content: "test"},
			},
			wantResponses: 1,
		},
		{
			name: "multiple responses",
			responses: []MockResponse{
				{Content: "first"},
				{Content: "second"},
			},
			wantResponses: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider(tt.responses...)

			if provider == nil {
				t.Fatal("NewMockProvider returned nil")
			}

			if len(provider.responses) != tt.wantResponses {
				t.Errorf("expected %d responses, got %d", tt.wantResponses, len(provider.responses))
			}

			if len(provider.requests) != 0 {
				t.Errorf("expected 0 initial requests, got %d", len(provider.requests))
			}
		})
	}
}

func TestMockProvider_Name(t *testing.T) {
	provider := NewMockProvider()

	if name := provider.Name(); name != "mock" {
		t.Errorf("expected Name() to return 'mock', got '%s'", name)
	}
}

func TestMockProvider_Capabilities(t *testing.T) {
	provider := NewMockProvider()
	caps := provider.Capabilities()

	if !caps.Streaming {
		t.Error("expected Streaming capability to be true")
	}

	if !caps.Tools {
		t.Error("expected Tools capability to be true")
	}

	if len(caps.Models) == 0 {
		t.Error("expected Models to be populated")
	}

	// Verify mock model details
	model := caps.Models[0]
	if model.ID != "mock-model" {
		t.Errorf("expected model ID 'mock-model', got '%s'", model.ID)
	}

	if !model.SupportsTools {
		t.Error("expected mock model to support tools")
	}
}

func TestMockProvider_Complete(t *testing.T) {
	tests := []struct {
		name           string
		responses      []MockResponse
		requestCount   int
		wantError      bool
		wantContent    string
		wantFinish     llm.FinishReason
		wantToolCalls  int
		checkRecording bool
	}{
		{
			name: "simple text response",
			responses: []MockResponse{
				{
					Content:      "Hello, World!",
					FinishReason: llm.FinishReasonStop,
					TokenUsage: llm.TokenUsage{
						InputTokens:  10,
						OutputTokens: 5,
					},
				},
			},
			requestCount:   1,
			wantContent:    "Hello, World!",
			wantFinish:     llm.FinishReasonStop,
			checkRecording: true,
		},
		{
			name: "response with tool calls",
			responses: []MockResponse{
				{
					ToolCalls: []llm.ToolCall{
						{
							ID:        "call_1",
							Name:      "get_weather",
							Arguments: `{"location": "San Francisco"}`,
						},
					},
					TokenUsage: llm.TokenUsage{
						InputTokens:  20,
						OutputTokens: 10,
					},
				},
			},
			requestCount:  1,
			wantFinish:    llm.FinishReasonToolCalls,
			wantToolCalls: 1,
		},
		{
			name: "error response",
			responses: []MockResponse{
				{
					Error: errors.New("mock error"),
				},
			},
			requestCount: 1,
			wantError:    true,
		},
		{
			name: "multiple responses in sequence",
			responses: []MockResponse{
				{Content: "first"},
				{Content: "second"},
			},
			requestCount: 2,
			wantContent:  "second",
		},
		{
			name:         "no responses configured",
			responses:    []MockResponse{},
			requestCount: 1,
			wantError:    true,
		},
		{
			name: "custom model name",
			responses: []MockResponse{
				{
					Content: "test",
					Model:   "custom-model",
				},
			},
			requestCount: 1,
			wantContent:  "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider(tt.responses...)
			ctx := context.Background()

			var lastResp *llm.CompletionResponse
			var lastErr error

			// Make the configured number of requests
			for i := 0; i < tt.requestCount; i++ {
				req := llm.CompletionRequest{
					Messages: []llm.Message{
						{Role: llm.MessageRoleUser, Content: "test prompt"},
					},
					Model: "mock-model",
				}

				lastResp, lastErr = provider.Complete(ctx, req)
			}

			// Check error expectation
			if tt.wantError {
				if lastErr == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if lastErr != nil {
				t.Fatalf("unexpected error: %v", lastErr)
			}

			// Check response content
			if tt.wantContent != "" && lastResp.Content != tt.wantContent {
				t.Errorf("expected content %q, got %q", tt.wantContent, lastResp.Content)
			}

			// Check finish reason
			if tt.wantFinish != "" && lastResp.FinishReason != tt.wantFinish {
				t.Errorf("expected finish reason %q, got %q", tt.wantFinish, lastResp.FinishReason)
			}

			// Check tool calls
			if tt.wantToolCalls > 0 && len(lastResp.ToolCalls) != tt.wantToolCalls {
				t.Errorf("expected %d tool calls, got %d", tt.wantToolCalls, len(lastResp.ToolCalls))
			}

			// Check token usage calculation
			if lastResp.Usage.TotalTokens != lastResp.Usage.InputTokens+lastResp.Usage.OutputTokens {
				t.Errorf("TotalTokens (%d) != InputTokens (%d) + OutputTokens (%d)",
					lastResp.Usage.TotalTokens, lastResp.Usage.InputTokens, lastResp.Usage.OutputTokens)
			}

			// Check request recording
			if tt.checkRecording {
				requests := provider.GetRequests()
				if len(requests) != tt.requestCount {
					t.Errorf("expected %d recorded requests, got %d", tt.requestCount, len(requests))
				}
			}

			// Check RequestID is populated
			if lastResp.RequestID == "" {
				t.Error("expected RequestID to be populated")
			}

			// Check Created timestamp is set
			if lastResp.Created.IsZero() {
				t.Error("expected Created timestamp to be set")
			}
		})
	}
}

func TestMockProvider_Stream(t *testing.T) {
	tests := []struct {
		name          string
		responses     []MockResponse
		wantError     bool
		wantContent   string
		wantFinish    llm.FinishReason
		wantToolCalls int
		wantChunks    int // minimum number of chunks expected
	}{
		{
			name: "simple text streaming",
			responses: []MockResponse{
				{
					Content:      "Hello World",
					FinishReason: llm.FinishReasonStop,
					TokenUsage: llm.TokenUsage{
						InputTokens:  10,
						OutputTokens: 5,
					},
				},
			},
			wantContent: "Hello World",
			wantFinish:  llm.FinishReasonStop,
			wantChunks:  2, // At least content chunks + final chunk
		},
		{
			name: "streaming with tool calls",
			responses: []MockResponse{
				{
					ToolCalls: []llm.ToolCall{
						{
							ID:        "call_1",
							Name:      "get_weather",
							Arguments: `{"location": "SF"}`,
						},
					},
					TokenUsage: llm.TokenUsage{
						InputTokens:  20,
						OutputTokens: 10,
					},
				},
			},
			wantFinish:    llm.FinishReasonToolCalls,
			wantToolCalls: 1,
			wantChunks:    2, // Tool call chunk + final chunk
		},
		{
			name: "streaming error",
			responses: []MockResponse{
				{
					Error: errors.New("stream error"),
				},
			},
			wantError: true,
		},
		{
			name: "empty content",
			responses: []MockResponse{
				{
					Content:      "",
					FinishReason: llm.FinishReasonStop,
				},
			},
			wantFinish: llm.FinishReasonStop,
			wantChunks: 1, // Just final chunk
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider(tt.responses...)
			ctx := context.Background()

			req := llm.CompletionRequest{
				Messages: []llm.Message{
					{Role: llm.MessageRoleUser, Content: "test prompt"},
				},
				Model: "mock-model",
			}

			chunks, err := provider.Stream(ctx, req)
			if err != nil {
				t.Fatalf("Stream() returned error: %v", err)
			}

			// Consume all chunks
			var content strings.Builder
			var finishReason llm.FinishReason
			var toolCallCount int
			var usage *llm.TokenUsage
			var streamError error
			chunkCount := 0

			for chunk := range chunks {
				chunkCount++

				if chunk.Error != nil {
					streamError = chunk.Error
					break
				}

				if chunk.Delta.Content != "" {
					content.WriteString(chunk.Delta.Content)
				}

				if chunk.Delta.ToolCallDelta != nil {
					toolCallCount++
				}

				if chunk.FinishReason != "" {
					finishReason = chunk.FinishReason
				}

				if chunk.Usage != nil {
					usage = chunk.Usage
				}
			}

			// Check error expectation
			if tt.wantError {
				if streamError == nil {
					t.Error("expected stream error, got nil")
				}
				return
			}

			if streamError != nil {
				t.Fatalf("unexpected stream error: %v", streamError)
			}

			// Check content
			if tt.wantContent != "" {
				gotContent := content.String()
				if gotContent != tt.wantContent {
					t.Errorf("expected content %q, got %q", tt.wantContent, gotContent)
				}
			}

			// Check finish reason
			if tt.wantFinish != "" && finishReason != tt.wantFinish {
				t.Errorf("expected finish reason %q, got %q", tt.wantFinish, finishReason)
			}

			// Check tool calls
			if tt.wantToolCalls > 0 && toolCallCount != tt.wantToolCalls {
				t.Errorf("expected %d tool calls, got %d", tt.wantToolCalls, toolCallCount)
			}

			// Check chunk count
			if chunkCount < tt.wantChunks {
				t.Errorf("expected at least %d chunks, got %d", tt.wantChunks, chunkCount)
			}

			// Check usage is set in final chunk
			if usage == nil {
				t.Error("expected usage to be set in final chunk")
			} else if usage.TotalTokens != usage.InputTokens+usage.OutputTokens {
				t.Errorf("TotalTokens (%d) != InputTokens (%d) + OutputTokens (%d)",
					usage.TotalTokens, usage.InputTokens, usage.OutputTokens)
			}

			// Check request was recorded
			requests := provider.GetRequests()
			if len(requests) != 1 {
				t.Errorf("expected 1 recorded request, got %d", len(requests))
			}
		})
	}
}

func TestMockProvider_StreamContext(t *testing.T) {
	provider := NewMockProvider(MockResponse{
		Content: "Hello World",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "test"},
		},
	}

	chunks, err := provider.Stream(ctx, req)
	if err != nil {
		t.Fatalf("Stream() returned error: %v", err)
	}

	// Should receive error chunk due to cancelled context
	var gotError bool
	for chunk := range chunks {
		if chunk.Error != nil {
			gotError = true
			break
		}
	}

	if !gotError {
		t.Error("expected error chunk due to cancelled context")
	}
}

func TestMockProvider_GetRequests(t *testing.T) {
	provider := NewMockProvider(
		MockResponse{Content: "first"},
		MockResponse{Content: "second"},
	)

	ctx := context.Background()

	// Make first request
	req1 := llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "prompt 1"},
		},
		Model: "model-1",
	}
	_, err := provider.Complete(ctx, req1)
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	// Make second request
	req2 := llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "prompt 2"},
		},
		Model: "model-2",
	}
	_, err = provider.Complete(ctx, req2)
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	// Get recorded requests
	requests := provider.GetRequests()

	if len(requests) != 2 {
		t.Fatalf("expected 2 recorded requests, got %d", len(requests))
	}

	// Verify first request
	if requests[0].Model != "model-1" {
		t.Errorf("expected first request model 'model-1', got '%s'", requests[0].Model)
	}
	if requests[0].Messages[0].Content != "prompt 1" {
		t.Errorf("expected first request prompt 'prompt 1', got '%s'", requests[0].Messages[0].Content)
	}

	// Verify second request
	if requests[1].Model != "model-2" {
		t.Errorf("expected second request model 'model-2', got '%s'", requests[1].Model)
	}
	if requests[1].Messages[0].Content != "prompt 2" {
		t.Errorf("expected second request prompt 'prompt 2', got '%s'", requests[1].Messages[0].Content)
	}
}

func TestMockProvider_Reset(t *testing.T) {
	provider := NewMockProvider(
		MockResponse{Content: "first"},
		MockResponse{Content: "second"},
	)

	ctx := context.Background()

	// Make a request
	req := llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "test"},
		},
	}
	_, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	// Verify request was recorded
	if len(provider.GetRequests()) != 1 {
		t.Error("expected 1 recorded request before reset")
	}

	// Reset the provider
	provider.Reset()

	// Verify reset cleared requests and index
	if len(provider.GetRequests()) != 0 {
		t.Error("expected 0 recorded requests after reset")
	}

	// Verify we can use the first response again
	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete() after reset error: %v", err)
	}

	if resp.Content != "first" {
		t.Errorf("expected first response after reset, got '%s'", resp.Content)
	}
}

func TestMockProvider_DefaultValues(t *testing.T) {
	tests := []struct {
		name         string
		response     MockResponse
		wantModel    string
		wantFinish   llm.FinishReason
		wantToolCall bool
	}{
		{
			name: "defaults for text response",
			response: MockResponse{
				Content: "test",
			},
			wantModel:  "mock-model",
			wantFinish: llm.FinishReasonStop,
		},
		{
			name: "defaults for tool call response",
			response: MockResponse{
				ToolCalls: []llm.ToolCall{
					{ID: "1", Name: "test"},
				},
			},
			wantModel:    "mock-model",
			wantFinish:   llm.FinishReasonToolCalls,
			wantToolCall: true,
		},
		{
			name: "custom model overrides default",
			response: MockResponse{
				Content: "test",
				Model:   "custom",
			},
			wantModel:  "custom",
			wantFinish: llm.FinishReasonStop,
		},
		{
			name: "explicit finish reason overrides default",
			response: MockResponse{
				Content:      "test",
				FinishReason: llm.FinishReasonLength,
			},
			wantModel:  "mock-model",
			wantFinish: llm.FinishReasonLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider(tt.response)
			ctx := context.Background()

			req := llm.CompletionRequest{
				Messages: []llm.Message{
					{Role: llm.MessageRoleUser, Content: "test"},
				},
			}

			resp, err := provider.Complete(ctx, req)
			if err != nil {
				t.Fatalf("Complete() error: %v", err)
			}

			if resp.Model != tt.wantModel {
				t.Errorf("expected model %q, got %q", tt.wantModel, resp.Model)
			}

			if resp.FinishReason != tt.wantFinish {
				t.Errorf("expected finish reason %q, got %q", tt.wantFinish, resp.FinishReason)
			}

			if tt.wantToolCall && len(resp.ToolCalls) == 0 {
				t.Error("expected tool calls, got none")
			}
		})
	}
}
