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
