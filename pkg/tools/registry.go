// Package tools provides a registry system for workflow tools.
//
// Tools are discrete functions that can be called by LLM agents or workflow steps.
// Each tool has a name, schema (defining its inputs/outputs), and an execution function.
//
// The registry allows tools to be registered, discovered, and executed in a type-safe manner.
package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/tombee/conductor/pkg/errors"
)

// Tool represents an executable tool that can be used in workflows or by agents.
type Tool interface {
	// Name returns the unique identifier for this tool
	Name() string

	// Description returns a human-readable description of what the tool does
	Description() string

	// Schema returns the JSON schema defining the tool's inputs and outputs
	Schema() *Schema

	// Execute runs the tool with the given inputs and returns outputs
	Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error)
}

// StreamingTool extends Tool with streaming execution support.
// Tools that implement this interface can emit incremental output chunks during execution,
// enabling real-time progress visibility for long-running operations.
type StreamingTool interface {
	Tool

	// ExecuteStream runs the tool and streams output chunks via a channel.
	// The channel is closed when execution completes.
	// Callers should drain the channel to get the complete result.
	//
	// Returns:
	//   - A receive-only channel of ToolChunk values
	//   - An error if the tool fails to start (startup errors only)
	//
	// Runtime errors during execution are delivered in the final chunk's Error field.
	// Exactly one chunk will have IsFinal=true, which is always the last chunk.
	ExecuteStream(ctx context.Context, inputs map[string]any) (<-chan ToolChunk, error)
}

// ToolChunk represents an incremental output from a streaming tool.
// Chunks are emitted during tool execution to provide real-time progress feedback.
type ToolChunk struct {
	// Data is the chunk content (e.g., a line of output from stdout/stderr)
	Data string

	// Stream identifies the output stream ("stdout", "stderr", or empty for general output)
	Stream string

	// IsFinal indicates this is the last chunk with complete results.
	// Exactly one chunk will have IsFinal=true, and it will be the last chunk sent.
	IsFinal bool

	// Result contains the final tool output (only set when IsFinal is true).
	// For non-final chunks, this field is nil.
	Result map[string]any

	// Error contains any execution error (only set when IsFinal is true).
	// For non-final chunks, this field is nil.
	Error error

	// Metadata contains optional additional information about the chunk.
	// Common uses include truncation indicators, timing data, or tool-specific context.
	Metadata map[string]any
}

// Schema defines the input and output schema for a tool using JSON Schema.
type Schema struct {
	// Inputs defines the expected input parameters
	Inputs *ParameterSchema `json:"inputs"`

	// Outputs defines the structure of returned data
	Outputs *ParameterSchema `json:"outputs"`
}

// ParameterSchema defines a set of parameters using JSON Schema conventions.
type ParameterSchema struct {
	// Type is the JSON type (e.g., "object", "string", "number")
	Type string `json:"type"`

	// Properties defines nested properties (for type="object")
	Properties map[string]*Property `json:"properties,omitempty"`

	// Required lists the required property names
	Required []string `json:"required,omitempty"`

	// Description provides human-readable context
	Description string `json:"description,omitempty"`
}

// Property defines a single property in a parameter schema.
type Property struct {
	// Type is the JSON type of this property
	Type string `json:"type"`

	// Description explains what this property represents
	Description string `json:"description,omitempty"`

	// Enum lists allowed values (for validation)
	Enum []interface{} `json:"enum,omitempty"`

	// Default provides a default value if not specified
	Default interface{} `json:"default,omitempty"`

	// Format specifies a format hint (e.g., "uri", "email", "date-time")
	Format string `json:"format,omitempty"`
}

// Registry maintains a collection of registered tools.
type Registry struct {
	mu           sync.RWMutex
	tools        map[string]Tool
	interceptor  Interceptor
	eventEmitter EventEmitter
}

// EventEmitter emits tool execution events.
// This allows the registry to publish streaming output events to the SDK event system.
type EventEmitter func(ctx context.Context, eventType string, workflowID string, stepID string, data any)

// Interceptor validates tool execution against security policy.
// This interface is defined here to avoid circular dependencies.
type Interceptor interface {
	// Intercept is called before tool execution
	Intercept(ctx context.Context, tool Tool, inputs map[string]interface{}) error

	// PostExecute is called after tool execution
	PostExecute(ctx context.Context, tool Tool, outputs map[string]interface{}, err error)
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// SetInterceptor sets the security interceptor for this registry.
// The interceptor will be called before and after each tool execution.
func (r *Registry) SetInterceptor(interceptor Interceptor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interceptor = interceptor
}

// SetEventEmitter sets the event emitter for this registry.
// The emitter will be called for each streaming tool output chunk.
func (r *Registry) SetEventEmitter(emitter EventEmitter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventEmitter = emitter
}

// Register adds a tool to the registry.
// Returns an error if a tool with the same name is already registered.
func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("cannot register nil tool")
	}

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool already registered: %s", name)
	}

	// Validate tool schema
	schema := tool.Schema()
	if schema == nil {
		return fmt.Errorf("tool schema cannot be nil: %s", name)
	}

	r.tools[name] = tool
	return nil
}

