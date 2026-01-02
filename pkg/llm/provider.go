// Package llm provides abstractions for Large Language Model providers.
// This package is designed to be embeddable in other Go applications and
// provides a provider-agnostic interface for LLM interactions.
package llm

import (
	"context"
	"time"
)

// Provider defines the interface that all LLM providers must implement.
// This interface supports both streaming and non-streaming completions,
// model capabilities querying, and cost estimation.
type Provider interface {
	// Name returns the unique identifier for this provider (e.g., "anthropic", "openai").
	Name() string

	// Capabilities returns the provider's supported features and model information.
	Capabilities() Capabilities

	// Complete sends a synchronous completion request and returns the full response.
	// This method blocks until the LLM response is complete.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Stream sends a streaming completion request and returns a channel of chunks.
	// The caller must consume all chunks from the channel until it closes.
	// Errors during streaming are sent as StreamChunk with Error field set.
	Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

// Capabilities describes what a provider supports.
type Capabilities struct {
	// Streaming indicates whether the provider supports streaming responses.
	Streaming bool

	// Tools indicates whether the provider supports tool/function calling.
	Tools bool

	// Models lists all models available from this provider with their metadata.
	Models []ModelInfo
}

// CompletionRequest contains all parameters for an LLM completion request.
type CompletionRequest struct {
	// Messages is the conversation history including the current prompt.
	Messages []Message

	// Model specifies which model to use. Can be a specific model ID or a tier.
	Model string

	// Temperature controls randomness (0.0 = deterministic, 1.0 = creative).
	// Valid range: 0.0-1.0. Default: provider-specific.
	Temperature *float64

	// MaxTokens limits the response length. If nil, uses provider default.
	MaxTokens *int

	// Tools defines available functions the model can call.
	Tools []Tool

	// StopSequences are strings that halt generation when encountered.
	StopSequences []string

	// Metadata contains request tracking information (correlation IDs, etc).
	Metadata map[string]string
}

// Message represents a single message in a conversation.
type Message struct {
	// Role indicates who sent this message (user, assistant, system, tool).
	Role MessageRole

	// Content is the text content of the message.
	Content string

	// ToolCalls contains any tool invocations made by the assistant.
	// Only valid when Role is "assistant".
	ToolCalls []ToolCall

	// ToolCallID links this message to a specific tool call.
	// Only valid when Role is "tool".
	ToolCallID string

	// Name identifies the tool that produced this result.
	// Only valid when Role is "tool".
	Name string
}

// MessageRole identifies the sender of a message.
type MessageRole string

const (
	// MessageRoleSystem indicates a system message (context, instructions).
	MessageRoleSystem MessageRole = "system"

	// MessageRoleUser indicates a message from the user.
	MessageRoleUser MessageRole = "user"

	// MessageRoleAssistant indicates a message from the LLM.
	MessageRoleAssistant MessageRole = "assistant"

	// MessageRoleTool indicates a tool execution result.
	MessageRoleTool MessageRole = "tool"
)

// ToolCall represents a function invocation by the LLM.
type ToolCall struct {
	// ID uniquely identifies this tool call within a completion.
	ID string

	// Name is the function name to invoke.
	Name string

	// Arguments contains the JSON-encoded function parameters.
	Arguments string
}

// Tool defines a function the LLM can invoke.
type Tool struct {
	// Name is the function identifier.
	Name string

	// Description explains what this function does.
	Description string

	// InputSchema is a JSON Schema describing the function parameters.
	InputSchema map[string]interface{}
}

// CompletionResponse contains the full response from a non-streaming completion.
type CompletionResponse struct {
	// Content is the generated text response.
	Content string

	// ToolCalls contains any tool invocations made by the model.
	ToolCalls []ToolCall

	// FinishReason explains why generation stopped.
	FinishReason FinishReason

	// Usage contains token consumption information.
	Usage TokenUsage

	// Model is the actual model ID that handled this request.
	Model string

	// RequestID is the unique identifier for this request (for tracing).
	RequestID string

	// Created is the timestamp when this response was generated.
	Created time.Time
}

// StreamChunk represents a single piece of a streaming response.
type StreamChunk struct {
	// Delta contains the incremental content added in this chunk.
	Delta StreamDelta

	// FinishReason is set on the final chunk to indicate why streaming stopped.
	FinishReason FinishReason

	// Usage is set on the final chunk with token consumption stats.
	Usage *TokenUsage

	// Error contains any error that occurred during streaming.
	// When set, this is the final chunk and the stream will close.
	Error error

	// RequestID is the unique identifier for this streaming request.
	RequestID string
}

// StreamDelta contains the incremental updates in a stream chunk.
type StreamDelta struct {
	// Content is the text added in this chunk.
	Content string

	// ToolCallDelta contains partial tool call information.
	// Tool calls may be built up over multiple chunks.
	ToolCallDelta *ToolCallDelta
}

// ToolCallDelta represents partial tool call information in a stream.
type ToolCallDelta struct {
	// Index identifies which tool call this delta belongs to.
	Index int

	// ID is the tool call identifier (may be empty in intermediate chunks).
	ID string

	// Name is the function name (may be empty in intermediate chunks).
	Name string

	// ArgumentsDelta contains additional argument JSON fragment.
	ArgumentsDelta string
}

// FinishReason indicates why completion generation stopped.
type FinishReason string

const (
	// FinishReasonStop indicates natural completion.
	FinishReasonStop FinishReason = "stop"

	// FinishReasonLength indicates max_tokens limit reached.
	FinishReasonLength FinishReason = "length"

	// FinishReasonToolCalls indicates the model wants to call functions.
	FinishReasonToolCalls FinishReason = "tool_calls"

	// FinishReasonContentFilter indicates content policy violation.
	FinishReasonContentFilter FinishReason = "content_filter"

	// FinishReasonError indicates an error occurred.
	FinishReasonError FinishReason = "error"
)

// TokenUsage tracks token consumption for cost calculation.
type TokenUsage struct {
	// InputTokens is the number of tokens in the input (prompt).
	InputTokens int

	// OutputTokens is the number of tokens in the output (completion).
	OutputTokens int

	// TotalTokens is the sum of input and output tokens.
	TotalTokens int

	// CacheCreationTokens tracks tokens written to cache (billed at full rate).
	CacheCreationTokens int

	// CacheReadTokens tracks tokens served from cache (reduced rate).
	CacheReadTokens int
}

// HealthCheckStep represents a step in the health check process
type HealthCheckStep string

const (
	// HealthCheckStepInstalled checks if the provider is installed/available
	HealthCheckStepInstalled HealthCheckStep = "installed"

	// HealthCheckStepAuthenticated checks if the provider is properly authenticated
	HealthCheckStepAuthenticated HealthCheckStep = "authenticated"

	// HealthCheckStepWorking checks if the provider can successfully make requests
	HealthCheckStepWorking HealthCheckStep = "working"
)

// HealthCheckResult contains the result of a provider health check
type HealthCheckResult struct {
	// Installed indicates whether the provider is installed/available
	Installed bool

	// Authenticated indicates whether the provider is properly authenticated
	Authenticated bool

	// Working indicates whether the provider can successfully make requests
	Working bool

	// Error contains any error that occurred during health check
	Error error

	// ErrorStep indicates which step failed if Error is set
	ErrorStep HealthCheckStep

	// Message provides additional context or actionable guidance
	Message string

	// Version contains version information if available
	Version string
}

// Healthy returns true if all health check steps passed
func (h HealthCheckResult) Healthy() bool {
	return h.Installed && h.Authenticated && h.Working && h.Error == nil
}

// Detectable is an optional interface that providers can implement to support detection
type Detectable interface {
	// Detect checks if the provider is available in the current environment
	Detect() (bool, error)
}

// HealthCheckable is an optional interface that providers can implement for health checks
type HealthCheckable interface {
	// HealthCheck performs a three-step verification of provider status
	HealthCheck(ctx context.Context) HealthCheckResult
}

// UsageTrackable is an optional interface that providers can implement for post-request usage tracking.
// This enables cost tracking even when the provider API doesn't include usage in the primary response.
type UsageTrackable interface {
	// GetLastUsage returns the token usage from the most recent request.
	// Returns nil if no usage data is available.
	GetLastUsage() *TokenUsage
}

// ModelTierMap defines the mapping from tier names to model IDs.
// This is populated from user configuration, not provider defaults.
type ModelTierMap struct {
	Fast      string // Model ID for fast tier
	Balanced  string // Model ID for balanced tier
	Strategic string // Model ID for strategic tier
}

// IsEmpty returns true if no tier mappings are configured.
func (m ModelTierMap) IsEmpty() bool {
	return m.Fast == "" && m.Balanced == "" && m.Strategic == ""
}

// Get returns the model ID for the given tier name.
// Returns empty string if the tier is not configured.
func (m ModelTierMap) Get(tier string) string {
	switch tier {
	case "fast":
		return m.Fast
	case "balanced":
		return m.Balanced
	case "strategic":
		return m.Strategic
	default:
		return ""
	}
}
