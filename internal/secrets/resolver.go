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
	"fmt"
	"sort"
)

// Resolver manages a chain of SecretBackends and resolves secrets
// by querying backends in priority order.
type Resolver struct {
	backends []SecretBackend
}

// NewResolver creates a new secret resolver with the given backends.
// Backends are automatically sorted by priority (highest first).
func NewResolver(backends ...SecretBackend) *Resolver {
	// Filter out unavailable backends
	available := make([]SecretBackend, 0, len(backends))
	for _, b := range backends {
		if b.Available() {
			available = append(available, b)
		}
	}

	// Sort by priority (descending)
	sort.Slice(available, func(i, j int) bool {
		return available[i].Priority() > available[j].Priority()
	})

	return &Resolver{
		backends: available,
	}
}

// Get retrieves a secret by querying backends in priority order.
// Returns the first successful result or ErrSecretNotFound if all backends fail.
func (r *Resolver) Get(ctx context.Context, key string) (string, error) {
	if len(r.backends) == 0 {
		return "", fmt.Errorf("%w: no available backends", ErrBackendUnavailable)
	}

	var lastErr error
	for _, backend := range r.backends {
		value, err := backend.Get(ctx, key)
		if err == nil {
			return value, nil
		}

		// Track the last error for debugging
		if !errors.Is(err, ErrSecretNotFound) {
			lastErr = err
		}
	}

	// If we have a non-NotFound error, return it with context
	if lastErr != nil {
		return "", fmt.Errorf("failed to get secret %q: %w", key, lastErr)
	}

	return "", fmt.Errorf("%w: %q", ErrSecretNotFound, key)
}

// Set stores a secret in the first available writable backend.
// If a specific backend is specified, only that backend is used.
func (r *Resolver) Set(ctx context.Context, key string, value string, backendName string) error {
	if len(r.backends) == 0 {
		return fmt.Errorf("%w: no available backends", ErrBackendUnavailable)
	}

	// If a specific backend is requested, use only that one
	if backendName != "" {
		for _, backend := range r.backends {
			if backend.Name() == backendName {
				if err := backend.Set(ctx, key, value); err != nil {
					return fmt.Errorf("failed to set secret in %s: %w", backendName, err)
				}
				return nil
			}
		}
		return fmt.Errorf("backend %q not found or unavailable", backendName)
	}

	// Try backends in priority order, skipping read-only ones
	for _, backend := range r.backends {
		// Skip read-only backends
		if ro, ok := backend.(ReadOnlyBackend); ok && ro.ReadOnly() {
			continue
		}

		if err := backend.Set(ctx, key, value); err != nil {
			if errors.Is(err, ErrReadOnlyBackend) {
				continue
			}
			return fmt.Errorf("failed to set secret in %s: %w", backend.Name(), err)
		}
		return nil
	}

	return fmt.Errorf("no writable backend available")
}

// Delete removes a secret from a specific backend or all writable backends.
func (r *Resolver) Delete(ctx context.Context, key string, backendName string) error {
	if len(r.backends) == 0 {
		return fmt.Errorf("%w: no available backends", ErrBackendUnavailable)
	}

	// If a specific backend is requested, use only that one
	if backendName != "" {
		for _, backend := range r.backends {
			if backend.Name() == backendName {
				if err := backend.Delete(ctx, key); err != nil {
					return fmt.Errorf("failed to delete secret from %s: %w", backendName, err)
				}
				return nil
			}
		}
		return fmt.Errorf("backend %q not found or unavailable", backendName)
	}

	// Delete from all writable backends that have the key
	deleted := false
	for _, backend := range r.backends {
		// Skip read-only backends
		if ro, ok := backend.(ReadOnlyBackend); ok && ro.ReadOnly() {
			continue
		}

		if err := backend.Delete(ctx, key); err != nil {
			if errors.Is(err, ErrSecretNotFound) || errors.Is(err, ErrReadOnlyBackend) {
				continue
			}
			return fmt.Errorf("failed to delete secret from %s: %w", backend.Name(), err)
		}
		deleted = true
	}

	if !deleted {
		return fmt.Errorf("%w: %q", ErrSecretNotFound, key)
	}

	return nil
}

// List returns all secret keys from all backends, along with metadata.
func (r *Resolver) List(ctx context.Context) ([]SecretMetadata, error) {
	if len(r.backends) == 0 {
		return nil, fmt.Errorf("%w: no available backends", ErrBackendUnavailable)
	}

	// Collect keys from all backends
	keyMap := make(map[string]SecretMetadata)

	for _, backend := range r.backends {
		keys, err := backend.List(ctx)
		if err != nil {
			// Log error but continue with other backends
			continue
		}

		for _, key := range keys {
			// Only add if not already seen (higher priority backend wins)
			if _, exists := keyMap[key]; !exists {
				readOnly := false
				if ro, ok := backend.(ReadOnlyBackend); ok {
					readOnly = ro.ReadOnly()
				}

				keyMap[key] = SecretMetadata{
					Key:      key,
					Backend:  backend.Name(),
					ReadOnly: readOnly,
				}
			}
		}
	}

	// Convert map to slice
	result := make([]SecretMetadata, 0, len(keyMap))
	for _, meta := range keyMap {
		result = append(result, meta)
	}

	return result, nil
}

// Backends returns the list of available backends in priority order.
func (r *Resolver) Backends() []SecretBackend {
	return r.backends
}
