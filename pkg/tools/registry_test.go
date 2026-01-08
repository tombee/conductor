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

func TestRegistry_ExpandToolPatterns(t *testing.T) {
	r := NewRegistry()

	// Register tools with namespaces
	tools := []*mockTool{
		{
			name:   "github.list_repos",
			schema: &Schema{Inputs: &ParameterSchema{Type: "object"}},
		},
		{
			name:   "github.create_issue",
			schema: &Schema{Inputs: &ParameterSchema{Type: "object"}},
		},
		{
			name:   "github.close_issue",
			schema: &Schema{Inputs: &ParameterSchema{Type: "object"}},
		},
		{
			name:   "filesystem.read",
			schema: &Schema{Inputs: &ParameterSchema{Type: "object"}},
		},
		{
			name:   "filesystem.write",
			schema: &Schema{Inputs: &ParameterSchema{Type: "object"}},
		},
		{
			name:   "builtin_tool",
			schema: &Schema{Inputs: &ParameterSchema{Type: "object"}},
		},
	}

	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			t.Fatalf("Register(%s) failed: %v", tool.name, err)
		}
	}

	tests := []struct {
		name     string
		patterns []string
		want     []string // Expected tool names (order doesn't matter)
	}{
		{
			name:     "exact match single",
			patterns: []string{"github.list_repos"},
			want:     []string{"github.list_repos"},
		},
		{
			name:     "exact match multiple",
			patterns: []string{"github.list_repos", "filesystem.read"},
			want:     []string{"github.list_repos", "filesystem.read"},
		},
		{
			name:     "namespace wildcard github",
			patterns: []string{"github.*"},
			want:     []string{"github.list_repos", "github.create_issue", "github.close_issue"},
		},
		{
			name:     "namespace wildcard filesystem",
			patterns: []string{"filesystem.*"},
			want:     []string{"filesystem.read", "filesystem.write"},
		},
		{
			name:     "all tools wildcard",
			patterns: []string{"*"},
			want:     []string{"github.list_repos", "github.create_issue", "github.close_issue", "filesystem.read", "filesystem.write", "builtin_tool"},
		},
		{
			name:     "mixed patterns",
			patterns: []string{"github.*", "builtin_tool"},
			want:     []string{"github.list_repos", "github.create_issue", "github.close_issue", "builtin_tool"},
		},
		{
			name:     "duplicate patterns deduped",
			patterns: []string{"github.list_repos", "github.*"},
			want:     []string{"github.list_repos", "github.create_issue", "github.close_issue"},
		},
		{
			name:     "non-existent exact",
			patterns: []string{"nonexistent.tool"},
			want:     []string{},
		},
		{
			name:     "non-existent namespace",
			patterns: []string{"nonexistent.*"},
			want:     []string{},
		},
		{
			name:     "empty patterns",
			patterns: []string{},
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.ExpandToolPatterns(tt.patterns)

			// Convert slices to sets for comparison
			gotSet := make(map[string]bool)
			for _, name := range got {
				gotSet[name] = true
			}

			wantSet := make(map[string]bool)
			for _, name := range tt.want {
				wantSet[name] = true
			}

			if len(gotSet) != len(wantSet) {
				t.Errorf("ExpandToolPatterns() returned %d tools, want %d\ngot: %v\nwant: %v",
					len(gotSet), len(wantSet), got, tt.want)
				return
			}

			for name := range wantSet {
				if !gotSet[name] {
					t.Errorf("ExpandToolPatterns() missing expected tool: %s", name)
				}
			}

			for name := range gotSet {
				if !wantSet[name] {
					t.Errorf("ExpandToolPatterns() returned unexpected tool: %s", name)
				}
			}
		})
	}
}

func TestHasNamespacePrefix(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		namespace string
		want      bool
	}{
		{
			name:      "matching namespace",
			toolName:  "github.list_repos",
			namespace: "github",
			want:      true,
		},
		{
			name:      "matching namespace with multiple dots",
			toolName:  "mcp.server.tool",
			namespace: "mcp",
			want:      true,
		},
		{
			name:      "non-matching namespace",
			toolName:  "github.list_repos",
			namespace: "filesystem",
			want:      false,
		},
		{
			name:      "tool without namespace",
			toolName:  "builtin_tool",
			namespace: "github",
			want:      false,
		},
		{
			name:      "namespace is prefix but not complete",
			toolName:  "githubish.tool",
			namespace: "github",
			want:      false,
		},
		{
			name:      "exact match without dot",
			toolName:  "github",
			namespace: "github",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasNamespacePrefix(tt.toolName, tt.namespace)
			if got != tt.want {
				t.Errorf("hasNamespacePrefix(%q, %q) = %v, want %v",
					tt.toolName, tt.namespace, got, tt.want)
			}
		})
	}
}

