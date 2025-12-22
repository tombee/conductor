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

/*
Package secrets provides secure credential storage and retrieval.

This package implements a multi-backend secret management system with support
for environment variables, OS keychains, and file-based storage. Secrets are
resolved through a priority-ordered chain of backends.

# Overview

Key features:

  - Multiple storage backends (env, keychain, file)
  - Priority-ordered resolution
  - Secure storage using OS keychain
  - Configuration file integration

# Backends

The package provides several secret backends:

	env      - Environment variables (CONDUCTOR_SECRET_*)
	keychain - OS keychain (macOS Keychain, Linux Secret Service)
	file     - Encrypted file storage (for development)

Each backend implements the SecretBackend interface:

	type SecretBackend interface {
	    Name() string
	    Priority() int
	    Available() bool
	    Get(ctx context.Context, key string) (string, error)
	    Set(ctx context.Context, key, value string) error
	    Delete(ctx context.Context, key string) error
	    List(ctx context.Context) ([]string, error)
	}

# Usage

Create a resolver with multiple backends:

	resolver := secrets.NewResolver(
	    secrets.NewKeychainBackend(),
	    secrets.NewEnvBackend(),
	    secrets.NewFileBackend("/path/to/secrets"),
	)

Retrieve a secret:

	apiKey, err := resolver.Get(ctx, "anthropic-api-key")

Store a secret:

	err := resolver.Set(ctx, "my-secret", "secret-value")

# Priority Order

Backends are queried in priority order (highest first):

 1. Keychain (priority 100) - Most secure, preferred
 2. File (priority 50) - Encrypted file storage
 3. Environment (priority 10) - Fallback for CI/containers

# Configuration Integration

Secrets can be referenced in configuration files:

	providers:
	  anthropic:
	    api_key: $secret:anthropic-api-key

The config loader resolves these references at load time.

# Environment Variables

The env backend looks for variables prefixed with CONDUCTOR_SECRET_:

	export CONDUCTOR_SECRET_ANTHROPIC_API_KEY=sk-ant-...

Key names are normalized:

  - anthropic-api-key → CONDUCTOR_SECRET_ANTHROPIC_API_KEY
  - my_secret → CONDUCTOR_SECRET_MY_SECRET

# Keychain Integration

On macOS, secrets are stored in the system Keychain.
On Linux, the Secret Service API (GNOME Keyring, KWallet) is used.

The keychain backend requires no configuration and provides:

  - Encryption at rest
  - User-level access control
  - Integration with system credential management

# Error Handling

Common errors:

  - ErrSecretNotFound: Secret doesn't exist in any backend
  - ErrBackendUnavailable: No backends are available
*/
package secrets
