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
	"fmt"
	"os"
)

// SupportedProviderTypes lists provider types officially supported in this release.
// Other provider types may work but are considered experimental.
var SupportedProviderTypes = []string{
	"claude-code",
}

// AllProviderTypes lists all provider types that have implementations,
// including experimental ones.
var AllProviderTypes = []string{
	"claude-code",
	"anthropic",
	"openai",
	"ollama",
}

// IsSupportedProvider returns true if the given provider type is officially supported.
func IsSupportedProvider(providerType string) bool {
	for _, supported := range SupportedProviderTypes {
		if providerType == supported {
			return true
		}
	}
	return false
}

// AllProvidersEnabled checks if the CONDUCTOR_ALL_PROVIDERS environment variable
// is set to enable all provider types (including experimental ones).
func AllProvidersEnabled() bool {
	return os.Getenv("CONDUCTOR_ALL_PROVIDERS") == "1"
}

// GetVisibleProviderTypes returns the list of provider types that should be
// shown in interactive prompts. Returns all types if CONDUCTOR_ALL_PROVIDERS=1,
// otherwise returns only officially supported types.
func GetVisibleProviderTypes() []string {
	if AllProvidersEnabled() {
		return AllProviderTypes
	}
	return SupportedProviderTypes
}

// WarnUnsupportedProvider writes a warning to stderr if the given provider type
// is not officially supported. This is a non-blocking warning.
func WarnUnsupportedProvider(providerType string) {
	if !IsSupportedProvider(providerType) {
		fmt.Fprintf(os.Stderr, "warning: Provider '%s' is not officially supported in this release. Use at your own risk.\n", providerType)
	}
}
