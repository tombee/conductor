package tools

import (
	"context"
	"testing"
)

// mockTool is a simple tool implementation for testing
type mockTool struct {
	name        string
	description string
	schema      *Schema
	executeFn   func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error)
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) Schema() *Schema {
	return m.schema
}

func (m *mockTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, inputs)
	}
	return map[string]interface{}{"result": "success"}, nil
}

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name    string
		tool    Tool
		wantErr bool
	}{
		{
			name: "valid tool",
			tool: &mockTool{
				name:        "test-tool",
				description: "A test tool",
				schema: &Schema{
					Inputs: &ParameterSchema{
						Type: "object",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "nil tool",
			tool:    nil,
			wantErr: true,
		},
		{
			name: "empty name",
			tool: &mockTool{
				name: "",
				schema: &Schema{
					Inputs: &ParameterSchema{
						Type: "object",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "nil schema",
			tool: &mockTool{
				name:   "test-tool",
				schema: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			err := r.Register(tt.tool)
			if (err != nil) != tt.wantErr {
				t.Errorf("Registry.Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()

	tool := &mockTool{
		name:        "test-tool",
		description: "A test tool",
		schema: &Schema{
			Inputs: &ParameterSchema{
				Type: "object",
			},
		},
	}

	// First registration should succeed
	if err := r.Register(tool); err != nil {
		t.Fatalf("First Register() failed: %v", err)
	}

	// Second registration with same name should fail
	if err := r.Register(tool); err == nil {
		t.Error("Second Register() should have failed with duplicate name")
	}
}

func TestRegistry_GetAndHas(t *testing.T) {
	r := NewRegistry()

	tool := &mockTool{
		name:        "test-tool",
		description: "A test tool",
		schema: &Schema{
			Inputs: &ParameterSchema{
				Type: "object",
			},
		},
	}

	// Tool should not exist initially
	if r.Has("test-tool") {
		t.Error("Has() returned true for unregistered tool")
	}

	// Get should fail for unregistered tool
	if _, err := r.Get("test-tool"); err == nil {
		t.Error("Get() should fail for unregistered tool")
	}

	// Register tool
	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Tool should exist now
	if !r.Has("test-tool") {
		t.Error("Has() returned false for registered tool")
	}

	// Get should succeed
	retrieved, err := r.Get("test-tool")
	if err != nil {
		t.Errorf("Get() failed: %v", err)
	}
	if retrieved.Name() != "test-tool" {
		t.Errorf("Get() returned wrong tool: got %s, want test-tool", retrieved.Name())
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()

	tool := &mockTool{
		name:        "test-tool",
		description: "A test tool",
		schema: &Schema{
			Inputs: &ParameterSchema{
				Type: "object",
			},
		},
	}

	// Register tool
	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Unregister should succeed
	if err := r.Unregister("test-tool"); err != nil {
		t.Errorf("Unregister() failed: %v", err)
	}

	// Tool should not exist after unregister
	if r.Has("test-tool") {
		t.Error("Has() returned true after Unregister()")
	}

	// Unregister non-existent tool should fail
	if err := r.Unregister("non-existent"); err == nil {
		t.Error("Unregister() should fail for non-existent tool")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	tools := []*mockTool{
		{
			name: "tool1",
			schema: &Schema{
				Inputs: &ParameterSchema{Type: "object"},
			},
		},
		{
			name: "tool2",
			schema: &Schema{
				Inputs: &ParameterSchema{Type: "object"},
			},
		},
		{
			name: "tool3",
			schema: &Schema{
				Inputs: &ParameterSchema{Type: "object"},
			},
		},
	}

	// Register all tools
	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			t.Fatalf("Register(%s) failed: %v", tool.name, err)
		}
	}

	// List should return all tool names
	names := r.List()
	if len(names) != len(tools) {
		t.Errorf("List() returned %d names, want %d", len(names), len(tools))
	}

	// Check all names are present
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}
	for _, tool := range tools {
		if !nameSet[tool.name] {
			t.Errorf("List() missing tool: %s", tool.name)
		}
	}
}

func TestRegistry_Execute(t *testing.T) {
	r := NewRegistry()

	executeCalled := false
	tool := &mockTool{
		name:        "test-tool",
		description: "A test tool",
		schema: &Schema{
			Inputs: &ParameterSchema{
				Type:     "object",
				Required: []string{"required-input"},
			},
		},
		executeFn: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			executeCalled = true
			return map[string]interface{}{"output": inputs["required-input"]}, nil
		},
	}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	tests := []struct {
		name    string
		inputs  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid inputs",
			inputs: map[string]interface{}{
				"required-input": "value",
			},
			wantErr: false,
		},
		{
			name:    "missing required input",
			inputs:  map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executeCalled = false
			ctx := context.Background()
			outputs, err := r.Execute(ctx, "test-tool", tt.inputs)

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && !executeCalled {
				t.Error("Execute() did not call tool's Execute method")
			}

			if !tt.wantErr && outputs == nil {
				t.Error("Execute() returned nil outputs")
			}
		})
	}
}

func TestRegistry_GetToolDescriptors(t *testing.T) {
	r := NewRegistry()

	tools := []*mockTool{
		{
			name:        "tool1",
			description: "First tool",
			schema: &Schema{
				Inputs: &ParameterSchema{Type: "object"},
			},
		},
		{
			name:        "tool2",
			description: "Second tool",
			schema: &Schema{
				Inputs: &ParameterSchema{Type: "object"},
			},
		},
	}

	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			t.Fatalf("Register() failed: %v", err)
		}
	}

	descriptors := r.GetToolDescriptors()
	if len(descriptors) != len(tools) {
		t.Errorf("GetToolDescriptors() returned %d descriptors, want %d", len(descriptors), len(tools))
	}

	// Check descriptors contain expected data
	descMap := make(map[string]ToolDescriptor)
	for _, desc := range descriptors {
		descMap[desc.Name] = desc
	}

	for _, tool := range tools {
		desc, ok := descMap[tool.name]
		if !ok {
			t.Errorf("GetToolDescriptors() missing descriptor for %s", tool.name)
			continue
		}
		if desc.Description != tool.description {
			t.Errorf("Descriptor for %s has wrong description: got %s, want %s",
				tool.name, desc.Description, tool.description)
		}
		if desc.Schema == nil {
			t.Errorf("Descriptor for %s has nil schema", tool.name)
		}
	}
}
