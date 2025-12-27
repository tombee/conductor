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
	"testing"
)

// mockProvider is a test implementation of SecretProvider
type mockProvider struct {
	scheme string
	values map[string]string
}

func (m *mockProvider) Scheme() string {
	return m.scheme
}

func (m *mockProvider) Resolve(ctx context.Context, reference string) (string, error) {
	if value, ok := m.values[reference]; ok {
		return value, nil
	}
	return "", fmt.Errorf("not found: %s", reference)
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	provider1 := &mockProvider{scheme: "env", values: map[string]string{}}
	provider2 := &mockProvider{scheme: "file", values: map[string]string{}}
	providerDupe := &mockProvider{scheme: "env", values: map[string]string{}}

	// First registration should succeed
	if err := registry.Register(provider1); err != nil {
		t.Errorf("Register() error = %v, want nil", err)
	}

	// Second provider with different scheme should succeed
	if err := registry.Register(provider2); err != nil {
		t.Errorf("Register() error = %v, want nil", err)
	}

	// Duplicate scheme should fail
	if err := registry.Register(providerDupe); err == nil {
		t.Error("Register() with duplicate scheme should error")
	}
}

func TestRegistry_ParseReference(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name       string
		reference  string
		wantScheme string
		wantKey    string
		wantError  bool
	}{
		{
			name:       "env scheme",
			reference:  "env:GITHUB_TOKEN",
			wantScheme: "env",
			wantKey:    "GITHUB_TOKEN",
			wantError:  false,
		},
		{
			name:       "file scheme",
			reference:  "file:/etc/secrets/token",
			wantScheme: "file",
			wantKey:    "/etc/secrets/token",
			wantError:  false,
		},
		{
			name:       "legacy ${VAR} syntax",
			reference:  "${API_KEY}",
			wantScheme: "env",
			wantKey:    "API_KEY",
			wantError:  false,
		},
		{
			name:       "legacy ${VAR} with underscore",
			reference:  "${GITHUB_API_KEY}",
			wantScheme: "env",
			wantKey:    "GITHUB_API_KEY",
			wantError:  false,
		},
		{
			name:       "vault scheme (future)",
			reference:  "vault:secret/data/prod",
			wantScheme: "vault",
			wantKey:    "secret/data/prod",
			wantError:  false,
		},
		{
			name:       "empty reference",
			reference:  "",
			wantError:  true,
		},
		{
			name:       "plain value (no scheme)",
			reference:  "plaintext-value",
			wantScheme: "plain",
			wantKey:    "plaintext-value",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, key, err := registry.parseReference(tt.reference)
			if (err != nil) != tt.wantError {
				t.Errorf("parseReference() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				if scheme != tt.wantScheme {
					t.Errorf("parseReference() scheme = %v, want %v", scheme, tt.wantScheme)
				}
				if key != tt.wantKey {
					t.Errorf("parseReference() key = %v, want %v", key, tt.wantKey)
				}
			}
		})
	}
}

func TestRegistry_Resolve(t *testing.T) {
	registry := NewRegistry()

	// Register mock providers
	envProvider := &mockProvider{
		scheme: "env",
		values: map[string]string{
			"GITHUB_TOKEN": "ghp_test123",
			"API_KEY":      "sk-test456",
		},
	}
	fileProvider := &mockProvider{
		scheme: "file",
		values: map[string]string{
			"/etc/secrets/token": "file-secret-value",
		},
	}

	if err := registry.Register(envProvider); err != nil {
		t.Fatalf("Failed to register env provider: %v", err)
	}
	if err := registry.Register(fileProvider); err != nil {
		t.Fatalf("Failed to register file provider: %v", err)
	}

	tests := []struct {
		name      string
		reference string
		want      string
		wantError bool
	}{
		{
			name:      "resolve env with explicit scheme",
			reference: "env:GITHUB_TOKEN",
			want:      "ghp_test123",
			wantError: false,
		},
		{
			name:      "resolve env with legacy syntax",
			reference: "${API_KEY}",
			want:      "sk-test456",
			wantError: false,
		},
		{
			name:      "resolve file",
			reference: "file:/etc/secrets/token",
			want:      "file-secret-value",
			wantError: false,
		},
		{
			name:      "unknown provider scheme",
			reference: "vault:secret/path",
			wantError: true,
		},
		{
			name:      "provider not found error",
			reference: "env:NONEXISTENT",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.Resolve(context.Background(), tt.reference)
			if (err != nil) != tt.wantError {
				t.Errorf("Resolve() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && got != tt.want {
				t.Errorf("Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistry_GetProvider(t *testing.T) {
	registry := NewRegistry()

	provider := &mockProvider{scheme: "env", values: map[string]string{}}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Get existing provider
	got := registry.GetProvider("env")
	if got == nil {
		t.Error("GetProvider() returned nil for existing provider")
	}
	if got.Scheme() != "env" {
		t.Errorf("GetProvider() scheme = %v, want env", got.Scheme())
	}

	// Get non-existent provider
	got = registry.GetProvider("vault")
	if got != nil {
		t.Error("GetProvider() should return nil for non-existent provider")
	}
}
