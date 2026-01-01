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
	"os"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/profile"
)

// mockCacheProvider is a test provider that tracks resolution calls.
type mockCacheProvider struct {
	scheme    string
	values    map[string]string
	callCount int
}

func (m *mockCacheProvider) Scheme() string {
	return m.scheme
}

func (m *mockCacheProvider) Resolve(ctx context.Context, reference string) (string, error) {
	m.callCount++
	if value, ok := m.values[reference]; ok {
		return value, nil
	}
	return "", profile.NewSecretResolutionError(
		profile.ErrorCategoryNotFound,
		m.scheme+":"+reference,
		m.scheme,
		"not found in mock provider",
		nil,
	)
}

func TestCache_Resolve(t *testing.T) {
	// Create mock provider
	mock := &mockCacheProvider{
		scheme: "env",
		values: map[string]string{
			"GITHUB_TOKEN": "ghp_test123",
			"API_KEY":      "key_test456",
		},
	}

	// Create registry and register mock provider
	registry := NewRegistry()
	if err := registry.Register(mock); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create cache
	cache := NewCache(registry)
	ctx := context.Background()

	// First resolution should call provider
	value1, err := cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value1 != "ghp_test123" {
		t.Errorf("Resolve() = %q, want %q", value1, "ghp_test123")
	}
	if mock.callCount != 1 {
		t.Errorf("provider callCount = %d, want 1", mock.callCount)
	}

	// Second resolution should use cache (no provider call)
	value2, err := cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value2 != "ghp_test123" {
		t.Errorf("Resolve() = %q, want %q", value2, "ghp_test123")
	}
	if mock.callCount != 1 {
		t.Errorf("provider callCount = %d, want 1 (should be cached)", mock.callCount)
	}
}

func TestCache_RunIsolation(t *testing.T) {
	// Create mock provider
	mock := &mockCacheProvider{
		scheme: "env",
		values: map[string]string{
			"GITHUB_TOKEN": "ghp_test123",
		},
	}

	// Create registry and register mock provider
	registry := NewRegistry()
	if err := registry.Register(mock); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create cache
	cache := NewCache(registry)
	ctx := context.Background()

	// Resolve for run-1
	value1, err := cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value1 != "ghp_test123" {
		t.Errorf("Resolve() = %q, want %q", value1, "ghp_test123")
	}

	// Resolve for run-2 (different run, should call provider again)
	value2, err := cache.Resolve(ctx, "run-2", "env:GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value2 != "ghp_test123" {
		t.Errorf("Resolve() = %q, want %q", value2, "ghp_test123")
	}

	// Provider should be called twice (once per run)
	if mock.callCount != 2 {
		t.Errorf("provider callCount = %d, want 2", mock.callCount)
	}

	// Resolve again for run-1 (should use cache)
	value3, err := cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value3 != "ghp_test123" {
		t.Errorf("Resolve() = %q, want %q", value3, "ghp_test123")
	}

	// Provider should still be called only twice
	if mock.callCount != 2 {
		t.Errorf("provider callCount = %d, want 2 (run-1 should be cached)", mock.callCount)
	}
}

func TestCache_Clear(t *testing.T) {
	// Create mock provider
	mock := &mockCacheProvider{
		scheme: "env",
		values: map[string]string{
			"GITHUB_TOKEN": "ghp_test123",
		},
	}

	// Create registry and register mock provider
	registry := NewRegistry()
	if err := registry.Register(mock); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create cache
	cache := NewCache(registry)
	ctx := context.Background()

	// Resolve secret
	_, err := cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Clear cache for run-1
	cache.Clear("run-1")

	// Resolve again (should call provider again)
	_, err = cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Provider should be called twice
	if mock.callCount != 2 {
		t.Errorf("provider callCount = %d, want 2 (cache was cleared)", mock.callCount)
	}
}

func TestCache_ClearAll(t *testing.T) {
	// Create mock provider
	mock := &mockCacheProvider{
		scheme: "env",
		values: map[string]string{
			"GITHUB_TOKEN": "ghp_test123",
		},
	}

	// Create registry and register mock provider
	registry := NewRegistry()
	if err := registry.Register(mock); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create cache
	cache := NewCache(registry)
	ctx := context.Background()

	// Resolve for multiple runs
	_, _ = cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	_, _ = cache.Resolve(ctx, "run-2", "env:GITHUB_TOKEN")

	// Clear all caches
	cache.ClearAll()

	// Resolve again for both runs
	_, _ = cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	_, _ = cache.Resolve(ctx, "run-2", "env:GITHUB_TOKEN")

	// Provider should be called 4 times (2 initial + 2 after clear)
	if mock.callCount != 4 {
		t.Errorf("provider callCount = %d, want 4", mock.callCount)
	}
}

