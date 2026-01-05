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

type PermissionDefinition struct {
	// Paths controls file system access
	Paths *PathPermissions `yaml:"paths,omitempty" json:"paths,omitempty"`

	// Network controls network access
	Network *NetworkPermissions `yaml:"network,omitempty" json:"network,omitempty"`

	// Secrets controls which secrets can be accessed
	Secrets *SecretPermissions `yaml:"secrets,omitempty" json:"secrets,omitempty"`

	// Tools controls which tools can be used
	Tools *ToolPermissions `yaml:"tools,omitempty" json:"tools,omitempty"`

	// Shell controls shell command execution
	Shell *ShellPermissions `yaml:"shell,omitempty" json:"shell,omitempty"`

	// Env controls environment variable access
	Env *EnvPermissions `yaml:"env,omitempty" json:"env,omitempty"`

	// AcceptUnenforceable allows running even when some permissions cannot be enforced
	// by the chosen provider. This must be explicitly set to true to acknowledge
	// that some security restrictions may not be enforced.
	AcceptUnenforceable bool `yaml:"accept_unenforceable,omitempty" json:"accept_unenforceable,omitempty"`

	// AcceptUnenforceableFor lists specific providers for which unenforceable
	// permissions are acceptable.
	AcceptUnenforceableFor []string `yaml:"accept_unenforceable_for,omitempty" json:"accept_unenforceable_for,omitempty"`
}

// PathPermissions controls file system access.
// Uses gitignore-style glob patterns with support for **, *, and ! negation.
type PathPermissions struct {
	// Read patterns for allowed read paths (default: ["**/*"] = all)
	Read []string `yaml:"read,omitempty" json:"read,omitempty"`

	// Write patterns for allowed write paths (default: ["$out/**"] = output dir only)
	Write []string `yaml:"write,omitempty" json:"write,omitempty"`
}

// NetworkPermissions controls network access.
type NetworkPermissions struct {
	// AllowedHosts patterns for allowed hosts (empty = all allowed)
	// Supports wildcards like "*.github.com", "api.openai.com"
	AllowedHosts []string `yaml:"allowed_hosts,omitempty" json:"allowed_hosts,omitempty"`

	// BlockedHosts patterns for blocked hosts (always blocked)
	// Default includes cloud metadata endpoints and private IP ranges
	BlockedHosts []string `yaml:"blocked_hosts,omitempty" json:"blocked_hosts,omitempty"`
}

// SecretPermissions controls which secrets can be accessed.
type SecretPermissions struct {
	// Allowed patterns for allowed secret names (default: ["*"] = all)
	// Supports wildcards like "GITHUB_*", "OPENAI_API_KEY"
	Allowed []string `yaml:"allowed,omitempty" json:"allowed,omitempty"`
}

// ToolPermissions controls which tools can be used by LLM steps.
type ToolPermissions struct {
	// Allowed patterns for allowed tool names (default: ["*"] = all)
	// Supports wildcards like "file.*", "transform.*"
	Allowed []string `yaml:"allowed,omitempty" json:"allowed,omitempty"`

	// Blocked patterns for blocked tool names (takes precedence over allowed)
	// Supports wildcards like "shell.*", "!shell.run"
	Blocked []string `yaml:"blocked,omitempty" json:"blocked,omitempty"`
}

// ShellPermissions controls shell command execution.
type ShellPermissions struct {
	// Enabled controls whether shell.run is allowed (default: false)
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// AllowedCommands restricts to specific command prefixes when enabled
	// Example: ["git", "npm"] allows "git status", "npm install", etc.
	AllowedCommands []string `yaml:"allowed_commands,omitempty" json:"allowed_commands,omitempty"`
}

// EnvPermissions controls environment variable access.
type EnvPermissions struct {
	// Inherit controls whether to inherit the process environment (default: false)
	Inherit bool `yaml:"inherit,omitempty" json:"inherit,omitempty"`

	// Allowed patterns for allowed environment variables when inherit is true
	// Default: ["CI", "PATH", "HOME", "USER", "TERM"]
	Allowed []string `yaml:"allowed,omitempty" json:"allowed,omitempty"`
}

// IsShellEnabled returns whether shell execution is enabled.
// Returns false if Enabled is nil (default is disabled).
func (s *ShellPermissions) IsShellEnabled() bool {
	if s == nil || s.Enabled == nil {
		return false
	}
	return *s.Enabled
}

