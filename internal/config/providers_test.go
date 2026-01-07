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

package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	conductorerrors "github.com/tombee/conductor/pkg/errors"
)

func TestResolveSecretReference(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		envKey      string
		envValue    string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "plaintext value",
			value:   "sk-ant-1234567890",
			want:    "sk-ant-1234567890",
			wantErr: false,
		},
		{
			name:    "empty value",
			value:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:     "secret reference with env backend",
			value:    "$secret:providers/anthropic/api_key",
			envKey:   "CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY",
			envValue: "sk-ant-resolved",
			want:     "sk-ant-resolved",
			wantErr:  false,
		},
		{
			name:        "secret reference not found",
			value:       "$secret:providers/nonexistent/api_key",
			wantErr:     true,
			errContains: "secret not found",
		},
		{
			name:     "secret reference with slashes",
			value:    "$secret:webhooks/github/secret",
			envKey:   "CONDUCTOR_SECRET_WEBHOOKS_GITHUB_SECRET",
			envValue: "webhook-secret-123",
			want:     "webhook-secret-123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment if needed
			if tt.envKey != "" && tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			ctx := context.Background()
			got, err := ResolveSecretReference(ctx, tt.value)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveSecretReference() expected error, got nil")
					return
				}
				// Verify it's a ConfigError
				var configErr *conductorerrors.ConfigError
				if !errors.As(err, &configErr) {
					t.Errorf("ResolveSecretReference() error should be ConfigError, got %T", err)
					return
				}
				// Check that the cause contains the expected substring
				if tt.errContains != "" && configErr.Cause != nil && !contains(configErr.Cause.Error(), tt.errContains) {
					t.Errorf("ResolveSecretReference() error cause = %v, want error containing %q", configErr.Cause, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ResolveSecretReference() unexpected error = %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("ResolveSecretReference() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProviderConfig_ResolveSecrets(t *testing.T) {
	tests := []struct {
		name         string
		config       ProviderConfig
		envKey       string
		envValue     string
		wantAPIKey   string
		wantWarnings int
		wantErr      bool
	}{
		{
			name: "plaintext API key warns",
			config: ProviderConfig{
				Type:   "anthropic",
				APIKey: "sk-ant-1234567890",
			},
			wantAPIKey:   "sk-ant-1234567890",
			wantWarnings: 1,
			wantErr:      false,
		},
		{
			name: "secret reference resolves",
			config: ProviderConfig{
				Type:   "anthropic",
				APIKey: "$secret:providers/anthropic/api_key",
			},
			envKey:       "CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY",
			envValue:     "sk-ant-resolved",
			wantAPIKey:   "sk-ant-resolved",
			wantWarnings: 0,
			wantErr:      false,
		},
		{
			name: "empty API key no warning",
			config: ProviderConfig{
				Type:   "claude-code",
				APIKey: "",
			},
			wantAPIKey:   "",
			wantWarnings: 0,
			wantErr:      false,
		},
		{
			name: "OpenAI key warns",
			config: ProviderConfig{
				Type:   "openai",
				APIKey: "sk-1234567890",
			},
			wantAPIKey:   "sk-1234567890",
			wantWarnings: 1,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment if needed
			if tt.envKey != "" && tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			ctx := context.Background()
			warnings, err := tt.config.ResolveSecrets(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveSecrets() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ResolveSecrets() unexpected error = %v", err)
				return
			}

			if len(warnings) != tt.wantWarnings {
				t.Errorf("ResolveSecrets() got %d warnings, want %d. Warnings: %v", len(warnings), tt.wantWarnings, warnings)
			}

			if tt.config.APIKey != tt.wantAPIKey {
				t.Errorf("ResolveSecrets() APIKey = %q, want %q", tt.config.APIKey, tt.wantAPIKey)
			}
		})
	}
}

func TestResolveSecretsInProviders(t *testing.T) {
	ctx := context.Background()

	// Set up test environment
	os.Setenv("CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY", "sk-ant-resolved")
	defer os.Unsetenv("CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY")

	providers := ProvidersMap{
		"claude": ProviderConfig{
			Type:   "claude-code",
			APIKey: "",
		},
		"anthropic": ProviderConfig{
			Type:   "anthropic",
			APIKey: "$secret:providers/anthropic/api_key",
		},
		"openai": ProviderConfig{
			Type:   "openai",
			APIKey: "sk-plaintext",
		},
	}

	warnings, err := ResolveSecretsInProviders(ctx, providers)
	if err != nil {
		t.Fatalf("ResolveSecretsInProviders() unexpected error = %v", err)
	}

	// Should have one warning from the openai provider
	if len(warnings) != 1 {
		t.Errorf("ResolveSecretsInProviders() got %d warnings, want 1. Warnings: %v", len(warnings), warnings)
	}

	// Check that anthropic API key was resolved
	if providers["anthropic"].APIKey != "sk-ant-resolved" {
		t.Errorf("ResolveSecretsInProviders() anthropic APIKey = %q, want %q", providers["anthropic"].APIKey, "sk-ant-resolved")
	}

	// Check that openai API key remained as plaintext (resolution doesn't modify it)
	if providers["openai"].APIKey != "sk-plaintext" {
		t.Errorf("ResolveSecretsInProviders() openai APIKey = %q, want %q", providers["openai"].APIKey, "sk-plaintext")
	}
}

func TestLoadWithSecrets(t *testing.T) {
	// Create a temporary config file with a secret reference
	tmpfile, err := os.CreateTemp("", "conductor-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	configContent := `
providers:
  anthropic:
    type: anthropic
    api_key: $secret:providers/anthropic/api_key
`
	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Set up environment for secret resolution
	os.Setenv("CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY", "sk-ant-test-key")
	defer os.Unsetenv("CONDUCTOR_SECRET_PROVIDERS_ANTHROPIC_API_KEY")

	// Load config with secrets
	cfg, warnings, err := LoadWithSecrets(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadWithSecrets() unexpected error = %v", err)
	}

	if len(warnings) > 0 {
		t.Errorf("LoadWithSecrets() got warnings: %v", warnings)
	}

	// Verify the secret was resolved
	if cfg.Providers["anthropic"].APIKey != "sk-ant-test-key" {
		t.Errorf("LoadWithSecrets() APIKey = %q, want %q", cfg.Providers["anthropic"].APIKey, "sk-ant-test-key")
	}
}

func TestWriteConfigWithSecrets(t *testing.T) {
	// Skip test if keychain is not available (e.g., on Linux CI)
	if !isKeychainAvailable() {
		t.Skip("keychain not available on this system")
	}

	ctx := context.Background()

	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "conductor-config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create providers with plaintext API keys
	providers := ProvidersMap{
		"anthropic": ProviderConfig{
			Type:   "anthropic",
			APIKey: "sk-ant-test-key-123",
		},
		"openai": ProviderConfig{
			Type:   "openai",
			APIKey: "sk-test-key-456",
		},
	}

	// Write config with secrets
	storedKeys, err := WriteConfigWithSecrets(ctx, providers, configPath, "")
	if err != nil {
		t.Fatalf("WriteConfigWithSecrets() error = %v", err)
	}

	// Verify that keys were stored
	if len(storedKeys) != 2 {
		t.Errorf("WriteConfigWithSecrets() stored %d keys, want 2. Keys: %v", len(storedKeys), storedKeys)
	}

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file was not created at %s", configPath)
	}

	// Read and verify config file content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// Verify that plaintext keys are NOT in the config
	if strings.Contains(content, "sk-ant-test-key-123") {
		t.Errorf("Config contains plaintext Anthropic API key")
	}
	if strings.Contains(content, "sk-test-key-456") {
		t.Errorf("Config contains plaintext OpenAI API key")
	}

	// Verify that secret references ARE in the config
	if !strings.Contains(content, "$secret:providers/anthropic/api_key") {
		t.Errorf("Config does not contain Anthropic secret reference")
	}
	if !strings.Contains(content, "$secret:providers/openai/api_key") {
		t.Errorf("Config does not contain OpenAI secret reference")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// isKeychainAvailable checks if the system keychain is accessible.
// Returns false on systems without a keychain (e.g., headless Linux CI).
func isKeychainAvailable() bool {
	resolver := createSecretResolver()
	// Try to access the keychain - if it fails with anything other than
	// "not found", the keychain is unavailable
	_, err := resolver.Get(context.Background(), "__keychain_availability_test__")
	if err == nil {
		return true
	}
	// If the error is "not found", the keychain is available but empty
	return strings.Contains(err.Error(), "not found")
}
