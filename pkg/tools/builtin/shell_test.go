package builtin

import (
	"bytes"
	"context"
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/security"
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

	// Allow shell expansion for this test (uses >&2 redirection)
	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig)
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

// TestValidateCommand_EmptyAllowlist tests that empty allowedCommands allows any command
func TestValidateCommand_EmptyAllowlist(t *testing.T) {
	tool := NewShellTool()

	tests := []string{
		"git",
		"/usr/bin/git",
		"/tmp/evil/git",
		"echo",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			err := tool.validateCommand(cmd)
			if err != nil {
				t.Errorf("validateCommand(%q) with empty allowlist should allow command, got error: %v", cmd, err)
			}
		})
	}
}

// TestValidateCommand_ExactMatch tests exact command matching
func TestValidateCommand_ExactMatch(t *testing.T) {
	tool := NewShellTool().WithAllowedCommands([]string{"git", "/usr/bin/python3"})

	tests := []struct {
		name      string
		command   string
		shouldErr bool
	}{
		{"allowed base command", "git", false},
		{"allowed full path", "/usr/bin/python3", false},
		{"disallowed command", "rm", true},
		{"disallowed command", "cat", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.validateCommand(tt.command)
			if tt.shouldErr && err == nil {
				t.Errorf("validateCommand(%q) should have returned error", tt.command)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("validateCommand(%q) unexpected error: %v", tt.command, err)
			}
		})
	}
}

// TestValidateCommand_BaseNameMatch tests base command name matching
func TestValidateCommand_BaseNameMatch(t *testing.T) {
	tool := NewShellTool().WithAllowedCommands([]string{"/usr/bin/git"})

	// When allowed list has full path, base command should be allowed
	err := tool.validateCommand("git")
	if err != nil {
		t.Errorf("validateCommand('git') should be allowed when '/usr/bin/git' is in allowlist, got error: %v", err)
	}
}

// TestValidateCommand_PathShadowing tests that path shadowing is blocked
func TestValidateCommand_PathShadowing(t *testing.T) {
	tool := NewShellTool().WithAllowedCommands([]string{"git"})

	tests := []string{
		"/tmp/evil/git",
		"/home/attacker/bin/git",
		"/var/tmp/git",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			err := tool.validateCommand(cmd)
			if err == nil {
				t.Errorf("validateCommand(%q) should block path shadowing when only 'git' is allowed", cmd)
			}
			if err != nil && !strings.Contains(err.Error(), "command execution blocked by policy") {
				t.Errorf("validateCommand(%q) should return generic error message, got: %v", cmd, err)
			}
		})
	}
}

// TestValidateCommand_PathTraversal tests that path traversal is blocked
func TestValidateCommand_PathTraversal(t *testing.T) {
	tool := NewShellTool().WithAllowedCommands([]string{"git"})

	tests := []string{
		"../bin/git",
		"/tmp/../usr/bin/git",
		"/usr/bin/../bin/git",
		"..\\bin\\git", // Windows-style
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			err := tool.validateCommand(cmd)
			if err == nil {
				t.Errorf("validateCommand(%q) should block path traversal", cmd)
			}
			if err != nil && !strings.Contains(err.Error(), "command execution blocked by policy") {
				t.Errorf("validateCommand(%q) should return generic error message, got: %v", cmd, err)
			}
		})
	}
}

// TestValidateCommand_RelativePaths tests that relative paths are blocked
func TestValidateCommand_RelativePaths(t *testing.T) {
	tool := NewShellTool().WithAllowedCommands([]string{"git"})

	tests := []string{
		"./git",
		"./bin/git",
		"../git",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			err := tool.validateCommand(cmd)
			if err == nil {
				t.Errorf("validateCommand(%q) should block relative paths", cmd)
			}
			if err != nil && !strings.Contains(err.Error(), "command execution blocked by policy") {
				t.Errorf("validateCommand(%q) should return generic error message, got: %v", cmd, err)
			}
		})
	}
}

// TestValidateCommand_InvalidCommands tests handling of invalid command names
func TestValidateCommand_InvalidCommands(t *testing.T) {
	tool := NewShellTool().WithAllowedCommands([]string{"git"})

	tests := []string{
		"",
		".",
		"..",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			err := tool.validateCommand(cmd)
			if err == nil {
				t.Errorf("validateCommand(%q) should reject invalid command", cmd)
			}
		})
	}
}

// TestValidateCommand_GenericErrorMessage tests that error messages don't leak information
func TestValidateCommand_GenericErrorMessage(t *testing.T) {
	tool := NewShellTool().WithAllowedCommands([]string{"git", "echo"})

	tests := []struct {
		name    string
		command string
	}{
		{"disallowed command", "rm"},
		{"path shadowing", "/tmp/evil/git"},
		{"relative path", "./git"},
		{"path traversal", "../bin/git"},
	}

	// All blocked commands should return the same generic error message
	expectedError := "command execution blocked by policy"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.validateCommand(tt.command)
			if err == nil {
				t.Errorf("validateCommand(%q) should return error", tt.command)
				return
			}

			if err.Error() != expectedError {
				t.Errorf("validateCommand(%q) error = %q, want %q", tt.command, err.Error(), expectedError)
			}

			// Ensure error message doesn't contain the command name
			if strings.Contains(err.Error(), tt.command) {
				t.Errorf("Error message should not contain command name %q, got: %v", tt.command, err)
			}
		})
	}
}

// TestValidateCommand_Logging tests that blocked commands are logged
func TestValidateCommand_Logging(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(oldLogger)

	tool := NewShellTool().WithAllowedCommands([]string{"git"})

	tests := []struct {
		name          string
		command       string
		expectedReason string
	}{
		{"path traversal", "../bin/git", "path_traversal"},
		{"not in allowlist", "rm", "not_in_allowlist"},
		{"path shadowing", "/tmp/evil/git", "not_in_allowlist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			err := tool.validateCommand(tt.command)
			if err == nil {
				t.Errorf("validateCommand(%q) should return error", tt.command)
				return
			}

			logOutput := buf.String()
			if !strings.Contains(logOutput, "shell command blocked") {
				t.Errorf("Log should contain 'shell command blocked', got: %s", logOutput)
			}

			if !strings.Contains(logOutput, tt.expectedReason) {
				t.Errorf("Log should contain reason %q, got: %s", tt.expectedReason, logOutput)
			}

			if !strings.Contains(logOutput, tt.command) {
				t.Errorf("Log should contain command %q for audit purposes, got: %s", tt.command, logOutput)
			}
		})
	}
}

// TestValidateCommand_CaseSensitivity tests case-sensitive matching on Unix
func TestValidateCommand_CaseSensitivity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping case sensitivity test on Windows")
	}

	tool := NewShellTool().WithAllowedCommands([]string{"git"})

	// "Git" should not match "git" on Unix
	err := tool.validateCommand("Git")
	if err == nil {
		t.Error("validateCommand('Git') should not match 'git' on Unix (case-sensitive)")
	}
}
