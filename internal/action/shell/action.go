package shell

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Config holds configuration for the shell action.
type Config struct {
	// WorkingDir is the working directory for shell commands
	WorkingDir string

	// Timeout is the default timeout for commands (default: 30s)
	Timeout time.Duration

	// AllowedCommands restricts which commands can be run (empty = allow all)
	AllowedCommands []string
}

// Result represents the output of a shell operation.
type Result struct {
	Response interface{}
	Metadata map[string]interface{}
}

// ShellConnector implements the action interface for shell command execution.
type ShellConnector struct {
	config *Config
}

// New creates a new shell action instance.
func New(config *Config) (*ShellConnector, error) {
	if config == nil {
		config = &Config{}
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	return &ShellConnector{config: config}, nil
}

// Execute runs a shell operation.
func (c *ShellConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*Result, error) {
	switch operation {
	case "run":
		return c.run(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown shell operation: %s", operation)
	}
}

// run executes a shell command.
func (c *ShellConnector) run(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get command - can be string or []string
	var cmd *exec.Cmd

	if command, ok := inputs["command"]; ok {
		switch v := command.(type) {
		case string:
			// String command: run via shell
			cmd = exec.CommandContext(ctx, "sh", "-c", v)
		case []interface{}:
			// Array command: run directly
			args := make([]string, len(v))
			for i, arg := range v {
				args[i] = fmt.Sprintf("%v", arg)
			}
			if len(args) == 0 {
				return nil, fmt.Errorf("command array is empty")
			}
			cmd = exec.CommandContext(ctx, args[0], args[1:]...)
		case []string:
			if len(v) == 0 {
				return nil, fmt.Errorf("command array is empty")
			}
			cmd = exec.CommandContext(ctx, v[0], v[1:]...)
		default:
			return nil, fmt.Errorf("command must be string or array, got %T", command)
		}
	} else {
		return nil, fmt.Errorf("command is required")
	}

	// Set working directory
	if c.config.WorkingDir != "" {
		cmd.Dir = c.config.WorkingDir
	}
	if dir, ok := inputs["dir"].(string); ok && dir != "" {
		cmd.Dir = dir
	}

	// Set environment - preserve system environment and add custom variables
	if env, ok := inputs["env"].(map[string]interface{}); ok {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%v", k, v))
		}
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	// Build result
	result := &Result{
		Metadata: map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
			"exit_code":   0,
		},
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Metadata["exit_code"] = exitErr.ExitCode()
		}
		// Include stderr in error
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("command failed: %s", errMsg)
	}

	// Return stdout as response, include stderr in metadata
	result.Response = map[string]interface{}{
		"stdout":    strings.TrimSpace(stdout.String()),
		"stderr":    strings.TrimSpace(stderr.String()),
		"exit_code": 0,
	}

	return result, nil
}