// Unregister removes a tool from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return &errors.NotFoundError{
			Resource: "tool",
			ID:       name,
		}
	}

	delete(r.tools, name)
	return nil
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return nil, &errors.NotFoundError{
			Resource: "tool",
			ID:       name,
		}
	}

	return tool, nil
}

// Has checks if a tool is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.tools[name]
	return exists
}

// List returns all registered tool names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}

	return names
}

// ListTools returns all registered tools.
func (r *Registry) ListTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	return tools
}

// Execute executes a tool by name with the given inputs.
func (r *Registry) Execute(ctx context.Context, name string, inputs map[string]interface{}) (map[string]interface{}, error) {
	tool, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	// Validate inputs against schema
	if err := r.validateInputs(tool, inputs); err != nil {
		return nil, &errors.ValidationError{
			Field:      "inputs",
			Message:    fmt.Sprintf("input validation failed for tool %s: %v", name, err),
			Suggestion: "Check the tool schema for required inputs and correct types",
		}
	}

	// Call security interceptor before execution
	r.mu.RLock()
	interceptor := r.interceptor
	r.mu.RUnlock()

	if interceptor != nil {
		if err := interceptor.Intercept(ctx, tool, inputs); err != nil {
			return nil, fmt.Errorf("security validation failed for tool %s: %w", name, err)
		}
	}

	// Execute the tool
	outputs, err := tool.Execute(ctx, inputs)

	// Call security interceptor after execution
	if interceptor != nil {
		interceptor.PostExecute(ctx, tool, outputs, err)
	}

	if err != nil {
		return nil, fmt.Errorf("tool execution failed for %s: %w", name, err)
	}

	return outputs, nil
}

// validateInputs validates inputs against a tool's schema.
// Phase 1: Basic validation (required fields check).
// Future: Full JSON Schema validation.
func (r *Registry) validateInputs(tool Tool, inputs map[string]interface{}) error {
	schema := tool.Schema()
	if schema == nil || schema.Inputs == nil {
		return nil // No validation required
	}

	// Check required fields
	for _, required := range schema.Inputs.Required {
		if _, exists := inputs[required]; !exists {
			return fmt.Errorf("required input missing: %s", required)
		}
	}

	return nil
}

// GetToolSchemas returns schemas for all registered tools.
// This is useful for LLM function calling where the agent needs to know
// what tools are available and how to use them.
func (r *Registry) GetToolSchemas() map[string]*Schema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemas := make(map[string]*Schema)
	for name, tool := range r.tools {
		schemas[name] = tool.Schema()
	}

	return schemas
}

// ToolDescriptors returns a list of tool descriptors for LLM function calling.
type ToolDescriptor struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Schema      *Schema `json:"schema"`
}

// GetToolDescriptors returns descriptors for all registered tools.
func (r *Registry) GetToolDescriptors() []ToolDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	descriptors := make([]ToolDescriptor, 0, len(r.tools))
	for _, tool := range r.tools {
		descriptors = append(descriptors, ToolDescriptor{
			Name:        tool.Name(),
			Description: tool.Description(),
			Schema:      tool.Schema(),
		})
	}

	return descriptors
}

// ExpandToolPatterns expands tool name patterns into concrete tool names.
// Supports:
//   - Exact names: "github.list_repos" -> ["github.list_repos"]
//   - Namespace wildcards: "github.*" -> ["github.list_repos", "github.create_issue", ...]
//   - All tools: "*" -> [all registered tools]
//
// This is used to resolve MCP server tool patterns in workflow definitions.
func (r *Registry) ExpandToolPatterns(patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	var result []string

	for _, pattern := range patterns {
		if pattern == "*" {
			// Match all tools
			for name := range r.tools {
				if !seen[name] {
					result = append(result, name)
					seen[name] = true
				}
			}
			continue
		}

		// Check if pattern is a namespace wildcard (e.g., "github.*")
		if len(pattern) > 2 && pattern[len(pattern)-2:] == ".*" {
			namespace := pattern[:len(pattern)-2]
			// Match all tools in this namespace
			for name := range r.tools {
				if hasNamespacePrefix(name, namespace) {
					if !seen[name] {
						result = append(result, name)
						seen[name] = true
					}
				}
			}
			continue
		}

		// Exact match
		if r.tools[pattern] != nil {
			if !seen[pattern] {
				result = append(result, pattern)
				seen[pattern] = true
			}
		}
	}

	return result
}

// hasNamespacePrefix checks if a tool name belongs to a given namespace.
// Example: hasNamespacePrefix("github.list_repos", "github") -> true
// Example: hasNamespacePrefix("filesystem.read", "github") -> false
func hasNamespacePrefix(toolName, namespace string) bool {
	prefix := namespace + "."
	return len(toolName) > len(prefix) && toolName[:len(prefix)] == prefix
}

