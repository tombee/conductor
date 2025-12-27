// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"fmt"
	"strings"
)

// MCPErrorCode represents a category of MCP error.
type MCPErrorCode string

const (
	// ErrorCodeNotFound indicates a server was not found.
	ErrorCodeNotFound MCPErrorCode = "NOT_FOUND"
	// ErrorCodeAlreadyExists indicates a server already exists.
	ErrorCodeAlreadyExists MCPErrorCode = "ALREADY_EXISTS"
	// ErrorCodeAlreadyRunning indicates a server is already running.
	ErrorCodeAlreadyRunning MCPErrorCode = "ALREADY_RUNNING"
	// ErrorCodeNotRunning indicates a server is not running.
	ErrorCodeNotRunning MCPErrorCode = "NOT_RUNNING"
	// ErrorCodeCommandNotFound indicates a command was not found.
	ErrorCodeCommandNotFound MCPErrorCode = "COMMAND_NOT_FOUND"
	// ErrorCodeStartFailed indicates a server failed to start.
	ErrorCodeStartFailed MCPErrorCode = "START_FAILED"
	// ErrorCodePingFailed indicates a ping to the server failed.
	ErrorCodePingFailed MCPErrorCode = "PING_FAILED"
	// ErrorCodeConnectionClosed indicates the server connection closed.
	ErrorCodeConnectionClosed MCPErrorCode = "CONNECTION_CLOSED"
	// ErrorCodeValidation indicates a validation error.
	ErrorCodeValidation MCPErrorCode = "VALIDATION"
	// ErrorCodeConfig indicates a configuration error.
	ErrorCodeConfig MCPErrorCode = "CONFIG"
	// ErrorCodeTimeout indicates a timeout occurred.
	ErrorCodeTimeout MCPErrorCode = "TIMEOUT"
	// ErrorCodeInternalError indicates an internal error.
	ErrorCodeInternalError MCPErrorCode = "INTERNAL"
)

// MCPError is an error type that includes suggestions for resolution.
type MCPError struct {
	// Code is the error category.
	Code MCPErrorCode
	// Message is the primary error message.
	Message string
	// Detail provides additional context.
	Detail string
	// Suggestions are actionable steps to resolve the error.
	Suggestions []string
	// Cause is the underlying error, if any.
	Cause error
}

// Error implements the error interface.
func (e *MCPError) Error() string {
	var sb strings.Builder

	sb.WriteString("Error: ")
	sb.WriteString(e.Message)
	sb.WriteString("\n")

	if e.Detail != "" {
		sb.WriteString("  → ")
		sb.WriteString(e.Detail)
		sb.WriteString("\n")
	}

	if len(e.Suggestions) > 0 {
		sb.WriteString("\n  Suggestions:\n")
		for _, s := range e.Suggestions {
			sb.WriteString("  - ")
			sb.WriteString(s)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// Unwrap returns the underlying error.
func (e *MCPError) Unwrap() error {
	return e.Cause
}

// IsUserVisible implements pkg/errors.UserVisibleError.
// MCP errors are always user-visible.
func (e *MCPError) IsUserVisible() bool {
	return true
}

// UserMessage implements pkg/errors.UserVisibleError.
// Returns a user-friendly message without technical details.
func (e *MCPError) UserMessage() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Detail)
	}
	return e.Message
}

// Suggestion implements pkg/errors.UserVisibleError.
// Returns actionable guidance for resolving the error.
func (e *MCPError) Suggestion() string {
	if len(e.Suggestions) == 0 {
		return ""
	}
	// Return the first suggestion as a simple string
	// The full list is available in Error() output
	return e.Suggestions[0]
}

// NewMCPError creates a new MCPError.
func NewMCPError(code MCPErrorCode, message string) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
	}
}

// WithDetail adds detail to the error.
func (e *MCPError) WithDetail(detail string) *MCPError {
	e.Detail = detail
	return e
}

// WithSuggestions adds suggestions to the error.
func (e *MCPError) WithSuggestions(suggestions ...string) *MCPError {
	e.Suggestions = suggestions
	return e
}

