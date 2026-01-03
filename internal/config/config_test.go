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
)

func TestDefault(t *testing.T) {
	cfg := Default()

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

	// Controller auth defaults - secure by default
	if !cfg.Controller.ControllerAuth.Enabled {
		t.Errorf("expected controller auth enabled by default, got disabled")
	}
	if !cfg.Controller.ControllerAuth.AllowUnixSocket {
		t.Errorf("expected controller auth allow_unix_socket true by default, got false")
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
			name: "invalid trace_days when observability enabled",
			modify: func(c *Config) {
				c.Controller.Observability.Enabled = true
				c.Controller.Observability.Storage.Retention.TraceDays = 0
			},
			wantErr: true,
			errText: "trace_days must be positive",
		},
		{
			name: "invalid event_days when observability enabled",
			modify: func(c *Config) {
				c.Controller.Observability.Enabled = true
				c.Controller.Observability.Storage.Retention.EventDays = -1
			},
			wantErr: true,
			errText: "event_days must be positive",
		},
		{
			name: "invalid aggregate_days when observability enabled",
			modify: func(c *Config) {
				c.Controller.Observability.Enabled = true
				c.Controller.Observability.Storage.Retention.AggregateDays = 0
			},
			wantErr: true,
			errText: "aggregate_days must be positive",
		},
		{
			name: "zero retention days allowed when observability disabled",
			modify: func(c *Config) {
				c.Controller.Observability.Enabled = false
				c.Controller.Observability.Storage.Retention.TraceDays = 0
			},
			wantErr: false,
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
		"LOG_LEVEL":  "debug",
		"LOG_FORMAT": "text",
		"LOG_SOURCE": "1",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
log:
  level: warn
  format: text
  add_source: true

providers:
  claude:
    type: claude-code
    models:
      haiku: {}
      sonnet: {}
      opus: {}

tiers:
  fast: claude/haiku
  balanced: claude/sonnet
  strategic: claude/opus
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
	if cfg.Log.Level != "warn" {
		t.Errorf("expected log level 'warn', got %q", cfg.Log.Level)
	}

	// Verify provider and tiers
	if len(cfg.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(cfg.Providers))
	}
	if cfg.Providers["claude"].Type != "claude-code" {
		t.Errorf("expected provider type 'claude-code', got %q", cfg.Providers["claude"].Type)
	}
	if cfg.Tiers["balanced"] != "claude/sonnet" {
		t.Errorf("expected balanced tier 'claude/sonnet', got %q", cfg.Tiers["balanced"])
	}
}

func TestLoadFromFileWithEnvOverride(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
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
log:
  level: invalid_level
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
		"LOG_LEVEL", "LOG_FORMAT", "LOG_SOURCE",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}
}

// TestMinimalConfigRoundTrip verifies that a minimal config with only
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

	if err := WriteConfigMinimal(providers, configPath); err != nil {
		t.Fatalf("failed to write minimal config: %v", err)
	}

	// Load the config back - this should work without validation errors
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load minimal config: %v", err)
	}

	// Verify defaults were applied
	if cfg.Log.Level != "info" {
		t.Errorf("expected log level 'info', got %q", cfg.Log.Level)
	}

	// Verify provider settings were preserved
	if len(cfg.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(cfg.Providers))
	}
	if cfg.Providers["claude"].Type != "claude-code" {
		t.Errorf("expected provider type 'claude-code', got %q", cfg.Providers["claude"].Type)
	}
}

