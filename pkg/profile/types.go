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

// Package profile provides workflow execution profile management.
//
// This package implements the separation between workflow definitions (portable logic)
// and execution profiles (runtime bindings). Profiles contain
// credentials, service bindings, and environment-specific configuration that workflows
// need at runtime.
package profile

import "time"

// Profile represents a named execution configuration within a workspace.
// Profiles contain the runtime bindings (credentials, MCP servers, environment settings)
// needed to execute workflows in a specific environment (dev/staging/prod).
//
// Profiles are nested within workspaces to prevent naming collisions and enable team isolation.
type Profile struct {
	// Name is the profile identifier (workspace-scoped)
	// Must match regex: ^[a-z0-9_-]+$
	// Max length: 64 characters
	// Reserved names: "default", "system"
	Name string `yaml:"name" json:"name"`

	// Description provides human-readable context about this profile
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// InheritEnv controls environment variable access for secret resolution
	// When true: ${VAR} references resolve from process environment
	// When false: strict isolation, only profile bindings are available
	// Can be boolean (true/false) or object with allowlist
	InheritEnv InheritEnvConfig `yaml:"inherit_env,omitempty" json:"inherit_env,omitempty"`

	// Bindings maps workflow requirements to concrete values
	// Structure: bindings.integrations.<name>.auth.token = ${SECRET}
	//           bindings.mcp_servers.<name>.env = {...}
	Bindings Bindings `yaml:"bindings,omitempty" json:"bindings,omitempty"`
}

// InheritEnvConfig controls environment variable inheritance for secret resolution.
// This can be either a simple boolean or an object with an allowlist for security.
type InheritEnvConfig struct {
	// Enabled controls whether environment variables are accessible
	// Default: true (backward compatibility)
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Allowlist specifies which environment variables can be accessed
	// Supports glob patterns: CONDUCTOR_*, CI, GITHUB_*
	// If empty and Enabled is true, all variables are accessible
	// Recommended for production: use allowlist to prevent leakage
	Allowlist []string `yaml:"allowlist,omitempty" json:"allowlist,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling to support both boolean and object syntax.
func (c *InheritEnvConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try boolean first
	var boolValue bool
	if err := unmarshal(&boolValue); err == nil {
		c.Enabled = boolValue
		c.Allowlist = nil
		return nil
	}

	// Try object form
	type plain InheritEnvConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return nil
}

// Bindings maps abstract workflow requirements to concrete runtime values.
type Bindings struct {
	// Integrations maps integration names to their configuration
	// Key: integration name from workflow requires section
	// Value: integration binding with auth and other settings
	Integrations map[string]IntegrationBinding `yaml:"integrations,omitempty" json:"integrations,omitempty"`

	// MCPServers maps MCP server names to their configuration
	// Key: MCP server name from workflow requires section
	// Value: MCP server binding with command, args, env
	MCPServers map[string]MCPServerBinding `yaml:"mcp_servers,omitempty" json:"mcp_servers,omitempty"`
}

// IntegrationBinding provides runtime configuration for an integration.
type IntegrationBinding struct {
	// Auth contains authentication credentials
	// Can reference secrets: ${GITHUB_TOKEN}, env:API_KEY, file:/path/to/secret
	Auth AuthBinding `yaml:"auth,omitempty" json:"auth,omitempty"`

	// BaseURL overrides the integration's default base URL (optional)
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`

	// Headers provides additional HTTP headers (optional)
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// AuthBinding contains authentication credentials for an integration.
// Supports multiple auth types: bearer tokens, basic auth, API keys.
type AuthBinding struct {
	// Token for bearer authentication
	// Supports secret references: ${GITHUB_TOKEN}, env:TOKEN, file:/path/to/token
	Token string `yaml:"token,omitempty" json:"token,omitempty"`

	// Username for basic authentication
	Username string `yaml:"username,omitempty" json:"username,omitempty"`

	// Password for basic authentication
	// Supports secret references
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// Header name for API key authentication (e.g., "X-API-Key")
	Header string `yaml:"header,omitempty" json:"header,omitempty"`

	// Value for API key authentication
	// Supports secret references
	Value string `yaml:"value,omitempty" json:"value,omitempty"`
}

// MCPServerBinding provides runtime configuration for an MCP server.
type MCPServerBinding struct {
	// Command is the executable to run (e.g., "npx", "python")
	Command string `yaml:"command" json:"command"`

	// Args are command-line arguments
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`

	// Env are environment variables to pass to the server
	// Supports secret references in values
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Timeout is the default timeout for tool calls in seconds
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// SecretReference represents a reference to a secret value.
// This type is used during binding resolution to track where a value comes from.
type SecretReference struct {
	// Raw is the original reference string (e.g., "${GITHUB_TOKEN}", "env:API_KEY")
	Raw string

	// Scheme identifies the secret provider (env, file, vault, etc.)
	Scheme string

	// Key is the provider-specific identifier
	// For env: environment variable name
	// For file: file path
	// For vault: secret path
	Key string

	// ResolvedAt tracks when this reference was resolved
	ResolvedAt time.Time

	// ResolvedBy identifies which provider resolved this reference
	ResolvedBy string
}

// SecretMetadata tracks information about secret resolution for audit logging.
// This is used to log secret access without exposing the actual values.
type SecretMetadata struct {
	// Reference is the truncated reference (e.g., "ref:vault/***-token")
	Reference string `json:"reference"`

	// Provider is the provider name that resolved this secret
	Provider string `json:"provider"`

	// Success indicates whether resolution succeeded
	Success bool `json:"success"`

	// RunID is the run that requested this secret
	RunID string `json:"run_id"`

	// Workspace is the workspace context
	Workspace string `json:"workspace"`

	// Profile is the profile name
	Profile string `json:"profile"`

	// Timestamp is when the resolution occurred
	Timestamp time.Time `json:"timestamp"`

	// ErrorCategory for failed resolutions (NOT_FOUND, ACCESS_DENIED, etc.)
	ErrorCategory string `json:"error_category,omitempty"`
}
