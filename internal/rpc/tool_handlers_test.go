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

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/tombee/conductor/pkg/tools"
)

// Helper function to create test messages
func toolTestMessage(method string, params interface{}) *Message {
	paramsJSON, _ := json.Marshal(params)
	return &Message{
		Type:          MessageTypeRequest,
		CorrelationID: "test-" + method,
		Method:        method,
		Params:        paramsJSON,
	}
}

// mockTool is a simple mock tool for testing.
type mockTool struct {
	name        string
	description string
	executeFunc func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error)
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) Schema() *tools.Schema {
	return &tools.Schema{
		Inputs: &tools.ParameterSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"input": {
					Type:        "string",
					Description: "Test input",
				},
			},
			Required: []string{"input"},
		},
		Outputs: &tools.ParameterSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"output": {
					Type:        "string",
					Description: "Test output",
				},
			},
		},
	}
}

func (m *mockTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, inputs)
	}
	return map[string]interface{}{
		"output": "mock result",
	}, nil
}

func TestToolHandlers_List(t *testing.T) {
	registry := tools.NewRegistry()

	// Register test tools
	tool1 := &mockTool{
		name:        "test_tool_1",
		description: "First test tool",
	}
	tool2 := &mockTool{
		name:        "test_tool_2",
		description: "Second test tool",
	}

	if err := registry.Register(tool1); err != nil {
		t.Fatalf("Failed to register tool1: %v", err)
	}
	if err := registry.Register(tool2); err != nil {
		t.Fatalf("Failed to register tool2: %v", err)
	}

	handlers := NewToolHandlers(registry)

	req := toolTestMessage("tool.list", map[string]interface{}{})

	resp, err := handlers.handleList(context.Background(), req)
	if err != nil {
		t.Fatalf("handleList() error = %v", err)
	}
	if resp == nil {
		t.Fatal("handleList() returned nil response")
	}

	var result map[string]interface{}
	if err := resp.UnmarshalResult(&result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	count, ok := result["count"].(float64) // JSON numbers unmarshal as float64
	if !ok {
		t.Fatalf("handleList() count is not a number, got %T", result["count"])
	}

	if int(count) != 2 {
		t.Errorf("Expected count 2, got %d", int(count))
	}
}

func TestToolHandlers_Get(t *testing.T) {
	registry := tools.NewRegistry()

	tool := &mockTool{
		name:        "test_tool",
		description: "Test tool",
	}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	handlers := NewToolHandlers(registry)

	tests := []struct {
		name      string
		params    interface{}
		wantError bool
	}{
		{
			name: "valid get",
			params: map[string]interface{}{
				"name": "test_tool",
			},
			wantError: false,
		},
		{
			name:      "missing name",
			params:    map[string]interface{}{},
			wantError: true,
		},
		{
			name: "not found",
			params: map[string]interface{}{
				"name": "nonexistent_tool",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := toolTestMessage("tool.get", tt.params)
			resp, err := handlers.handleGet(context.Background(), req)
			if (err != nil) != tt.wantError {
				t.Errorf("handleGet() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && resp == nil {
				t.Error("handleGet() returned nil response")
			}
		})
	}
}

func TestToolHandlers_Execute(t *testing.T) {
	tests := []struct {
		name      string
		tool      *mockTool
		params    interface{}
		wantError bool
	}{
		{
			name: "valid execute",
			tool: &mockTool{
				name:        "test_tool",
				description: "Test tool",
				executeFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
					input, ok := inputs["input"].(string)
					if !ok {
						return nil, fmt.Errorf("input is required")
					}
					return map[string]interface{}{
						"output": "processed: " + input,
					}, nil
				},
			},
			params: map[string]interface{}{
				"name": "test_tool",
				"inputs": map[string]interface{}{
					"input": "test data",
				},
			},
			wantError: false,
		},
		{
			name: "missing tool name",
			tool: &mockTool{
				name:        "test_tool",
				description: "Test tool",
			},
			params: map[string]interface{}{
				"inputs": map[string]interface{}{
					"input": "test data",
				},
			},
			wantError: true,
		},
		{
			name: "tool not found",
			tool: &mockTool{
				name:        "test_tool",
				description: "Test tool",
			},
			params: map[string]interface{}{
				"name": "nonexistent_tool",
				"inputs": map[string]interface{}{
					"input": "test data",
				},
			},
			wantError: true,
		},
		{
			name: "missing required input",
			tool: &mockTool{
				name:        "test_tool",
				description: "Test tool",
			},
			params: map[string]interface{}{
				"name":   "test_tool",
				"inputs": map[string]interface{}{},
			},
			wantError: true,
		},
		{
			name: "tool execution error",
			tool: &mockTool{
				name:        "test_tool",
				description: "Test tool",
				executeFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
					return nil, fmt.Errorf("execution failed")
				},
			},
			params: map[string]interface{}{
				"name": "test_tool",
				"inputs": map[string]interface{}{
					"input": "test data",
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh registry for each test
			testRegistry := tools.NewRegistry()
			if err := testRegistry.Register(tt.tool); err != nil {
				t.Fatalf("Failed to register tool: %v", err)
			}

			handlers := NewToolHandlers(testRegistry)

			req := toolTestMessage("tool.execute", tt.params)
			resp, err := handlers.handleExecute(context.Background(), req)
			if (err != nil) != tt.wantError {
				t.Errorf("handleExecute() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && resp == nil {
				t.Error("handleExecute() returned nil response")
			}
		})
	}
}

func TestToolHandlers_Register(t *testing.T) {
	registry := tools.NewRegistry()
	handlers := NewToolHandlers(registry)
	rpcRegistry := NewRegistry()

	handlers.Register(rpcRegistry)

	methods := []string{
		"tool.list",
		"tool.execute",
		"tool.get",
	}

	for _, method := range methods {
		if !rpcRegistry.HasMethod(method) {
			t.Errorf("Method %s not registered", method)
		}
	}
}
