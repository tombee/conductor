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

package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tombee/conductor/internal/operation"
)

// mockRegistry is a test double for operation.Registry
type mockRegistry struct {
	operations map[string]mockOperation
}

type mockOperation struct {
	result interface{}
	err    error
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		operations: make(map[string]mockOperation),
	}
}

func (m *mockRegistry) Register(name string, result interface{}, err error) {
	m.operations[name] = mockOperation{result: result, err: err}
}

func (m *mockRegistry) Execute(ctx context.Context, reference string, inputs map[string]interface{}) (*operation.Result, error) {
	op, exists := m.operations[reference]
	if !exists {
		return nil, &operation.Error{
			Type:    operation.ErrorTypeValidation,
			Message: fmt.Sprintf("operation provider %q not found", reference),
		}
	}

	if op.err != nil {
		return nil, op.err
	}

	return &operation.Result{
		Response: op.result,
	}, nil
}

func (m *mockRegistry) List() []string {
	var names []string
	for name := range m.operations {
		names = append(names, name)
	}
	return names
}

func (m *mockRegistry) Get(name string) (operation.Provider, error) {
	if _, exists := m.operations[name]; !exists {
		return nil, fmt.Errorf("operation %q not found", name)
	}
	return nil, nil // We don't need to return actual provider for testing
}

func TestExecuteTools_Success(t *testing.T) {
	mockReg := newMockRegistry()
	mockReg.Register("file.read", map[string]interface{}{
		"content": "file contents here",
	}, nil)

	p := NewWithRegistry(mockReg)

	calls := []ToolCall{
		{
			ID:    "tool_123",
			Name:  "file.read",
			Input: json.RawMessage(`{"path": "/tmp/test.txt"}`),
		},
	}

	results := p.executeTools(context.Background(), calls)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].IsError {
		t.Errorf("expected success, got error: %s", results[0].Content)
	}

	if !strings.Contains(results[0].Content, "file contents here") {
		t.Errorf("result content doesn't contain expected data: %s", results[0].Content)
	}
}

func TestExecuteTools_UnknownTool(t *testing.T) {
	mockReg := newMockRegistry()
	mockReg.Register("file.read", "success", nil)

	p := NewWithRegistry(mockReg)

	calls := []ToolCall{
		{
			ID:    "tool_456",
			Name:  "unknown.tool",
			Input: json.RawMessage(`{}`),
		},
	}

	results := p.executeTools(context.Background(), calls)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !results[0].IsError {
		t.Error("expected error for unknown tool")
	}

	if !strings.Contains(results[0].Content, "unknown tool") {
		t.Errorf("error message should mention unknown tool, got: %s", results[0].Content)
	}

	if !strings.Contains(results[0].Content, "Available tools") {
		t.Errorf("error message should list available tools, got: %s", results[0].Content)
	}
}

func TestExecuteTools_NoRegistry(t *testing.T) {
	p := New() // No registry

	calls := []ToolCall{
		{
			ID:    "tool_789",
			Name:  "file.read",
			Input: json.RawMessage(`{"path": "/tmp/test.txt"}`),
		},
	}

	results := p.executeTools(context.Background(), calls)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !results[0].IsError {
		t.Error("expected error when registry not configured")
	}

	if !strings.Contains(results[0].Content, "not configured") {
		t.Errorf("error should mention configuration issue, got: %s", results[0].Content)
	}
}

func TestExecuteTools_InvalidInput(t *testing.T) {
	mockReg := newMockRegistry()
	mockReg.Register("file.read", "success", nil)

	p := NewWithRegistry(mockReg)

	calls := []ToolCall{
		{
			ID:    "tool_invalid",
			Name:  "file.read",
			Input: json.RawMessage(`{invalid json`),
		},
	}

	results := p.executeTools(context.Background(), calls)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !results[0].IsError {
		t.Error("expected error for invalid JSON input")
	}

	if !strings.Contains(results[0].Content, "invalid") {
		t.Errorf("error should mention invalid input, got: %s", results[0].Content)
	}
}

func TestExecuteTools_MultipleTools(t *testing.T) {
	mockReg := newMockRegistry()
	mockReg.Register("file.read", "file content", nil)
	mockReg.Register("shell.run", "command output", nil)

	p := NewWithRegistry(mockReg)

	calls := []ToolCall{
		{
			ID:    "tool_1",
			Name:  "file.read",
			Input: json.RawMessage(`{"path": "/tmp/test.txt"}`),
		},
		{
			ID:    "tool_2",
			Name:  "shell.run",
			Input: json.RawMessage(`{"command": "ls -la"}`),
		},
	}

	results := p.executeTools(context.Background(), calls)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].IsError {
		t.Errorf("first tool should succeed, got error: %s", results[0].Content)
	}

	if results[1].IsError {
		t.Errorf("second tool should succeed, got error: %s", results[1].Content)
	}

	if results[0].ID != "tool_1" {
		t.Errorf("first result ID mismatch: got %q, want %q", results[0].ID, "tool_1")
	}

	if results[1].ID != "tool_2" {
		t.Errorf("second result ID mismatch: got %q, want %q", results[1].ID, "tool_2")
	}
}

func TestFormatOperationResult(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		want   string
	}{
		{
			name:  "nil result",
			input: nil,
			want:  "Operation completed successfully",
		},
		{
			name:  "string result",
			input: "simple output",
			want:  "simple output",
		},
		{
			name: "struct result",
			input: map[string]interface{}{
				"status": "ok",
				"count":  42,
			},
			want: "{\n  \"count\": 42,\n  \"status\": \"ok\"\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatOperationResult(tt.input)
			if got != tt.want {
				t.Errorf("formatOperationResult() = %q, want %q", got, tt.want)
			}
		})
	}
}
