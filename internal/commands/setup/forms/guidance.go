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
	"regexp"
)

// APIKeyGuidance provides provider-specific guidance for API key configuration.
type APIKeyGuidance struct {
	// URL is where users can obtain the API key
	URL string
	// FormatHint describes the expected format of the API key
	FormatHint string
	// ValidationRegex is used for real-time format validation
	ValidationRegex *regexp.Regexp
}

// GetAPIKeyGuidance returns guidance for a given provider type.
// Returns nil if the provider doesn't require an API key or guidance is not available.
func GetAPIKeyGuidance(providerType string) *APIKeyGuidance {
	guidance, ok := apiKeyGuidanceMap[providerType]
	if !ok {
		return nil
	}
	return guidance
}

// apiKeyGuidanceMap contains API key guidance for each provider type that requires one.
var apiKeyGuidanceMap = map[string]*APIKeyGuidance{
	"anthropic": {
		URL:             "console.anthropic.com/settings/keys",
		FormatHint:      "Starts with sk-ant-api03-...",
		ValidationRegex: regexp.MustCompile(`^sk-ant-api\d{2}-[A-Za-z0-9_-]{95}$`),
	},
	"openai": {
		URL:             "platform.openai.com/api-keys",
		FormatHint:      "Starts with sk-...",
		ValidationRegex: regexp.MustCompile(`^sk-[A-Za-z0-9]{48,}$`),
	},
	"openai-compatible": {
		URL:             "",
		FormatHint:      "Format varies by provider",
		ValidationRegex: regexp.MustCompile(`.{10,}`), // At least 10 characters
	},
	"google": {
		URL:             "aistudio.google.com/app/apikey",
		FormatHint:      "Starts with AIza...",
		ValidationRegex: regexp.MustCompile(`^AIza[A-Za-z0-9_-]{35}$`),
	},
	"openrouter": {
		URL:             "openrouter.ai/keys",
		FormatHint:      "Starts with sk-or-...",
		ValidationRegex: regexp.MustCompile(`^sk-or-v1-[A-Za-z0-9]{64}$`),
	},
	"groq": {
		URL:             "console.groq.com/keys",
		FormatHint:      "Starts with gsk_...",
		ValidationRegex: regexp.MustCompile(`^gsk_[A-Za-z0-9]{52}$`),
	},
}

// ValidateAPIKeyFormat checks if an API key matches the expected format for a provider.
// Returns true if valid, false if invalid.
// Always returns true if no guidance is available for the provider.
func ValidateAPIKeyFormat(providerType, apiKey string) bool {
	guidance := GetAPIKeyGuidance(providerType)
	if guidance == nil || guidance.ValidationRegex == nil {
		// No validation available, accept any non-empty key
		return apiKey != ""
	}

	return guidance.ValidationRegex.MatchString(apiKey)
}
