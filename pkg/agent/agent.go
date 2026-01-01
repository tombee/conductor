// Package agent provides an LLM-powered agent that can use tools to accomplish tasks.
//
// The agent runs a loop that:
// 1. Sends a prompt to an LLM
// 2. Receives a response (which may include tool calls)
// 3. Executes requested tools
// 4. Feeds tool results back to the LLM
// 5. Repeats until the LLM indicates completion
//
// This implements the ReAct (Reasoning + Acting) pattern for LLM agents.
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/tombee/conductor/pkg/tools"
)

// Agent represents an LLM-powered agent that can use tools.
type Agent struct {
	// llm is the language model provider
	llm LLMProvider

	// registry provides access to available tools
	registry *tools.Registry

	// maxIterations limits the number of loop iterations
	maxIterations int

	// contextManager tracks token usage and manages context window
	contextManager *ContextManager

	// streamHandler receives streaming events (optional)
	streamHandler StreamHandler
}

// LLMProvider defines the interface for LLM interactions.
type LLMProvider interface {
	// Complete makes a synchronous LLM call
	Complete(ctx context.Context, messages []Message) (*Response, error)

	// Stream makes a streaming LLM call
	Stream(ctx context.Context, messages []Message) (<-chan StreamEvent, error)
}

// Message represents a message in the conversation.
type Message struct {
	// Role is the message sender (system, user, assistant, tool)
	Role string `json:"role"`

	// Content is the message text
	Content string `json:"content"`

	// ToolCalls are tool invocations requested by the assistant (optional)
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID links a tool result to its corresponding call (optional)
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCall represents a request to execute a tool.
type ToolCall struct {
	// ID is a unique identifier for this tool call
	ID string `json:"id"`

	// Name is the tool to execute
	Name string `json:"name"`

	// Arguments are the tool inputs (JSON string or map)
	Arguments interface{} `json:"arguments"`
}

// Response represents an LLM response.
type Response struct {
	// Content is the text response
	Content string

	// ToolCalls are tools the LLM wants to execute
	ToolCalls []ToolCall

	// FinishReason indicates why the response ended
	FinishReason string

	// Usage tracks token consumption
	Usage TokenUsage
}

// TokenUsage tracks token consumption for a request.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// StreamEvent represents a streaming event from the LLM.
type StreamEvent struct {
	// Type is the event type (text_delta, tool_use_start, etc.)
	Type string

	// Content is the event data
	Content interface{}
}

// StreamHandler receives streaming events.
type StreamHandler func(event StreamEvent)

// Result represents the final result of an agent execution.
type Result struct {
	// Success indicates if the task completed successfully
	Success bool

	// FinalResponse is the agent's final text response
	FinalResponse string

	// ToolExecutions is a log of all tool calls made
	ToolExecutions []ToolExecution

	// Iterations is the number of loop iterations
	Iterations int

	// TokensUsed tracks total token consumption
	TokensUsed TokenUsage

	// Duration is the total execution time
	Duration time.Duration

	// Error contains error information if the agent failed
	Error string
}

// ToolExecution records a single tool execution.
type ToolExecution struct {
	// ToolName is the name of the tool
	ToolName string

	// Inputs are the tool inputs
	Inputs map[string]interface{}

	// Outputs are the tool outputs
	Outputs map[string]interface{}

	// Success indicates if the tool succeeded
	Success bool

	// Error contains error information if the tool failed
	Error string

	// Duration is how long the tool took to execute
	Duration time.Duration
}

// NewAgent creates a new agent.
func NewAgent(llm LLMProvider, registry *tools.Registry) *Agent {
	return &Agent{
		llm:            llm,
		registry:       registry,
		maxIterations:  20,                        // Default max iterations
		contextManager: NewContextManager(100000), // 100k token context window
	}
}

// WithMaxIterations sets the maximum number of loop iterations.
func (a *Agent) WithMaxIterations(max int) *Agent {
	a.maxIterations = max
	return a
}

// WithStreamHandler sets a handler for streaming events.
func (a *Agent) WithStreamHandler(handler StreamHandler) *Agent {
	a.streamHandler = handler
	return a
}

// Run executes the agent loop.
func (a *Agent) Run(ctx context.Context, systemPrompt string, userPrompt string) (*Result, error) {
	startTime := time.Now()
	result := &Result{
		ToolExecutions: []ToolExecution{},
	}

	// Initialize conversation with system and user messages
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Main agent loop
	for iteration := 1; iteration <= a.maxIterations; iteration++ {
		result.Iterations = iteration

		// Call LLM
		response, err := a.llm.Complete(ctx, messages)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("LLM call failed: %v", err)
			result.Duration = time.Since(startTime)
			return result, fmt.Errorf("LLM call failed: %w", err)
		}

		// Track token usage
		result.TokensUsed.InputTokens += response.Usage.InputTokens
		result.TokensUsed.OutputTokens += response.Usage.OutputTokens
		result.TokensUsed.TotalTokens += response.Usage.TotalTokens

		// Add assistant message to conversation
		assistantMsg := Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		}
		messages = append(messages, assistantMsg)

		// Check if agent is done
		if response.FinishReason == "stop" && len(response.ToolCalls) == 0 {
			result.Success = true
			result.FinalResponse = response.Content
			result.Duration = time.Since(startTime)
			return result, nil
		}

		// Execute tool calls if any
		if len(response.ToolCalls) > 0 {
			for _, toolCall := range response.ToolCalls {
				execution := a.executeTool(ctx, toolCall)
				result.ToolExecutions = append(result.ToolExecutions, execution)

				// Add tool result to conversation
				toolMsg := Message{
					Role:       "tool",
					Content:    formatToolResult(execution),
					ToolCallID: toolCall.ID,
				}
				messages = append(messages, toolMsg)
			}
		}

		// Check context window and prune if needed
		if a.contextManager.ShouldPrune(messages) {
			messages = a.contextManager.Prune(messages)
		}
	}

	// Max iterations reached
	result.Success = false
	result.Error = fmt.Sprintf("max iterations (%d) reached without completion", a.maxIterations)
	result.Duration = time.Since(startTime)
	return result, fmt.Errorf("max iterations reached")
}

