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
	"strings"
)

const (
	// EnvBackendPriority is the priority for environment variable backend.
	// This is the highest priority to allow environment overrides.
	EnvBackendPriority = 100

	// EnvSecretPrefix is the prefix for conductor-specific secret environment variables.
	envSecretPrefix = "CONDUCTOR_SECRET_"
)

// EnvBackend provides read-only access to secrets via environment variables.
// It supports multiple naming conventions:
//  1. CONDUCTOR_SECRET_<KEY> (normalized, e.g., CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY)
//  2. Provider-specific variables (e.g., ANTHROPIC_API_KEY, OPENAI_API_KEY)
type EnvBackend struct{}

// NewEnvBackend creates a new environment variable backend.
func NewEnvBackend() *EnvBackend {
	return &EnvBackend{}
}

// Name returns the backend identifier.
func (e *EnvBackend) Name() string {
	return "env"
}

// Get retrieves a secret from environment variables.
// It checks both CONDUCTOR_SECRET_* and provider-specific variables.
func (e *EnvBackend) Get(ctx context.Context, key string) (string, error) {
	// Try CONDUCTOR_SECRET_* first
	envKey := e.normalizeKey(key)
	if value := os.Getenv(envKey); value != "" {
		return value, nil
	}

	// Try provider-specific aliases
	if aliasKey := e.providerAlias(key); aliasKey != "" {
		if value := os.Getenv(aliasKey); value != "" {
			return value, nil
		}
	}

	return "", fmt.Errorf("%w: environment variable not set", ErrSecretNotFound)
}

// Set returns ErrReadOnlyBackend as environment backend is read-only.
func (e *EnvBackend) Set(ctx context.Context, key string, value string) error {
	return ErrReadOnlyBackend
}

// Delete returns ErrReadOnlyBackend as environment backend is read-only.
func (e *EnvBackend) Delete(ctx context.Context, key string) error {
	return ErrReadOnlyBackend
}

// List returns all CONDUCTOR_SECRET_* environment variables.
func (e *EnvBackend) List(ctx context.Context) ([]string, error) {
	var keys []string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, envSecretPrefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 && parts[1] != "" {
				// Convert CONDUCTOR_SECRET_FOO_BAR back to foo/bar
				key := e.denormalizeKey(parts[0])
				keys = append(keys, key)
			}
		}
	}
	return keys, nil
}

// Available returns true as environment variables are always available.
func (e *EnvBackend) Available() bool {
	return true
}

// Priority returns the backend priority (highest).
func (e *EnvBackend) Priority() int {
	return EnvBackendPriority
}

// ReadOnly returns true as environment backend is read-only.
func (e *EnvBackend) ReadOnly() bool {
	return true
}

// normalizeKey converts a secret key to an environment variable name.
// Example: "providers/anthropic/api_key" -> "CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY"
func (e *EnvBackend) normalizeKey(key string) string {
	// Replace slashes with underscores and convert to uppercase
	normalized := strings.ToUpper(strings.ReplaceAll(key, "/", "_"))
	return envSecretPrefix + normalized
}

// denormalizeKey converts an environment variable name back to a secret key.
// Example: "CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY" -> "providers/anthropic/api_key"
func (e *EnvBackend) denormalizeKey(envVar string) string {
	// Remove prefix
	key := strings.TrimPrefix(envVar, envSecretPrefix)

	// This is a lossy conversion since we can't distinguish between
	// underscores that were originally slashes vs. underscores that were
	// part of the key (e.g., "api_key"). We use a simple heuristic:
	// Convert to lowercase first, then only the first two underscores
	// are converted to slashes (for "providers/<name>/<key>" pattern).
	key = strings.ToLower(key)

	// Split on underscores and rejoin with slashes for the first 3 parts
	parts := strings.Split(key, "_")
	if len(parts) >= 3 {
		// Rejoin first part, second part, and rest with underscores preserved in the rest
		return parts[0] + "/" + parts[1] + "/" + strings.Join(parts[2:], "_")
	}

	// For simpler keys without the 3-part structure, just replace all underscores
	return strings.ReplaceAll(key, "_", "/")
}

// providerAlias returns a provider-specific environment variable name if applicable.
// Example: "providers/anthropic/api_key" -> "ANTHROPIC_API_KEY"
func (e *EnvBackend) providerAlias(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) >= 3 && parts[0] == "providers" && parts[2] == "api_key" {
		return strings.ToUpper(parts[1]) + "_API_KEY"
	}
	return ""
}
