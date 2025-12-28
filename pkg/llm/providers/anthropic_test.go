package providers

import (
	"context"
	"testing"

	"github.com/tombee/conductor/pkg/llm"
)

func TestNewAnthropicProvider(t *testing.T) {
	// Test with valid API key
	provider, err := NewAnthropicProvider("test-api-key")
	if err != nil {
		t.Fatalf("failed to create provider with valid API key: %v", err)
	}
	if provider == nil {
		t.Fatal("expected provider, got nil")
	}

	// Test with empty API key
	_, err = NewAnthropicProvider("")
	if err == nil {
		t.Error("expected error with empty API key, got nil")
	}
}

func TestAnthropicProvider_Name(t *testing.T) {
	provider, _ := NewAnthropicProvider("test-api-key")
	if provider.Name() != "anthropic" {
		t.Errorf("expected provider name 'anthropic', got '%s'", provider.Name())
	}
}

func TestAnthropicProvider_Capabilities(t *testing.T) {
	provider, _ := NewAnthropicProvider("test-api-key")
	caps := provider.Capabilities()

	if !caps.Streaming {
		t.Error("expected streaming capability")
	}
	if !caps.Tools {
		t.Error("expected tools capability")
	}
	if len(caps.Models) == 0 {
		t.Error("expected at least one model")
	}

	// Verify model tiers are covered
	hasFast, hasBalanced, hasStrategic := false, false, false
	for _, model := range caps.Models {
		switch model.Tier {
		case llm.ModelTierFast:
			hasFast = true
		case llm.ModelTierBalanced:
			hasBalanced = true
		case llm.ModelTierStrategic:
			hasStrategic = true
		}
	}

	if !hasFast || !hasBalanced || !hasStrategic {
		t.Error("not all model tiers are represented")
	}
}

func TestAnthropicProvider_ResolveModel(t *testing.T) {
	provider, _ := NewAnthropicProvider("test-api-key")

	tests := []struct {
		input    string
		expected string
	}{
		{string(llm.ModelTierFast), "claude-3-5-haiku-20241022"},
		{string(llm.ModelTierBalanced), "claude-3-5-sonnet-20241022"},
		{string(llm.ModelTierStrategic), "claude-3-opus-20240229"},
		{"claude-custom-model", "claude-custom-model"},
	}

	for _, tt := range tests {
		result := provider.resolveModel(tt.input)
		if result != tt.expected {
			t.Errorf("resolveModel(%s): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

func TestAnthropicProvider_GetModelInfo(t *testing.T) {
	provider, _ := NewAnthropicProvider("test-api-key")

	// Test finding existing model
	modelInfo, err := provider.GetModelInfo("claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("failed to get model info: %v", err)
	}
	if modelInfo.Name != "Claude 3.5 Sonnet" {
		t.Errorf("expected model name 'Claude 3.5 Sonnet', got '%s'", modelInfo.Name)
	}

	// Test non-existent model
	_, err = provider.GetModelInfo("nonexistent-model")
	if err == nil {
		t.Error("expected error for non-existent model, got nil")
	}
}

func TestMockAnthropicProvider_Complete(t *testing.T) {
	mock := NewMockAnthropicProvider()

	req := llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Hello"},
		},
		Model: string(llm.ModelTierBalanced),
	}

	resp, err := mock.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("mock complete failed: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	if resp.Content == "" {
		t.Error("expected non-empty content")
	}

	if resp.RequestID == "" {
		t.Error("expected non-empty request ID")
	}

	if resp.Usage.TotalTokens == 0 {
		t.Error("expected non-zero token usage")
	}
}

func TestMockAnthropicProvider_Stream(t *testing.T) {
	mock := NewMockAnthropicProvider()

	req := llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Hello"},
		},
		Model: string(llm.ModelTierFast),
	}

	chunks, err := mock.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("mock stream failed: %v", err)
	}

	if chunks == nil {
		t.Fatal("expected chunks channel, got nil")
	}

	// Consume chunks
	chunkCount := 0
	var finalUsage *llm.TokenUsage

	for chunk := range chunks {
		chunkCount++
		if chunk.Error != nil {
			t.Errorf("unexpected error in chunk: %v", chunk.Error)
		}
		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}
	}

	if chunkCount == 0 {
		t.Error("expected at least one chunk")
	}

	if finalUsage == nil {
		t.Error("expected final chunk with usage")
	}
}

func TestMockAnthropicProvider_CustomMockFunction(t *testing.T) {
	mock := NewMockAnthropicProvider()

	// Set custom mock function
	customContent := "Custom mock response"
	mock.MockComplete = func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
		return CreateMockResponse(customContent, req.Model), nil
	}

	req := llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Hello"},
		},
		Model: "test-model",
	}

	resp, err := mock.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("custom mock complete failed: %v", err)
	}

	if resp.Content != customContent {
		t.Errorf("expected content '%s', got '%s'", customContent, resp.Content)
	}
}

