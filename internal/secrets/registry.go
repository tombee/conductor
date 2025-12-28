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
	"regexp"
	"strings"

	"github.com/tombee/conductor/pkg/profile"
)

// Registry implements scheme-based secret provider routing.
// Unlike the existing Resolver which uses priority-based resolution,
// Registry routes secret references to specific providers based on URI schemes.
//
// Supported reference formats:
//   - env:VAR_NAME -> environment variable provider
//   - file:/path/to/secret -> file provider
//   - ${VAR_NAME} -> environment variable provider (legacy syntax)
//   - vault:secret/path -> vault provider (future)
type Registry struct {
	providers map[string]profile.SecretProvider
}

var (
	// legacyEnvVarRegex matches ${VAR_NAME} syntax
	legacyEnvVarRegex = regexp.MustCompile(`^\$\{([A-Z_][A-Z0-9_]*)\}$`)

	// schemeRegex matches scheme:reference format
	schemeRegex = regexp.MustCompile(`^([a-z][a-z0-9]*):(.+)$`)
)

// NewRegistry creates a new secret provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]profile.SecretProvider),
	}
}

// Register adds a provider to the registry.
// Returns an error if a provider with the same scheme already exists.
func (r *Registry) Register(provider profile.SecretProvider) error {
	scheme := provider.Scheme()
	if _, exists := r.providers[scheme]; exists {
		return fmt.Errorf("provider for scheme %q already registered", scheme)
	}
	r.providers[scheme] = provider
	return nil
}

// Resolve routes a secret reference to the appropriate provider and returns the value.
//
// Reference formats:
//   - "env:GITHUB_TOKEN" -> env provider
//   - "file:/etc/secrets/token" -> file provider
//   - "${API_KEY}" -> env provider (legacy)
//   - "vault:secret/data/prod#token" -> vault provider (future)
//
// Returns sanitized errors that don't leak secret values or paths.
func (r *Registry) Resolve(ctx context.Context, reference string) (string, error) {
	// Parse reference to extract scheme and key
	scheme, key, err := r.parseReference(reference)
	if err != nil {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryInvalidSyntax,
			reference,
			"",
			"invalid secret reference syntax",
			err,
		)
	}

	// Get provider for scheme
	provider, exists := r.providers[scheme]
	if !exists {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryNotFound,
			reference,
			scheme,
			fmt.Sprintf("no provider registered for scheme %q", scheme),
			nil,
		)
	}

	// Resolve secret through provider
	value, err := provider.Resolve(ctx, key)
	if err != nil {
		// Wrap provider error with sanitized error
		// The provider error is kept as OriginalError for server-side logging only
		return "", profile.NewSecretResolutionError(
			categorizeProviderError(err),
			reference,
			scheme,
			"secret resolution failed",
			err,
		)
	}

	return value, nil
}

// GetProvider returns the provider for the given scheme.
// Returns nil if no provider is registered for the scheme.
func (r *Registry) GetProvider(scheme string) profile.SecretProvider {
	return r.providers[scheme]
}

// parseReference extracts the scheme and key from a secret reference.
//
// Supported formats:
//   - "env:VAR_NAME" -> ("env", "VAR_NAME")
//   - "file:/path/to/secret" -> ("file", "/path/to/secret")
//   - "${VAR_NAME}" -> ("env", "VAR_NAME")  // legacy syntax
func (r *Registry) parseReference(reference string) (scheme, key string, err error) {
	if reference == "" {
		return "", "", fmt.Errorf("empty reference")
	}

	// Check for legacy ${VAR} syntax
	if matches := legacyEnvVarRegex.FindStringSubmatch(reference); matches != nil {
		return "env", matches[1], nil
	}

	// Check for scheme:reference format
	if matches := schemeRegex.FindStringSubmatch(reference); matches != nil {
		scheme := matches[1]
		key := matches[2]

		// Validate key is not empty
		if strings.TrimSpace(key) == "" {
			return "", "", fmt.Errorf("empty key for scheme %q", scheme)
		}

		return scheme, key, nil
	}

	// No scheme specified - treat as plain value (for backward compatibility)
	// This allows profiles to use plain strings alongside secret references
	return "plain", reference, nil
}

// categorizeProviderError categorizes provider errors into error categories.
func categorizeProviderError(err error) profile.ErrorCategory {
	if err == nil {
		return ""
	}

	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	// Check for common error patterns
	switch {
	case strings.Contains(errMsgLower, "not found"):
		return profile.ErrorCategoryNotFound
	case strings.Contains(errMsgLower, "permission denied"), strings.Contains(errMsgLower, "access denied"):
		return profile.ErrorCategoryAccessDenied
	case strings.Contains(errMsgLower, "timeout"), strings.Contains(errMsgLower, "deadline exceeded"):
		return profile.ErrorCategoryTimeout
	case strings.Contains(errMsgLower, "invalid"), strings.Contains(errMsgLower, "malformed"):
		return profile.ErrorCategoryInvalidSyntax
	default:
		return profile.ErrorCategoryNotFound // Default to NOT_FOUND
	}
}
