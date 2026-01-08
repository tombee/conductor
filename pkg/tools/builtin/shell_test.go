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
	"github.com/tombee/conductor/pkg/tools"
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
		name           string
		command        string
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

// TestShellTool_ExecuteStream_BasicStreaming tests basic streaming functionality
func TestShellTool_ExecuteStream_BasicStreaming(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping streaming test on Windows")
	}

	tool := NewShellTool()
	ctx := context.Background()

	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "echo",
		"args":    []interface{}{"hello"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var receivedChunks []string
	var finalChunk *struct {
		result map[string]interface{}
		found  bool
	}
	finalChunk = &struct {
		result map[string]interface{}
		found  bool
	}{}

	for chunk := range chunks {
		if chunk.IsFinal {
			finalChunk.found = true
			finalChunk.result = chunk.Result
		} else {
			receivedChunks = append(receivedChunks, chunk.Data)
			if chunk.Stream != "stdout" && chunk.Stream != "stderr" {
				t.Errorf("Chunk stream = %s, expected stdout or stderr", chunk.Stream)
			}
		}
	}

	if !finalChunk.found {
		t.Fatal("No final chunk received")
	}

	if finalChunk.result == nil {
		t.Fatal("Final chunk result is nil")
	}

	// Verify final chunk contains required fields
	if _, ok := finalChunk.result["exit_code"]; !ok {
		t.Error("Final chunk result missing exit_code")
	}

	if _, ok := finalChunk.result["duration"]; !ok {
		t.Error("Final chunk result missing duration")
	}

	if _, ok := finalChunk.result["status"]; !ok {
		t.Error("Final chunk result missing status")
	}

	success, ok := finalChunk.result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if !success {
		t.Errorf("Command should have succeeded: %v", finalChunk.result)
	}
}

// TestShellTool_ExecuteStream_StdoutStderr tests separate stdout/stderr streaming
func TestShellTool_ExecuteStream_StdoutStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping stdout/stderr test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig)
	ctx := context.Background()

	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", "echo stdout-line; echo stderr-line >&2"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var stdoutChunks []string
	var stderrChunks []string

	for chunk := range chunks {
		if chunk.IsFinal {
			continue
		}

		switch chunk.Stream {
		case "stdout":
			stdoutChunks = append(stdoutChunks, chunk.Data)
		case "stderr":
			stderrChunks = append(stderrChunks, chunk.Data)
		}
	}

	if len(stdoutChunks) == 0 {
		t.Error("No stdout chunks received")
	}

	if len(stderrChunks) == 0 {
		t.Error("No stderr chunks received")
	}

	// Verify content
	stdoutText := strings.Join(stdoutChunks, "\n")
	if !strings.Contains(stdoutText, "stdout-line") {
		t.Errorf("stdout does not contain expected text, got: %s", stdoutText)
	}

	stderrText := strings.Join(stderrChunks, "\n")
	if !strings.Contains(stderrText, "stderr-line") {
		t.Errorf("stderr does not contain expected text, got: %s", stderrText)
	}
}

// TestShellTool_ExecuteStream_ExitCode tests that exit code is captured in final chunk
func TestShellTool_ExecuteStream_ExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping exit code test on Windows")
	}

	tool := NewShellTool()
	ctx := context.Background()

	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", "exit 42"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var finalResult map[string]interface{}
	for chunk := range chunks {
		if chunk.IsFinal {
			finalResult = chunk.Result
		}
	}

	if finalResult == nil {
		t.Fatal("No final result received")
	}

	exitCode, ok := finalResult["exit_code"].(int)
	if !ok {
		t.Fatalf("exit_code is not an int: %T", finalResult["exit_code"])
	}

	if exitCode != 42 {
		t.Errorf("exit_code = %d, want 42", exitCode)
	}

	success, ok := finalResult["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if success {
		t.Error("Command with non-zero exit code should not succeed")
	}
}

// TestShellTool_ExecuteStream_Duration tests that duration is included in final chunk
func TestShellTool_ExecuteStream_Duration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping duration test on Windows")
	}

	tool := NewShellTool()
	ctx := context.Background()

	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "echo",
		"args":    []interface{}{"test"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var finalResult map[string]interface{}
	for chunk := range chunks {
		if chunk.IsFinal {
			finalResult = chunk.Result
		}
	}

	if finalResult == nil {
		t.Fatal("No final result received")
	}

	duration, ok := finalResult["duration"].(int64)
	if !ok {
		t.Fatalf("duration is not an int64: %T", finalResult["duration"])
	}

	if duration < 0 {
		t.Errorf("duration = %d, should be non-negative", duration)
	}
}