func TestCreateMockResponse(t *testing.T) {
	content := "Test content"
	model := "test-model"

	resp := CreateMockResponse(content, model)

	if resp.Content != content {
		t.Errorf("expected content '%s', got '%s'", content, resp.Content)
	}

	if resp.Model != model {
		t.Errorf("expected model '%s', got '%s'", model, resp.Model)
	}

	if resp.RequestID == "" {
		t.Error("expected non-empty request ID")
	}

	if resp.FinishReason != llm.FinishReasonStop {
		t.Errorf("expected finish reason stop, got %s", resp.FinishReason)
	}
}

func TestMarshalUnmarshalToolInput(t *testing.T) {
	type TestInput struct {
		Field1 string `json:"field1"`
		Field2 int    `json:"field2"`
	}

	input := TestInput{
		Field1: "test",
		Field2: 42,
	}

	// Marshal
	jsonStr, err := MarshalToolInput(input)
	if err != nil {
		t.Fatalf("failed to marshal tool input: %v", err)
	}

	if jsonStr == "" {
		t.Error("expected non-empty JSON string")
	}

	// Unmarshal
	var output TestInput
	err = UnmarshalToolInput(jsonStr, &output)
	if err != nil {
		t.Fatalf("failed to unmarshal tool input: %v", err)
	}

	if output.Field1 != input.Field1 {
		t.Errorf("expected field1 '%s', got '%s'", input.Field1, output.Field1)
	}

	if output.Field2 != input.Field2 {
		t.Errorf("expected field2 %d, got %d", input.Field2, output.Field2)
	}
}

func TestEstimateTokens(t *testing.T) {
	messages := []llm.Message{
		{Content: "1234"},      // 1 token
		{Content: "12345678"},  // 2 tokens
		{Content: "123456789012"}, // 3 tokens
	}

	tokens := estimateTokens(messages)
	expected := (4 + 8 + 12) / 4 // = 24/4 = 6
	if tokens != expected {
		t.Errorf("expected %d tokens, got %d", expected, tokens)
	}
}

func TestAnthropicProvider_Complete_Validation(t *testing.T) {
	provider, _ := NewAnthropicProvider("test-api-key")

	// Test with empty messages
	req := llm.CompletionRequest{
		Messages: []llm.Message{},
		Model:    string(llm.ModelTierBalanced),
	}

	_, err := provider.Complete(context.Background(), req)
	if err == nil {
		t.Error("expected error with empty messages, got nil")
	}
}

func TestAnthropicProvider_Stream_Validation(t *testing.T) {
	provider, _ := NewAnthropicProvider("test-api-key")

	// Test with empty messages
	req := llm.CompletionRequest{
		Messages: []llm.Message{},
		Model:    string(llm.ModelTierFast),
	}

	chunks, err := provider.Stream(context.Background(), req)
	if err == nil {
		// Should get error in stream
		for chunk := range chunks {
			if chunk.Error == nil {
				t.Error("expected error chunk")
			}
		}
	}
}

func TestAnthropicModels_Coverage(t *testing.T) {
	// Verify all models in the list
	if len(anthropicModels) < 3 {
		t.Errorf("expected at least 3 models, got %d", len(anthropicModels))
	}

	for _, model := range anthropicModels {
		if model.ID == "" {
			t.Error("found model with empty ID")
		}
		if model.Name == "" {
			t.Error("found model with empty Name")
		}
		if !model.SupportsTools {
			t.Errorf("model %s should support tools", model.ID)
		}
		if !model.SupportsVision {
			t.Errorf("model %s should support vision", model.ID)
		}
		if model.InputPricePerMillion <= 0 {
			t.Errorf("model %s has invalid input price", model.ID)
		}
	}
}

func TestMockAnthropicProvider_EmptyMessages(t *testing.T) {
	mock := NewMockAnthropicProvider()

	req := llm.CompletionRequest{
		Messages: []llm.Message{},
		Model:    string(llm.ModelTierBalanced),
	}

	// Should still return a response (mock doesn't validate)
	resp, err := mock.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("mock failed: %v", err)
	}
	if resp == nil {
		t.Error("expected response even with empty messages")
	}
}

func TestMarshalToolInput_Error(t *testing.T) {
	// Try to marshal something that can't be marshaled
	badInput := make(chan int)

	_, err := MarshalToolInput(badInput)
	if err == nil {
		t.Error("expected error marshaling channel, got nil")
	}
}

func TestUnmarshalToolInput_Error(t *testing.T) {
	var output struct {
		Field string `json:"field"`
	}

	// Invalid JSON
	err := UnmarshalToolInput("{invalid json", &output)
	if err == nil {
		t.Error("expected error unmarshaling invalid JSON, got nil")
	}
}

