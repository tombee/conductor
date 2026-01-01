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

package tools

import (
	"context"
	"errors"
	"testing"
)

// mockInterceptor implements the Interceptor interface for testing
type mockInterceptor struct {
	interceptCalled   bool
	postExecuteCalled bool
	shouldBlockAccess bool
}

func (m *mockInterceptor) Intercept(ctx context.Context, tool Tool, inputs map[string]interface{}) error {
	m.interceptCalled = true
	if m.shouldBlockAccess {
		return errors.New("access denied by interceptor")
	}
	return nil
}

func (m *mockInterceptor) PostExecute(ctx context.Context, tool Tool, outputs map[string]interface{}, err error) {
	m.postExecuteCalled = true
}

// interceptedTool implements the Tool interface for testing interceptor integration
type interceptedTool struct {
	name        string
	description string
	executed    bool
}

func (m *interceptedTool) Name() string {
	return m.name
}

func (m *interceptedTool) Description() string {
	return m.description
}

func (m *interceptedTool) Schema() *Schema {
	return &Schema{
		Inputs: &ParameterSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type: "string",
				},
			},
		},
		Outputs: &ParameterSchema{
			Type: "object",
			Properties: map[string]*Property{
				"output": {
					Type: "string",
				},
			},
		},
	}
}

func (m *interceptedTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	m.executed = true
	return map[string]interface{}{
		"output": "test output",
	}, nil
}

func TestRegistryWithInterceptor(t *testing.T) {
	registry := NewRegistry()
	tool := &interceptedTool{
		name:        "test-tool",
		description: "A test tool",
	}

	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	interceptor := &mockInterceptor{
		shouldBlockAccess: false,
	}
	registry.SetInterceptor(interceptor)

	ctx := context.Background()
	inputs := map[string]interface{}{
		"input": "test input",
	}

	outputs, err := registry.Execute(ctx, "test-tool", inputs)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !interceptor.interceptCalled {
		t.Error("Interceptor.Intercept was not called")
	}

	if !interceptor.postExecuteCalled {
		t.Error("Interceptor.PostExecute was not called")
	}

	if !tool.executed {
		t.Error("Tool was not executed")
	}

	if outputs == nil {
		t.Error("Execute returned nil outputs")
	}
}

func TestRegistryWithBlockingInterceptor(t *testing.T) {
	registry := NewRegistry()
	tool := &interceptedTool{
		name:        "test-tool",
		description: "A test tool",
	}

	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	interceptor := &mockInterceptor{
		shouldBlockAccess: true,
	}
	registry.SetInterceptor(interceptor)

	ctx := context.Background()
	inputs := map[string]interface{}{
		"input": "test input",
	}

	_, err = registry.Execute(ctx, "test-tool", inputs)
	if err == nil {
		t.Fatal("Execute should have failed with blocked access")
	}

	if !interceptor.interceptCalled {
		t.Error("Interceptor.Intercept was not called")
	}

	if tool.executed {
		t.Error("Tool should not have been executed when access is blocked")
	}

	// PostExecute should still be called even when intercept blocks
	// Actually, looking at the implementation, PostExecute is only called after Execute
	// So if Intercept fails, PostExecute won't be called - this is correct behavior
	if interceptor.postExecuteCalled {
		t.Error("Interceptor.PostExecute should not be called when access is blocked")
	}
}

func TestRegistryWithoutInterceptor(t *testing.T) {
	registry := NewRegistry()
	tool := &interceptedTool{
		name:        "test-tool",
		description: "A test tool",
	}

	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// No interceptor set

	ctx := context.Background()
	inputs := map[string]interface{}{
		"input": "test input",
	}

	outputs, err := registry.Execute(ctx, "test-tool", inputs)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !tool.executed {
		t.Error("Tool was not executed")
	}

	if outputs == nil {
		t.Error("Execute returned nil outputs")
	}
}
