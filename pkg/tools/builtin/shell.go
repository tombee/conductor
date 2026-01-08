package builtin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tombee/conductor/pkg/errors"
	"github.com/tombee/conductor/pkg/security"
	"github.com/tombee/conductor/pkg/tools"
)

// ShellTool provides sandboxed shell command execution.
type ShellTool struct {
	// timeout sets the maximum execution time
	timeout time.Duration

	// workingDir sets the working directory for commands
	workingDir string

	// allowedCommands restricts which commands can be executed
	// If empty, all commands are allowed
	allowedCommands []string

	// securityConfig provides enhanced security controls
	securityConfig *security.ShellSecurityConfig

	// redactor redacts sensitive data from output
	redactor *tools.Redactor
}

// NewShellTool creates a new shell tool with default settings.
func NewShellTool() *ShellTool {
	return &ShellTool{
		timeout:         30 * time.Second,                      // 30 second default
		workingDir:      "",                                    // Current directory
		allowedCommands: []string{},                            // No restrictions by default
		securityConfig:  security.DefaultShellSecurityConfig(), // Secure defaults
		redactor:        tools.NewRedactor(),                   // Default redaction patterns
	}
}

// WithTimeout sets the command execution timeout.
func (t *ShellTool) WithTimeout(timeout time.Duration) *ShellTool {
	t.timeout = timeout
	return t
}

// WithWorkingDir sets the working directory for commands.
func (t *ShellTool) WithWorkingDir(dir string) *ShellTool {
	t.workingDir = dir
	return t
}

// WithAllowedCommands restricts which commands can be executed.
func (t *ShellTool) WithAllowedCommands(commands []string) *ShellTool {
	t.allowedCommands = commands
	return t
}

// WithSecurityConfig sets the security configuration.
func (t *ShellTool) WithSecurityConfig(config *security.ShellSecurityConfig) *ShellTool {
	t.securityConfig = config
	return t
}

// Name returns the tool identifier.
func (t *ShellTool) Name() string {
	return "shell"
}

// Description returns a human-readable description.
func (t *ShellTool) Description() string {
	return "Execute shell commands in a sandboxed environment"
}

// Schema returns the tool's input/output schema.
func (t *ShellTool) Schema() *tools.Schema {
	return &tools.Schema{
		Inputs: &tools.ParameterSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"command": {
					Type:        "string",
					Description: "The shell command to execute",
				},
				"args": {
					Type:        "array",
					Description: "Command arguments (optional)",
				},
			},
			Required: []string{"command"},
		},
		Outputs: &tools.ParameterSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"success": {
					Type:        "boolean",
					Description: "Whether the command succeeded (exit code 0)",
				},
				"stdout": {
					Type:        "string",
					Description: "Standard output from the command",
				},
				"stderr": {
					Type:        "string",
					Description: "Standard error from the command",
				},
				"exit_code": {
					Type:        "number",
					Description: "The command's exit code",
				},
				"status": {
					Type:        "string",
					Description: "Execution status (completed, timeout, error)",
				},
			},
		},
	}
}

