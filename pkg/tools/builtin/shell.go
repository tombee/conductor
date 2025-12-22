package builtin

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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
}

// NewShellTool creates a new shell tool with default settings.
func NewShellTool() *ShellTool {
	return &ShellTool{
		timeout:         30 * time.Second,                  // 30 second default
		workingDir:      "",                                // Current directory
		allowedCommands: []string{},                        // No restrictions by default
		securityConfig:  security.DefaultShellSecurityConfig(), // Secure defaults
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
		return nil, fmt.Errorf("command must be a string")
	}

	// Extract arguments (optional)
	var args []string
	if argsRaw, ok := inputs["args"]; ok {
		argsSlice, ok := argsRaw.([]interface{})
		if !ok {
			return nil, fmt.Errorf("args must be an array")
		}
		args = make([]string, len(argsSlice))
		for i, arg := range argsSlice {
			argStr, ok := arg.(string)
			if !ok {
				return nil, fmt.Errorf("all args must be strings")
			}
			args[i] = argStr
		}
	}

	// Validate command
	if err := t.validateCommand(command); err != nil {
		return nil, err
	}

	// Validate with security config
	if t.securityConfig != nil {
		if err := t.securityConfig.ValidateCommand(command, args); err != nil {
			return nil, fmt.Errorf("security validation failed: %w", err)
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
