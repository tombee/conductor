package sdk

import (
	"context"
	"time"

	"github.com/tombee/conductor/pkg/llm"
)

// EventType identifies the type of event.
type EventType string

const (
	EventWorkflowStarted   EventType = "workflow.started"
	EventWorkflowCompleted EventType = "workflow.completed"
	EventWorkflowFailed    EventType = "workflow.failed"
	EventStepStarted       EventType = "step.started"
	EventStepCompleted     EventType = "step.completed"
	EventStepFailed        EventType = "step.failed"
	EventLLMToken          EventType = "llm.token"         // Streaming token
	EventLLMToolCall       EventType = "llm.tool_call"     // Tool invocation
	EventLLMToolResult     EventType = "llm.tool_result"   // Tool result
	EventAgentIteration    EventType = "agent.iteration"   // Agent loop iteration
	EventAgentToolCall     EventType = "agent.tool_call"   // Agent tool invocation
	EventAgentToolResult   EventType = "agent.tool_result" // Agent tool result
	EventAgentComplete     EventType = "agent.complete"    // Agent completion
	EventTokenUpdate       EventType = "token.update"      // Token usage update
	EventToolOutput        EventType = "tool.output"       // Streaming tool output chunk
)

// Event represents a workflow event.
type Event struct {
	Type       EventType
	Timestamp  time.Time
	WorkflowID string
	StepID     string
	Data       any // Type depends on EventType
}

// EventHandler processes events.
// Handlers are called synchronously during workflow execution.
// If a handler panics, the panic is recovered and logged.
type EventHandler func(ctx context.Context, event *Event)

// TokenEvent is the Data for EventLLMToken.
type TokenEvent struct {
	Token string
	Index int
}

// StepCompletedEvent is the Data for EventStepCompleted.
type StepCompletedEvent struct {
	Output   map[string]any
	Duration time.Duration
	Tokens   TokenUsage
}

// ToolCallEvent is the Data for EventLLMToolCall.
type ToolCallEvent struct {
	ToolCallID string
	ToolName   string
	Inputs     map[string]any
}

// ToolResultEvent is the Data for EventLLMToolResult.
type ToolResultEvent struct {
	ToolCallID string
	ToolName   string
	Outputs    map[string]any
	Error      error
}

// AgentIterationEvent is the Data for EventAgentIteration.
type AgentIterationEvent struct {
	Iteration           int
	State               string            // Current agent state (thinking, acting, responding)
	ToolCalls           []ToolCallSummary // Tools called in this iteration
	TokensThisIteration int               // Tokens used in this iteration
}

// ToolCallSummary provides a summary of a tool call for event reporting.
type ToolCallSummary struct {
	Name  string         // Tool name
	Input map[string]any // Tool input parameters
}

// AgentToolCallEvent is the Data for EventAgentToolCall.
type AgentToolCallEvent struct {
	Tool  string // Tool name
	Input any    // Tool input (typically map[string]any)
}

// AgentToolResultEvent is the Data for EventAgentToolResult.
type AgentToolResultEvent struct {
	Tool       string // Tool name
	Status     string // "success" or "error"
	Output     any    // Tool output
	DurationMs int    // Execution duration in milliseconds
}

// AgentCompleteEvent is the Data for EventAgentComplete.
type AgentCompleteEvent struct {
	Status     string // "completed", "limit_exceeded", or "error"
	Reason     string // Reason for completion (e.g., "task_completed", "max_iterations", "token_limit")
	Iterations int    // Number of iterations completed
	TokensUsed int    // Total tokens consumed
}

// TokenUsage tracks token consumption for cost calculation.
// This is an SDK-level type that mirrors pkg/llm.TokenUsage.
type TokenUsage struct {
	InputTokens         int
	OutputTokens        int
	TotalTokens         int
	CacheCreationTokens int
	CacheReadTokens     int
}

// fromLLMTokenUsage converts pkg/llm.TokenUsage to sdk.TokenUsage
func fromLLMTokenUsage(usage llm.TokenUsage) TokenUsage {
	return TokenUsage{
		InputTokens:         usage.InputTokens,
		OutputTokens:        usage.OutputTokens,
		TotalTokens:         usage.TotalTokens,
		CacheCreationTokens: usage.CacheCreationTokens,
		CacheReadTokens:     usage.CacheReadTokens,
	}
}

// ToolOutputEvent is the Data for EventToolOutput.
// It represents a streaming output chunk from a tool execution.
type ToolOutputEvent struct {
	ToolCallID string         // Links to the tool call
	ToolName   string         // Name of the tool
	Stream     string         // "stdout", "stderr", or ""
	Data       string         // Chunk content
	IsFinal    bool           // True for the final chunk
	Metadata   map[string]any // Optional metadata
}