// Execute runs the shell command.
func (t *ShellTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Extract command
	command, ok := inputs["command"].(string)
	if !ok {
		return nil, &errors.ValidationError{
			Field:      "command",
			Message:    "command must be a string",
			Suggestion: "Provide the command as a string",
		}
	}

	// Extract arguments (optional)
	var args []string
	if argsRaw, ok := inputs["args"]; ok {
		argsSlice, ok := argsRaw.([]interface{})
		if !ok {
			return nil, &errors.ValidationError{
				Field:      "args",
				Message:    "args must be an array",
				Suggestion: "Provide arguments as an array of strings",
			}
		}
		args = make([]string, len(argsSlice))
		for i, arg := range argsSlice {
			argStr, ok := arg.(string)
			if !ok {
				return nil, &errors.ValidationError{
					Field:      fmt.Sprintf("args[%d]", i),
					Message:    "all args must be strings",
					Suggestion: "Ensure all arguments are strings",
				}
			}
			args[i] = argStr
		}
	}

	// Validate command
	if err := t.validateCommand(command); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Validate with security config
	if t.securityConfig != nil {
		if err := t.securityConfig.ValidateCommand(command, args); err != nil {
			return nil, fmt.Errorf("security validation failed for command %s: %w", command, err)
		}
	}

	// Set timeout
	execCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(execCtx, command, args...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	// Sanitize environment if configured
	if t.securityConfig != nil && t.securityConfig.SanitizeEnv {
		cmd.Env = t.securityConfig.SanitizeEnvironment(os.Environ())
	}

	// Capture output
	var stdout, stderr []byte
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return map[string]interface{}{
			"success":   false,
			"stdout":    "",
			"stderr":    "",
			"exit_code": -1,
			"status":    "error",
		}, nil
	}

	// Read output in goroutines
	stdoutDone := make(chan []byte)
	stderrDone := make(chan []byte)

	go func() {
		data, _ := readAll(stdoutPipe)
		stdoutDone <- data
	}()

	go func() {
		data, _ := readAll(stderrPipe)
		stderrDone <- data
	}()

	// Wait for command to complete
	err = cmd.Wait()

	// Collect output
	stdout = <-stdoutDone
	stderr = <-stderrDone

	// Determine status and exit code
	var exitCode int
	var status string

	if ctx.Err() == context.DeadlineExceeded || execCtx.Err() == context.DeadlineExceeded {
		// Timeout occurred
		status = "timeout"
		exitCode = -1

		// Try to kill the process
		if cmd.Process != nil {
			// Send SIGTERM first
			cmd.Process.Signal(syscall.SIGTERM)

			// Wait 2 seconds for graceful shutdown
			time.Sleep(2 * time.Second)

			// If still running, SIGKILL
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
				cmd.Process.Kill()
			}
		}
	} else if err != nil {
		// Command failed
		status = "completed"
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	} else {
		// Command succeeded
		status = "completed"
		exitCode = 0
	}

	// Truncate output if max size configured
	stdoutStr := string(stdout)
	stderrStr := string(stderr)
	if t.securityConfig != nil && t.securityConfig.MaxOutputSize > 0 {
		if int64(len(stdoutStr)) > t.securityConfig.MaxOutputSize {
			stdoutStr = stdoutStr[:t.securityConfig.MaxOutputSize] + "\n[output truncated]"
		}
		if int64(len(stderrStr)) > t.securityConfig.MaxOutputSize {
			stderrStr = stderrStr[:t.securityConfig.MaxOutputSize] + "\n[output truncated]"
		}
	}

	return map[string]interface{}{
		"success":   exitCode == 0,
		"stdout":    stdoutStr,
		"stderr":    stderrStr,
		"exit_code": exitCode,
		"status":    status,
	}, nil
}

