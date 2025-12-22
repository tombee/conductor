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
	"encoding/json"
	"testing"
)

func TestMCPTool_Name(t *testing.T) {
	toolDef := ToolDefinition{
		Name:        "list_repos",
		Description: "List repositories",
		InputSchema: []byte(`{"type":"object"}`),
	}

	tool := NewMCPTool("github", toolDef, nil)

	if tool.Name() != "github.list_repos" {
		t.Errorf("Name() = %s, want github.list_repos", tool.Name())
	}
}

func TestMCPTool_Description(t *testing.T) {
	toolDef := ToolDefinition{
		Name:        "list_repos",
		Description: "List all repositories for a user",
		InputSchema: []byte(`{"type":"object"}`),
	}

	tool := NewMCPTool("github", toolDef, nil)

	if tool.Description() != "List all repositories for a user" {
		t.Errorf("Description() = %s, want 'List all repositories for a user'", tool.Description())
	}
}

func TestMCPTool_Schema(t *testing.T) {
	tests := []struct {
		name        string
		inputSchema string
		checkSchema func(t *testing.T, tool *MCPTool)
	}{
		{
			name:        "simple object schema",
			inputSchema: `{"type":"object","properties":{"user":{"type":"string","description":"Username"}}}`,
			checkSchema: func(t *testing.T, tool *MCPTool) {
				schema := tool.Schema()
				if schema == nil {
					t.Fatal("Schema() returned nil")
				}
				if schema.Inputs == nil {
					t.Fatal("Schema().Inputs is nil")
				}
				if schema.Inputs.Type != "object" {
					t.Errorf("Schema().Inputs.Type = %s, want object", schema.Inputs.Type)
				}
				if len(schema.Inputs.Properties) != 1 {
					t.Errorf("Schema().Inputs.Properties count = %d, want 1", len(schema.Inputs.Properties))
				}
				if userProp, ok := schema.Inputs.Properties["user"]; ok {
					if userProp.Type != "string" {
						t.Errorf("user property type = %s, want string", userProp.Type)
					}
					if userProp.Description != "Username" {
						t.Errorf("user property description = %s, want Username", userProp.Description)
					}
				} else {
					t.Error("user property not found in schema")
				}
			},
		},
		{
			name:        "schema with required fields",
			inputSchema: `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
			checkSchema: func(t *testing.T, tool *MCPTool) {
				schema := tool.Schema()
				if len(schema.Inputs.Required) != 1 {
					t.Errorf("Schema().Inputs.Required count = %d, want 1", len(schema.Inputs.Required))
				}
				if schema.Inputs.Required[0] != "name" {
					t.Errorf("Schema().Inputs.Required[0] = %s, want name", schema.Inputs.Required[0])
				}
			},
		},
		{
			name:        "schema with enum",
			inputSchema: `{"type":"object","properties":{"status":{"type":"string","enum":["open","closed"]}}}`,
			checkSchema: func(t *testing.T, tool *MCPTool) {
				schema := tool.Schema()
				if statusProp, ok := schema.Inputs.Properties["status"]; ok {
					if len(statusProp.Enum) != 2 {
						t.Errorf("status enum count = %d, want 2", len(statusProp.Enum))
					}
				} else {
					t.Error("status property not found in schema")
				}
			},
		},
		{
			name:        "invalid json schema",
			inputSchema: `{invalid json}`,
			checkSchema: func(t *testing.T, tool *MCPTool) {
				schema := tool.Schema()
				if schema == nil {
					t.Fatal("Schema() returned nil")
				}
				// Should return a minimal schema on parse error
				if schema.Inputs == nil {
					t.Fatal("Schema().Inputs is nil")
				}
				if schema.Inputs.Type != "object" {
					t.Errorf("Schema().Inputs.Type = %s, want object", schema.Inputs.Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolDef := ToolDefinition{
				Name:        "test_tool",
				Description: "Test tool",
				InputSchema: []byte(tt.inputSchema),
			}
			tool := NewMCPTool("test", toolDef, nil)
			tt.checkSchema(t, tool)
		})
	}
}

func TestConvertJSONSchemaToParameterSchema(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]interface{}
		check  func(t *testing.T, result interface{})
	}{
		{
			name:   "nil schema",
			schema: nil,
			check: func(t *testing.T, result interface{}) {
				schema := result.(*MCPTool).Schema()
				if schema.Inputs.Type != "object" {
					t.Errorf("Type = %s, want object", schema.Inputs.Type)
				}
			},
		},
		{
			name: "schema with description",
			schema: map[string]interface{}{
				"type":        "object",
				"description": "Test description",
			},
			check: func(t *testing.T, result interface{}) {
				schema := result.(*MCPTool).Schema()
				if schema.Inputs.Description != "Test description" {
					t.Errorf("Description = %s, want 'Test description'", schema.Inputs.Description)
				}
			},
		},
		{
			name: "schema with default values",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"count": map[string]interface{}{
						"type":    "number",
						"default": 10,
					},
				},
			},
			check: func(t *testing.T, result interface{}) {
				schema := result.(*MCPTool).Schema()
				if countProp, ok := schema.Inputs.Properties["count"]; ok {
					// JSON unmarshaling converts numbers to float64
					if countProp.Default != float64(10) {
						t.Errorf("default = %v (type %T), want 10 (type float64)", countProp.Default, countProp.Default)
					}
				} else {
					t.Error("count property not found")
				}
			},
		},
		{
			name: "schema with format",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":   "string",
						"format": "uri",
					},
				},
			},
			check: func(t *testing.T, result interface{}) {
				schema := result.(*MCPTool).Schema()
				if urlProp, ok := schema.Inputs.Properties["url"]; ok {
					if urlProp.Format != "uri" {
						t.Errorf("format = %s, want uri", urlProp.Format)
					}
				} else {
					t.Error("url property not found")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize schema to JSON to test the full conversion path
			var schemaBytes []byte
			if tt.schema != nil {
				var err error
				schemaBytes, err = json.Marshal(tt.schema)
				if err != nil {
					t.Fatalf("Failed to marshal schema: %v", err)
				}
			}

			toolDef := ToolDefinition{
				Name:        "test_tool",
				Description: "Test",
				InputSchema: schemaBytes,
			}
			tool := NewMCPTool("test", toolDef, nil)
			tt.check(t, tool)
		})
	}
}

func TestNewMCPTool(t *testing.T) {
	toolDef := ToolDefinition{
		Name:        "list_repos",
		Description: "List repositories",
		InputSchema: []byte(`{"type":"object"}`),
	}

	tool := NewMCPTool("github", toolDef, nil)

	if tool == nil {
		t.Fatal("NewMCPTool returned nil")
	}
	if tool.serverName != "github" {
		t.Errorf("serverName = %s, want github", tool.serverName)
	}
	if tool.toolDef.Name != "list_repos" {
		t.Errorf("toolDef.Name = %s, want list_repos", tool.toolDef.Name)
	}
}