// TestShellTool_ExecuteStream_InvalidInputs tests error handling for invalid inputs
func TestShellTool_ExecuteStream_InvalidInputs(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.ExecuteStream(ctx, tt.inputs)
			if err == nil {
				t.Error("ExecuteStream() should fail with invalid inputs")
			}
		})
	}
}

// TestShellTool_ExecuteStream_MultipleLines tests streaming of multiple output lines
func TestShellTool_ExecuteStream_MultipleLines(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping multi-line test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig)
	ctx := context.Background()

	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", "echo line1; echo line2; echo line3"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var outputLines []string
	for chunk := range chunks {
		if !chunk.IsFinal && chunk.Stream == "stdout" {
			outputLines = append(outputLines, chunk.Data)
		}
	}

	if len(outputLines) < 3 {
		t.Errorf("Expected at least 3 output lines, got %d", len(outputLines))
	}

	// Verify we got the expected lines
	output := strings.Join(outputLines, "\n")
	for _, expected := range []string{"line1", "line2", "line3"} {
		if !strings.Contains(output, expected) {
			t.Errorf("Output does not contain %s, got: %s", expected, output)
		}
	}
}

// TestShellTool_ExecuteStream_ChannelClosed tests that channel is properly closed
func TestShellTool_ExecuteStream_ChannelClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping channel test on Windows")
	}

	tool := NewShellTool()
	ctx := context.Background()

	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "echo",
		"args":    []interface{}{"test"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	// Drain the channel
	chunkCount := 0
	for range chunks {
		chunkCount++
	}

	if chunkCount == 0 {
		t.Error("Expected at least one chunk (final chunk)")
	}

	// Try to read from closed channel - should return immediately with zero value
	chunk, ok := <-chunks
	if ok {
		t.Errorf("Channel should be closed, but received chunk: %+v", chunk)
	}
}

// TestShellTool_ExecuteStream_BinaryFallback tests 4KB fallback for binary data without newlines
func TestShellTool_ExecuteStream_BinaryFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping binary fallback test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig)
	ctx := context.Background()

	// Generate 5KB of data without newlines to test 4KB fallback
	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", "printf '%5120s' | tr ' ' 'x'"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var dataChunks []string
	for chunk := range chunks {
		if !chunk.IsFinal && chunk.Stream == "stdout" {
			dataChunks = append(dataChunks, chunk.Data)
		}
	}

	// Should have at least 2 chunks: one 4KB chunk and one for remaining data
	if len(dataChunks) < 2 {
		t.Errorf("Expected at least 2 chunks for 5KB binary data, got %d", len(dataChunks))
	}

	// First chunk should be around 4KB (4096 bytes)
	if len(dataChunks[0]) < 4000 {
		t.Errorf("First chunk size = %d, expected around 4KB", len(dataChunks[0]))
	}
}

// TestShellTool_ExecuteStream_ContextCancellation tests cleanup on context cancellation
func TestShellTool_ExecuteStream_ContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping context cancellation test on Windows")
	}

	tool := NewShellTool()
	ctx, cancel := context.WithCancel(context.Background())

	// Start a long-running command
	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sleep",
		"args":    []interface{}{"10"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	// Cancel context after a short delay
	time.AfterFunc(100*time.Millisecond, cancel)

	// Drain the channel
	var receivedFinal bool
	for chunk := range chunks {
		if chunk.IsFinal {
			receivedFinal = true
		}
	}

	// Should receive final chunk even after cancellation
	if !receivedFinal {
		t.Error("Should receive final chunk after context cancellation")
	}
}