func TestCache_GetStats(t *testing.T) {
	// Create mock provider
	mock := &mockCacheProvider{
		scheme: "env",
		values: map[string]string{
			"GITHUB_TOKEN": "ghp_test123",
			"API_KEY":      "key_test456",
		},
	}

	// Create registry and register mock provider
	registry := NewRegistry()
	if err := registry.Register(mock); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create cache
	cache := NewCache(registry)
	ctx := context.Background()

	// Initially empty
	stats := cache.GetStats()
	if stats.RunCount != 0 {
		t.Errorf("RunCount = %d, want 0", stats.RunCount)
	}
	if stats.SecretCount != 0 {
		t.Errorf("SecretCount = %d, want 0", stats.SecretCount)
	}

	// Resolve some secrets
	_, _ = cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	_, _ = cache.Resolve(ctx, "run-1", "env:API_KEY")
	_, _ = cache.Resolve(ctx, "run-2", "env:GITHUB_TOKEN")

	// Check stats
	stats = cache.GetStats()
	if stats.RunCount != 2 {
		t.Errorf("RunCount = %d, want 2", stats.RunCount)
	}
	if stats.SecretCount != 3 {
		t.Errorf("SecretCount = %d, want 3", stats.SecretCount)
	}
	if stats.RunStats["run-1"] != 2 {
		t.Errorf("RunStats[run-1] = %d, want 2", stats.RunStats["run-1"])
	}
	if stats.RunStats["run-2"] != 1 {
		t.Errorf("RunStats[run-2] = %d, want 1", stats.RunStats["run-2"])
	}
	if stats.OldestSecret.IsZero() {
		t.Error("OldestSecret should not be zero")
	}
	if stats.NewestSecret.IsZero() {
		t.Error("NewestSecret should not be zero")
	}
}

func TestCache_MultipleReferences(t *testing.T) {
	// Create mock provider
	mock := &mockCacheProvider{
		scheme: "env",
		values: map[string]string{
			"GITHUB_TOKEN": "ghp_test123",
			"API_KEY":      "key_test456",
			"SECRET":       "sec_test789",
		},
	}

	// Create registry and register mock provider
	registry := NewRegistry()
	if err := registry.Register(mock); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create cache
	cache := NewCache(registry)
	ctx := context.Background()

	// Resolve multiple different secrets for same run
	_, _ = cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	_, _ = cache.Resolve(ctx, "run-1", "env:API_KEY")
	_, _ = cache.Resolve(ctx, "run-1", "env:SECRET")

	// All should be cached (3 provider calls)
	if mock.callCount != 3 {
		t.Errorf("provider callCount = %d, want 3", mock.callCount)
	}

	// Resolve again (should use cache)
	_, _ = cache.Resolve(ctx, "run-1", "env:GITHUB_TOKEN")
	_, _ = cache.Resolve(ctx, "run-1", "env:API_KEY")
	_, _ = cache.Resolve(ctx, "run-1", "env:SECRET")

	// Still 3 calls (all cached)
	if mock.callCount != 3 {
		t.Errorf("provider callCount = %d, want 3 (all cached)", mock.callCount)
	}
}

func TestCache_ProviderError(t *testing.T) {
	// Create mock provider with no values (all lookups will fail)
	mock := &mockCacheProvider{
		scheme: "env",
		values: map[string]string{},
	}

	// Create registry and register mock provider
	registry := NewRegistry()
	if err := registry.Register(mock); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create cache
	cache := NewCache(registry)
	ctx := context.Background()

	// Resolve non-existent secret (should fail)
	_, err := cache.Resolve(ctx, "run-1", "env:NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for non-existent secret")
	}

	// Error should not be cached (provider should be called again)
	_, err = cache.Resolve(ctx, "run-1", "env:NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for non-existent secret")
	}

	// Provider should be called twice (errors not cached)
	if mock.callCount != 2 {
		t.Errorf("provider callCount = %d, want 2 (errors not cached)", mock.callCount)
	}
}

func TestCache_WithRealEnvProvider(t *testing.T) {
	// Set test environment variable
	testKey := "CONDUCTOR_TEST_CACHE_" + time.Now().Format("20060102150405")
	testValue := "test-value-123"
	os.Setenv(testKey, testValue)
	defer os.Unsetenv(testKey)

	// Create real env provider
	envProvider := NewEnvProvider(profile.InheritEnvConfig{
		Enabled: true,
	})

	// Create registry
	registry := NewRegistry()
	if err := registry.Register(envProvider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Create cache
	cache := NewCache(registry)
	ctx := context.Background()

	// Resolve secret
	value, err := cache.Resolve(ctx, "run-1", "env:"+testKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != testValue {
		t.Errorf("Resolve() = %q, want %q", value, testValue)
	}

	// Resolve again (should use cache)
	value2, err := cache.Resolve(ctx, "run-1", "env:"+testKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value2 != testValue {
		t.Errorf("Resolve() = %q, want %q", value2, testValue)
	}
}

func TestParseReferenceScheme(t *testing.T) {
	tests := []struct {
		name       string
		reference  string
		wantScheme string
		wantKey    string
	}{
		{
			name:       "env scheme",
			reference:  "env:GITHUB_TOKEN",
			wantScheme: "env",
			wantKey:    "GITHUB_TOKEN",
		},
		{
			name:       "env: syntax",
			reference:  "env:API_KEY",
			wantScheme: "env",
			wantKey:    "API_KEY",
		},
		{
			name:       "file: syntax",
			reference:  "file:/path/to/secret",
			wantScheme: "file",
			wantKey:    "/path/to/secret",
		},
		{
			name:       "plain value",
			reference:  "plain-text",
			wantScheme: "plain",
			wantKey:    "plain-text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, key, err := parseReferenceScheme(tt.reference)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if scheme != tt.wantScheme {
				t.Errorf("scheme = %q, want %q", scheme, tt.wantScheme)
			}
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}