// WithCause adds an underlying cause to the error.
func (e *MCPError) WithCause(cause error) *MCPError {
	e.Cause = cause
	return e
}

// ErrServerNotFound creates an error for when a server is not found.
func ErrServerNotFound(name string) *MCPError {
	return NewMCPError(ErrorCodeNotFound, fmt.Sprintf("MCP server '%s' not found", name)).
		WithSuggestions(
			fmt.Sprintf("Check the server name: conductor mcp list"),
			fmt.Sprintf("Register the server: conductor mcp add %s --command <cmd>", name),
		)
}

// ErrServerAlreadyExists creates an error for when a server already exists.
func ErrServerAlreadyExists(name string) *MCPError {
	return NewMCPError(ErrorCodeAlreadyExists, fmt.Sprintf("MCP server '%s' already exists", name)).
		WithSuggestions(
			fmt.Sprintf("Use a different name for the new server"),
			fmt.Sprintf("Remove existing server: conductor mcp remove %s", name),
		)
}

// ErrServerAlreadyRunning creates an error for when a server is already running.
func ErrServerAlreadyRunning(name string) *MCPError {
	return NewMCPError(ErrorCodeAlreadyRunning, fmt.Sprintf("MCP server '%s' is already running", name)).
		WithSuggestions(
			fmt.Sprintf("Check status: conductor mcp status %s", name),
			fmt.Sprintf("Restart if needed: conductor mcp restart %s", name),
		)
}

// ErrServerNotRunning creates an error for when a server is not running.
func ErrServerNotRunning(name string) *MCPError {
	return NewMCPError(ErrorCodeNotRunning, fmt.Sprintf("MCP server '%s' is not running", name)).
		WithSuggestions(
			fmt.Sprintf("Start the server: conductor mcp start %s", name),
			fmt.Sprintf("Check status: conductor mcp status %s", name),
		)
}

// ErrCommandNotFound creates an error for when a command is not found.
func ErrCommandNotFound(command string) *MCPError {
	suggestions := []string{
		fmt.Sprintf("Verify the command is installed and in your PATH"),
		fmt.Sprintf("Use an absolute path: --command /path/to/%s", command),
	}

	// Add specific suggestions based on common commands
	switch {
	case command == "npx" || command == "node":
		suggestions = append(suggestions, "Install Node.js: https://nodejs.org/")
	case command == "python" || command == "python3":
		suggestions = append(suggestions, "Install Python: https://python.org/")
	case command == "pip" || command == "uvx":
		suggestions = append(suggestions, "Install pip/uvx for Python package management")
	}

	return NewMCPError(ErrorCodeCommandNotFound, fmt.Sprintf("Command '%s' not found", command)).
		WithDetail(fmt.Sprintf("Command '%s' not found in PATH", command)).
		WithSuggestions(suggestions...)
}

// ErrStartFailed creates an error for when a server fails to start.
func ErrStartFailed(name string, cause error) *MCPError {
	return NewMCPError(ErrorCodeStartFailed, fmt.Sprintf("Failed to start MCP server '%s'", name)).
		WithDetail(cause.Error()).
		WithCause(cause).
		WithSuggestions(
			fmt.Sprintf("Check server logs: conductor mcp logs %s", name),
			"Verify the command and arguments are correct",
			"Ensure required environment variables are set",
			fmt.Sprintf("Validate configuration: conductor mcp validate %s", name),
		)
}

// ErrPingFailed creates an error for when a server ping fails.
func ErrPingFailed(name string, timeout int) *MCPError {
	return NewMCPError(ErrorCodePingFailed, fmt.Sprintf("MCP server '%s' failed to respond", name)).
		WithDetail(fmt.Sprintf("Server started but ping timed out after %ds", timeout)).
		WithSuggestions(
			fmt.Sprintf("Check server logs: conductor mcp logs %s", name),
			"Verify server implements MCP protocol correctly",
			fmt.Sprintf("Try increasing timeout: conductor mcp add %s --timeout %d", name, timeout*2),
		)
}

