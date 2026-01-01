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

package forms

import (
	"context"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/commands/setup"
	"github.com/tombee/conductor/internal/config"
)

func TestDetectCLIProvidersParallel(t *testing.T) {
	ctx := context.Background()

	// Get all provider types
	allProviders := setup.GetProviderTypes()

	// Filter CLI providers
	var cliProviders []setup.ProviderType
	for _, pt := range allProviders {
		if pt.IsCLI() {
			cliProviders = append(cliProviders, pt)
		}
	}

	if len(cliProviders) == 0 {
		t.Skip("No CLI providers registered")
	}

	t.Run("completes within timeout", func(t *testing.T) {
		start := time.Now()
		results := detectCLIProvidersParallel(ctx, cliProviders)
		duration := time.Since(start)

		// Should complete within 3 seconds (2s timeout + 1s buffer)
		if duration > 3*time.Second {
			t.Errorf("detectCLIProvidersParallel took %v, expected < 3s", duration)
		}

		// Should return results for all providers
		if len(results) != len(cliProviders) {
			t.Errorf("got %d results, want %d", len(results), len(cliProviders))
		}
	})

	t.Run("returns result for each provider", func(t *testing.T) {
		results := detectCLIProvidersParallel(ctx, cliProviders)

		// Verify each provider has a result
		providerNames := make(map[string]bool)
		for _, result := range results {
			providerNames[result.ProviderType.Name()] = true
		}

		for _, pt := range cliProviders {
			if !providerNames[pt.Name()] {
				t.Errorf("missing result for provider %s", pt.Name())
			}
		}
	})

	t.Run("detection status is set", func(t *testing.T) {
		results := detectCLIProvidersParallel(ctx, cliProviders)

		for _, result := range results {
			// Each result should have a provider type
			if result.ProviderType == nil {
				t.Error("result missing ProviderType")
			}

			// If detected, should have a path
			if result.Detected && result.Path == "" {
				t.Errorf("provider %s marked as detected but has no path", result.ProviderType.Name())
			}
		}
	})
}

func TestDetectCLIProvidersParallel_WithCancelledContext(t *testing.T) {
	// Get all provider types
	allProviders := setup.GetProviderTypes()

	// Filter CLI providers
	var cliProviders []setup.ProviderType
	for _, pt := range allProviders {
		if pt.IsCLI() {
			cliProviders = append(cliProviders, pt)
		}
	}

	if len(cliProviders) == 0 {
		t.Skip("No CLI providers registered")
	}

	t.Run("handles cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		results := detectCLIProvidersParallel(ctx, cliProviders)

		// Should still return results for all providers
		if len(results) != len(cliProviders) {
			t.Errorf("got %d results, want %d", len(results), len(cliProviders))
		}
	})
}

func TestProviderDetectionResult(t *testing.T) {
	// This is mainly to ensure the type is correctly defined
	result := ProviderDetectionResult{
		ProviderType: nil,
		Detected:     false,
		Path:         "",
		Error:        nil,
	}

	if result.Detected {
		t.Error("expected Detected to be false")
	}
}

func TestBuildProviderListSummary(t *testing.T) {
	tests := []struct {
		name     string
		state    *setup.SetupState
		wantText string
	}{
		{
			name: "empty providers",
			state: &setup.SetupState{
				Working: &config.Config{
					Providers: make(config.ProvidersMap),
				},
			},
			wantText: "No providers configured yet.",
		},
		{
			name: "single provider no default",
			state: &setup.SetupState{
				Working: &config.Config{
					Providers: config.ProvidersMap{
						"anthropic": config.ProviderConfig{Type: "anthropic"},
					},
				},
			},
			wantText: "anthropic (anthropic)",
		},
		{
			name: "multiple providers with default",
			state: &setup.SetupState{
				Working: &config.Config{
					Providers: config.ProvidersMap{
						"anthropic": config.ProviderConfig{Type: "anthropic"},
						"openai":    config.ProviderConfig{Type: "openai"},
					},
					DefaultProvider: "anthropic",
				},
			},
			wantText: "anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildProviderListSummary(tt.state)
			if !contains(result, tt.wantText) {
				t.Errorf("buildProviderListSummary() = %q, want to contain %q", result, tt.wantText)
			}
		})
	}
}

func TestParseCredentialRef(t *testing.T) {
	tests := []struct {
		name        string
		ref         string
		wantBackend string
		wantKey     string
	}{
		{
			name:        "keychain reference",
			ref:         "$keychain:ANTHROPIC_API_KEY",
			wantBackend: "keychain",
			wantKey:     "ANTHROPIC_API_KEY",
		},
		{
			name:        "env reference",
			ref:         "$env:OPENAI_API_KEY",
			wantBackend: "env",
			wantKey:     "OPENAI_API_KEY",
		},
		{
			name:        "secret reference",
			ref:         "$secret:MY_SECRET",
			wantBackend: "secret",
			wantKey:     "MY_SECRET",
		},
		{
			name:        "not a reference",
			ref:         "plain-text-key",
			wantBackend: "",
			wantKey:     "",
		},
		{
			name:        "malformed reference",
			ref:         "$nocolon",
			wantBackend: "",
			wantKey:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, key := parseCredentialRef(tt.ref)
			if backend != tt.wantBackend {
				t.Errorf("parseCredentialRef() backend = %q, want %q", backend, tt.wantBackend)
			}
			if key != tt.wantKey {
				t.Errorf("parseCredentialRef() key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}

func TestSanitizeErrorDetails(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSafe bool // true if output should NOT contain the sensitive part
		check    string
	}{
		{
			name:     "sanitize API key",
			input:    "Error: invalid API key sk-ant-1234567890abcdef",
			wantSafe: true,
			check:    "sk-ant-1234567890abcdef",
		},
		{
			name:     "sanitize file path",
			input:    "Failed to read /home/user/.conductor/config.yaml",
			wantSafe: true,
			check:    "/home/user/.conductor/config.yaml",
		},
		{
			name:     "sanitize IP address",
			input:    "Connection failed to 192.168.1.100",
			wantSafe: true,
			check:    "192.168.1.100",
		},
		{
			name:     "safe error message",
			input:    "Connection timeout",
			wantSafe: false,
			check:    "Connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeErrorDetails(tt.input)
			containsCheck := contains(result, tt.check)
			if tt.wantSafe && containsCheck {
				t.Errorf("sanitizeErrorDetails() still contains sensitive data %q in output %q", tt.check, result)
			}
			if !tt.wantSafe && !containsCheck {
				t.Errorf("sanitizeErrorDetails() removed non-sensitive data %q from output %q", tt.check, result)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
