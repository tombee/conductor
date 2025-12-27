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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	// Server defaults
	if cfg.Server.Port != 9876 {
		t.Errorf("expected port 9876, got %d", cfg.Server.Port)
	}
	if cfg.Server.ShutdownTimeout != 5*time.Second {
		t.Errorf("expected shutdown timeout 5s, got %v", cfg.Server.ShutdownTimeout)
	}

	// Auth defaults
	if cfg.Auth.TokenLength != 32 {
		t.Errorf("expected token length 32, got %d", cfg.Auth.TokenLength)
	}

	// Log defaults
	if cfg.Log.Level != "info" {
		t.Errorf("expected log level 'info', got %q", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("expected log format 'json', got %q", cfg.Log.Format)
	}
	if cfg.Log.AddSource {
		t.Errorf("expected log add_source false, got true")
	}

	// LLM defaults
	if cfg.LLM.DefaultProvider != "anthropic" {
		t.Errorf("expected default provider 'anthropic', got %q", cfg.LLM.DefaultProvider)
	}
	if cfg.LLM.RequestTimeout != 5*time.Second {
		t.Errorf("expected request timeout 5s, got %v", cfg.LLM.RequestTimeout)
	}
	if cfg.LLM.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", cfg.LLM.MaxRetries)
	}
	if cfg.LLM.RetryBackoffBase != 100*time.Millisecond {
		t.Errorf("expected retry backoff base 100ms, got %v", cfg.LLM.RetryBackoffBase)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
		errText string
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "invalid port too low",
			modify: func(c *Config) {
				c.Server.Port = 1023
			},
			wantErr: true,
			errText: "server.port must be between 1024 and 65535",
		},
		{
			name: "invalid port too high",
			modify: func(c *Config) {
				c.Server.Port = 65536
			},
			wantErr: true,
			errText: "server.port must be between 1024 and 65535",
		},
		{
			name: "invalid shutdown timeout",
			modify: func(c *Config) {
				c.Server.ShutdownTimeout = 0
			},
			wantErr: true,
			errText: "shutdown_timeout must be positive",
		},
		{
			name: "invalid token length",
			modify: func(c *Config) {
				c.Auth.TokenLength = 15
			},
			wantErr: true,
			errText: "token_length must be at least 16 bytes",
		},
		{
			name: "invalid log level",
			modify: func(c *Config) {
				c.Log.Level = "invalid"
			},
			wantErr: true,
			errText: "log.level must be one of [debug, info, warn, warning, error]",
		},
		{
			name: "invalid log format",
			modify: func(c *Config) {
				c.Log.Format = "invalid"
			},
			wantErr: true,
			errText: "log.format must be one of [json, text]",
		},
		{
			name: "invalid llm provider",
			modify: func(c *Config) {
				// Add a provider so validation runs against configured providers
				c.Providers = ProvidersMap{
					"my-provider": ProviderConfig{Type: "claude-code"},
				}
				c.LLM.DefaultProvider = "nonexistent-provider"
			},
			wantErr: true,
			errText: "llm.default_provider \"nonexistent-provider\" not found in configured providers",
		},
		{
			name: "invalid llm request timeout",
			modify: func(c *Config) {
				c.LLM.RequestTimeout = 0
			},
			wantErr: true,
			errText: "llm.request_timeout must be positive",
		},
		{
			name: "invalid llm max retries",
			modify: func(c *Config) {
				c.LLM.MaxRetries = -1
			},
			wantErr: true,
			errText: "llm.max_retries must be non-negative",
		},
		{
			name: "invalid llm retry backoff base",
			modify: func(c *Config) {
				c.LLM.RetryBackoffBase = -1
			},
			wantErr: true,
			errText: "llm.retry_backoff_base must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.modify(cfg)
			err := cfg.Validate()

			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errText) {
				t.Errorf("expected error to contain %q, got %q", tt.errText, err.Error())
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save and restore environment
	oldEnv := saveEnv()
	defer restoreEnv(oldEnv)

	// Clear all config-related env vars
	clearConfigEnv()

	// Set test environment variables
	envVars := map[string]string{
		"SERVER_SHUTDOWN_TIMEOUT": "10s",
		"AUTH_TOKEN_LENGTH":       "64",
		"LOG_LEVEL":               "debug",
		"LOG_FORMAT":              "text",
		"LOG_SOURCE":              "1",
		"LLM_DEFAULT_PROVIDER":    "openai",
		"LLM_REQUEST_TIMEOUT":     "10s",
		"LLM_MAX_RETRIES":         "5",
		"LLM_RETRY_BACKOFF_BASE":  "200ms",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify server config
	// Port should use default (no env var for port)
	if cfg.Server.Port != 9876 {
		t.Errorf("expected default port 9876, got %d", cfg.Server.Port)
	}
	if cfg.Server.ShutdownTimeout != 10*time.Second {
		t.Errorf("expected shutdown timeout 10s, got %v", cfg.Server.ShutdownTimeout)
	}

	// Verify auth config
	if cfg.Auth.TokenLength != 64 {
		t.Errorf("expected token length 64, got %d", cfg.Auth.TokenLength)
	}

	// Verify log config
	if cfg.Log.Level != "debug" {
		t.Errorf("expected log level 'debug', got %q", cfg.Log.Level)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("expected log format 'text', got %q", cfg.Log.Format)
	}
	if !cfg.Log.AddSource {
		t.Errorf("expected log add_source true, got false")
	}

	// Verify LLM config
	if cfg.LLM.DefaultProvider != "openai" {
		t.Errorf("expected default provider 'openai', got %q", cfg.LLM.DefaultProvider)
	}
	if cfg.LLM.RequestTimeout != 10*time.Second {
		t.Errorf("expected request timeout 10s, got %v", cfg.LLM.RequestTimeout)
	}
	if cfg.LLM.MaxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", cfg.LLM.MaxRetries)
	}
	if cfg.LLM.RetryBackoffBase != 200*time.Millisecond {
		t.Errorf("expected retry backoff base 200ms, got %v", cfg.LLM.RetryBackoffBase)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
server:
  port: 8080
  shutdown_timeout: 15s

auth:
  token_length: 48

log:
  level: warn
  format: text
  add_source: true

llm:
  default_provider: ollama
  request_timeout: 8s
  max_retries: 4
  retry_backoff_base: 150ms
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Save and restore environment
	oldEnv := saveEnv()
	defer restoreEnv(oldEnv)
	clearConfigEnv()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify loaded values
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Auth.TokenLength != 48 {
		t.Errorf("expected token length 48, got %d", cfg.Auth.TokenLength)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("expected log level 'warn', got %q", cfg.Log.Level)
	}
	if cfg.LLM.DefaultProvider != "ollama" {
		t.Errorf("expected default provider 'ollama', got %q", cfg.LLM.DefaultProvider)
	}
}

func TestLoadFromFileWithEnvOverride(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
server:
  port: 8080
log:
  level: info
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Save and restore environment
	oldEnv := saveEnv()
	defer restoreEnv(oldEnv)
	clearConfigEnv()

	// Set env var to override file value
	os.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify env overrides file
	if cfg.Log.Level != "debug" {
		t.Errorf("expected log level 'debug' from env, got %q", cfg.Log.Level)
	}
	// Port should use file value (no env var override for port)
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080 from file, got %d", cfg.Server.Port)
	}
}

func TestLoadInvalidFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Errorf("expected error for nonexistent file, got nil")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.yaml")

	if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Errorf("expected error for invalid YAML, got nil")
	}
}

func TestLoadValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid-config.yaml")

	// Config with invalid values
	yamlContent := `
server:
  port: 100  # Too low
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Save and restore environment
	oldEnv := saveEnv()
	defer restoreEnv(oldEnv)
	clearConfigEnv()

	_, err := Load(configPath)
	if err == nil {
		t.Errorf("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected validation error message, got %q", err.Error())
	}
}

// Helper functions for environment management
func saveEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

func restoreEnv(env map[string]string) {
	os.Clearenv()
	for k, v := range env {
		os.Setenv(k, v)
	}
}

func clearConfigEnv() {
	envVars := []string{
		"SERVER_SHUTDOWN_TIMEOUT",
		"AUTH_TOKEN_LENGTH",
		"LOG_LEVEL", "LOG_FORMAT", "LOG_SOURCE",
		"LLM_DEFAULT_PROVIDER", "LLM_REQUEST_TIMEOUT", "LLM_MAX_RETRIES",
		"LLM_RETRY_BACKOFF_BASE",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}
}

// TestMinimalConfigRoundTrip verifies that a minimal config (SPEC-50) with only
// provider settings can be written and loaded back with sensible defaults.
func TestMinimalConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Save and restore environment
	oldEnv := saveEnv()
	defer restoreEnv(oldEnv)
	clearConfigEnv()

	// Create minimal config like conductor init does
	providers := ProvidersMap{
		"claude": ProviderConfig{
			Type: "claude-code",
		},
	}

	if err := WriteConfigMinimal("claude", providers, configPath); err != nil {
		t.Fatalf("failed to write minimal config: %v", err)
	}

	// Load the config back - this should work without validation errors
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load minimal config: %v", err)
	}

	// Verify defaults were applied
	if cfg.Server.Port != 9876 {
		t.Errorf("expected port 9876, got %d", cfg.Server.Port)
	}
	if cfg.Auth.TokenLength != 32 {
		t.Errorf("expected token length 32, got %d", cfg.Auth.TokenLength)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("expected log level 'info', got %q", cfg.Log.Level)
	}
	if cfg.LLM.RequestTimeout != 5*time.Second {
		t.Errorf("expected request timeout 5s, got %v", cfg.LLM.RequestTimeout)
	}

	// Verify provider settings were preserved
	if cfg.DefaultProvider != "claude" {
		t.Errorf("expected default provider 'claude', got %q", cfg.DefaultProvider)
	}
	if len(cfg.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(cfg.Providers))
	}
	if cfg.Providers["claude"].Type != "claude-code" {
		t.Errorf("expected provider type 'claude-code', got %q", cfg.Providers["claude"].Type)
	}
}
