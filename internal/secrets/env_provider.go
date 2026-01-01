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

package secrets

import (
	"context"
	"fmt"
	"os"

	"github.com/tombee/conductor/pkg/profile"
)

// EnvProvider implements secret resolution from environment variables.
//
// Reference format:
//   - env:GITHUB_TOKEN -> resolves GITHUB_TOKEN environment variable
//
// The provider respects profile inherit_env settings for access control.
type EnvProvider struct {
	// inheritEnv controls environment variable access
	inheritEnv profile.InheritEnvConfig
}

// NewEnvProvider creates a new environment variable secret provider.
// The inheritEnv config controls which environment variables can be accessed.
func NewEnvProvider(inheritEnv profile.InheritEnvConfig) *EnvProvider {
	return &EnvProvider{
		inheritEnv: inheritEnv,
	}
}

// Scheme returns the provider's URI scheme identifier.
func (e *EnvProvider) Scheme() string {
	return "env"
}

// Resolve retrieves a secret value from an environment variable.
//
// The reference should be the environment variable name.
// Example: "GITHUB_TOKEN"
//
// Access control:
//   - If inherit_env.enabled is false, all access is denied
//   - If inherit_env.allowlist is specified, only matching variables are accessible
//   - If inherit_env.enabled is true with no allowlist, all variables are accessible
func (e *EnvProvider) Resolve(ctx context.Context, reference string) (string, error) {
	// Check if environment variable access is allowed
	if !e.inheritEnv.Enabled {
		return "", fmt.Errorf("environment variable access disabled by profile")
	}

	// Check allowlist if specified
	if len(e.inheritEnv.Allowlist) > 0 {
		if !e.isAllowed(reference) {
			return "", profile.NewSecretResolutionError(
				profile.ErrorCategoryAccessDenied,
				"env:"+reference,
				"env",
				"environment variable not in allowlist",
				nil,
			)
		}
	}

	// Retrieve environment variable
	value := os.Getenv(reference)
	if value == "" {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryNotFound,
			"env:"+reference,
			"env",
			"environment variable not set",
			nil,
		)
	}

	return value, nil
}

// isAllowed checks if an environment variable name matches the allowlist.
// Supports glob patterns (simplified - only prefix matching with *).
func (e *EnvProvider) isAllowed(varName string) bool {
	for _, pattern := range e.inheritEnv.Allowlist {
		if matchesPattern(varName, pattern) {
			return true
		}
	}
	return false
}

// matchesPattern performs simple glob pattern matching.
// Supports:
//   - Exact match: "FOO" matches "FOO"
//   - Prefix wildcard: "FOO_*" matches "FOO_BAR", "FOO_BAZ"
//   - Suffix wildcard: "*_KEY" matches "API_KEY", "SECRET_KEY"
func matchesPattern(value, pattern string) bool {
	// Exact match
	if pattern == value {
		return true
	}

	// No wildcard
	if len(pattern) == 0 {
		return false
	}

	// Prefix wildcard: FOO_*
	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(value) >= len(prefix) && value[:len(prefix)] == prefix
	}

	// Suffix wildcard: *_KEY
	if pattern[0] == '*' {
		suffix := pattern[1:]
		return len(value) >= len(suffix) && value[len(value)-len(suffix):] == suffix
	}

	return false
}
