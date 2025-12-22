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
	"testing"

	internalConfig "github.com/tombee/conductor/internal/config"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         *internalConfig.Config
		wantValid      bool
		wantErrorCount int
		wantWarnCount  int
	}{
		{
			name: "valid config with all fields",
			config: &internalConfig.Config{
				Version: 1,
				Providers: internalConfig.ProvidersMap{
					"anthropic": internalConfig.ProviderConfig{
						Type: "anthropic",
						Models: map[string]internalConfig.ModelConfig{
							"claude-3-5-haiku-20241022": {
								ContextWindow: 200000,
							},
						},
					},
				},
				Tiers: map[string]string{
					"fast":     "anthropic/claude-3-5-haiku-20241022",
					"balanced": "anthropic/claude-3-5-haiku-20241022",
				},
			},
			wantValid:      true,
			wantErrorCount: 0,
			wantWarnCount:  1, // Missing strategic tier
		},
		{
			name: "config with no version",
			config: &internalConfig.Config{
				Version: 0,
				Providers: internalConfig.ProvidersMap{
					"anthropic": internalConfig.ProviderConfig{
						Type: "anthropic",
						Models: map[string]internalConfig.ModelConfig{
							"claude-3-5-haiku-20241022": {},
						},
					},
				},
			},
			wantValid:      true,
			wantErrorCount: 0,
			wantWarnCount:  2, // Missing version + no tiers
		},
		{
			name: "config with no providers",
			config: &internalConfig.Config{
				Version:   1,
				Providers: internalConfig.ProvidersMap{},
			},
			wantValid:      true,
			wantErrorCount: 0,
			wantWarnCount:  2, // No providers + no tiers
		},
		{
			name: "config with provider missing type",
			config: &internalConfig.Config{
				Version: 1,
				Providers: internalConfig.ProvidersMap{
					"bad-provider": internalConfig.ProviderConfig{
						Type: "", // Missing type
					},
				},
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantWarnCount:  2, // No models + no tiers
		},
		{
			name: "config with orphaned tier mapping",
			config: &internalConfig.Config{
				Version:   1,
				Providers: internalConfig.ProvidersMap{},
				Tiers: map[string]string{
					"fast": "missing/model",
				},
			},
			wantValid:      false,
			wantErrorCount: 1, // Orphaned tier
			wantWarnCount:  3, // No providers + missing balanced + missing strategic
		},
		{
			name: "config with invalid tier name",
			config: &internalConfig.Config{
				Version: 1,
				Providers: internalConfig.ProvidersMap{
					"anthropic": internalConfig.ProviderConfig{
						Type: "anthropic",
						Models: map[string]internalConfig.ModelConfig{
							"claude-3-5-haiku-20241022": {},
						},
					},
				},
				Tiers: map[string]string{
					"custom": "anthropic/claude-3-5-haiku-20241022",
				},
			},
			wantValid:      false,
			wantErrorCount: 1, // Invalid tier name
			wantWarnCount:  3, // Missing all standard tiers
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateConfig(tt.config)

			if result.Valid != tt.wantValid {
				t.Errorf("validateConfig() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if len(result.Errors) != tt.wantErrorCount {
				t.Errorf("validateConfig() Errors count = %d, want %d", len(result.Errors), tt.wantErrorCount)
				for _, err := range result.Errors {
					t.Logf("  Error: %s", err)
				}
			}

			if len(result.Warnings) != tt.wantWarnCount {
				t.Errorf("validateConfig() Warnings count = %d, want %d", len(result.Warnings), tt.wantWarnCount)
				for _, warn := range result.Warnings {
					t.Logf("  Warning: %s", warn)
				}
			}
		})
	}
}

func TestValidateConfig_AllStandardTiers(t *testing.T) {
	config := &internalConfig.Config{
		Version: 1,
		Providers: internalConfig.ProvidersMap{
			"anthropic": internalConfig.ProviderConfig{
				Type: "anthropic",
				Models: map[string]internalConfig.ModelConfig{
					"claude-3-5-haiku-20241022":  {},
					"claude-3-5-sonnet-20241022": {},
					"claude-opus-4-20250514":     {},
				},
			},
		},
		Tiers: map[string]string{
			"fast":      "anthropic/claude-3-5-haiku-20241022",
			"balanced":  "anthropic/claude-3-5-sonnet-20241022",
			"strategic": "anthropic/claude-opus-4-20250514",
		},
	}

	result := validateConfig(config)

	if !result.Valid {
		t.Errorf("validateConfig() with all standard tiers should be valid")
	}

	if len(result.Errors) != 0 {
		t.Errorf("validateConfig() should have no errors, got %d", len(result.Errors))
	}

	if len(result.Warnings) != 0 {
		t.Errorf("validateConfig() should have no warnings, got %d", len(result.Warnings))
		for _, warn := range result.Warnings {
			t.Logf("  Warning: %s", warn)
		}
	}
}