// Validate checks if the permission definition is valid.
func (p *PermissionDefinition) Validate() error {
	if p == nil {
		return nil
	}

	// Validate path permissions
	if p.Paths != nil {
		if err := p.Paths.Validate(); err != nil {
			return fmt.Errorf("paths: %w", err)
		}
	}

	// Validate network permissions
	if p.Network != nil {
		if err := p.Network.Validate(); err != nil {
			return fmt.Errorf("network: %w", err)
		}
	}

	// Validate secrets permissions
	if p.Secrets != nil {
		if err := p.Secrets.Validate(); err != nil {
			return fmt.Errorf("secrets: %w", err)
		}
	}

	// Validate tools permissions
	if p.Tools != nil {
		if err := p.Tools.Validate(); err != nil {
			return fmt.Errorf("tools: %w", err)
		}
	}

	// Validate shell permissions
	if p.Shell != nil {
		if err := p.Shell.Validate(); err != nil {
			return fmt.Errorf("shell: %w", err)
		}
	}

	// Validate env permissions
	if p.Env != nil {
		if err := p.Env.Validate(); err != nil {
			return fmt.Errorf("env: %w", err)
		}
	}

	return nil
}

// Validate checks if path permissions are valid.
func (p *PathPermissions) Validate() error {
	// Validate glob patterns
	for _, pattern := range p.Read {
		if err := validateGlobPattern(pattern); err != nil {
			return fmt.Errorf("read pattern %q: %w", pattern, err)
		}
	}
	for _, pattern := range p.Write {
		if err := validateGlobPattern(pattern); err != nil {
			return fmt.Errorf("write pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// Validate checks if network permissions are valid.
func (n *NetworkPermissions) Validate() error {
	// Validate host patterns
	for _, pattern := range n.AllowedHosts {
		if err := validateHostPattern(pattern); err != nil {
			return fmt.Errorf("allowed_hosts pattern %q: %w", pattern, err)
		}
	}
	for _, pattern := range n.BlockedHosts {
		if err := validateHostPattern(pattern); err != nil {
			return fmt.Errorf("blocked_hosts pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// Validate checks if secret permissions are valid.
func (s *SecretPermissions) Validate() error {
	// Validate name patterns
	for _, pattern := range s.Allowed {
		if err := validateNamePattern(pattern); err != nil {
			return fmt.Errorf("allowed pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// Validate checks if tool permissions are valid.
func (t *ToolPermissions) Validate() error {
	// Validate tool name patterns
	for _, pattern := range t.Allowed {
		if err := validateToolPattern(pattern); err != nil {
			return fmt.Errorf("allowed pattern %q: %w", pattern, err)
		}
	}
	for _, pattern := range t.Blocked {
		if err := validateToolPattern(pattern); err != nil {
			return fmt.Errorf("blocked pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// Validate checks if shell permissions are valid.
func (s *ShellPermissions) Validate() error {
	// Validate command names (basic check - no path separators)
	for _, cmd := range s.AllowedCommands {
		if cmd == "" {
			return fmt.Errorf("empty command name not allowed")
		}
		// Command should not contain path separators
		for _, ch := range cmd {
			if ch == '/' || ch == '\\' {
				return fmt.Errorf("command %q should not contain path separators", cmd)
			}
		}
	}
	return nil
}

// Validate checks if env permissions are valid.
func (e *EnvPermissions) Validate() error {
	// Validate env var name patterns
	for _, pattern := range e.Allowed {
		if err := validateNamePattern(pattern); err != nil {
			return fmt.Errorf("allowed pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// validateGlobPattern validates a gitignore-style glob pattern.
func validateGlobPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty pattern not allowed")
	}
	// Check for invalid characters
	// Glob patterns can contain most characters, but we check for obvious issues
	// Actual matching will use doublestar library which handles validation
	return nil
}

// validateHostPattern validates a host pattern (e.g., "*.github.com").
func validateHostPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty pattern not allowed")
	}
	// Basic validation - host patterns can have wildcards but not path components
	for _, ch := range pattern {
		if ch == '/' || ch == '\\' {
			return fmt.Errorf("host pattern should not contain path separators")
		}
	}
	return nil
}

// validateNamePattern validates a name pattern (for secrets, env vars).
func validateNamePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty pattern not allowed")
	}
	return nil
}

// validateToolPattern validates a tool name pattern (e.g., "file.*", "!shell.run").
func validateToolPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty pattern not allowed")
	}
	// Remove leading ! for negation patterns
	if len(pattern) > 0 && pattern[0] == '!' {
		pattern = pattern[1:]
	}
	if pattern == "" {
		return fmt.Errorf("empty pattern after negation not allowed")
	}
	return nil
}
