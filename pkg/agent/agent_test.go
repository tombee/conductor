package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tombee/conductor/pkg/tools"
)

// mockLLMProvider is a simple LLM provider for testing
type mockLLMProvider struct {
	responses []Response
	callCount int
}

func (m *mockLLMProvider) Complete(ctx context.Context, messages []Message) (*Response, error) {
	if m.callCount >= len(m.responses) {
		return nil, errors.New("no more responses available")
	}
	response := m.responses[m.callCount]
	m.callCount++
	return &response, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
	return nil, errors.New("streaming not implemented in mock")
}

// mockTool is a simple tool for testing
type mockTool struct {
	name      string
	executed  bool
	output    map[string]interface{}
	shouldErr bool
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return "A mock tool"
}

func (m *mockTool) Schema() *tools.Schema {
	return &tools.Schema{
		Inputs: &tools.ParameterSchema{
			Type: "object",
		},
		Outputs: &tools.ParameterSchema{
			Type: "object",
		},
	}
}

func (m *mockTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	m.executed = true
	if m.shouldErr {
		return nil, errors.New("tool execution failed")
	}
	return m.output, nil
}

func TestNewAgent(t *testing.T) {
	registry := tools.NewRegistry()
	llm := &mockLLMProvider{}

	agent := NewAgent(llm, registry)
	if agent == nil {
		t.Fatal("NewAgent() returned nil")
	}

	if agent.maxIterations != 20 {
		t.Errorf("Default maxIterations = %d, want 20", agent.maxIterations)
	}
}

func TestAgent_WithMaxIterations(t *testing.T) {
	registry := tools.NewRegistry()
	llm := &mockLLMProvider{}

	agent := NewAgent(llm, registry).WithMaxIterations(5)
	if agent.maxIterations != 5 {
		t.Errorf("maxIterations = %d, want 5", agent.maxIterations)
	}
}

func TestAgent_RunSimpleCompletion(t *testing.T) {
	registry := tools.NewRegistry()
	llm := &mockLLMProvider{
		responses: []Response{
			{
				Content:      "Task completed successfully",
				FinishReason: "stop",
				Usage: TokenUsage{
					InputTokens:  10,
					OutputTokens: 5,
					TotalTokens:  15,
				},
			},
		},
	}

	agent := NewAgent(llm, registry)
	ctx := context.Background()

	result, err := agent.Run(ctx, "You are a helpful assistant", "Complete this task")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !result.Success {
		t.Errorf("Result.Success = false, want true")
	}

	if result.FinalResponse != "Task completed successfully" {
		t.Errorf("FinalResponse = %q, want %q", result.FinalResponse, "Task completed successfully")
	}

	if result.Iterations != 1 {
		t.Errorf("Iterations = %d, want 1", result.Iterations)
	}

	if result.TokensUsed.TotalTokens != 15 {
		t.Errorf("TokensUsed = %d, want 15", result.TokensUsed.TotalTokens)
	}
}

func TestAgent_RunWithToolCall(t *testing.T) {
	// Create a mock tool
	tool := &mockTool{
		name:   "test-tool",
		output: map[string]interface{}{"result": "tool output"},
	}

	registry := tools.NewRegistry()
	if err := registry.Register(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// LLM first requests a tool call, then completes
	llm := &mockLLMProvider{
		responses: []Response{
			{
				Content:      "I need to use a tool",
				FinishReason: "tool_calls",
				ToolCalls: []ToolCall{
					{
						ID:        "call-1",
						Name:      "test-tool",
						Arguments: map[string]interface{}{"input": "test"},
					},
				},
				Usage: TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			},
			{
				Content:      "Task completed with tool result",
				FinishReason: "stop",
				Usage:        TokenUsage{InputTokens: 15, OutputTokens: 10, TotalTokens: 25},
			},
		},
	}

	agent := NewAgent(llm, registry)
	ctx := context.Background()

	result, err := agent.Run(ctx, "You are a helpful assistant", "Use the tool")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !result.Success {
		t.Errorf("Result.Success = false, want true")
	}

	if result.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2", result.Iterations)
	}

	if len(result.ToolExecutions) != 1 {
		t.Errorf("ToolExecutions count = %d, want 1", len(result.ToolExecutions))
	}

	if !tool.executed {
		t.Error("Tool was not executed")
	}

	execution := result.ToolExecutions[0]
	if execution.ToolName != "test-tool" {
		t.Errorf("ToolName = %s, want test-tool", execution.ToolName)
	}

	if !execution.Success {
		t.Errorf("Tool execution failed: %s", execution.Error)
	}
}

