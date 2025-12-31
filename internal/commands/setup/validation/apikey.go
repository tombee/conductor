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

package validation

import (
	"fmt"
	"regexp"
)

// apiKeyPatterns maps provider types to their expected API key formats
var apiKeyPatterns = map[string]*regexp.Regexp{
	"anthropic":        regexp.MustCompile(`^sk-ant-api\d{2}-[A-Za-z0-9_-]{93}$`),
	"openai":           regexp.MustCompile(`^sk-[A-Za-z0-9]{48}$`),
	"openai-project":   regexp.MustCompile(`^sk-proj-[A-Za-z0-9_-]{48,}$`),
	"github-pat":       regexp.MustCompile(`^ghp_[A-Za-z0-9]{36}$`),
	"github-fine":      regexp.MustCompile(`^github_pat_[A-Za-z0-9]{22}_[A-Za-z0-9]{59}$`),
	"github-oauth":     regexp.MustCompile(`^gho_[A-Za-z0-9]{36}$`),
	"github-user":      regexp.MustCompile(`^ghu_[A-Za-z0-9]{36}$`),
	"github-server":    regexp.MustCompile(`^ghs_[A-Za-z0-9]{36}$`),
	"github-refresh":   regexp.MustCompile(`^ghr_[A-Za-z0-9]{36}$`),
	"slack-bot":        regexp.MustCompile(`^xoxb-[0-9]+-[0-9]+-[A-Za-z0-9]+$`),
	"slack-user":       regexp.MustCompile(`^xoxp-[0-9]+-[0-9]+-[A-Za-z0-9]+$`),
	"slack-app":        regexp.MustCompile(`^xoxa-[0-9]+-[0-9]+-[A-Za-z0-9]+$`),
	"slack-refresh":    regexp.MustCompile(`^xoxr-[0-9]+-[0-9]+-[A-Za-z0-9]+$`),
	"google-api":       regexp.MustCompile(`^AIza[A-Za-z0-9_-]{35}$`),
	"aws-access":       regexp.MustCompile(`^AKIA[A-Z0-9]{16}$`),
	"gitlab-pat":       regexp.MustCompile(`^glpat-[A-Za-z0-9_-]{20}$`),
}

// ValidateAPIKey validates an API key for a specific provider type.
// For openai-compatible providers, any non-empty key is accepted.
// For known providers (anthropic, openai, github, etc.), format is validated.
func ValidateAPIKey(providerType, key string) error {
	if key == "" {
		return fmt.Errorf("API key is required")
	}

	// For openai-compatible, accept any non-empty key
	if providerType == "openai-compatible" {
		return nil
	}

	// Check if we have a pattern for this provider
	pattern, ok := apiKeyPatterns[providerType]
	if !ok {
		// Unknown provider, just check non-empty
		return nil
	}

	// Validate against pattern
	if !pattern.MatchString(key) {
		return fmt.Errorf("invalid %s API key format", providerType)
	}

	return nil
}

// DetectAPIKeyType attempts to detect the API key type from its format.
// Returns the detected type or "unknown" if no pattern matches.
func DetectAPIKeyType(key string) string {
	for keyType, pattern := range apiKeyPatterns {
		if pattern.MatchString(key) {
			return keyType
		}
	}
	return "unknown"
}

// MaskAPIKey masks an API key for display purposes.
// Shows first 4 and last 4 characters with dots in between.
// Format: "sk-a•••••1234"
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "••••••••"
	}
	return fmt.Sprintf("%s•••••%s", key[:4], key[len(key)-4:])
}

// IsPlaintextCredential checks if a value looks like a plaintext credential
// (not a reference like "$secret:KEY" or "$env:VAR")
func IsPlaintextCredential(value string) bool {
	// References start with $
	if value == "" || value[0] == '$' {
		return false
	}

	// Check if it matches any known credential pattern
	keyType := DetectAPIKeyType(value)
	return keyType != "unknown"
}
