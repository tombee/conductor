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

// Package server implements an MCP server that exposes Conductor functionality as tools.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server and provides Conductor tools
type Server struct {
	mcpServer   *server.MCPServer
	name        string
	version     string
	rateLimiter *RateLimiter
	logger      *slog.Logger
}

// ServerConfig configures the MCP server
type ServerConfig struct {
	// Name is the server name (default: "conductor")
	Name string

	// Version is the Conductor version
	Version string

	// LogLevel controls logging verbosity (debug, info, warn, error)
	LogLevel string

	// OperationRegistry optionally provides operation registry actions as tools.
	// If set, builtin actions (file, shell, etc.) will be exposed as MCP tools.
	OperationRegistry OperationRegistry
}

// createLogger creates a logger with the specified log level.
// Writes to stderr to avoid interfering with MCP stdio protocol.
func createLogger(levelStr string) (*slog.Logger, error) {
	var level slog.Level

	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "info", "":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", levelStr)
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler), nil
}

// NewServer creates a new MCP server instance
func NewServer(config ServerConfig) (*Server, error) {
	if config.Name == "" {
		config.Name = "conductor"
	}
	if config.Version == "" {
		config.Version = "dev"
	}

	// Create logger with configured level
	logger, err := createLogger(config.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Create the underlying MCP server
	mcpServer := server.NewMCPServer(config.Name, config.Version)

	// Create rate limiter (10 runs/min, 100 calls/min)
	rateLimiter := NewRateLimiter(10, 100)

	s := &Server{
		mcpServer:   mcpServer,
		name:        config.Name,
		version:     config.Version,
		rateLimiter: rateLimiter,
		logger:      logger,
	}

	// Register Conductor workflow tools (validate, run, etc.)
	if err := s.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	// Register operation registry tools if provided
	if config.OperationRegistry != nil {
		if err := s.registerOperationTools(config.OperationRegistry); err != nil {
			return nil, fmt.Errorf("failed to register operation tools: %w", err)
		}
	}

	return s, nil
}

// registerTools registers all Conductor tools with the MCP server
func (s *Server) registerTools() error {
	// Tool: conductor_validate
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "conductor_validate",
		Description: "Validate workflow YAML content without executing it. Returns structured errors with line numbers and suggestions.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"workflow_yaml": map[string]interface{}{
					"type":        "string",
					"description": "The complete YAML content of the workflow to validate",
				},
			},
			Required: []string{"workflow_yaml"},
		},
	}, s.handleValidate)

	// Tool: conductor_schema
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "conductor_schema",
		Description: "Return the JSON Schema for Conductor workflow definitions. Use this for accurate workflow generation.",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, s.handleSchema)

	// Tool: conductor_list_templates
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "conductor_list_templates",
		Description: "List available workflow templates with descriptions and parameters. Templates can be filtered by category.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Filter by category (e.g., 'code-review', 'automation', 'analysis')",
				},
			},
		},
	}, s.handleListTemplates)

	// Tool: conductor_scaffold
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "conductor_scaffold",
		Description: "Generate a workflow from a template. Returns valid workflow YAML ready for customization.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"template": map[string]interface{}{
					"type":        "string",
					"description": "Template name (from conductor_list_templates)",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Name for the generated workflow",
				},
				"parameters": map[string]interface{}{
					"type":        "object",
					"description": "Template parameter values",
				},
			},
			Required: []string{"template", "name"},
		},
	}, s.handleScaffold)

	// Tool: conductor_run
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "conductor_run",
		Description: "Execute a workflow with optional dry-run mode. IMPORTANT: dry_run defaults to true for safety. Set dry_run=false explicitly to execute.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"workflow_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the workflow YAML file",
				},
				"inputs": map[string]interface{}{
					"type":        "object",
					"description": "Input values for workflow parameters",
				},
				"dry_run": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, show execution plan without running (default: true)",
					"default":     true,
				},
			},
			Required: []string{"workflow_path"},
		},
	}, s.handleRun)

	// Tool: conductor_health
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "conductor_health",
		Description: "Check Conductor installation and configuration health. Returns diagnostic information and remediation steps.",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, s.handleHealth)

	return nil
}

// Run starts the MCP server using stdio transport
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("Starting Conductor MCP server", slog.String("version", s.version))

	// Serve via stdio
	if err := server.ServeStdio(s.mcpServer); err != nil {
		return fmt.Errorf("MCP server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down Conductor MCP server")
	// The mcp-go server doesn't have an explicit shutdown method
	// Returning from ServeStdio() is sufficient
	return nil
}

// Helper function to create error response
func errorResponse(message string) *mcp.CallToolResult {
	return mcp.NewToolResultError(message)
}

// Helper function to create success response
func textResponse(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(text),
		},
	}
}

// checkmark returns a UTF-8 checkmark for boolean status
func checkMark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}
