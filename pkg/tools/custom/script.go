package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tombee/conductor/pkg/tools"
	"github.com/tombee/conductor/pkg/workflow"
)

// ScriptCustomTool implements the tools.Tool interface for script-based tools.
// It executes shell scripts with JSON input/output.
type ScriptCustomTool struct {
	name            string
	description     string
	command         string
	workflowDir     string
	inputSchema     *tools.Schema
	timeout         time.Duration
	maxResponseSize int64
}

// NewScriptCustomTool creates a script custom tool from a function definition.
func NewScriptCustomTool(def workflow.FunctionDefinition, workflowDir string) (*ScriptCustomTool, error) {
	// Validate this is a script function
	if def.Type != workflow.ToolTypeScript {
		return nil, fmt.Errorf("function type must be script, got: %s", def.Type)
	}

	// Apply defaults
	timeout := time.Duration(def.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	maxResponseSize := def.MaxResponseSize
	if maxResponseSize == 0 {
		maxResponseSize = 1024 * 1024 // 1MB default
	}

	// Convert input schema to tools.Schema
	var inputSchema *tools.Schema
	if def.InputSchema != nil {
		paramSchema, err := convertToParameterSchema(def.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("invalid input schema: %w", err)
		}
		inputSchema = &tools.Schema{
			Inputs: paramSchema,
		}
	}

	return &ScriptCustomTool{
		name:            def.Name,
		description:     def.Description,
		command:         def.Command,
		workflowDir:     workflowDir,
		inputSchema:     inputSchema,
		timeout:         timeout,
		maxResponseSize: maxResponseSize,
	}, nil
}

// Name returns the tool name.
func (s *ScriptCustomTool) Name() string {
	return s.name
}

// Description returns the tool description.
func (s *ScriptCustomTool) Description() string {
	return s.description
}

// Schema returns the tool's input/output schema.
func (s *ScriptCustomTool) Schema() *tools.Schema {
	if s.inputSchema == nil {
		// Return empty schema if none defined
		return &tools.Schema{
			Inputs: &tools.ParameterSchema{
				Type:       "object",
				Properties: make(map[string]*tools.Property),
			},
		}
	}
	return s.inputSchema
}

// Execute runs the script with JSON input on stdin and captures stdout.
func (s *ScriptCustomTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Resolve script path relative to workflow directory
	scriptPath, err := s.resolveScriptPath()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve script path: %w", err)
	}

	// Prepare JSON input
	inputJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal inputs: %w", err)
	}

	// Create command
	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Dir = s.workflowDir

	// Set up stdin with JSON input
	cmd.Stdin = bytes.NewReader(inputJSON)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	err = cmd.Run()

	// Check for context timeout
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("script execution timed out after %v", s.timeout)
	}

	// Check for execution error
	if err != nil {
		stderrStr := stderr.String()
		return nil, fmt.Errorf("script execution failed: %w (stderr: %s)", err, stderrStr)
	}

	// Parse stdout as JSON or return as string
	stdoutBytes := stdout.Bytes()

	// Check size limit
	if int64(len(stdoutBytes)) > s.maxResponseSize {
		return nil, fmt.Errorf("script output exceeds maximum size of %d bytes", s.maxResponseSize)
	}

	if len(stdoutBytes) == 0 {
		return map[string]interface{}{
			"output": "",
			"stderr": stderr.String(),
		}, nil
	}

	// Try to parse as JSON
	var result interface{}
	if err := json.Unmarshal(stdoutBytes, &result); err != nil {
		// If not JSON, return as string
		result = string(stdoutBytes)
	}

	return map[string]interface{}{
		"output": result,
		"stderr": stderr.String(),
	}, nil
}

// resolveScriptPath resolves the script path relative to the workflow directory
// and validates it to prevent directory traversal attacks.
func (s *ScriptCustomTool) resolveScriptPath() (string, error) {
	// If absolute path, validate it's within allowed directory
	if filepath.IsAbs(s.command) {
		// For now, allow absolute paths but could add whitelist in future
		return s.command, nil
	}

	// Resolve relative to workflow directory
	scriptPath := filepath.Join(s.workflowDir, s.command)

	// Clean the path to resolve any .. components
	scriptPath = filepath.Clean(scriptPath)

	// Validate the resolved path is still within or below the workflow directory
	// This prevents directory traversal attacks like ../../etc/passwd
	absWorkflowDir, err := filepath.Abs(s.workflowDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve workflow directory: %w", err)
	}

	absScriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve script path: %w", err)
	}

	// Check if script path is within workflow directory
	if !strings.HasPrefix(absScriptPath, absWorkflowDir) {
		return "", fmt.Errorf("script path %s is outside workflow directory %s (security violation)", absScriptPath, absWorkflowDir)
	}

	return absScriptPath, nil
}