// ExecuteStream runs the shell command and streams output line-by-line.
// It implements the StreamingTool interface for real-time output visibility.
func (t *ShellTool) ExecuteStream(ctx context.Context, inputs map[string]interface{}) (<-chan tools.ToolChunk, error) {
	// Extract and validate command and args
	command, ok := inputs["command"].(string)
	if !ok {
		return nil, &errors.ValidationError{
			Field:      "command",
			Message:    "command must be a string",
			Suggestion: "Provide the command as a string",
		}
	}

	// Extract arguments (optional)
	var args []string
	if argsRaw, ok := inputs["args"]; ok {
		argsSlice, ok := argsRaw.([]interface{})
		if !ok {
			return nil, &errors.ValidationError{
				Field:      "args",
				Message:    "args must be an array",
				Suggestion: "Provide arguments as an array of strings",
			}
		}
		args = make([]string, len(argsSlice))
		for i, arg := range argsSlice {
			argStr, ok := arg.(string)
			if !ok {
				return nil, &errors.ValidationError{
					Field:      fmt.Sprintf("args[%d]", i),
					Message:    "all args must be strings",
					Suggestion: "Ensure all arguments are strings",
				}
			}
			args[i] = argStr
		}
	}

	// Validate command
	if err := t.validateCommand(command); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Validate with security config
	if t.securityConfig != nil {
		if err := t.securityConfig.ValidateCommand(command, args); err != nil {
			return nil, fmt.Errorf("security validation failed for command %s: %w", command, err)
		}
	}

	// Create bounded channel for streaming chunks
	chunks := make(chan tools.ToolChunk, 256)

	// Start streaming execution in a goroutine
	go func() {
		defer close(chunks)

		// Track execution time
		startTime := time.Now()

		// Set timeout
		execCtx, cancel := context.WithTimeout(ctx, t.timeout)
		defer cancel()

		// Create command
		cmd := exec.CommandContext(execCtx, command, args...)
		if t.workingDir != "" {
			cmd.Dir = t.workingDir
		}

		// Sanitize environment if configured
		if t.securityConfig != nil && t.securityConfig.SanitizeEnv {
			cmd.Env = t.securityConfig.SanitizeEnvironment(os.Environ())
		}

		// Create pipes for stdout and stderr
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			chunks <- tools.ToolChunk{
				IsFinal: true,
				Error:   fmt.Errorf("failed to create stdout pipe: %w", err),
			}
			return
		}

		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			chunks <- tools.ToolChunk{
				IsFinal: true,
				Error:   fmt.Errorf("failed to create stderr pipe: %w", err),
			}
			return
		}

		// Start command
		if err := cmd.Start(); err != nil {
			duration := time.Since(startTime).Milliseconds()
			chunks <- tools.ToolChunk{
				IsFinal: true,
				Result: map[string]interface{}{
					"success":   false,
					"stdout":    "",
					"stderr":    "",
					"exit_code": -1,
					"status":    "error",
					"duration":  duration,
				},
			}
			return
		}

		// Use WaitGroup to coordinate stdout/stderr goroutines
		var wg sync.WaitGroup
		wg.Add(2)

		// Track total output size across both streams
		var totalSize int64
		var sizeMutex sync.Mutex
		var truncated bool

		// Stream stdout with enhanced streamPipe helper
		go t.streamPipe(execCtx, chunks, stdoutPipe, "stdout", &wg, &totalSize, &sizeMutex, &truncated)

		// Stream stderr with enhanced streamPipe helper
		go t.streamPipe(execCtx, chunks, stderrPipe, "stderr", &wg, &totalSize, &sizeMutex, &truncated)

		// Wait for command to complete
		cmdErr := cmd.Wait()

		// Wait for all output to be streamed
		wg.Wait()

		// Calculate duration
		duration := time.Since(startTime).Milliseconds()

		// Determine status and exit code
		var exitCode int
		var status string

		if ctx.Err() == context.DeadlineExceeded || execCtx.Err() == context.DeadlineExceeded {
			// Timeout occurred
			status = "timeout"
			exitCode = -1

			// Try to kill the process
			if cmd.Process != nil {
				// Send SIGTERM first
				cmd.Process.Signal(syscall.SIGTERM)

				// Wait 2 seconds for graceful shutdown
				time.Sleep(2 * time.Second)

				// If still running, SIGKILL
				if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
					cmd.Process.Kill()
				}
			}
		} else if cmdErr != nil {
			// Command failed
			status = "completed"
			if exitErr, ok := cmdErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		} else {
			// Command succeeded
			status = "completed"
			exitCode = 0
		}

		// Send final chunk with result
		finalChunk := tools.ToolChunk{
			IsFinal: true,
			Result: map[string]interface{}{
				"success":   exitCode == 0,
				"exit_code": exitCode,
				"status":    status,
				"duration":  duration,
			},
		}

		// Add truncation metadata and error if output was truncated
		if truncated {
			finalChunk.Metadata = map[string]interface{}{
				"truncated": true,
			}
			finalChunk.Error = fmt.Errorf("output truncated: exceeded size limit of %d bytes", t.securityConfig.MaxOutputSize)
		}

		chunks <- finalChunk
	}()

	return chunks, nil
}

// emitChunkWithSizeCheck checks size limits before emitting a chunk.
// Returns true if the chunk was emitted, false if truncated.
func (t *ShellTool) emitChunkWithSizeCheck(chunks chan<- tools.ToolChunk, data, stream string, totalSize *int64, sizeMutex *sync.Mutex, truncated *bool) bool {
	// Check if output limit is configured
	if t.securityConfig == nil || t.securityConfig.MaxOutputSize <= 0 {
		// No limit configured, emit normally
		chunks <- tools.ToolChunk{
			Data:   data,
			Stream: stream,
		}
		return true
	}

	// Check size under lock
	sizeMutex.Lock()
	defer sizeMutex.Unlock()

	// Check if already truncated
	if *truncated {
		return false
	}

	dataSize := int64(len(data))
	newTotal := *totalSize + dataSize

	// Check if this would exceed the limit
	if newTotal > t.securityConfig.MaxOutputSize {
		// Mark as truncated
		*truncated = true
		return false
	}

	// Update size and emit chunk
	*totalSize = newTotal
	chunks <- tools.ToolChunk{
		Data:   data,
		Stream: stream,
	}
	return true
}

