package integration

import (
	"time"

	"github.com/tombee/conductor/pkg/llm"
)

// SimpleWorkflowDefinition returns a minimal workflow definition for testing.
func SimpleWorkflowDefinition() string {
	return `
name: test-workflow
description: Simple workflow for integration tests
steps:
  - name: greet
    prompt: "Say hello"
`
}

// MultiStepWorkflowDefinition returns a workflow with multiple steps for testing.
func MultiStepWorkflowDefinition() string {
	return `
name: multi-step-test
description: Multi-step workflow for integration tests
steps:
  - name: step1
    prompt: "First step"
  - name: step2
    prompt: "Second step"
  - name: step3
    prompt: "Third step"
`
}

// SimpleMessage creates a basic user message for LLM testing.
func SimpleMessage(content string) llm.Message {
	return llm.Message{
		Role:    llm.MessageRoleUser,
		Content: content,
	}
}

// SimpleCompletionRequest creates a basic completion request for testing.
func SimpleCompletionRequest(model string, content string) llm.CompletionRequest {
	return llm.CompletionRequest{
		Messages: []llm.Message{SimpleMessage(content)},
		Model:    model,
	}
}

// StreamingCompletionRequest creates a completion request suitable for streaming tests.
func StreamingCompletionRequest(model string, content string) llm.CompletionRequest {
	maxTokens := 100
	return llm.CompletionRequest{
		Messages:  []llm.Message{SimpleMessage(content)},
		Model:     model,
		MaxTokens: &maxTokens,
	}
}

// ToolCallingRequest creates a completion request with tool definitions for testing.
func ToolCallingRequest(model string, content string, tools []llm.Tool) llm.CompletionRequest {
	return llm.CompletionRequest{
		Messages: []llm.Message{SimpleMessage(content)},
		Model:    model,
		Tools:    tools,
	}
}

// SimpleTool creates a basic tool definition for testing.
func SimpleTool(name, description string) llm.Tool {
	return llm.Tool{
		Name:        name,
		Description: description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "The input parameter",
				},
			},
			"required": []string{"input"},
		},
	}
}

// MockTokenUsage creates a TokenUsage struct for testing.
func MockTokenUsage(prompt, completion int) llm.TokenUsage {
	return llm.TokenUsage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}
}

// MockCompletionResponse creates a mock CompletionResponse for testing.
func MockCompletionResponse(content, model string, usage llm.TokenUsage) *llm.CompletionResponse {
	return &llm.CompletionResponse{
		Content:      content,
		FinishReason: llm.FinishReasonStop,
		Usage:        usage,
		Model:        model,
		RequestID:    "test-request-id",
		Created:      time.Now(),
	}
}

// CalculatorTool returns a realistic tool definition for calculator testing.
func CalculatorTool() llm.Tool {
	return llm.Tool{
		Name:        "calculator",
		Description: "Performs basic arithmetic operations",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "The operation to perform",
					"enum":        []string{"add", "subtract", "multiply", "divide"},
				},
				"a": map[string]interface{}{
					"type":        "number",
					"description": "First operand",
				},
				"b": map[string]interface{}{
					"type":        "number",
					"description": "Second operand",
				},
			},
			"required": []string{"operation", "a", "b"},
		},
	}
}
