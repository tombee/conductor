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
	"errors"
	"testing"
)

func TestParseModelReference(t *testing.T) {
	tests := []struct {
		name         string
		ref          string
		wantProvider string
		wantModel    string
		wantErr      bool
		wantErrIs    error
	}{
		{
			name:         "valid reference",
			ref:          "anthropic/claude-3-5-haiku-20241022",
			wantProvider: "anthropic",
			wantModel:    "claude-3-5-haiku-20241022",
			wantErr:      false,
		},
		{
			name:         "valid reference with hyphen",
			ref:          "openai/gpt-4",
			wantProvider: "openai",
			wantModel:    "gpt-4",
			wantErr:      false,
		},
		{
			name:      "missing model",
			ref:       "anthropic/",
			wantErr:   true,
			wantErrIs: ErrInvalidTierReference,
		},
		{
			name:      "missing provider",
			ref:       "/claude-3-5-haiku",
			wantErr:   true,
			wantErrIs: ErrInvalidTierReference,
		},
		{
			name:      "no slash separator",
			ref:       "anthropic-claude-3-5-haiku",
			wantErr:   true,
			wantErrIs: ErrInvalidTierReference,
		},
		{
			name:      "empty string",
			ref:       "",
			wantErr:   true,
			wantErrIs: ErrInvalidTierReference,
		},
		{
			name:         "reference with spaces trimmed",
			ref:          " anthropic / claude-3-5-haiku ",
			wantProvider: "anthropic",
			wantModel:    "claude-3-5-haiku",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, model, err := ParseModelReference(tt.ref)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseModelReference() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("ParseModelReference() error = %v, wantErrIs %v", err, tt.wantErrIs)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseModelReference() unexpected error = %v", err)
				return
			}

			if provider != tt.wantProvider {
				t.Errorf("ParseModelReference() provider = %v, want %v", provider, tt.wantProvider)
			}
			if model != tt.wantModel {
				t.Errorf("ParseModelReference() model = %v, want %v", model, tt.wantModel)
			}
		})
	}
}

func TestValidateTierName(t *testing.T) {
	tests := []struct {
		name     string
		tierName string
		wantErr  bool
	}{
		{
			name:     "valid fast tier",
			tierName: "fast",
			wantErr:  false,
		},
		{
			name:     "valid balanced tier",
			tierName: "balanced",
			wantErr:  false,
		},
		{
			name:     "valid strategic tier",
			tierName: "strategic",
			wantErr:  false,
		},
		{
			name:     "invalid tier",
			tierName: "custom",
			wantErr:  true,
		},
		{
			name:     "empty tier",
			tierName: "",
			wantErr:  true,
		},
		{
			name:     "case sensitive",
			tierName: "Fast",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTierName(tt.tierName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTierName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveTier(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		tierName     string
		wantProvider string
		wantModel    string
		wantErr      bool
		wantErrIs    error
	}{
		{
			name: "resolve existing tier",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{
						Type: "anthropic",
						Models: map[string]ModelConfig{
							"claude-3-5-haiku-20241022": {},
						},
					},
				},
				Tiers: map[string]string{
					"fast": "anthropic/claude-3-5-haiku-20241022",
				},
			},
			tierName:     "fast",
			wantProvider: "anthropic",
			wantModel:    "claude-3-5-haiku-20241022",
			wantErr:      false,
		},
		{
			name: "tier not mapped",
			config: &Config{
				Providers: ProvidersMap{},
				Tiers:     map[string]string{},
			},
			tierName:  "fast",
			wantErr:   true,
			wantErrIs: ErrTierNotMapped,
		},
		{
			name: "provider not found",
			config: &Config{
				Providers: ProvidersMap{},
				Tiers: map[string]string{
					"fast": "missing/claude-3-5-haiku",
				},
			},
			tierName:  "fast",
			wantErr:   true,
			wantErrIs: ErrProviderNotFound,
		},
		{
			name: "model not found",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{
						Type:   "anthropic",
						Models: map[string]ModelConfig{},
					},
				},
				Tiers: map[string]string{
					"fast": "anthropic/missing-model",
				},
			},
			tierName:  "fast",
			wantErr:   true,
			wantErrIs: ErrModelNotFound,
		},
		{
			name: "invalid tier reference format",
			config: &Config{
				Providers: ProvidersMap{},
				Tiers: map[string]string{
					"fast": "invalid-reference",
				},
			},
			tierName:  "fast",
			wantErr:   true,
			wantErrIs: ErrInvalidTierReference,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, model, err := tt.config.ResolveTier(tt.tierName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveTier() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("ResolveTier() error = %v, wantErrIs %v", err, tt.wantErrIs)
				}
				return
			}

			if err != nil {
				t.Errorf("ResolveTier() unexpected error = %v", err)
				return
			}

			if provider != tt.wantProvider {
				t.Errorf("ResolveTier() provider = %v, want %v", provider, tt.wantProvider)
			}
			if model != tt.wantModel {
				t.Errorf("ResolveTier() model = %v, want %v", model, tt.wantModel)
			}
		})
	}
}

func TestGetModelConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		providerName string
		modelName    string
		wantErr      bool
		wantErrIs    error
	}{
		{
			name: "get existing model",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{
						Type: "anthropic",
						Models: map[string]ModelConfig{
							"claude-3-5-haiku-20241022": {
								ContextWindow: 200000,
							},
						},
					},
				},
			},
			providerName: "anthropic",
			modelName:    "claude-3-5-haiku-20241022",
			wantErr:      false,
		},
		{
			name: "provider not found",
			config: &Config{
				Providers: ProvidersMap{},
			},
			providerName: "missing",
			modelName:    "some-model",
			wantErr:      true,
			wantErrIs:    ErrProviderNotFound,
		},
		{
			name: "model not found",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{
						Type:   "anthropic",
						Models: map[string]ModelConfig{},
					},
				},
			},
			providerName: "anthropic",
			modelName:    "missing-model",
			wantErr:      true,
			wantErrIs:    ErrModelNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := tt.config.GetModelConfig(tt.providerName, tt.modelName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetModelConfig() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("GetModelConfig() error = %v, wantErrIs %v", err, tt.wantErrIs)
				}
				return
			}

			if err != nil {
				t.Errorf("GetModelConfig() unexpected error = %v", err)
				return
			}

			if cfg == nil {
				t.Error("GetModelConfig() returned nil config")
			}
		})
	}
}

func TestListModels(t *testing.T) {
	config := &Config{
		Providers: ProvidersMap{
			"anthropic": ProviderConfig{
				Type: "anthropic",
				Models: map[string]ModelConfig{
					"claude-3-5-haiku-20241022":  {},
					"claude-3-5-sonnet-20241022": {},
				},
			},
			"openai": ProviderConfig{
				Type: "openai",
				Models: map[string]ModelConfig{
					"gpt-4": {},
				},
			},
		},
	}

	models := config.ListModels()

	// Should return all models in provider/model format
	expectedModels := map[string]bool{
		"anthropic/claude-3-5-haiku-20241022":  true,
		"anthropic/claude-3-5-sonnet-20241022": true,
		"openai/gpt-4":                         true,
	}

	if len(models) != len(expectedModels) {
		t.Errorf("ListModels() returned %d models, want %d", len(models), len(expectedModels))
	}

	for _, model := range models {
		if !expectedModels[model] {
			t.Errorf("ListModels() returned unexpected model: %s", model)
		}
	}
}

func TestValidateTiers(t *testing.T) {
	tests := []struct {
		name       string
		config     *Config
		wantErrLen int
	}{
		{
			name: "valid tiers",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{
						Type: "anthropic",
						Models: map[string]ModelConfig{
							"claude-3-5-haiku-20241022":  {},
							"claude-3-5-sonnet-20241022": {},
						},
					},
				},
				Tiers: map[string]string{
					"fast":     "anthropic/claude-3-5-haiku-20241022",
					"balanced": "anthropic/claude-3-5-sonnet-20241022",
				},
			},
			wantErrLen: 0,
		},
		{
			name: "invalid tier name",
			config: &Config{
				Providers: ProvidersMap{
					"anthropic": ProviderConfig{
						Type: "anthropic",
						Models: map[string]ModelConfig{
							"claude-3-5-haiku-20241022": {},
						},
					},
				},
				Tiers: map[string]string{
					"custom": "anthropic/claude-3-5-haiku-20241022",
				},
			},
			wantErrLen: 1,
		},
		{
			name: "orphaned tier mapping",
			config: &Config{
				Providers: ProvidersMap{},
				Tiers: map[string]string{
					"fast": "missing/model",
				},
			},
			wantErrLen: 1,
		},
		{
			name: "multiple errors",
			config: &Config{
				Providers: ProvidersMap{},
				Tiers: map[string]string{
					"custom": "missing/model",
				},
			},
			wantErrLen: 1, // Invalid tier name (stops before checking orphaned mapping)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.config.ValidateTiers()

			if len(errs) != tt.wantErrLen {
				t.Errorf("ValidateTiers() returned %d errors, want %d", len(errs), tt.wantErrLen)
				for _, err := range errs {
					t.Logf("  Error: %v", err)
				}
			}
		})
	}
}
