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

package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tombee/conductor/pkg/profile"
)

// PlaintextCredentialPattern represents a pattern for detecting plaintext credentials.
type PlaintextCredentialPattern struct {
	Name    string
	Pattern *regexp.Regexp
}

var (
	// PlaintextCredentialPatterns contains patterns for detecting common credential formats.
	// These patterns are used to warn users about embedded credentials in profiles.
	PlaintextCredentialPatterns = []PlaintextCredentialPattern{
		{
			Name:    "GitHub Token",
			Pattern: regexp.MustCompile(`\b(ghp_|gho_|ghu_|ghs_|ghr_)[a-zA-Z0-9]{36,}\b`),
		},
		{
			Name:    "Anthropic API Key",
			Pattern: regexp.MustCompile(`\bsk-ant-[a-zA-Z0-9-]{95,}\b`),
		},
		{
			Name:    "OpenAI API Key",
			Pattern: regexp.MustCompile(`\bsk-[a-zA-Z0-9]{20,}\b`),
		},
		{
			Name:    "AWS Access Key",
			Pattern: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		},
		{
			Name:    "Slack Token",
			Pattern: regexp.MustCompile(`\b(xoxb-|xoxp-|xoxa-|xoxr-)[0-9]{10,13}-[0-9]{10,13}-[a-zA-Z0-9]{24,}\b`),
		},
	}
)

// ValidateProfiles validates all profiles in all workspaces.
// Returns validation errors and warnings about plaintext credentials.
func ValidateProfiles(workspaces map[string]Workspace) ([]string, []string, error) {
	var errors []string
	var warnings []string

	for workspaceName, workspace := range workspaces {
		// Validate workspace has at least one profile
		if len(workspace.Profiles) == 0 {
			warnings = append(warnings, fmt.Sprintf("workspace %q has no profiles", workspaceName))
			continue
		}

		// Validate default profile exists if specified
		if workspace.DefaultProfile != "" {
			if _, exists := workspace.Profiles[workspace.DefaultProfile]; !exists {
				errors = append(errors, fmt.Sprintf("workspace %q: default_profile %q not found in profiles", workspaceName, workspace.DefaultProfile))
			}
		}

		// Validate each profile
		for profileName, prof := range workspace.Profiles {
			profilePath := fmt.Sprintf("workspaces.%s.profiles.%s", workspaceName, profileName)

			// Validate profile name
			// Allow "default" profile in "default" workspace for backward compatibility
			if !(workspaceName == "default" && profileName == "default") {
				if err := profile.ValidateProfileName(profileName); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", profilePath, err))
				}
			}

			// Check for plaintext credentials in bindings
			credWarnings := detectPlaintextCredentials(profilePath, prof)
			warnings = append(warnings, credWarnings...)

			// Validate inherit_env allowlist patterns
			if prof.InheritEnv.Enabled && len(prof.InheritEnv.Allowlist) > 0 {
				for _, pattern := range prof.InheritEnv.Allowlist {
					if err := validateAllowlistPattern(pattern); err != nil {
						errors = append(errors, fmt.Sprintf("%s.inherit_env.allowlist: invalid pattern %q: %v", profilePath, pattern, err))
					}
				}
			}

			// Validate connector bindings
			for connectorName, binding := range prof.Bindings.Integrations {
				connectorPath := fmt.Sprintf("%s.bindings.connectors.%s", profilePath, connectorName)
				if err := validateConnectorBinding(connectorPath, binding); err != nil {
					errors = append(errors, err.Error())
				}
			}

			// Validate MCP server bindings
			for serverName, binding := range prof.Bindings.MCPServers {
				serverPath := fmt.Sprintf("%s.bindings.mcp_servers.%s", profilePath, serverName)
				if err := validateMCPServerBinding(serverPath, binding); err != nil {
					errors = append(errors, err.Error())
				}
			}
		}
	}

	if len(errors) > 0 {
		return errors, warnings, fmt.Errorf("profile validation failed: %d error(s) found", len(errors))
	}

	return nil, warnings, nil
}