func TestAgent_MaxIterationsReached(t *testing.T) {
	// LLM never returns "stop", causing max iterations to be reached
	llm := &mockLLMProvider{
		responses: []Response{
			{Content: "Still working...", FinishReason: "length", Usage: TokenUsage{TotalTokens: 10}},
			{Content: "Still working...", FinishReason: "length", Usage: TokenUsage{TotalTokens: 10}},
			{Content: "Still working...", FinishReason: "length", Usage: TokenUsage{TotalTokens: 10}},
		},
	}

	registry := tools.NewRegistry()
	agent := NewAgent(llm, registry).WithMaxIterations(3)
	ctx := context.Background()

	result, err := agent.Run(ctx, "System", "Task")
	if err == nil {
		t.Error("Run() should return error when max iterations reached")
	}

	if result.Success {
		t.Error("Result.Success should be false when max iterations reached")
	}

	if result.Iterations != 3 {
		t.Errorf("Iterations = %d, want 3", result.Iterations)
	}
}

func TestAgent_ToolExecutionFailure(t *testing.T) {
	// Create a tool that fails
	tool := &mockTool{
		name:      "failing-tool",
		shouldErr: true,
	}

	registry := tools.NewRegistry()
	if err := registry.Register(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	llm := &mockLLMProvider{
		responses: []Response{
			{
				Content:      "Using tool",
				FinishReason: "tool_calls",
				ToolCalls: []ToolCall{
					{
						ID:        "call-1",
						Name:      "failing-tool",
						Arguments: map[string]interface{}{},
					},
				},
				Usage: TokenUsage{TotalTokens: 10},
			},
			{
				Content:      "Completed despite tool failure",
				FinishReason: "stop",
				Usage:        TokenUsage{TotalTokens: 10},
			},
		},
	}

	agent := NewAgent(llm, registry)
	ctx := context.Background()

	result, err := agent.Run(ctx, "System", "Task")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(result.ToolExecutions) != 1 {
		t.Fatalf("ToolExecutions count = %d, want 1", len(result.ToolExecutions))
	}

	execution := result.ToolExecutions[0]
	if execution.Success {
		t.Error("Tool execution should have failed")
	}

	if execution.Error == "" {
		t.Error("Tool execution should have error message")
	}
}

func TestAgent_WithStreamHandler(t *testing.T) {
	registry := tools.NewRegistry()
	llm := &mockLLMProvider{}

	streamCalled := false
	agent := NewAgent(llm, registry).WithStreamHandler(func(event StreamEvent) {
		streamCalled = true
	})

	if agent.streamHandler == nil {
		t.Error("StreamHandler should be set")
	}

	// Call the handler to verify it works
	agent.streamHandler(StreamEvent{Type: "test", Content: "data"})
	if !streamCalled {
		t.Error("Stream handler was not called")
	}
}

func TestAgent_LLMCallFailure(t *testing.T) {
	registry := tools.NewRegistry()
	llm := &mockLLMProvider{
		responses: []Response{}, // No responses, will cause error
	}

	agent := NewAgent(llm, registry)
	ctx := context.Background()

	result, err := agent.Run(ctx, "System", "Task")
	if err == nil {
		t.Error("Run() should return error when LLM call fails")
	}

	if result.Success {
		t.Error("Result.Success should be false when LLM fails")
	}

	if result.Error == "" {
		t.Error("Result.Error should contain error message")
	}
}

func TestAgent_ContextPruning(t *testing.T) {
	registry := tools.NewRegistry()

	// Create responses that will trigger context pruning
	var responses []Response
	for i := 0; i < 10; i++ {
		responses = append(responses, Response{
			Content:      strings.Repeat("a", 1000), // Large content to trigger pruning
			FinishReason: "length",
			Usage:        TokenUsage{TotalTokens: 100},
		})
	}
	// Final response to complete
	responses = append(responses, Response{
		Content:      "Done",
		FinishReason: "stop",
		Usage:        TokenUsage{TotalTokens: 10},
	})

	llm := &mockLLMProvider{responses: responses}

	// Create agent with small context window to force pruning
	agent := NewAgent(llm, registry)
	agent.contextManager = NewContextManager(500) // Small context window
	agent.maxIterations = 15

	ctx := context.Background()
	result, err := agent.Run(ctx, "System", "Task")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !result.Success {
		t.Error("Result.Success should be true")
	}
}

func TestAgent_InvalidToolArguments(t *testing.T) {
	tool := &mockTool{
		name:   "test-tool",
		output: map[string]interface{}{"result": "success"},
	}

	registry := tools.NewRegistry()
	if err := registry.Register(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	tests := []struct {
		name      string
		arguments interface{}
		wantError bool
	}{
		{
			name:      "valid map arguments",
			arguments: map[string]interface{}{"input": "test"},
			wantError: false,
		},
		{
			name:      "string arguments",
			arguments: "raw string",
			wantError: false,
		},
		{
			name:      "invalid arguments type",
			arguments: 123,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llm := &mockLLMProvider{
				responses: []Response{
					{
						Content:      "Using tool",
						FinishReason: "tool_calls",
						ToolCalls: []ToolCall{
							{
								ID:        "call-1",
								Name:      "test-tool",
								Arguments: tt.arguments,
							},
						},
						Usage: TokenUsage{TotalTokens: 10},
					},
					{
						Content:      "Completed",
						FinishReason: "stop",
						Usage:        TokenUsage{TotalTokens: 10},
					},
				},
			}

			agent := NewAgent(llm, registry)
			ctx := context.Background()

			result, err := agent.Run(ctx, "System", "Task")
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			if len(result.ToolExecutions) != 1 {
				t.Fatalf("ToolExecutions count = %d, want 1", len(result.ToolExecutions))
			}

			execution := result.ToolExecutions[0]
			if tt.wantError && execution.Success {
				t.Error("Tool execution should have failed for invalid arguments")
			}
			if !tt.wantError && !execution.Success {
				t.Errorf("Tool execution should have succeeded: %v", execution.Error)
			}
		})
	}
}

func TestAgent_MultipleToolCalls(t *testing.T) {
	// Create multiple tools
	tool1 := &mockTool{
		name:   "tool-1",
		output: map[string]interface{}{"result": "output1"},
	}
	tool2 := &mockTool{
		name:   "tool-2",
		output: map[string]interface{}{"result": "output2"},
	}

	registry := tools.NewRegistry()
	registry.Register(tool1)
	registry.Register(tool2)

	llm := &mockLLMProvider{
		responses: []Response{
			{
				Content:      "Using multiple tools",
				FinishReason: "tool_calls",
				ToolCalls: []ToolCall{
					{
						ID:        "call-1",
						Name:      "tool-1",
						Arguments: map[string]interface{}{},
					},
					{
						ID:        "call-2",
						Name:      "tool-2",
						Arguments: map[string]interface{}{},
					},
				},
				Usage: TokenUsage{TotalTokens: 10},
			},
			{
				Content:      "Completed with both tools",
				FinishReason: "stop",
				Usage:        TokenUsage{TotalTokens: 10},
			},
		},
	}

	agent := NewAgent(llm, registry)
	ctx := context.Background()

	result, err := agent.Run(ctx, "System", "Task")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(result.ToolExecutions) != 2 {
		t.Fatalf("ToolExecutions count = %d, want 2", len(result.ToolExecutions))
	}

	if !tool1.executed {
		t.Error("Tool 1 was not executed")
	}
	if !tool2.executed {
		t.Error("Tool 2 was not executed")
	}
}

func TestAgent_StreamingExecution(t *testing.T) {
	registry := tools.NewRegistry()

	streamingLLM := &mockStreamingLLMProvider{
		streamEvents: []StreamEvent{
			{Type: "text_delta", Content: "Hello"},
			{Type: "text_delta", Content: " World"},
			{Type: "message_end", Content: nil},
		},
	}

	var capturedEvents []StreamEvent
	agent := NewAgent(streamingLLM, registry).WithStreamHandler(func(event StreamEvent) {
		capturedEvents = append(capturedEvents, event)
	})

	// For this test, we'll simulate streaming by calling the stream handler
	// in the mock provider
	if agent.streamHandler != nil {
		for _, event := range streamingLLM.streamEvents {
			agent.streamHandler(event)
		}
	}

	if len(capturedEvents) != 3 {
		t.Errorf("Captured %d events, want 3", len(capturedEvents))
	}
}

// mockStreamingLLMProvider extends mock to support streaming
type mockStreamingLLMProvider struct {
	streamEvents []StreamEvent
	responses    []Response
	callCount    int
}

func (m *mockStreamingLLMProvider) Complete(ctx context.Context, messages []Message) (*Response, error) {
	if m.callCount >= len(m.responses) {
		return &Response{
			Content:      "Streaming completed",
			FinishReason: "stop",
			Usage:        TokenUsage{TotalTokens: 10},
		}, nil
	}
	response := m.responses[m.callCount]
	m.callCount++
	return &response, nil
}

func (m *mockStreamingLLMProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, len(m.streamEvents))
	for _, event := range m.streamEvents {
		ch <- event
	}
	close(ch)
	return ch, nil
}