func TestTransformTools(t *testing.T) {
	tests := []struct {
		name    string
		tools   []llm.Tool
		wantErr bool
	}{
		{
			name: "valid simple tool",
			tools: []llm.Tool{
				{
					Name:        "get_weather",
					Description: "Get current weather",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "City name",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid tool with nested schema",
			tools: []llm.Tool{
				{
					Name:        "complex_tool",
					Description: "Complex tool with nesting",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"level1": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"level2": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "schema too deep",
			tools: []llm.Tool{
				{
					Name:        "deep_tool",
					Description: "Tool with overly nested schema",
					InputSchema: createDeeplyNestedSchema(12),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformTools(tt.tools)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformTools() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(result) != len(tt.tools) {
					t.Errorf("expected %d tools, got %d", len(tt.tools), len(result))
				}
				for i, tool := range result {
					if tool.Name != tt.tools[i].Name {
						t.Errorf("tool %d: expected name %s, got %s", i, tt.tools[i].Name, tool.Name)
					}
				}
			}
		})
	}
}

func TestValidateSchemaDepth(t *testing.T) {
	tests := []struct {
		name     string
		schema   map[string]interface{}
		maxDepth int
		wantErr  bool
	}{
		{
			name: "simple schema within limit",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"field": map[string]interface{}{
						"type": "string",
					},
				},
			},
			maxDepth: 10,
			wantErr:  false,
		},
		{
			name:     "deeply nested schema exceeds limit",
			schema:   createDeeplyNestedSchema(11),
			maxDepth: 10,
			wantErr:  true,
		},
		{
			name:     "schema at exact limit",
			schema:   createDeeplyNestedSchema(10),
			maxDepth: 10,
			wantErr:  false,
		},
		{
			name: "array with nested objects",
			schema: map[string]interface{}{
				"type": "array",
				"items": []interface{}{
					map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"nested": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
			maxDepth: 10,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchemaDepth(tt.schema, 0, tt.maxDepth)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSchemaDepth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseToolUses(t *testing.T) {
	tests := []struct {
		name          string
		contentBlocks []interface{}
		wantCalls     int
		wantErr       bool
	}{
		{
			name: "single tool use",
			contentBlocks: []interface{}{
				map[string]interface{}{
					"type":  "tool_use",
					"id":    "call_123",
					"name":  "get_weather",
					"input": map[string]interface{}{"location": "London"},
				},
			},
			wantCalls: 1,
			wantErr:   false,
		},
		{
			name: "multiple tool uses",
			contentBlocks: []interface{}{
				map[string]interface{}{
					"type":  "tool_use",
					"id":    "call_1",
					"name":  "tool1",
					"input": map[string]interface{}{"param": "value1"},
				},
				map[string]interface{}{
					"type":  "tool_use",
					"id":    "call_2",
					"name":  "tool2",
					"input": map[string]interface{}{"param": "value2"},
				},
			},
			wantCalls: 2,
			wantErr:   false,
		},
		{
			name: "mixed content blocks",
			contentBlocks: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Some text",
				},
				map[string]interface{}{
					"type":  "tool_use",
					"id":    "call_123",
					"name":  "get_weather",
					"input": map[string]interface{}{"location": "Paris"},
				},
			},
			wantCalls: 1,
			wantErr:   false,
		},
		{
			name: "no tool uses",
			contentBlocks: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Just text",
				},
			},
			wantCalls: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, err := parseToolUses(tt.contentBlocks)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseToolUses() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(calls) != tt.wantCalls {
				t.Errorf("parseToolUses() got %d calls, want %d", len(calls), tt.wantCalls)
			}
			for _, call := range calls {
				if call.ID == "" {
					t.Error("tool call missing ID")
				}
				if call.Name == "" {
					t.Error("tool call missing Name")
				}
				if call.Arguments == "" {
					t.Error("tool call missing Arguments")
				}
			}
		})
	}
}

func TestValidateToolArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		wantErr   bool
	}{
		{
			name:      "valid JSON object",
			arguments: `{"key": "value"}`,
			wantErr:   false,
		},
		{
			name:      "valid JSON array",
			arguments: `["value1", "value2"]`,
			wantErr:   false,
		},
		{
			name:      "valid empty object",
			arguments: `{}`,
			wantErr:   false,
		},
		{
			name:      "invalid JSON",
			arguments: `{invalid}`,
			wantErr:   true,
		},
		{
			name:      "incomplete JSON",
			arguments: `{"key":`,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateToolArguments(tt.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateToolArguments() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function to create deeply nested schema for testing
func createDeeplyNestedSchema(depth int) map[string]interface{} {
	if depth <= 1 {
		return map[string]interface{}{
			"type": "string",
		}
	}
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"nested": createDeeplyNestedSchema(depth - 1),
		},
	}
}