// detectPlaintextCredentials scans profile bindings for plaintext credentials.
func detectPlaintextCredentials(profilePath string, prof profile.Profile) []string {
	var warnings []string

	// Check connector auth fields
	for connectorName, binding := range prof.Bindings.Integrations {
		credPath := fmt.Sprintf("%s.bindings.connectors.%s.auth", profilePath, connectorName)

		// Check token
		if binding.Auth.Token != "" && !isSecretReference(binding.Auth.Token) {
			for _, pattern := range PlaintextCredentialPatterns {
				if pattern.Pattern.MatchString(binding.Auth.Token) {
					warnings = append(warnings, fmt.Sprintf("%s.token: detected plaintext %s (use secret reference instead)", credPath, pattern.Name))
				}
			}
		}

		// Check password
		if binding.Auth.Password != "" && !isSecretReference(binding.Auth.Password) {
			for _, pattern := range PlaintextCredentialPatterns {
				if pattern.Pattern.MatchString(binding.Auth.Password) {
					warnings = append(warnings, fmt.Sprintf("%s.password: detected plaintext %s (use secret reference instead)", credPath, pattern.Name))
				}
			}
		}

		// Check API key value
		if binding.Auth.Value != "" && !isSecretReference(binding.Auth.Value) {
			for _, pattern := range PlaintextCredentialPatterns {
				if pattern.Pattern.MatchString(binding.Auth.Value) {
					warnings = append(warnings, fmt.Sprintf("%s.value: detected plaintext %s (use secret reference instead)", credPath, pattern.Name))
				}
			}
		}
	}

	// Check MCP server env vars
	for serverName, binding := range prof.Bindings.MCPServers {
		for envKey, envValue := range binding.Env {
			if envValue != "" && !isSecretReference(envValue) {
				for _, pattern := range PlaintextCredentialPatterns {
					if pattern.Pattern.MatchString(envValue) {
						warnings = append(warnings, fmt.Sprintf("%s.bindings.mcp_servers.%s.env.%s: detected plaintext %s (use secret reference instead)", profilePath, serverName, envKey, pattern.Name))
					}
				}
			}
		}
	}

	return warnings
}

// isSecretReference checks if a value is a secret reference (${VAR}, env:VAR, file:path, etc.)
func isSecretReference(value string) bool {
	// Check for ${...} syntax
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return true
	}

	// Check for explicit scheme (env:, file:, vault:, etc.)
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		scheme := parts[0]
		// Common secret provider schemes
		validSchemes := []string{"env", "file", "vault", "1password", "aws-secrets", "ref"}
		for _, s := range validSchemes {
			if scheme == s {
				return true
			}
		}
	}

	return false
}

// validateAllowlistPattern validates an environment variable allowlist pattern.
func validateAllowlistPattern(pattern string) error {
	// Pattern must not be empty
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}

	// Pattern can contain alphanumeric, underscore, and wildcard
	// Valid: CONDUCTOR_*, CI, GITHUB_*, MY_VAR_*
	validPattern := regexp.MustCompile(`^[A-Z0-9_*]+$`)
	if !validPattern.MatchString(pattern) {
		return fmt.Errorf("pattern must contain only uppercase letters, digits, underscores, and asterisks")
	}

	// Asterisk can only appear at the end
	if strings.Contains(pattern, "*") {
		if !strings.HasSuffix(pattern, "*") {
			return fmt.Errorf("wildcard (*) can only appear at the end of pattern")
		}
		if strings.Count(pattern, "*") > 1 {
			return fmt.Errorf("only one wildcard (*) is allowed per pattern")
		}
	}

	return nil
}

// validateConnectorBinding validates a connector binding.
func validateConnectorBinding(path string, binding profile.IntegrationBinding) error {
	// If basic auth is used, both username and password should be present
	if binding.Auth.Username != "" && binding.Auth.Password == "" {
		return fmt.Errorf("%s: basic auth requires both username and password", path)
	}
	if binding.Auth.Password != "" && binding.Auth.Username == "" {
		return fmt.Errorf("%s: basic auth requires both username and password", path)
	}

	// If header auth is used, both header and value should be present
	if binding.Auth.Header != "" && binding.Auth.Value == "" {
		return fmt.Errorf("%s: header auth requires both header name and value", path)
	}
	if binding.Auth.Value != "" && binding.Auth.Header == "" {
		return fmt.Errorf("%s: header auth requires both header name and value", path)
	}

	// Multiple auth methods should not be mixed
	authMethods := 0
	if binding.Auth.Token != "" {
		authMethods++
	}
	if binding.Auth.Username != "" && binding.Auth.Password != "" {
		authMethods++
	}
	if binding.Auth.Header != "" && binding.Auth.Value != "" {
		authMethods++
	}
	if authMethods > 1 {
		return fmt.Errorf("%s: cannot use multiple auth methods (token, basic auth, header auth) simultaneously", path)
	}

	return nil
}

// validateMCPServerBinding validates an MCP server binding.
func validateMCPServerBinding(path string, binding profile.MCPServerBinding) error {
	// Command is required
	if binding.Command == "" {
		return fmt.Errorf("%s: command is required", path)
	}

	// Timeout should be positive if specified
	if binding.Timeout < 0 {
		return fmt.Errorf("%s: timeout must be non-negative, got %d", path, binding.Timeout)
	}

	return nil
}