// executeTool executes a single tool call.
func (a *Agent) executeTool(ctx context.Context, toolCall ToolCall) ToolExecution {
	startTime := time.Now()
	execution := ToolExecution{
		ToolName: toolCall.Name,
	}

	// Parse arguments
	var inputs map[string]interface{}
	switch args := toolCall.Arguments.(type) {
	case map[string]interface{}:
		inputs = args
	case string:
		// Parse JSON string
		// Phase 1: Simple passthrough, future: actual JSON parsing
		inputs = map[string]interface{}{
			"raw": args,
		}
	default:
		execution.Success = false
		execution.Error = "invalid tool arguments format"
		execution.Duration = time.Since(startTime)
		return execution
	}

	execution.Inputs = inputs

	// Execute tool
	outputs, err := a.registry.Execute(ctx, toolCall.Name, inputs)
	execution.Duration = time.Since(startTime)

	if err != nil {
		execution.Success = false
		execution.Error = err.Error()
		return execution
	}

	execution.Success = true
	execution.Outputs = outputs
	return execution
}

// formatToolResult formats a tool execution result for inclusion in the conversation.
func formatToolResult(execution ToolExecution) string {
	if !execution.Success {
		return fmt.Sprintf("Error executing %s: %s", execution.ToolName, execution.Error)
	}

	// Format outputs as a simple string
	// Phase 1: Basic formatting, future: structured JSON
	result := fmt.Sprintf("Tool %s completed successfully", execution.ToolName)
	if len(execution.Outputs) > 0 {
		result += fmt.Sprintf(": %v", execution.Outputs)
	}
	return result
}