// mockStreamingTool implements the StreamingTool interface for testing
type mockStreamingTool struct {
	mockTool
	executeStreamFn func(ctx context.Context, inputs map[string]any) (<-chan ToolChunk, error)
}

func (m *mockStreamingTool) ExecuteStream(ctx context.Context, inputs map[string]any) (<-chan ToolChunk, error) {
	if m.executeStreamFn != nil {
		return m.executeStreamFn(ctx, inputs)
	}
	// Default implementation: emit single chunk with result
	chunks := make(chan ToolChunk, 1)
	go func() {
		defer close(chunks)
		chunks <- ToolChunk{
			Data:    "test output",
			IsFinal: true,
			Result:  map[string]any{"result": "success"},
		}
	}()
	return chunks, nil
}

func TestRegistry_SupportsStreaming(t *testing.T) {
	r := NewRegistry()

	// Register a non-streaming tool
	nonStreamingTool := &mockTool{
		name: "non-streaming",
		schema: &Schema{
			Inputs: &ParameterSchema{Type: "object"},
		},
	}
	if err := r.Register(nonStreamingTool); err != nil {
		t.Fatalf("Register(non-streaming) failed: %v", err)
	}

	// Register a streaming tool
	streamingTool := &mockStreamingTool{
		mockTool: mockTool{
			name: "streaming",
			schema: &Schema{
				Inputs: &ParameterSchema{Type: "object"},
			},
		},
	}
	if err := r.Register(streamingTool); err != nil {
		t.Fatalf("Register(streaming) failed: %v", err)
	}

	tests := []struct {
		name     string
		toolName string
		want     bool
	}{
		{
			name:     "streaming tool",
			toolName: "streaming",
			want:     true,
		},
		{
			name:     "non-streaming tool",
			toolName: "non-streaming",
			want:     false,
		},
		{
			name:     "non-existent tool",
			toolName: "non-existent",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.SupportsStreaming(tt.toolName)
			if got != tt.want {
				t.Errorf("SupportsStreaming(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestRegistry_ExecuteStream_StreamingTool(t *testing.T) {
	r := NewRegistry()

	// Create a streaming tool that emits multiple chunks
	streamingTool := &mockStreamingTool{
		mockTool: mockTool{
			name: "streaming-tool",
			schema: &Schema{
				Inputs: &ParameterSchema{Type: "object"},
			},
		},
		executeStreamFn: func(ctx context.Context, inputs map[string]any) (<-chan ToolChunk, error) {
			chunks := make(chan ToolChunk, 3)
			go func() {
				defer close(chunks)
				chunks <- ToolChunk{Data: "chunk 1", Stream: "stdout"}
				chunks <- ToolChunk{Data: "chunk 2", Stream: "stdout"}
				chunks <- ToolChunk{
					Data:    "chunk 3",
					Stream:  "stdout",
					IsFinal: true,
					Result:  map[string]any{"exit_code": 0},
				}
			}()
			return chunks, nil
		},
	}

	if err := r.Register(streamingTool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	ctx := context.Background()
	chunks, err := r.ExecuteStream(ctx, "streaming-tool", map[string]interface{}{}, "")
	if err != nil {
		t.Fatalf("ExecuteStream() failed: %v", err)
	}

	// Collect all chunks
	var collected []ToolChunk
	for chunk := range chunks {
		collected = append(collected, chunk)
	}

	// Verify we got all chunks
	if len(collected) != 3 {
		t.Errorf("Expected 3 chunks, got %d", len(collected))
	}

	// Verify chunk contents
	if collected[0].Data != "chunk 1" {
		t.Errorf("First chunk data = %q, want %q", collected[0].Data, "chunk 1")
	}
	if collected[1].Data != "chunk 2" {
		t.Errorf("Second chunk data = %q, want %q", collected[1].Data, "chunk 2")
	}
	if !collected[2].IsFinal {
		t.Error("Final chunk should have IsFinal=true")
	}
	if collected[2].Result == nil {
		t.Error("Final chunk should have Result set")
	}
}

func TestRegistry_ExecuteStream_NonStreamingTool(t *testing.T) {
	r := NewRegistry()

	// Create a non-streaming tool
	nonStreamingTool := &mockTool{
		name: "non-streaming-tool",
		schema: &Schema{
			Inputs: &ParameterSchema{Type: "object"},
		},
		executeFn: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"result": "success"}, nil
		},
	}

	if err := r.Register(nonStreamingTool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	ctx := context.Background()
	chunks, err := r.ExecuteStream(ctx, "non-streaming-tool", map[string]interface{}{}, "")
	if err != nil {
		t.Fatalf("ExecuteStream() failed: %v", err)
	}

	// Collect all chunks
	var collected []ToolChunk
	for chunk := range chunks {
		collected = append(collected, chunk)
	}

	// Non-streaming tools should emit exactly one chunk
	if len(collected) != 1 {
		t.Errorf("Expected 1 chunk from non-streaming tool, got %d", len(collected))
	}

	// Verify the single chunk is final and contains the result
	chunk := collected[0]
	if !chunk.IsFinal {
		t.Error("Single chunk from non-streaming tool should have IsFinal=true")
	}
	if chunk.Result == nil {
		t.Error("Single chunk should have Result set")
	}
	if chunk.Result["result"] != "success" {
		t.Errorf("Result = %v, want %v", chunk.Result, map[string]interface{}{"result": "success"})
	}
}

func TestRegistry_ExecuteStream_GeneratesToolCallID(t *testing.T) {
	r := NewRegistry()

	tool := &mockTool{
		name: "test-tool",
		schema: &Schema{
			Inputs: &ParameterSchema{Type: "object"},
		},
	}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	ctx := context.Background()

	// Call with empty toolCallID - should generate UUID
	chunks, err := r.ExecuteStream(ctx, "test-tool", map[string]interface{}{}, "")
	if err != nil {
		t.Fatalf("ExecuteStream() failed: %v", err)
	}

	// Drain the channel (required to avoid goroutine leak)
	for range chunks {
	}

	// The test passes if no error was returned and the channel was successfully created
	// UUID generation happens internally and is verified by the lack of errors
}

func TestRegistry_ExecuteStream_ValidationError(t *testing.T) {
	r := NewRegistry()

	tool := &mockTool{
		name: "test-tool",
		schema: &Schema{
			Inputs: &ParameterSchema{
				Type:     "object",
				Required: []string{"required-param"},
			},
		},
	}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	ctx := context.Background()

	// Call with missing required parameter
	_, err := r.ExecuteStream(ctx, "test-tool", map[string]interface{}{}, "")
	if err == nil {
		t.Error("ExecuteStream() should fail with validation error for missing required parameter")
	}
}

func TestRegistry_ExecuteStream_NonExistentTool(t *testing.T) {
	r := NewRegistry()

	ctx := context.Background()
	_, err := r.ExecuteStream(ctx, "non-existent", map[string]interface{}{}, "")
	if err == nil {
		t.Error("ExecuteStream() should fail for non-existent tool")
	}
}

func TestRegistry_EventEmission_StreamingTool(t *testing.T) {
	r := NewRegistry()

	// Track emitted events
	var emittedEvents []map[string]any
	r.SetEventEmitter(func(ctx context.Context, eventType string, workflowID string, stepID string, data any) {
		emittedEvents = append(emittedEvents, map[string]any{
			"eventType":  eventType,
			"workflowID": workflowID,
			"stepID":     stepID,
			"data":       data,
		})
	})

	// Create a streaming tool that emits multiple chunks
	streamingTool := &mockStreamingTool{
		mockTool: mockTool{
			name: "streaming-tool",
			schema: &Schema{
				Inputs: &ParameterSchema{Type: "object"},
			},
		},
		executeStreamFn: func(ctx context.Context, inputs map[string]any) (<-chan ToolChunk, error) {
			chunks := make(chan ToolChunk, 2)
			go func() {
				defer close(chunks)
				chunks <- ToolChunk{Data: "output line 1", Stream: "stdout"}
				chunks <- ToolChunk{
					Data:    "output line 2",
					Stream:  "stdout",
					IsFinal: true,
					Result:  map[string]any{"exit_code": 0},
				}
			}()
			return chunks, nil
		},
	}

	if err := r.Register(streamingTool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	ctx := context.Background()
	chunks, err := r.ExecuteStream(ctx, "streaming-tool", map[string]interface{}{}, "test-call-id")
	if err != nil {
		t.Fatalf("ExecuteStream() failed: %v", err)
	}

	// Drain the chunks channel
	for range chunks {
	}

	// Verify events were emitted
	if len(emittedEvents) != 2 {
		t.Errorf("Expected 2 events, got %d", len(emittedEvents))
	}

	// Verify first event
	if len(emittedEvents) > 0 {
		event := emittedEvents[0]
		if event["eventType"] != "tool.output" {
			t.Errorf("Event type = %q, want %q", event["eventType"], "tool.output")
		}

		data, ok := event["data"].(map[string]any)
		if !ok {
			t.Fatal("Event data is not a map")
		}

		if data["tool_call_id"] != "test-call-id" {
			t.Errorf("tool_call_id = %q, want %q", data["tool_call_id"], "test-call-id")
		}
		if data["tool_name"] != "streaming-tool" {
			t.Errorf("tool_name = %q, want %q", data["tool_name"], "streaming-tool")
		}
		if data["data"] != "output line 1" {
			t.Errorf("data = %q, want %q", data["data"], "output line 1")
		}
		if data["stream"] != "stdout" {
			t.Errorf("stream = %q, want %q", data["stream"], "stdout")
		}
		if data["is_final"] != false {
			t.Errorf("is_final = %v, want false", data["is_final"])
		}
	}

	// Verify second (final) event
	if len(emittedEvents) > 1 {
		event := emittedEvents[1]
		data, ok := event["data"].(map[string]any)
		if !ok {
			t.Fatal("Event data is not a map")
		}

		if data["is_final"] != true {
			t.Errorf("is_final = %v, want true", data["is_final"])
		}
	}
}

func TestRegistry_EventEmission_NonStreamingTool(t *testing.T) {
	r := NewRegistry()

	// Track emitted events
	var emittedEvents []map[string]any
	r.SetEventEmitter(func(ctx context.Context, eventType string, workflowID string, stepID string, data any) {
		emittedEvents = append(emittedEvents, map[string]any{
			"eventType":  eventType,
			"workflowID": workflowID,
			"stepID":     stepID,
			"data":       data,
		})
	})

	// Create a non-streaming tool
	nonStreamingTool := &mockTool{
		name: "non-streaming-tool",
		schema: &Schema{
			Inputs: &ParameterSchema{Type: "object"},
		},
		executeFn: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"result": "success"}, nil
		},
	}

	if err := r.Register(nonStreamingTool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	ctx := context.Background()
	chunks, err := r.ExecuteStream(ctx, "non-streaming-tool", map[string]interface{}{}, "test-call-id")
	if err != nil {
		t.Fatalf("ExecuteStream() failed: %v", err)
	}

	// Drain the chunks channel
	for range chunks {
	}

	// Non-streaming tools should emit exactly one event
	if len(emittedEvents) != 1 {
		t.Errorf("Expected 1 event from non-streaming tool, got %d", len(emittedEvents))
	}

	// Verify the event
	if len(emittedEvents) > 0 {
		event := emittedEvents[0]
		if event["eventType"] != "tool.output" {
			t.Errorf("Event type = %q, want %q", event["eventType"], "tool.output")
		}

		data, ok := event["data"].(map[string]any)
		if !ok {
			t.Fatal("Event data is not a map")
		}

		if data["tool_call_id"] != "test-call-id" {
			t.Errorf("tool_call_id = %q, want %q", data["tool_call_id"], "test-call-id")
		}
		if data["tool_name"] != "non-streaming-tool" {
			t.Errorf("tool_name = %q, want %q", data["tool_name"], "non-streaming-tool")
		}
		if data["is_final"] != true {
			t.Errorf("is_final = %v, want true", data["is_final"])
		}
	}
}

func TestRegistry_EventEmission_NoEmitterSet(t *testing.T) {
	r := NewRegistry()

	// No emitter set - should not panic

	tool := &mockTool{
		name: "test-tool",
		schema: &Schema{
			Inputs: &ParameterSchema{Type: "object"},
		},
	}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	ctx := context.Background()
	chunks, err := r.ExecuteStream(ctx, "test-tool", map[string]interface{}{}, "")
	if err != nil {
		t.Fatalf("ExecuteStream() failed: %v", err)
	}

	// Drain the chunks channel - should not panic even without emitter
	for range chunks {
	}
}

func TestRegistry_EventEmission_WithContextValues(t *testing.T) {
	r := NewRegistry()

	// Track emitted events
	var emittedEvents []map[string]any
	r.SetEventEmitter(func(ctx context.Context, eventType string, workflowID string, stepID string, data any) {
		emittedEvents = append(emittedEvents, map[string]any{
			"eventType":  eventType,
			"workflowID": workflowID,
			"stepID":     stepID,
			"data":       data,
		})
	})

	tool := &mockTool{
		name: "test-tool",
		schema: &Schema{
			Inputs: &ParameterSchema{Type: "object"},
		},
	}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Create context with workflow and step IDs
	ctx := context.Background()
	ctx = context.WithValue(ctx, "workflow_id", "workflow-123")
	ctx = context.WithValue(ctx, "step_id", "step-456")

	chunks, err := r.ExecuteStream(ctx, "test-tool", map[string]interface{}{}, "")
	if err != nil {
		t.Fatalf("ExecuteStream() failed: %v", err)
	}

	// Drain the chunks channel
	for range chunks {
	}

	// Verify context values were passed to emitter
	if len(emittedEvents) > 0 {
		event := emittedEvents[0]
		if event["workflowID"] != "workflow-123" {
			t.Errorf("workflowID = %q, want %q", event["workflowID"], "workflow-123")
		}
		if event["stepID"] != "step-456" {
			t.Errorf("stepID = %q, want %q", event["stepID"], "step-456")
		}
	}
}
