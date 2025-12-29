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

package security

// AccessConfig holds all resource access declarations for a workflow.
// This is the new explicit access control model that replaces preset-based profiles.
type AccessConfig struct {
	Filesystem FilesystemAccess `yaml:"filesystem,omitempty" json:"filesystem,omitempty"`
	Network    NetworkAccess    `yaml:"network,omitempty" json:"network,omitempty"`
	Shell      ShellAccess      `yaml:"shell,omitempty" json:"shell,omitempty"`
}

// FilesystemAccess declares filesystem permissions.
type FilesystemAccess struct {
	// Read patterns allow reading files matching these globs.
	// Supports: ./relative/**, /absolute/path, $cwd/**, $temp/**
	Read []string `yaml:"read,omitempty" json:"read,omitempty"`

	// Write patterns allow writing files matching these globs.
	Write []string `yaml:"write,omitempty" json:"write,omitempty"`

	// Deny patterns block access even if Read/Write would allow.
	// Applied after Read/Write, useful for excluding sensitive paths.
	Deny []string `yaml:"deny,omitempty" json:"deny,omitempty"`
}

// NetworkAccess declares network permissions.
type NetworkAccess struct {
	// Allow permits connections to these hostnames or IP ranges.
	// Supports: exact match, *.wildcard.com, host:port, CIDR (10.0.0.0/8)
	Allow []string `yaml:"allow,omitempty" json:"allow,omitempty"`

	// Deny blocks connections to these hostnames or IP ranges.
	// Applied after Allow - use to carve out exceptions.
	Deny []string `yaml:"deny,omitempty" json:"deny,omitempty"`
}

// ShellAccess declares shell execution permissions.
type ShellAccess struct {
	// Commands allows executing these commands.
	// Supports: command name, command subcommand, command *
	Commands []string `yaml:"commands,omitempty" json:"commands,omitempty"`

	// DenyPatterns blocks commands matching these patterns.
	// Applied after Commands, useful for blocking dangerous flags.
	DenyPatterns []string `yaml:"deny_patterns,omitempty" json:"deny_patterns,omitempty"`
}

// BoundaryConfig defines the maximum allowed permissions for any workflow.
// This is the security ceiling set by platform operators.
type BoundaryConfig struct {
	Filesystem FilesystemAccess `yaml:"filesystem,omitempty" json:"filesystem,omitempty"`
	Network    NetworkAccess    `yaml:"network,omitempty" json:"network,omitempty"`
	Shell      ShellAccess      `yaml:"shell,omitempty" json:"shell,omitempty"`
}

// ActionsConfig controls which action types are available.
type ActionsConfig struct {
	Shell ShellActionConfig `yaml:"shell,omitempty" json:"shell,omitempty"`
}

// ShellActionConfig controls shell action availability.
type ShellActionConfig struct {
	// Enabled controls whether shell action is available (default: true)
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// MCPSecurityConfig controls MCP server availability.
type MCPSecurityConfig struct {
	// Enabled controls whether MCP servers are available (default: true)
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Allowlist limits which MCP servers can be used (empty = all allowed)
	Allowlist []string `yaml:"allowlist,omitempty" json:"allowlist,omitempty"`
}