// streamPipe reads from a pipe and emits chunks with line-buffering and redaction.
// It handles edge cases including:
// - Line-buffered output (emits on \n)
// - 4KB fallback for binary data without newlines
// - Panic recovery with error reporting
// - Context cancellation for cleanup
// - Redaction of sensitive data before emission
// - Output size limit enforcement with truncation
func (t *ShellTool) streamPipe(ctx context.Context, chunks chan<- tools.ToolChunk, pipe io.ReadCloser, stream string, wg *sync.WaitGroup, totalSize *int64, sizeMutex *sync.Mutex, truncated *bool) {
	defer wg.Done()

	// Recover from panics and report as error
	defer func() {
		if r := recover(); r != nil {
			chunks <- tools.ToolChunk{
				Stream: stream,
				Data:   "",
				Error:  fmt.Errorf("panic in %s stream: %v", stream, r),
			}
		}
	}()

	// Ensure pipe is closed on exit
	defer pipe.Close()

	const maxChunkSize = 4 * 1024 // 4KB fallback for binary data
	buf := make([]byte, maxChunkSize)
	var pending []byte

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			// Context cancelled, emit any pending data and exit
			if len(pending) > 0 {
				data := string(pending)
				redacted := t.redactor.Redact(data)
				t.emitChunkWithSizeCheck(chunks, redacted, stream, totalSize, sizeMutex, truncated)
			}
			return
		default:
		}

		// Read from pipe
		n, err := pipe.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)

			// Process complete lines (line-buffered output)
			for {
				idx := bytes.IndexByte(pending, '\n')
				if idx == -1 {
					break
				}

				// Emit line with redaction and size check
				line := string(pending[:idx])
				redacted := t.redactor.Redact(line)
				if !t.emitChunkWithSizeCheck(chunks, redacted, stream, totalSize, sizeMutex, truncated) {
					// Size limit exceeded, stop processing
					return
				}
				pending = pending[idx+1:]
			}

			// If buffer exceeds 4KB without newline (binary data), emit it
			if len(pending) >= maxChunkSize {
				data := string(pending)
				redacted := t.redactor.Redact(data)
				if !t.emitChunkWithSizeCheck(chunks, redacted, stream, totalSize, sizeMutex, truncated) {
					// Size limit exceeded, stop processing
					return
				}
				pending = pending[:0]
			}
		}

		if err != nil {
			// Report non-EOF errors
			if err != io.EOF {
				chunks <- tools.ToolChunk{
					Stream: stream,
					Error:  fmt.Errorf("error reading %s: %w", stream, err),
				}
			}

			// Emit any remaining data (partial line at EOF)
			if len(pending) > 0 {
				data := string(pending)
				redacted := t.redactor.Redact(data)
				t.emitChunkWithSizeCheck(chunks, redacted, stream, totalSize, sizeMutex, truncated)
			}
			return
		}
	}
}

// validateCommand checks if a command is allowed.
func (t *ShellTool) validateCommand(command string) error {
	// Empty list means all commands allowed (preserve existing behavior)
	if len(t.allowedCommands) == 0 {
		return nil
	}

	// Reject relative paths and path traversal attempts
	if strings.HasPrefix(command, "./") || strings.HasPrefix(command, "../") ||
		strings.Contains(command, "/..") || strings.Contains(command, "..\\") {
		slog.Warn("shell command blocked",
			slog.String("reason", "path_traversal"),
			slog.String("command", command))
		return fmt.Errorf("command execution blocked by policy")
	}

	// Extract base command name
	cmdName := filepath.Base(command)
	if cmdName == "" || cmdName == "." || cmdName == ".." {
		slog.Warn("shell command blocked",
			slog.String("reason", "invalid_command"),
			slog.String("command", command))
		return fmt.Errorf("command execution blocked by policy")
	}

	// Check against allowed list
	for _, allowed := range t.allowedCommands {
		allowedBase := filepath.Base(allowed)

		// Match scenarios:
		// 1. Exact match: command == allowed
		// 2. Base name match when command has no path components
		if command == allowed {
			return nil
		}
		if !strings.Contains(command, "/") && !strings.Contains(command, "\\") && cmdName == allowedBase {
			return nil
		}
	}

	// Log blocked attempt (command logged for audit, but not in error message)
	slog.Warn("shell command blocked",
		slog.String("reason", "not_in_allowlist"),
		slog.String("command", command))

	return fmt.Errorf("command execution blocked by policy")
}

// readAll reads all data from a pipe.
func readAll(pipe interface {
	Read(p []byte) (n int, err error)
}) ([]byte, error) {
	var result []byte
	buf := make([]byte, 4096)
	for {
		n, err := pipe.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return result, nil
}
