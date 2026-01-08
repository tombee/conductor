package sdk

import (
	"context"

	"github.com/tombee/conductor/pkg/tools"
)

// Tool defines a custom tool implementation.
// Tools can be registered with the SDK and used by LLM agents or workflow steps.
type Tool interface {
	// Name returns the tool identifier.
	Name() string

	// Description returns what the tool does.
	Description() string

	// InputSchema returns the JSON Schema for inputs.
	InputSchema() map[string]any

	// Execute runs the tool with the given inputs.
	Execute(ctx context.Context, inputs map[string]any) (map[string]any, error)
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
	ExecuteStream(ctx context.Context, inputs map[string]any) (<-chan tools.ToolChunk, error)
}

// FuncTool creates a tool from a function.
// This is a convenience wrapper for simple tools that don't need complex state.
//
// Example:
//
//	tool := sdk.FuncTool(
//		"get_weather",
//		"Get current weather for a location",
//		map[string]any{
//			"type": "object",
//			"properties": map[string]any{
//				"location": map[string]any{"type": "string"},
//			},
//			"required": []string{"location"},
//		},
//		func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
//			location := inputs["location"].(string)
//			// ... fetch weather ...
//			return map[string]any{
//				"temperature": 72,
//				"conditions":  "sunny",
//			}, nil
//		},
//	)
func FuncTool(name, description string, schema map[string]any, fn func(ctx context.Context, inputs map[string]any) (map[string]any, error)) Tool {
	return &funcTool{
		name:        name,
		description: description,
		schema:      schema,
		fn:          fn,
	}
}

// funcTool implements Tool using a simple function.
type funcTool struct {
	name        string
	description string
	schema      map[string]any
	fn          func(ctx context.Context, inputs map[string]any) (map[string]any, error)
}

func (t *funcTool) Name() string {
	return t.name
}

func (t *funcTool) Description() string {
	return t.description
}

func (t *funcTool) InputSchema() map[string]any {
	return t.schema
}

func (t *funcTool) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	return t.fn(ctx, inputs)
}

// FuncStreamingTool creates a streaming tool from a function.
// This is a convenience wrapper for streaming tools that don't need complex state.
//
// Example:
//
//	tool := sdk.FuncStreamingTool(
//		"tail_log",
//		"Stream log file contents in real-time",
//		map[string]any{
//			"type": "object",
//			"properties": map[string]any{
//				"file": map[string]any{"type": "string"},
//			},
//			"required": []string{"file"},
//		},
//		func(ctx context.Context, inputs map[string]any) (<-chan tools.ToolChunk, error) {
//			file := inputs["file"].(string)
//			chunks := make(chan tools.ToolChunk)
//			go func() {
//				defer close(chunks)
//				// ... stream file contents ...
//				chunks <- tools.ToolChunk{
//					Data:   "log line 1\n",
//					Stream: "stdout",
//				}
//				chunks <- tools.ToolChunk{
//					IsFinal: true,
//					Result:  map[string]any{"lines": 1},
//				}
//			}()
//			return chunks, nil
//		},
//	)
func FuncStreamingTool(name, description string, schema map[string]any, fn func(ctx context.Context, inputs map[string]any) (<-chan tools.ToolChunk, error)) StreamingTool {
	return &funcStreamingTool{
		name:        name,
		description: description,
		schema:      schema,
		fn:          fn,
	}
}

// funcStreamingTool implements StreamingTool using a simple function.
type funcStreamingTool struct {
	name        string
	description string
	schema      map[string]any
	fn          func(ctx context.Context, inputs map[string]any) (<-chan tools.ToolChunk, error)
}

func (t *funcStreamingTool) Name() string {
	return t.name
}

func (t *funcStreamingTool) Description() string {
	return t.description
}

func (t *funcStreamingTool) InputSchema() map[string]any {
	return t.schema
}

func (t *funcStreamingTool) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	// For non-streaming execution, collect all chunks and return the final result
	chunks, err := t.fn(ctx, inputs)
	if err != nil {
		return nil, err
	}

	// Drain the channel and return the final result
	var result map[string]any
	var execError error
	for chunk := range chunks {
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

func (t *funcStreamingTool) ExecuteStream(ctx context.Context, inputs map[string]any) (<-chan tools.ToolChunk, error) {
	return t.fn(ctx, inputs)
}

// sdkToolAdapter adapts an SDK Tool to pkg/tools.Tool interface
type sdkToolAdapter struct {
	tool Tool
}

func (a *sdkToolAdapter) Name() string {
	return a.tool.Name()
}

func (a *sdkToolAdapter) Description() string {
	return a.tool.Description()
}

func (a *sdkToolAdapter) Schema() *tools.Schema {
	inputSchema := a.tool.InputSchema()

	// Convert map[string]any to tools.ParameterSchema
	params := &tools.ParameterSchema{
		Type: "object",
	}

	// Extract properties if present
	if props, ok := inputSchema["properties"].(map[string]any); ok {
		params.Properties = make(map[string]*tools.Property)
		for k, v := range props {
			if propMap, ok := v.(map[string]any); ok {
				prop := &tools.Property{}
				if typ, ok := propMap["type"].(string); ok {
					prop.Type = typ
				}
				if desc, ok := propMap["description"].(string); ok {
					prop.Description = desc
				}
				params.Properties[k] = prop
			}
		}
	}

	// Extract required fields if present
	if req, ok := inputSchema["required"].([]string); ok {
		params.Required = req
	} else if req, ok := inputSchema["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				params.Required = append(params.Required, s)
			}
		}
	}

	return &tools.Schema{
		Inputs:  params,
		Outputs: nil, // SDK tools don't specify output schema
	}
}

func (a *sdkToolAdapter) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	return a.tool.Execute(ctx, inputs)
}

// ExecuteStream implements the pkg/tools.StreamingTool interface by delegating to
// the wrapped SDK tool if it implements SDK StreamingTool.
// This method is only called if the adapter is used as a StreamingTool.
func (a *sdkToolAdapter) ExecuteStream(ctx context.Context, inputs map[string]any) (<-chan tools.ToolChunk, error) {
	// Check if the wrapped SDK tool implements streaming
	if streamingTool, ok := a.tool.(StreamingTool); ok {
		return streamingTool.ExecuteStream(ctx, inputs)
	}

	// This should never happen because the registry checks SupportsStreaming first,
	// but provide a fallback just in case
	chunks := make(chan tools.ToolChunk, 1)
	go func() {
		defer close(chunks)
		result, err := a.tool.Execute(ctx, inputs)
		chunks <- tools.ToolChunk{
			IsFinal: true,
			Result:  result,
			Error:   err,
		}
	}()
	return chunks, nil
}

// RegisterTool adds a custom tool to the SDK.
// The tool will be available to agent loops and LLM steps with tool use.
//
// Example:
//
//	tool := sdk.FuncTool("get_weather", "Get weather", schema, fn)
//	if err := s.RegisterTool(tool); err != nil {
//		return err
//	}
func (s *SDK) RegisterTool(tool Tool) error {
	pkgTool := &sdkToolAdapter{tool: tool}
	return s.toolRegistry.Register(pkgTool)
}

// UnregisterTool removes a tool from the SDK.
// Returns an error if the tool doesn't exist.
//
// Example:
//
//	if err := s.UnregisterTool("get_weather"); err != nil {
//		return err
//	}
func (s *SDK) UnregisterTool(name string) error {
	return s.toolRegistry.Unregister(name)
}
