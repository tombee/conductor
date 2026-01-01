package sdk

import (
	"context"
	"os"
	"testing"
)

// TestWithBuiltinActions verifies that WithBuiltinActions() enables builtin actions.
func TestWithBuiltinActions(t *testing.T) {
	s, err := New(
		WithBuiltinActions(),
	)
	if err != nil {
		t.Fatalf("New() with WithBuiltinActions failed: %v", err)
	}
	defer s.Close()

	if !s.builtinActionsEnabled {
		t.Error("WithBuiltinActions() did not enable builtin actions")
	}
}

// TestWithBuiltinIntegrations verifies that WithBuiltinIntegrations() enables builtin integrations.
func TestWithBuiltinIntegrations(t *testing.T) {
	s, err := New(
		WithBuiltinIntegrations(),
	)
	if err != nil {
		t.Fatalf("New() with WithBuiltinIntegrations failed: %v", err)
	}
	defer s.Close()

	if !s.builtinIntegrationsEnabled {
		t.Error("WithBuiltinIntegrations() did not enable builtin integrations")
	}
}

// TestFuncTool verifies that FuncTool creates a working tool.
func TestFuncTool(t *testing.T) {
	tool := FuncTool(
		"test_tool",
		"A test tool",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{"type": "string"},
			},
			"required": []string{"input"},
		},
		func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{
				"output": "processed",
			}, nil
		},
	)

	if tool.Name() != "test_tool" {
		t.Errorf("FuncTool name = %s, want test_tool", tool.Name())
	}

	result, err := tool.Execute(context.Background(), map[string]any{"input": "test"})
	if err != nil {
		t.Fatalf("tool.Execute() failed: %v", err)
	}

	if result["output"] != "processed" {
		t.Errorf("tool result = %v, want {output: processed}", result)
	}
}

// TestRegisterTool verifies tool registration and unregistration.
func TestRegisterTool(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	tool := FuncTool(
		"custom_tool",
		"Custom tool",
		map[string]any{"type": "object"},
		func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{}, nil
		},
	)

	// Register tool
	if err := s.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool() failed: %v", err)
	}

	// Unregister tool
	if err := s.UnregisterTool("custom_tool"); err != nil {
		t.Fatalf("UnregisterTool() failed: %v", err)
	}
}

// TestLoadWorkflow verifies basic YAML workflow loading.
func TestLoadWorkflow(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	yamlContent := []byte(`
name: test-workflow
inputs:
  - name: message
    type: string
steps:
  - id: greet
    model: claude-sonnet-4-20250514
    prompt: "Say: {{.inputs.message}}"
`)

	wf, err := s.LoadWorkflow(yamlContent)
	if err != nil {
		t.Fatalf("LoadWorkflow() failed: %v", err)
	}

	if wf.Name != "test-workflow" {
		t.Errorf("workflow name = %s, want test-workflow", wf.Name)
	}
}

// TestLoadWorkflowFile verifies loading from a file.
func TestLoadWorkflowFile(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	// Create a temporary workflow file
	tmpfile, err := os.CreateTemp("", "workflow-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := []byte(`
name: file-workflow
steps:
  - id: step1
    model: claude-sonnet-4-20250514
    prompt: "Hello"
`)

	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	wf, err := s.LoadWorkflowFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadWorkflowFile() failed: %v", err)
	}

	if wf.Name != "file-workflow" {
		t.Errorf("workflow name = %s, want file-workflow", wf.Name)
	}
}

// TestLoadWorkflowWithWarnings verifies warning detection for platform features.
func TestLoadWorkflowWithWarnings(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	yamlContent := []byte(`
name: platform-workflow
trigger:
  webhook:
    path: /webhook
steps:
  - id: step1
    model: claude-sonnet-4-20250514
    prompt: "Hello"
`)

	wf, warnings, err := s.LoadWorkflowWithWarnings(yamlContent)
	if err != nil {
		t.Fatalf("LoadWorkflowWithWarnings() failed: %v", err)
	}

	if wf.Name != "platform-workflow" {
		t.Errorf("workflow name = %s, want platform-workflow", wf.Name)
	}

	if len(warnings) == 0 {
		t.Error("expected warnings for listen config, got none")
	}
}

// TestLoadWorkflowSizeLimits verifies security limits on file size.
func TestLoadWorkflowSizeLimits(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	// Create a workflow larger than 10MB
	largeContent := make([]byte, 11*1024*1024)
	_, err = s.LoadWorkflow(largeContent)
	if err == nil {
		t.Error("LoadWorkflow() should reject files larger than 10MB")
	}
}

// TestExtendWorkflow verifies workflow extension.
func TestExtendWorkflow(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	// Load base workflow
	baseYAML := []byte(`
name: base-workflow
inputs:
  - name: input1
    type: string
steps:
  - id: step1
    model: claude-sonnet-4-20250514
    prompt: "First step"
`)

	baseWf, err := s.LoadWorkflow(baseYAML)
	if err != nil {
		t.Fatalf("LoadWorkflow() failed: %v", err)
	}

	// Extend it
	extendedWf, err := s.ExtendWorkflow(baseWf).
		Step("step2").LLM().
		Model("claude-sonnet-4-20250514").
		Prompt("Second step").
		Done().
		Build()

	if err != nil {
		t.Fatalf("ExtendWorkflow().Build() failed: %v", err)
	}

	if extendedWf.Name != "base-workflow" {
		t.Errorf("extended workflow name = %s, want base-workflow", extendedWf.Name)
	}

	// Should have 2 steps now (1 from YAML + 1 added)
	if extendedWf.StepCount() != 2 {
		t.Errorf("extended workflow steps = %d, want 2", extendedWf.StepCount())
	}
}

// TestMCPServerConfiguration verifies MCP server config storage.
func TestMCPServerConfiguration(t *testing.T) {
	config := MCPConfig{
		Transport:      "stdio",
		Command:        "test-mcp",
		Args:           []string{"--arg1"},
		ConnectTimeout: 5000,
	}

	s, err := New(
		WithMCPServer("test-server", config),
	)
	if err != nil {
		t.Fatalf("New() with WithMCPServer failed: %v", err)
	}
	defer s.Close()

	s.mcpMu.RLock()
	storedConfig, exists := s.mcpServers["test-server"]
	s.mcpMu.RUnlock()

	if !exists {
		t.Error("MCP server config not stored")
	}

	if storedConfig.Command != "test-mcp" {
		t.Errorf("MCP server command = %s, want test-mcp", storedConfig.Command)
	}
}

