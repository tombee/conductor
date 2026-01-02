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
	"fmt"
	"strings"
)

var (
	// ErrTierNotMapped is returned when a tier has no mapping configured.
	ErrTierNotMapped = errors.New("tier not mapped")

	// ErrInvalidTierReference is returned when a tier reference has invalid format.
	ErrInvalidTierReference = errors.New("invalid tier reference format")

	// ErrProviderNotFound is returned when a tier references a non-existent provider.
	ErrProviderNotFound = errors.New("provider not found")

	// ErrModelNotFound is returned when a tier references a non-existent model.
	ErrModelNotFound = errors.New("model not found")
)

// ValidTiers lists the supported tier names.
var ValidTiers = []string{"fast", "balanced", "strategic"}

// ResolveTier resolves a tier name to its provider and model.
// Returns the provider name, model name, and any error.
//
// Tier references use the format "provider/model" (e.g., "anthropic/claude-3-5-haiku-20241022").
// The function validates that:
//   - The tier exists in the config's Tiers map
//   - The reference follows "provider/model" format
//   - The provider exists in the config's Providers map
//   - The model exists under that provider's Models map
func (c *Config) ResolveTier(tierName string) (provider string, model string, err error) {
	// Check if tier is mapped
	tierRef, exists := c.Tiers[tierName]
	if !exists {
		return "", "", fmt.Errorf("%w: tier %q not configured", ErrTierNotMapped, tierName)
	}

	// Parse provider/model reference
	provider, model, err = ParseModelReference(tierRef)
	if err != nil {
		return "", "", fmt.Errorf("tier %q: %w", tierName, err)
	}

	// Validate provider exists
	providerCfg, exists := c.Providers[provider]
	if !exists {
		return "", "", fmt.Errorf("%w: tier %q references unknown provider %q", ErrProviderNotFound, tierName, provider)
	}

	// Validate model exists under provider
	if providerCfg.Models == nil {
		return "", "", fmt.Errorf("%w: provider %q has no models registered", ErrModelNotFound, provider)
	}

	if _, exists := providerCfg.Models[model]; !exists {
		return "", "", fmt.Errorf("%w: provider %q has no model %q", ErrModelNotFound, provider, model)
	}

	return provider, model, nil
}

// ParseModelReference parses a "provider/model" reference into its components.
// Returns an error if the format is invalid.
func ParseModelReference(ref string) (provider string, model string, err error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("%w: expected 'provider/model', got %q", ErrInvalidTierReference, ref)
	}

	provider = strings.TrimSpace(parts[0])
	model = strings.TrimSpace(parts[1])

	if provider == "" || model == "" {
		return "", "", fmt.Errorf("%w: provider and model cannot be empty in %q", ErrInvalidTierReference, ref)
	}

	return provider, model, nil
}

// ValidateTierName checks if a tier name is one of the supported tiers.
func ValidateTierName(tierName string) error {
	for _, valid := range ValidTiers {
		if tierName == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid tier name %q: must be one of %v", tierName, ValidTiers)
}

// ValidateTiers validates all tier mappings in the config.
// Returns a list of validation errors.
func (c *Config) ValidateTiers() []error {
	var errs []error

	for tierName := range c.Tiers {
		// Validate tier name is supported
		if err := ValidateTierName(tierName); err != nil {
			errs = append(errs, err)
			continue
		}

		// Try to resolve the tier
		if _, _, err := c.ResolveTier(tierName); err != nil {
			errs = append(errs, fmt.Errorf("tier %q: %w", tierName, err))
		}
	}

	// Check for orphaned models (models referenced in tiers that no longer exist)
	// Note: This is already handled by ResolveTier above, but we can add additional checks

	return errs
}

// GetModelConfig retrieves the model configuration for a provider/model reference.
func (c *Config) GetModelConfig(providerName, modelName string) (*ModelConfig, error) {
	providerCfg, exists := c.Providers[providerName]
	if !exists {
		return nil, fmt.Errorf("%w: provider %q not found", ErrProviderNotFound, providerName)
	}

	if providerCfg.Models == nil {
		return nil, fmt.Errorf("%w: provider %q has no models", ErrModelNotFound, providerName)
	}

	modelCfg, exists := providerCfg.Models[modelName]
	if !exists {
		return nil, fmt.Errorf("%w: model %q not found in provider %q", ErrModelNotFound, modelName, providerName)
	}

	return &modelCfg, nil
}

// ListModels returns all registered models in provider/model format.
func (c *Config) ListModels() []string {
	var models []string

	for providerName, providerCfg := range c.Providers {
		for modelName := range providerCfg.Models {
			models = append(models, fmt.Sprintf("%s/%s", providerName, modelName))
		}
	}

	return models
}

// GetTierModel returns the model reference for a tier (without validation).
// Use ResolveTier for validated resolution.
func (c *Config) GetTierModel(tierName string) (string, bool) {
	ref, exists := c.Tiers[tierName]
	return ref, exists
}
