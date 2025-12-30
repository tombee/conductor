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
	EventLLMToken          EventType = "llm.token"       // Streaming token
	EventLLMToolCall       EventType = "llm.tool_call"   // Tool invocation
	EventLLMToolResult     EventType = "llm.tool_result" // Tool result
	EventAgentIteration    EventType = "agent.iteration" // Agent loop iteration
	EventTokenUpdate       EventType = "token.update"    // Token usage update
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
	Iteration int
	State     string // Current agent state (thinking, acting, responding)
}

// TokenUsage tracks token consumption for cost calculation.
// This is an SDK-level type that mirrors pkg/llm.TokenUsage.
type TokenUsage struct {
	PromptTokens        int
	CompletionTokens    int
	TotalTokens         int
	CacheCreationTokens int
	CacheReadTokens     int
}

// fromLLMTokenUsage converts pkg/llm.TokenUsage to sdk.TokenUsage
func fromLLMTokenUsage(usage llm.TokenUsage) TokenUsage {
	return TokenUsage{
		PromptTokens:        usage.PromptTokens,
		CompletionTokens:    usage.CompletionTokens,
		TotalTokens:         usage.TotalTokens,
		CacheCreationTokens: usage.CacheCreationTokens,
		CacheReadTokens:     usage.CacheReadTokens,
	}
}