// mockStreamingTool implements StreamingTool for testing
type mockStreamingTool struct {
	name   string
	chunks []tools.ToolChunk
}

func (m *mockStreamingTool) Name() string {
	return m.name
}

func (m *mockStreamingTool) Description() string {
	return "A mock streaming tool"
}

func (m *mockStreamingTool) Schema() *tools.Schema {
	return &tools.Schema{
		Inputs: &tools.ParameterSchema{
			Type: "object",
		},
		Outputs: &tools.ParameterSchema{
			Type: "object",
		},
	}
}

func (m *mockStreamingTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	// For non-streaming execution, collect all chunks and return the final result
	ch, err := m.ExecuteStream(ctx, inputs)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	var execError error
	for chunk := range ch {
		if chunk.IsFinal {
			result = chunk.Result
			execError = chunk.Error
		}
	}

	if execError != nil {
		return nil, execError
	}
	return result, nil
}

func (m *mockStreamingTool) ExecuteStream(ctx context.Context, inputs map[string]interface{}) (<-chan tools.ToolChunk, error) {
	ch := make(chan tools.ToolChunk, len(m.chunks))
	go func() {
		defer close(ch)
		for _, chunk := range m.chunks {
			ch <- chunk
		}
	}()
	return ch, nil
}

func TestAgent_ToolStreamingExecution(t *testing.T) {
	// Create a streaming tool that emits chunks
	streamingTool := &mockStreamingTool{
		name: "streaming-tool",
		chunks: []tools.ToolChunk{
			{
				Data:   "Line 1\n",
				Stream: "stdout",
			},
			{
				Data:   "Line 2\n",
				Stream: "stdout",
			},
			{
				Data:   "Error message\n",
				Stream: "stderr",
			},
			{
				IsFinal: true,
				Result: map[string]interface{}{
					"exit_code": 0,
					"duration":  100,
				},
			},
		},
	}

	registry := tools.NewRegistry()
	if err := registry.Register(streamingTool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	llm := &mockLLMProvider{
		responses: []Response{
			{
				Content:      "Using streaming tool",
				FinishReason: "tool_calls",
				ToolCalls: []ToolCall{
					{
						ID:        "call-1",
						Name:      "streaming-tool",
						Arguments: map[string]interface{}{},
					},
				},
				Usage: TokenUsage{TotalTokens: 10},
			},
			{
				Content:      "Completed with streaming output",
				FinishReason: "stop",
				Usage:        TokenUsage{TotalTokens: 10},
			},
		},
	}

	// Track events emitted via callback
	var capturedEvents []map[string]interface{}
	agent := NewAgent(llm, registry).WithEventCallback(func(eventType string, data interface{}) {
		if eventType == "tool.output" {
			if eventData, ok := data.(map[string]interface{}); ok {
				capturedEvents = append(capturedEvents, eventData)
			}
		}
	})

	ctx := context.Background()
	result, err := agent.Run(ctx, "System", "Task")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify tool execution
	if len(result.ToolExecutions) != 1 {
		t.Fatalf("ToolExecutions count = %d, want 1", len(result.ToolExecutions))
	}

	execution := result.ToolExecutions[0]

	// Verify output chunks are captured in execution
	if len(execution.OutputChunks) != 4 {
		t.Errorf("OutputChunks count = %d, want 4", len(execution.OutputChunks))
	}

	// Verify chunk content
	if execution.OutputChunks[0].Data != "Line 1\n" {
		t.Errorf("Chunk 0 data = %q, want %q", execution.OutputChunks[0].Data, "Line 1\n")
	}
	if execution.OutputChunks[0].Stream != "stdout" {
		t.Errorf("Chunk 0 stream = %q, want %q", execution.OutputChunks[0].Stream, "stdout")
	}

	if execution.OutputChunks[2].Stream != "stderr" {
		t.Errorf("Chunk 2 stream = %q, want %q", execution.OutputChunks[2].Stream, "stderr")
	}

	// Verify final chunk
	if !execution.OutputChunks[3].IsFinal {
		t.Error("Last chunk should have IsFinal=true")
	}

	// Verify events were emitted
	if len(capturedEvents) != 4 {
		t.Errorf("Captured %d events, want 4", len(capturedEvents))
	}

	// Verify event structure
	if len(capturedEvents) > 0 {
		firstEvent := capturedEvents[0]
		if firstEvent["tool_name"] != "streaming-tool" {
			t.Errorf("Event tool_name = %q, want %q", firstEvent["tool_name"], "streaming-tool")
		}
		if firstEvent["tool_call_id"] != "call-1" {
			t.Errorf("Event tool_call_id = %q, want %q", firstEvent["tool_call_id"], "call-1")
		}
		if firstEvent["data"] != "Line 1\n" {
			t.Errorf("Event data = %q, want %q", firstEvent["data"], "Line 1\n")
		}
	}

	// Verify execution succeeded
	if !execution.Success {
		t.Error("Tool execution should have succeeded")
	}

	// Verify final result
	if execution.Outputs == nil {
		t.Error("Execution outputs should not be nil")
	}
	if exitCode, ok := execution.Outputs["exit_code"].(int); !ok || exitCode != 0 {
		t.Errorf("Exit code = %v, want 0", execution.Outputs["exit_code"])
	}
}
