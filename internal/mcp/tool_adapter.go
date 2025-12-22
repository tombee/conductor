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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tombee/conductor/pkg/tools"
)

// MCPTool adapts an MCP tool to the tools.Tool interface.
// It wraps an MCP tool definition and routes execution through the MCP client.
type MCPTool struct {
	// serverName is the MCP server that provides this tool
	serverName string

	// toolDef is the MCP tool definition
	toolDef ToolDefinition

	// client is the MCP client for executing this tool
	client ClientProvider
}

// NewMCPTool creates a new MCP tool adapter.
func NewMCPTool(serverName string, toolDef ToolDefinition, client ClientProvider) *MCPTool {
	return &MCPTool{
		serverName: serverName,
		toolDef:    toolDef,
		client:     client,
	}
}

// Name returns the namespaced tool name (e.g., "github.list_repos").
func (t *MCPTool) Name() string {
	return t.serverName + "." + t.toolDef.Name
}

// Description returns the tool description from the MCP definition.
func (t *MCPTool) Description() string {
	return t.toolDef.Description
}

// Schema converts the MCP tool schema to Conductor's tool schema format.
func (t *MCPTool) Schema() *tools.Schema {
	// Parse the MCP inputSchema (JSON Schema)
	var inputSchema map[string]interface{}
	if len(t.toolDef.InputSchema) > 0 {
		if err := json.Unmarshal(t.toolDef.InputSchema, &inputSchema); err != nil {
			// If schema parsing fails, return a minimal schema
			return &tools.Schema{
				Inputs: &tools.ParameterSchema{
					Type:        "object",
					Description: "Tool input parameters",
				},
			}
		}
	}

	// Convert JSON Schema to Conductor's ParameterSchema
	paramSchema := convertJSONSchemaToParameterSchema(inputSchema)

	return &tools.Schema{
		Inputs: paramSchema,
		Outputs: &tools.ParameterSchema{
			Type:        "object",
			Description: "Tool execution result",
		},
	}
}

// Execute runs the MCP tool through the client.
func (t *MCPTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Call the MCP tool
	req := ToolCallRequest{
		Name:      t.toolDef.Name, // Use the original tool name (without namespace)
		Arguments: inputs,
	}

	resp, err := t.client.CallTool(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mcp tool call failed: %w", err)
	}

	// Check if the response indicates an error
	if resp.IsError {
		// Collect error messages from content
		var errorMsg string
		for _, content := range resp.Content {
			if content.Type == "text" && content.Text != "" {
				if errorMsg != "" {
					errorMsg += "; "
				}
				errorMsg += content.Text
			}
		}
		if errorMsg == "" {
			errorMsg = "tool execution failed"
		}
		return nil, fmt.Errorf("%s", errorMsg)
	}

	// Convert MCP response to Conductor output format
	result := make(map[string]interface{})

	// If there's a single text content, put it in "result"
	if len(resp.Content) == 1 && resp.Content[0].Type == "text" {
		result["result"] = resp.Content[0].Text
		return result, nil
	}

	// Multiple content items or non-text content
	contentItems := make([]map[string]interface{}, len(resp.Content))
	for i, content := range resp.Content {
		item := make(map[string]interface{})
		item["type"] = content.Type
		if content.Text != "" {
			item["text"] = content.Text
		}
		if content.Data != "" {
			item["data"] = content.Data
		}
		if content.MimeType != "" {
			item["mimeType"] = content.MimeType
		}
		contentItems[i] = item
	}
	result["content"] = contentItems

	return result, nil
}

// convertJSONSchemaToParameterSchema converts a JSON Schema to Conductor's ParameterSchema.
// This is a simplified conversion that handles the most common cases.
func convertJSONSchemaToParameterSchema(schema map[string]interface{}) *tools.ParameterSchema {
	if schema == nil {
		return &tools.ParameterSchema{
			Type: "object",
		}
	}

	paramSchema := &tools.ParameterSchema{}

	// Extract type
	if schemaType, ok := schema["type"].(string); ok {
		paramSchema.Type = schemaType
	} else {
		paramSchema.Type = "object"
	}

	// Extract description
	if desc, ok := schema["description"].(string); ok {
		paramSchema.Description = desc
	}

	// Extract properties for object types
	if paramSchema.Type == "object" {
		if props, ok := schema["properties"].(map[string]interface{}); ok {
			paramSchema.Properties = make(map[string]*tools.Property)
			for propName, propSchema := range props {
				if propMap, ok := propSchema.(map[string]interface{}); ok {
					prop := &tools.Property{}

					if propType, ok := propMap["type"].(string); ok {
						prop.Type = propType
					}
					if propDesc, ok := propMap["description"].(string); ok {
						prop.Description = propDesc
					}
					if propEnum, ok := propMap["enum"].([]interface{}); ok {
						prop.Enum = propEnum
					}
					if propDefault, ok := propMap["default"]; ok {
						prop.Default = propDefault
					}
					if propFormat, ok := propMap["format"].(string); ok {
						prop.Format = propFormat
					}

					paramSchema.Properties[propName] = prop
				}
			}
		}

		// Extract required fields
		if required, ok := schema["required"].([]interface{}); ok {
			paramSchema.Required = make([]string, len(required))
			for i, req := range required {
				if reqStr, ok := req.(string); ok {
					paramSchema.Required[i] = reqStr
				}
			}
		}
	}

	return paramSchema
}
