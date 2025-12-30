// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// OperationRegistry defines the interface for executing operations.
// This allows the MCP server to expose operation registry actions as tools.
type OperationRegistry interface {
	Execute(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error)
	List() []string
}

// OperationResult defines the result interface for operations.
type OperationResult interface {
	GetResponse() interface{}
}

// operationToolDef defines the schema for an operation exposed as an MCP tool.
type operationToolDef struct {
	name        string
	description string
	schema      map[string]interface{}
}

// builtinOperationDefs defines the MCP tool definitions for builtin actions.
var builtinOperationDefs = map[string][]operationToolDef{
	"file": {
		{
			name:        "file.read",
			description: "Read the contents of a file at the specified path",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			name:        "file.write",
			description: "Write content to a file at the specified path",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to write",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			name:        "file.list",
			description: "List files in a directory",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The directory path to list",
					},
				},
				"required": []string{"path"},
			},
		},
	},
	"shell": {
		{
			name:        "shell.run",
			description: "Execute a shell command",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The command to execute",
					},
				},
				"required": []string{"command"},
			},
		},
	},
}

// registerOperationTools registers operation registry actions as MCP tools.
func (s *Server) registerOperationTools(registry OperationRegistry) error {
	if registry == nil {
		return nil
	}

	// Get available providers from registry
	providers := registry.List()

	for _, providerName := range providers {
		// Check if we have definitions for this provider
		defs, ok := builtinOperationDefs[providerName]
		if !ok {
			continue
		}

		for _, def := range defs {
			// Capture for closure
			toolRef := def.name

			// Create MCP tool
			tool := mcp.Tool{
				Name:        def.name,
				Description: def.description,
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: def.schema["properties"].(map[string]interface{}),
				},
			}

			// Add required fields if present
			if required, ok := def.schema["required"].([]string); ok {
				tool.InputSchema.Required = required
			}

			// Create handler that routes to registry
			handler := s.createOperationHandler(registry, toolRef)
			s.mcpServer.AddTool(tool, handler)
		}
	}

	return nil
}

// createOperationHandler creates an MCP tool handler that routes to the operation registry.
func (s *Server) createOperationHandler(registry OperationRegistry, reference string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.Debug("Executing operation via registry",
			"reference", reference,
			"args", request.Params.Arguments,
		)

		// Convert arguments to map[string]interface{}
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return errorResponse("Invalid arguments format"), nil
		}

		// Execute via registry
		result, err := registry.Execute(ctx, reference, args)
		if err != nil {
			s.logger.Error("Operation execution failed",
				"reference", reference,
				"error", err,
			)
			return errorResponse(fmt.Sprintf("Operation failed: %v", err)), nil
		}

		// Format response
		response := result.GetResponse()
		if response == nil {
			return textResponse("Operation completed successfully"), nil
		}

		// Format as string or JSON
		switch v := response.(type) {
		case string:
			return textResponse(v), nil
		case *string:
			if v != nil {
				return textResponse(*v), nil
			}
			return textResponse(""), nil
		default:
			// Marshal to JSON for complex types
			data, err := json.MarshalIndent(response, "", "  ")
			if err != nil {
				return textResponse(fmt.Sprintf("%v", response)), nil
			}
			return textResponse(string(data)), nil
		}
	}
}
