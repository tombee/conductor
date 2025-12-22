package builtin

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestShellTool_Name(t *testing.T) {
	tool := NewShellTool()
	if tool.Name() != "shell" {
		t.Errorf("Name() = %s, want shell", tool.Name())
	}
}

func TestShellTool_Description(t *testing.T) {
	tool := NewShellTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestShellTool_Schema(t *testing.T) {
	tool := NewShellTool()
	schema := tool.Schema()

	if schema == nil {
		t.Fatal("Schema() returned nil")
	}

	if schema.Inputs == nil {
		t.Fatal("Schema inputs is nil")
	}

	// Check required fields
	found := false
	for _, field := range schema.Inputs.Required {
		if field == "command" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Required field 'command' not in schema")
	}
}

func TestShellTool_SimpleCommand(t *testing.T) {
	tool := NewShellTool()
	ctx := context.Background()

	// Use a simple command that works on all platforms
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "cmd"
	} else {
		cmd = "echo"
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"command": cmd,
		"args":    []interface{}{"hello"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	success, ok := result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if !success && runtime.GOOS != "windows" {
		t.Errorf("Command should have succeeded: %v", result)
	}

	if _, ok := result["stdout"]; !ok {
		t.Error("Result should contain stdout")
	}
}

func TestShellTool_CommandWithTimeout(t *testing.T) {
	tool := NewShellTool().WithTimeout(100 * time.Millisecond)
	ctx := context.Background()

	// Use a command that will timeout
	var cmd string
	var args []interface{}
	if runtime.GOOS == "windows" {
		cmd = "timeout"
		args = []interface{}{"5"}
	} else {
		cmd = "sleep"
		args = []interface{}{"5"}
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"command": cmd,
		"args":    args,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	status, ok := result["status"].(string)
	if !ok {
		t.Fatal("status field is not a string")
	}

	if status != "timeout" {
		t.Errorf("Status = %s, want timeout", status)
	}

	success, ok := result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if success {
		t.Error("Command should not have succeeded after timeout")
	}
}

func TestShellTool_InvalidCommand(t *testing.T) {
	tool := NewShellTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"command": "nonexistent-command-12345",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	success, ok := result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if success {
		t.Error("Invalid command should not succeed")
	}

	status, ok := result["status"].(string)
	if !ok {
		t.Fatal("status field is not a string")
	}

	if status != "error" {
		t.Errorf("Status = %s, want error", status)
	}
}

func TestShellTool_WithWorkingDir(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewShellTool().WithWorkingDir(tmpDir)

	if tool.workingDir != tmpDir {
		t.Errorf("workingDir = %s, want %s", tool.workingDir, tmpDir)
	}
}

func TestShellTool_AllowedCommands(t *testing.T) {
	tool := NewShellTool().WithAllowedCommands([]string{"echo", "ls"})
	ctx := context.Background()

	tests := []struct {
		name      string
		command   string
		shouldErr bool
	}{
		{
			name:      "allowed command",
			command:   "echo",
			shouldErr: false,
		},
		{
			name:      "disallowed command",
			command:   "cat",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, map[string]interface{}{
				"command": tt.command,
			})

			if tt.shouldErr && err == nil {
				t.Error("Execute() should have failed for disallowed command")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Execute() unexpected error: %v", err)
			}
		})
	}
}

func TestShellTool_InvalidInputs(t *testing.T) {
	tool := NewShellTool()
	ctx := context.Background()

	tests := []struct {
		name   string
		inputs map[string]interface{}
	}{
		{
			name:   "missing command",
			inputs: map[string]interface{}{},
		},
		{
			name: "invalid command type",
			inputs: map[string]interface{}{
				"command": 123,
			},
		},
		{
			name: "invalid args type",
			inputs: map[string]interface{}{
				"command": "echo",
				"args":    "not an array",
			},
		},
		{
			name: "invalid arg element type",
			inputs: map[string]interface{}{
				"command": "echo",
				"args":    []interface{}{123},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, tt.inputs)
			if err == nil {
				t.Error("Execute() should fail with invalid inputs")
			}
		})
	}
}

func TestShellTool_CaptureStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping stderr test on Windows")
	}

	tool := NewShellTool()
	ctx := context.Background()

	// Command that writes to stderr
	result, err := tool.Execute(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", "echo error >&2"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	stderr, ok := result["stderr"].(string)
	if !ok {
		t.Fatal("stderr field is not a string")
	}

	if stderr == "" {
		t.Error("stderr should contain error output")
	}
}

func TestShellTool_ExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping exit code test on Windows")
	}

	tool := NewShellTool()
	ctx := context.Background()

	// Command that exits with non-zero code
	result, err := tool.Execute(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", "exit 42"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	exitCode, ok := result["exit_code"].(int)
	if !ok {
		t.Fatal("exit_code field is not an int")
	}

	if exitCode != 42 {
		t.Errorf("exit_code = %d, want 42", exitCode)
	}

	success, ok := result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if success {
		t.Error("Command with non-zero exit code should not succeed")
	}
}