// TestShellTool_ExecuteStream_Redaction tests sensitive data redaction in output
func TestShellTool_ExecuteStream_Redaction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping redaction test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig)
	ctx := context.Background()

	tests := []struct {
		name     string
		output   string
		contains string
		notContains string
	}{
		{
			name:        "AWS access key",
			output:      "AWS Key: AKIAIOSFODNN7EXAMPLE",
			contains:    "[REDACTED]",
			notContains: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:        "Bearer token",
			output:      "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			contains:    "[REDACTED]",
			notContains: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:        "Password in URL",
			output:      "Database: postgresql://user:secretpass123@localhost/db",
			contains:    "[REDACTED]",
			notContains: "secretpass123",
		},
		{
			name:        "API key",
			output:      "API_KEY=sk_live_1234567890abcdefghij",
			contains:    "[REDACTED]",
			notContains: "sk_live_1234567890abcdefghij",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
				"command": "echo",
				"args":    []interface{}{tt.output},
			})
			if err != nil {
				t.Fatalf("ExecuteStream() error = %v", err)
			}

			var outputData []string
			for chunk := range chunks {
				if !chunk.IsFinal && chunk.Stream == "stdout" {
					outputData = append(outputData, chunk.Data)
				}
			}

			output := strings.Join(outputData, "\n")

			if !strings.Contains(output, tt.contains) {
				t.Errorf("Output should contain %q, got: %s", tt.contains, output)
			}

			if strings.Contains(output, tt.notContains) {
				t.Errorf("Output should NOT contain sensitive data %q, got: %s", tt.notContains, output)
			}
		})
	}
}

// TestShellTool_ExecuteStream_PartialLineAtEOF tests handling of partial lines at EOF
func TestShellTool_ExecuteStream_PartialLineAtEOF(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping partial line test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig)
	ctx := context.Background()

	// Use printf without newline to test partial line handling
	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "printf",
		"args":    []interface{}{"no-newline-here"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var outputData []string
	for chunk := range chunks {
		if !chunk.IsFinal && chunk.Stream == "stdout" {
			outputData = append(outputData, chunk.Data)
		}
	}

	// Should receive the partial line
	if len(outputData) == 0 {
		t.Error("Should receive partial line at EOF")
	}

	output := strings.Join(outputData, "")
	if !strings.Contains(output, "no-newline-here") {
		t.Errorf("Output should contain partial line, got: %s", output)
	}
}

// TestShellTool_ExecuteStream_LineBuffering tests that lines are emitted immediately on newline
func TestShellTool_ExecuteStream_LineBuffering(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping line buffering test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig)
	ctx := context.Background()

	// Output multiple lines with delays to test line buffering
	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", "echo first; echo second; echo third"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var lines []string
	for chunk := range chunks {
		if !chunk.IsFinal && chunk.Stream == "stdout" {
			lines = append(lines, chunk.Data)
		}
	}

	// Should receive each line as a separate chunk
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 line chunks, got %d", len(lines))
	}

	// Verify each line contains expected content (may have multiple words per chunk)
	output := strings.Join(lines, "\n")
	for _, expected := range []string{"first", "second", "third"} {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain %q, got: %s", expected, output)
		}
	}
}

// TestShellTool_ExecuteStream_SizeLimit tests that output is truncated when size limit is exceeded
func TestShellTool_ExecuteStream_SizeLimit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping size limit test on Windows")
	}

	// Configure a small output size limit (100 bytes)
	secConfig := security.DefaultShellSecurityConfig()
	secConfig.MaxOutputSize = 100
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig)
	ctx := context.Background()

	// Generate output larger than the limit (500+ bytes)
	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", "for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do echo 'This is a line of output that should be truncated'; done"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var totalSize int
	var finalChunk *tools.ToolChunk
	for chunk := range chunks {
		if chunk.IsFinal {
			finalChunk = &chunk
		} else if chunk.Stream == "stdout" {
			totalSize += len(chunk.Data)
		}
	}

	// Verify output was truncated
	if totalSize > int(secConfig.MaxOutputSize) {
		t.Errorf("Total output size %d exceeds limit %d", totalSize, secConfig.MaxOutputSize)
	}

	// Verify final chunk has truncation metadata
	if finalChunk == nil {
		t.Fatal("No final chunk received")
	}

	if finalChunk.Metadata == nil {
		t.Fatal("Final chunk should have metadata when truncated")
	}

	truncated, ok := finalChunk.Metadata["truncated"].(bool)
	if !ok || !truncated {
		t.Error("Final chunk should have truncated=true in metadata")
	}

	// Verify error message indicates truncation
	if finalChunk.Error == nil {
		t.Error("Final chunk should have error when truncated")
	} else if !strings.Contains(finalChunk.Error.Error(), "truncated") {
		t.Errorf("Error should mention truncation, got: %v", finalChunk.Error)
	}
}
