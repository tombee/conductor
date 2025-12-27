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

import "context"

// SecretProvider defines the interface for secret resolution backends.
//
// Providers implement different secret storage mechanisms (environment variables,
// files, external services like Vault or 1Password) and are registered with a
// scheme (e.g., "env", "file", "vault").
//
// Secret references in profiles are resolved through providers at run time:
//   - env:VAR_NAME -> environment variable
//   - file:/path/to/secret -> file contents
//   - ${VAR} -> environment variable (backward compatibility)
//   - vault:secret/path -> Vault KV secret (future)
//
// Providers must implement timeout and cancellation via the context parameter
// (SPEC-130 NFR10: 30 second timeout per provider).
type SecretProvider interface {
	// Scheme returns the provider's URI scheme identifier.
	// Examples: "env", "file", "vault", "1password"
	// The scheme is used to route secret references to the correct provider.
	Scheme() string

	// Resolve retrieves the secret value for the given reference.
	//
	// The reference format is provider-specific:
	//   - env provider: "VAR_NAME" (no prefix)
	//   - file provider: "/absolute/path/to/secret"
	//   - vault provider: "secret/data/path#field" (future)
	//
	// Returns:
	//   - The secret value (plaintext)
	//   - SecretResolutionError if resolution fails
	//
	// The implementation must:
	//   - Respect context timeout/cancellation
	//   - Not log secret values
	//   - Return sanitized errors (use NewSecretResolutionError)
	//
	// Context support (NFR10):
	//   - Must complete within 30 seconds or context deadline
	//   - Must handle context cancellation gracefully
	//   - Must return appropriate error category on timeout
	Resolve(ctx context.Context, reference string) (string, error)
}

// SecretProviderRegistry manages registered secret providers and routes
// references to the appropriate provider based on scheme.
//
// This interface is implemented by internal/secrets/registry.go and used
// by the binding resolver to resolve secret references at runtime.
type SecretProviderRegistry interface {
	// Register adds a provider to the registry.
	// Returns an error if a provider with the same scheme already exists.
	Register(provider SecretProvider) error

	// Resolve routes a secret reference to the appropriate provider and returns the value.
	// Reference format: "scheme:reference" or "${VAR}" for backward compatibility.
	//
	// Examples:
	//   - "env:GITHUB_TOKEN" -> env provider
	//   - "file:/etc/secrets/token" -> file provider
	//   - "${API_KEY}" -> env provider (legacy syntax)
	//   - "vault:secret/data/prod#token" -> vault provider (future)
	//
	// Returns sanitized errors that don't leak secret values or paths.
	Resolve(ctx context.Context, reference string) (string, error)

	// GetProvider returns the provider for the given scheme.
	// Returns nil if no provider is registered for the scheme.
	GetProvider(scheme string) SecretProvider
}
