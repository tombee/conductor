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

package provider

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tombee/conductor/internal/config"
)

// Reserved provider names that cannot be used.
var reservedNames = map[string]bool{
	"add":    true,
	"remove": true,
	"list":   true,
	"test":   true,
	"edit":   true,
}

// providerNameRegex matches valid provider names:
// - Must start with letter or underscore
// - Can contain alphanumeric, dash, underscore
// - 1-64 characters total
var providerNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]{0,63}$`)

// ValidateProviderName validates a provider name against naming rules.
// Returns an error if the name is invalid.
func ValidateProviderName(name string, existingProviders config.ProvidersMap) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	if len(name) > 64 {
		return fmt.Errorf("provider name too long (max 64 characters)")
	}

	if !providerNameRegex.MatchString(name) {
		return fmt.Errorf("provider name must start with a letter or underscore and contain only alphanumeric characters, dashes, or underscores")
	}

	if reservedNames[strings.ToLower(name)] {
		return fmt.Errorf("'%s' is a reserved name and cannot be used", name)
	}

	if existingProviders != nil {
		if _, exists := existingProviders[name]; exists {
			return fmt.Errorf("provider '%s' already exists", name)
		}
	}

	return nil
}

// ValidateProviderNameFunc returns a validation function for use with huh forms.
func ValidateProviderNameFunc(existingProviders config.ProvidersMap) func(string) error {
	return func(name string) error {
		return ValidateProviderName(name, existingProviders)
	}
}

// ValidateProviderNameOrEmptyFunc returns a validation function that allows empty
// (which will default to defaultName) or validates the provided name.
func ValidateProviderNameOrEmptyFunc(existingProviders config.ProvidersMap, defaultName string) func(string) error {
	return func(name string) error {
		if name == "" {
			// Empty is allowed, will default to defaultName - but check if default exists
			if existingProviders != nil {
				if _, exists := existingProviders[defaultName]; exists {
					return fmt.Errorf("provider '%s' already exists", defaultName)
				}
			}
			return nil
		}
		return ValidateProviderName(name, existingProviders)
	}
}

// Placeholder patterns that indicate invalid API keys.
var placeholderPatterns = []string{
	"xxx",
	"dummy",
	"test",
	"sk-test-",
	"placeholder",
	"your-api-key",
	"api_key_here",
}

// ValidateAPIKey validates an API key value.
// Returns an error if the key is invalid.
func ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	if len(apiKey) < 8 {
		return fmt.Errorf("API key too short (minimum 8 characters)")
	}

	if len(apiKey) > 8192 {
		return fmt.Errorf("API key too long (maximum 8192 characters)")
	}

	// Check for common placeholder patterns
	lowerKey := strings.ToLower(apiKey)
	for _, pattern := range placeholderPatterns {
		if strings.Contains(lowerKey, pattern) {
			return fmt.Errorf("API key appears to be a placeholder value")
		}
	}

	return nil
}

// ValidateEnvVarName validates an environment variable name.
func ValidateEnvVarName(envVar string) error {
	if envVar == "" {
		return fmt.Errorf("environment variable name cannot be empty")
	}

	// Environment variable names should be alphanumeric with underscores
	validEnvVar := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	if !validEnvVar.MatchString(envVar) {
		return fmt.Errorf("invalid environment variable name: must start with a letter or underscore and contain only alphanumeric characters or underscores")
	}

	return nil
}
