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
	"fmt"
	"regexp"
	"strings"

	conductorerrors "github.com/tombee/conductor/pkg/errors"
	"github.com/tombee/conductor/internal/secrets"
)

var (
	// secretRefPattern matches $secret:key references in config values
	secretRefPattern = regexp.MustCompile(`^\$secret:(.+)$`)

	// plaintextAPIKeyPattern matches common plaintext API key formats
	plaintextAPIKeyPattern = regexp.MustCompile(`^(sk-ant-|sk-|gsk-|xai-)`)
)

// ProviderConfig defines configuration for a single provider instance
type ProviderConfig struct {
	// Type specifies the provider implementation (e.g., "claude-code", "anthropic", "openai")
	Type string `yaml:"type" json:"type"`

	// APIKey for direct API access providers (optional, can use env vars)
	APIKey string `yaml:"api_key,omitempty" json:"api_key,omitempty"`

	// BaseURL for API providers that support custom endpoints (e.g., openai-compatible)
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`

	// ConfigPath for CLI-based providers that need custom config locations
	ConfigPath string `yaml:"config_path,omitempty" json:"config_path,omitempty"`

	// Models maps abstract model tiers to provider-specific model names
	// If not specified, provider-specific defaults are used
	Models ModelTierMap `yaml:"models,omitempty" json:"models,omitempty"`
}

// ProvidersMap is a map of provider names to their configurations
// Each key is a unique provider instance name chosen by the user
type ProvidersMap map[string]ProviderConfig

// AgentMappings maps agent names to provider instance names
// This allows workflows to define agents and users to map them to their providers
type AgentMappings map[string]string

// ModelTierMap maps abstract model tiers to provider-specific model names
type ModelTierMap struct {
	Fast       string `yaml:"fast,omitempty" json:"fast,omitempty"`
	Balanced   string `yaml:"balanced,omitempty" json:"balanced,omitempty"`
	Strategic  string `yaml:"strategic,omitempty" json:"strategic,omitempty"`
}

// ResolveSecretReference resolves a $secret:key reference to its actual value.
// If the value doesn't start with $secret:, it's returned as-is.
// This function uses a shared resolver with all available backends.
func ResolveSecretReference(ctx context.Context, value string) (string, error) {
	if value == "" {
		return "", nil
	}

	// Check if this is a secret reference
	matches := secretRefPattern.FindStringSubmatch(value)
	if len(matches) != 2 {
		// Not a secret reference, return as-is
		return value, nil
	}

	// Extract the secret key
	key := matches[1]

	// Create resolver with all available backends
	resolver := createSecretResolver()

	// Resolve the secret
	secretValue, err := resolver.Get(ctx, key)
	if err != nil {
		return "", &conductorerrors.ConfigError{
			Key:    key,
			Reason: fmt.Sprintf("failed to resolve secret reference %q", key),
			Cause:  err,
		}
	}

	return secretValue, nil
}

// createSecretResolver creates a secrets resolver with all available backends.
func createSecretResolver() *secrets.Resolver {
	// Create file backend with default path and empty master key
	fileBackend, _ := secrets.NewFileBackend("", "")

	backends := []secrets.SecretBackend{
		secrets.NewEnvBackend(),
		secrets.NewKeychainBackend(),
		fileBackend,
	}
	return secrets.NewResolver(backends...)
}

// ResolveProviderSecrets resolves all secret references in a provider configuration.
// It modifies the config in place and returns any warnings about plaintext API keys.
func (p *ProviderConfig) ResolveSecrets(ctx context.Context) (warnings []string, err error) {
	// Resolve APIKey
	if p.APIKey != "" {
		// Check for plaintext API key before resolution
		if plaintextAPIKeyPattern.MatchString(p.APIKey) && !strings.HasPrefix(p.APIKey, "$secret:") {
			warnings = append(warnings, fmt.Sprintf(
				"Plaintext API key detected in config. Consider migrating to secrets: conductor secrets migrate",
			))
		}

		// Resolve if it's a secret reference
		resolved, err := ResolveSecretReference(ctx, p.APIKey)
		if err != nil {
			return warnings, &conductorerrors.ConfigError{
				Key:    "api_key",
				Reason: "failed to resolve API key secret reference",
				Cause:  err,
			}
		}
		p.APIKey = resolved
	}

	return warnings, nil
}

// ResolveSecretsInProviders resolves all secret references in all providers.
// Returns aggregated warnings about plaintext API keys.
func ResolveSecretsInProviders(ctx context.Context, providers ProvidersMap) (warnings []string, err error) {
	for name, provider := range providers {
		providerWarnings, err := provider.ResolveSecrets(ctx)
		if err != nil {
			return warnings, &conductorerrors.ConfigError{
				Key:    fmt.Sprintf("providers.%s", name),
				Reason: "failed to resolve provider secrets",
				Cause:  err,
			}
		}

		// Prefix warnings with provider name
		for _, w := range providerWarnings {
			warnings = append(warnings, fmt.Sprintf("provider %q: %s", name, w))
		}

		// Update the map with resolved config
		providers[name] = provider
	}

	return warnings, nil
}
