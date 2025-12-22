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
	"errors"
	"time"
)

var (
	// ErrSecretNotFound is returned when a secret key does not exist in the backend.
	ErrSecretNotFound = errors.New("secret not found")

	// ErrBackendUnavailable is returned when a backend cannot be used in the current environment.
	ErrBackendUnavailable = errors.New("backend unavailable")

	// ErrReadOnlyBackend is returned when attempting to modify a read-only backend.
	ErrReadOnlyBackend = errors.New("backend is read-only")
)

// SecretBackend provides secure storage for sensitive values.
// Backends implement different storage mechanisms (keychain, environment, files, etc.)
// and are queried in priority order by the Resolver.
type SecretBackend interface {
	// Name returns the backend identifier (e.g., "keychain", "vault", "env").
	Name() string

	// Get retrieves a secret by key. Returns ErrSecretNotFound if not present.
	Get(ctx context.Context, key string) (string, error)

	// Set stores a secret. Returns ErrReadOnlyBackend if not supported.
	Set(ctx context.Context, key string, value string) error

	// Delete removes a secret. Returns ErrSecretNotFound if not present.
	// Returns ErrReadOnlyBackend if not supported.
	Delete(ctx context.Context, key string) error

	// List returns all secret keys (not values) managed by this backend.
	List(ctx context.Context) ([]string, error)

	// Available returns true if this backend is usable in the current environment.
	// For example, keychain returns false if the keyring service is unavailable.
	Available() bool

	// Priority returns the resolution priority (higher = checked first).
	// Standard priorities: env (100), vault (75), keychain (50), file (25).
	Priority() int
}

// ReadOnlyBackend is a marker interface for backends that don't support writes.
// Backends implementing this interface should return ErrReadOnlyBackend from
// Set and Delete operations.
type ReadOnlyBackend interface {
	SecretBackend
	ReadOnly() bool
}

// SecretChange represents a change notification for watchable backends.
type SecretChange struct {
	Key       string
	Timestamp time.Time
	Deleted   bool
}

// WatchableBackend supports watching for secret changes.
// This is used for cache invalidation when secrets are rotated.
type WatchableBackend interface {
	SecretBackend
	// Watch returns a channel that receives notifications when the specified key changes.
	// The channel is closed when the context is canceled or the watch fails.
	Watch(ctx context.Context, key string) (<-chan SecretChange, error)
}

// SecretMetadata provides additional information about a secret.
type SecretMetadata struct {
	Key          string
	Backend      string
	LastModified *time.Time
	ReadOnly     bool
}