// TestGetPrimaryProvider tests the GetPrimaryProvider method.
func TestGetPrimaryProvider(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name:     "empty config returns empty string",
			config:   &Config{},
			expected: "",
		},
		{
			name: "single provider no tiers returns provider",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{Type: "anthropic"},
				},
			},
			expected: "anthropic",
		},
		{
			name: "multiple providers no tiers returns alphabetically first",
			config: &Config{
				Providers: ProvidersMap{
					"zebra":    ProviderConfig{Type: "openai"},
					"anthropic": ProviderConfig{Type: "anthropic"},
					"claude":   ProviderConfig{Type: "claude-code"},
				},
			},
			expected: "anthropic",
		},
		{
			name: "balanced tier takes precedence",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{Type: "anthropic"},
					"openai":    ProviderConfig{Type: "openai"},
				},
				Tiers: map[string]string{
					"balanced": "openai/gpt-4",
				},
			},
			expected: "openai",
		},
		{
			name: "fast tier used if no balanced",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{Type: "anthropic"},
					"openai":    ProviderConfig{Type: "openai"},
				},
				Tiers: map[string]string{
					"fast": "openai/gpt-3.5",
				},
			},
			expected: "openai",
		},
		{
			name: "strategic tier used if no balanced or fast",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{Type: "anthropic"},
					"openai":    ProviderConfig{Type: "openai"},
				},
				Tiers: map[string]string{
					"strategic": "anthropic/claude-opus",
				},
			},
			expected: "anthropic",
		},
		{
			name: "balanced takes precedence over fast and strategic",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{Type: "anthropic"},
					"openai":    ProviderConfig{Type: "openai"},
					"google":    ProviderConfig{Type: "google"},
				},
				Tiers: map[string]string{
					"fast":      "openai/gpt-3.5",
					"balanced":  "google/gemini",
					"strategic": "anthropic/claude-opus",
				},
			},
			expected: "google",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetPrimaryProvider()
			if result != tt.expected {
				t.Errorf("GetPrimaryProvider() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestPublicAPIConfig tests the public API configuration.
func TestPublicAPIConfig(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
		errText string
	}{
		{
			name: "public API disabled by default",
			modify: func(c *Config) {
				// Default config should have public API disabled
			},
			wantErr: false,
		},
		{
			name: "public API enabled with TCP address",
			modify: func(c *Config) {
				c.Controller.Listen.PublicAPI.Enabled = true
				c.Controller.Listen.PublicAPI.TCP = ":9001"
			},
			wantErr: false,
		},
		{
			name: "public API enabled without TCP address",
			modify: func(c *Config) {
				c.Controller.Listen.PublicAPI.Enabled = true
				c.Controller.Listen.PublicAPI.TCP = ""
			},
			wantErr: true,
			errText: "controller.listen.public_api.tcp is required when public_api.enabled is true",
		},
		{
			name: "public API disabled with TCP address",
			modify: func(c *Config) {
				c.Controller.Listen.PublicAPI.Enabled = false
				c.Controller.Listen.PublicAPI.TCP = ":9001"
			},
			wantErr: false, // TCP can be set but ignored when disabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.modify(cfg)

			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected validation error, got nil")
				} else if tt.errText != "" && !strings.Contains(err.Error(), tt.errText) {
					t.Errorf("expected error containing %q, got %q", tt.errText, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected validation error: %v", err)
				}
			}
		})
	}
}

// TestPublicAPIFromEnv tests loading public API config from environment variables.
func TestPublicAPIFromEnv(t *testing.T) {
	// Save and restore environment
	oldEnv := saveEnv()
	defer restoreEnv(oldEnv)
	clearConfigEnv()

	// Set public API environment variables
	os.Setenv("CONDUCTOR_PUBLIC_API_ENABLED", "true")
	os.Setenv("CONDUCTOR_PUBLIC_API_TCP", ":9001")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Controller.Listen.PublicAPI.Enabled {
		t.Errorf("expected public API enabled, got disabled")
	}
	if cfg.Controller.Listen.PublicAPI.TCP != ":9001" {
		t.Errorf("expected TCP :9001, got %q", cfg.Controller.Listen.PublicAPI.TCP)
	}
}

// TestPublicAPIFromFile tests loading public API config from YAML file.
func TestPublicAPIFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
controller:
  listen:
    socket_path: /tmp/conductor.sock
    tcp_addr: :9000
    public_api:
      enabled: true
      tcp: :9001
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

	if !cfg.Controller.Listen.PublicAPI.Enabled {
		t.Errorf("expected public API enabled, got disabled")
	}
	if cfg.Controller.Listen.PublicAPI.TCP != ":9001" {
		t.Errorf("expected TCP :9001, got %q", cfg.Controller.Listen.PublicAPI.TCP)
	}
}