// ErrConnectionClosed creates an error for when a server connection is closed.
func ErrConnectionClosed(name string) *MCPError {
	return NewMCPError(ErrorCodeConnectionClosed, fmt.Sprintf("Connection to MCP server '%s' closed", name)).
		WithSuggestions(
			fmt.Sprintf("Check if server is still running: conductor mcp status %s", name),
			fmt.Sprintf("Restart the server: conductor mcp restart %s", name),
			fmt.Sprintf("Check server logs for crash details: conductor mcp logs %s", name),
		)
}

// ErrInvalidServerName creates an error for an invalid server name.
func ErrInvalidServerName(name string) *MCPError {
	return NewMCPError(ErrorCodeValidation, fmt.Sprintf("Invalid server name '%s'", name)).
		WithDetail("Names must start with a letter, contain only letters/numbers/hyphens/underscores, and be at most 64 characters").
		WithSuggestions(
			"Use only letters, numbers, hyphens (-), and underscores (_)",
			"Start the name with a letter",
			fmt.Sprintf("Example valid names: my-server, server_1, mcpServer"),
		)
}

// ErrInvalidConfig creates an error for invalid configuration.
func ErrInvalidConfig(detail string) *MCPError {
	return NewMCPError(ErrorCodeConfig, "Invalid MCP server configuration").
		WithDetail(detail).
		WithSuggestions(
			"Check the configuration syntax in mcp.yaml",
			"Ensure all required fields are provided",
			"See documentation for configuration format",
		)
}

// ErrTimeout creates an error for a timeout.
func ErrTimeout(operation string, duration int) *MCPError {
	return NewMCPError(ErrorCodeTimeout, fmt.Sprintf("Operation '%s' timed out after %ds", operation, duration)).
		WithSuggestions(
			"Check if the server is responding",
			"Try increasing the timeout value",
			"Check server logs for issues",
		)
}

// ErrNoLogsAvailable creates an error for when logs are not available.
func ErrNoLogsAvailable(name string) *MCPError {
	return NewMCPError(ErrorCodeNotFound, fmt.Sprintf("No logs available for MCP server '%s'", name)).
		WithDetail("Server has not been started or logs have been cleared").
		WithSuggestions(
			fmt.Sprintf("Start the server first: conductor mcp start %s", name),
			fmt.Sprintf("Check if server exists: conductor mcp status %s", name),
		)
}

// ErrPackageResolutionFailed creates an error for when package resolution fails.
func ErrPackageResolutionFailed(source, version string, cause error) *MCPError {
	suggestions := []string{
		"Check that the package name and version are correct",
		"Verify network connectivity",
	}

	// Add specific suggestions based on source type
	if strings.HasPrefix(source, "npm:") {
		suggestions = append(suggestions,
			"Ensure npm is installed: npm --version",
			"Check npm registry access: npm ping",
		)
	} else if strings.HasPrefix(source, "pypi:") {
		suggestions = append(suggestions,
			"Ensure pip is installed: pip --version",
			"Check PyPI access: pip index versions <package>",
		)
	}

	return NewMCPError(ErrorCodeConfig, fmt.Sprintf("Failed to resolve package '%s' version '%s'", source, version)).
		WithDetail(cause.Error()).
		WithCause(cause).
		WithSuggestions(suggestions...)
}

// WrapError wraps a standard error in an MCPError if it isn't one already.
func WrapError(err error, code MCPErrorCode, message string) *MCPError {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr
	}
	return NewMCPError(code, message).WithDetail(err.Error()).WithCause(err)
}

// IsMCPError checks if an error is an MCPError.
func IsMCPError(err error) bool {
	_, ok := err.(*MCPError)
	return ok
}

// GetMCPError extracts an MCPError from an error chain.
func GetMCPError(err error) *MCPError {
	if mcpErr, ok := err.(*MCPError); ok {
		return mcpErr
	}
	return nil
}
