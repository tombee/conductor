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

package profile

import (
	"fmt"
	"regexp"
)

// Profile name validation constraints
const (
	// MaxProfileNameLength is the maximum length for profile names
	MaxProfileNameLength = 64

	// MaxWorkspaceNameLength is the maximum length for workspace names
	MaxWorkspaceNameLength = 64
)

var (
	// profileNameRegex matches valid profile names: lowercase alphanumeric, underscore, hyphen
	// Pattern: ^[a-z0-9_-]+$
	profileNameRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)

	// reservedProfileNames cannot be used for user-defined profiles
	reservedProfileNames = map[string]bool{
		"default": true,
		"system":  true,
	}

	// reservedWorkspaceNames cannot be used for user-defined workspaces
	reservedWorkspaceNames = map[string]bool{
		"default": true,
		"system":  true,
	}
)

// ValidateProfileName checks if a profile name is valid.
// Valid names:
//   - Match regex: ^[a-z0-9_-]+$
//   - Length: 1-64 characters
//   - Not reserved: "default", "system"
func ValidateProfileName(name string) error {
	if name == "" {
		return &ValidationError{
			Field:   "name",
			Message: "profile name cannot be empty",
		}
	}

	if len(name) > MaxProfileNameLength {
		return &ValidationError{
			Field:   "name",
			Message: fmt.Sprintf("profile name exceeds maximum length of %d characters", MaxProfileNameLength),
			Value:   name,
		}
	}

	if !profileNameRegex.MatchString(name) {
		return &ValidationError{
			Field:   "name",
			Message: "profile name must contain only lowercase letters, numbers, underscores, and hyphens",
			Value:   name,
		}
	}

	if reservedProfileNames[name] {
		return &ValidationError{
			Field:   "name",
			Message: fmt.Sprintf("profile name %q is reserved", name),
			Value:   name,
		}
	}

	return nil
}

// ValidateWorkspaceName checks if a workspace name is valid.
// Uses same rules as profile names for consistency.
func ValidateWorkspaceName(name string) error {
	if name == "" {
		return &ValidationError{
			Field:   "name",
			Message: "workspace name cannot be empty",
		}
	}

	if len(name) > MaxWorkspaceNameLength {
		return &ValidationError{
			Field:   "name",
			Message: fmt.Sprintf("workspace name exceeds maximum length of %d characters", MaxWorkspaceNameLength),
			Value:   name,
		}
	}

	if !profileNameRegex.MatchString(name) {
		return &ValidationError{
			Field:   "name",
			Message: "workspace name must contain only lowercase letters, numbers, underscores, and hyphens",
			Value:   name,
		}
	}

	if reservedWorkspaceNames[name] {
		return &ValidationError{
			Field:   "name",
			Message: fmt.Sprintf("workspace name %q is reserved", name),
			Value:   name,
		}
	}

	return nil
}

// Validate checks if a Profile is valid.
func (p *Profile) Validate() error {
	// Validate profile name
	if err := ValidateProfileName(p.Name); err != nil {
		return err
	}

	// Validate bindings
	if err := p.Bindings.Validate(); err != nil {
		return fmt.Errorf("bindings: %w", err)
	}

	// Validate inherit_env allowlist patterns if specified
	if len(p.InheritEnv.Allowlist) > 0 {
		for _, pattern := range p.InheritEnv.Allowlist {
			if pattern == "" {
				return &ValidationError{
					Field:   "inherit_env.allowlist",
					Message: "allowlist patterns cannot be empty",
				}
			}
		}
	}

	return nil
}

// Validate checks if Bindings are valid.
func (b *Bindings) Validate() error {
	// Validate integration bindings
	for name, binding := range b.Integrations {
		if name == "" {
			return &ValidationError{
				Field:   "integrations",
				Message: "integration name cannot be empty",
			}
		}
		if err := binding.Validate(); err != nil {
			return fmt.Errorf("integration %q: %w", name, err)
		}
	}

	// Validate MCP server bindings
	for name, binding := range b.MCPServers {
		if name == "" {
			return &ValidationError{
				Field:   "mcp_servers",
				Message: "MCP server name cannot be empty",
			}
		}
		if err := binding.Validate(); err != nil {
			return fmt.Errorf("mcp_server %q: %w", name, err)
		}
	}

	return nil
}

// Validate checks if a IntegrationBinding is valid.
func (c *IntegrationBinding) Validate() error {
	// Auth validation
	if err := c.Auth.Validate(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	return nil
}

// Validate checks if an AuthBinding is valid.
func (a *AuthBinding) Validate() error {
	// At least one auth field should be present
	hasAuth := a.Token != "" || a.Username != "" || a.Password != "" || a.Value != ""
	if !hasAuth {
		return &ValidationError{
			Field:   "auth",
			Message: "at least one authentication credential must be provided (token, username/password, or header/value)",
		}
	}

	// If using basic auth, both username and password required
	if (a.Username != "" || a.Password != "") && (a.Username == "" || a.Password == "") {
		return &ValidationError{
			Field:   "auth",
			Message: "basic authentication requires both username and password",
		}
	}

	// If using API key auth, both header and value required
	if (a.Header != "" || a.Value != "") && (a.Header == "" || a.Value == "") {
		return &ValidationError{
			Field:   "auth",
			Message: "API key authentication requires both header and value",
		}
	}

	return nil
}

// Validate checks if an MCPServerBinding is valid.
func (m *MCPServerBinding) Validate() error {
	if m.Command == "" {
		return &ValidationError{
			Field:   "command",
			Message: "MCP server command is required",
		}
	}

	// Validate timeout if specified
	if m.Timeout < 0 {
		return &ValidationError{
			Field:   "timeout",
			Message: "timeout must be non-negative",
			Value:   fmt.Sprintf("%d", m.Timeout),
		}
	}

	return nil
}
