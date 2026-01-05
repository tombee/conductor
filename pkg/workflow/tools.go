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

package workflow

import "fmt"

// AgentDefinition describes an agent with provider preferences and capability requirements.
type AgentDefinition struct {
	// Prefers is a hint about which provider family works best (not enforced)
	Prefers string `yaml:"prefers,omitempty" json:"prefers,omitempty"`

	// Capabilities lists required provider capabilities (vision, long-context, tool-use, streaming, json-mode)
	Capabilities []string `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
}

// FunctionDefinition describes a custom function that can be called by LLM steps.
// Functions are defined at the workflow level and can be either HTTP endpoints or shell scripts.
type FunctionDefinition struct {
	// Name is the unique function identifier
	Name string `yaml:"name" json:"name"`

	// Type specifies the function implementation (http or script)
	Type ToolType `yaml:"type" json:"type"`

	// Description provides human-readable context about what the function does
	Description string `yaml:"description" json:"description"`

	// Method is the HTTP method (GET, POST, etc.) for HTTP functions
	Method string `yaml:"method,omitempty" json:"method,omitempty"`

	// URL is the endpoint URL template for HTTP functions (supports {{.param}} interpolation)
	URL string `yaml:"url,omitempty" json:"url,omitempty"`

	// Headers are HTTP headers for HTTP functions (supports {{.env.VAR}} interpolation)
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// Command is the script path for script functions (relative to workflow file directory)
	Command string `yaml:"command,omitempty" json:"command,omitempty"`

	// InputSchema defines the expected input parameters using JSON Schema
	InputSchema map[string]interface{} `yaml:"input_schema,omitempty" json:"input_schema,omitempty"`

	// AutoApprove indicates whether the function can execute without user approval
	// Defaults to false for security
	AutoApprove bool `yaml:"auto_approve,omitempty" json:"auto_approve,omitempty"`

	// Timeout is the maximum execution time in seconds (default: 30s, max: 300s)
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// MaxResponseSize is the maximum response size in bytes (default: 1MB)
	MaxResponseSize int64 `yaml:"max_response_size,omitempty" json:"max_response_size,omitempty"`
}

// ToolType represents the type of custom tool.
type ToolType string

const (
	// ToolTypeHTTP is an HTTP endpoint tool
	ToolTypeHTTP ToolType = "http"

	// ToolTypeScript is a shell script tool
	ToolTypeScript ToolType = "script"
)

// ValidToolTypes for validation
var ValidToolTypes = map[ToolType]bool{
	ToolTypeHTTP:   true,
	ToolTypeScript: true,
}

// MCPServerConfig defines configuration for an MCP (Model Context Protocol) server.
// MCP servers provide tools that can be used in workflow steps via the tool registry.
type MCPServerConfig struct {
	// Name is the unique identifier for this MCP server
	Name string `yaml:"name" json:"name"`

	// Command is the executable to run (e.g., "npx", "python", "/usr/bin/mcp-server")
	Command string `yaml:"command" json:"command"`

	// Args are command-line arguments to pass to the server
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`

	// Env are environment variables to pass to the server (e.g., ["API_KEY=xyz"])
	Env []string `yaml:"env,omitempty" json:"env,omitempty"`

	// Timeout is the default timeout for tool calls in seconds (defaults to 30)
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// MCPServerRequirement struct represents a required MCP server dependency.
type MCPServerRequirement struct {
	// Name is the MCP server identifier (must match profile binding key)
	Name string `yaml:"name" json:"name"`

	// Optional indicates this MCP server is not required for the workflow to function
	Optional bool `yaml:"optional,omitempty" json:"optional,omitempty"`
}

// RequirementsDefinition declares abstract service dependencies for a workflow.
// This enables portable workflow definitions that don't embed credentials.
// Runtime bindings are provided by workspaces.
type RequirementsDefinition struct {
	// Integrations lists required integration dependencies.
	// Supports two formats:
	//   - Simple: "github" (requires integration of type github)
	//   - Aliased: "github as source" (requires github, bound to alias "source")
	Integrations []string `yaml:"integrations,omitempty" json:"integrations,omitempty"`

	// MCPServers lists required MCP server dependencies
	MCPServers []MCPServerRequirement `yaml:"mcp_servers,omitempty" json:"mcp_servers,omitempty"`
}

// Validate checks if the MCP server config is valid.
func (m *MCPServerConfig) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("mcp_server name is required")
	}

	if m.Command == "" {
		return fmt.Errorf("mcp_server command is required")
	}

	// Validate timeout if specified
	if m.Timeout < 0 {
		return fmt.Errorf("mcp_server timeout must be non-negative")
	}

	return nil
}

// Validate checks if the function definition is valid.
func (t *FunctionDefinition) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("function name is required")
	}

	if t.Type == "" {
		return fmt.Errorf("function type is required")
	}

	// Validate function type
	if !ValidToolTypes[t.Type] {
		return fmt.Errorf("invalid function type: %s (must be http or script)", t.Type)
	}

	if t.Description == "" {
		return fmt.Errorf("function description is required")
	}

	// Type-specific validation
	switch t.Type {
	case ToolTypeHTTP:
		if t.URL == "" {
			return fmt.Errorf("url is required for http function")
		}
		if t.Method == "" {
			return fmt.Errorf("method is required for http function")
		}
		// Validate HTTP method
		validMethods := map[string]bool{
			"GET":     true,
			"POST":    true,
			"PUT":     true,
			"PATCH":   true,
			"DELETE":  true,
			"HEAD":    true,
			"OPTIONS": true,
		}
		if !validMethods[t.Method] {
			return fmt.Errorf("invalid HTTP method: %s", t.Method)
		}

	case ToolTypeScript:
		if t.Command == "" {
			return fmt.Errorf("command is required for script tool")
		}
	}

	// Validate timeout if specified
	if t.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}
	if t.Timeout > 300 {
		return fmt.Errorf("timeout must not exceed 300 seconds")
	}

	// Validate max response size if specified
	if t.MaxResponseSize < 0 {
		return fmt.Errorf("max_response_size must be non-negative")
	}

	return nil
}