// Filter creates a new registry containing only the specified tools.
// Returns an error if the tools array is empty or if any tool name is not found.
func (r *Registry) Filter(allowedNames []string) (*Registry, error) {
	// Validate empty array
	if len(allowedNames) == 0 {
		return nil, &errors.ValidationError{
			Field:      "tools",
			Message:    "tools array cannot be empty",
			Suggestion: "specify at least one tool name",
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create new registry with filtered tools
	filtered := NewRegistry()
	filtered.interceptor = r.interceptor

	// Add each allowed tool to the filtered registry
	for _, name := range allowedNames {
		tool, exists := r.tools[name]
		if !exists {
			return nil, &errors.ValidationError{
				Field:      "tools",
				Message:    fmt.Sprintf("unknown tool: %s", name),
				Suggestion: fmt.Sprintf("tool %s is not registered in the tool registry", name),
			}
		}
		// Register returns error if tool already exists, but we control the new registry
		// so this should never fail
		_ = filtered.Register(tool)
	}

	return filtered, nil
}

// SupportsStreaming checks if a tool implements the StreamingTool interface.
// Returns false if the tool is not registered.
func (r *Registry) SupportsStreaming(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return false
	}

	_, ok := tool.(StreamingTool)
	return ok
}

// ExecuteStream executes a tool with streaming output support.
// If the tool implements StreamingTool, it delegates to the tool's ExecuteStream method.
// If the tool does not implement StreamingTool, it wraps the standard Execute method
// to emit a single chunk with IsFinal=true containing the complete result.
//
// The toolCallID parameter is used to correlate tool execution with LLM tool calls.
// If empty, a UUID will be generated automatically.
//
// Returns a channel that emits ToolChunk values during execution.
// The channel is closed when execution completes.
// Exactly one chunk will have IsFinal=true, which is always the last chunk.
func (r *Registry) ExecuteStream(ctx context.Context, name string, inputs map[string]interface{}, toolCallID string) (<-chan ToolChunk, error) {
	// Generate UUID for toolCallID if not provided
	if toolCallID == "" {
		toolCallID = uuid.New().String()
	}

	tool, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	// Validate inputs against schema
	if err := r.validateInputs(tool, inputs); err != nil {
		return nil, &errors.ValidationError{
			Field:      "inputs",
			Message:    fmt.Sprintf("input validation failed for tool %s: %v", name, err),
			Suggestion: "Check the tool schema for required inputs and correct types",
		}
	}

	// Call security interceptor before execution
	r.mu.RLock()
	interceptor := r.interceptor
	r.mu.RUnlock()

	if interceptor != nil {
		if err := interceptor.Intercept(ctx, tool, inputs); err != nil {
			return nil, fmt.Errorf("security validation failed for tool %s: %w", name, err)
		}
	}

	// Check if tool supports streaming
	if streamingTool, ok := tool.(StreamingTool); ok {
		// Use native streaming support
		chunks, err := streamingTool.ExecuteStream(ctx, inputs)
		if err != nil {
			return nil, fmt.Errorf("tool execution failed for %s: %w", name, err)
		}

		// Wrap channel to emit events and call post-execute interceptor
		wrappedChunks := make(chan ToolChunk)
		go func() {
			defer close(wrappedChunks)
			var finalResult map[string]interface{}
			var finalError error

			for chunk := range chunks {
				// Emit event for this chunk
				r.emitToolOutputEvent(ctx, toolCallID, name, chunk)

				wrappedChunks <- chunk
				if chunk.IsFinal {
					finalResult = chunk.Result
					finalError = chunk.Error
				}
			}

			// Call post-execute interceptor after streaming completes
			if interceptor != nil {
				interceptor.PostExecute(ctx, tool, finalResult, finalError)
			}
		}()
		return wrappedChunks, nil
	}

	// Tool does not support streaming - wrap standard Execute method
	// to emit a single chunk with IsFinal=true
	chunks := make(chan ToolChunk, 1)

	go func() {
		defer close(chunks)

		result, err := tool.Execute(ctx, inputs)

		// Call post-execute interceptor
		if interceptor != nil {
			interceptor.PostExecute(ctx, tool, result, err)
		}

		// Create single final chunk
		chunk := ToolChunk{
			IsFinal: true,
			Result:  result,
			Error:   err,
		}

		// Emit event for this chunk
		r.emitToolOutputEvent(ctx, toolCallID, name, chunk)

		// Emit single final chunk
		chunks <- chunk
	}()

	return chunks, nil
}

// emitToolOutputEvent emits a tool output event for a chunk.
// This is called for every chunk produced by ExecuteStream.
func (r *Registry) emitToolOutputEvent(ctx context.Context, toolCallID string, toolName string, chunk ToolChunk) {
	r.mu.RLock()
	emitter := r.eventEmitter
	r.mu.RUnlock()

	if emitter == nil {
		return
	}

	// Extract workflow and step IDs from context if available
	workflowID, _ := ctx.Value("workflow_id").(string)
	stepID, _ := ctx.Value("step_id").(string)

	// Create ToolOutputEvent matching sdk/events.go structure
	eventData := map[string]any{
		"tool_call_id": toolCallID,
		"tool_name":    toolName,
		"stream":       chunk.Stream,
		"data":         chunk.Data,
		"is_final":     chunk.IsFinal,
		"metadata":     chunk.Metadata,
	}

	emitter(ctx, "tool.output", workflowID, stepID, eventData)
}
