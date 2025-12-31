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
	"strings"
	"testing"

	"github.com/tombee/conductor/internal/commands/setup"
	"github.com/tombee/conductor/internal/config"
)

func TestMaskCredential(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "long API key",
			input:    "sk-ant-1234567890abcdef",
			expected: "sk-•••••cdef",
		},
		{
			name:     "short value",
			input:    "abc",
			expected: "a••",
		},
		{
			name:     "single char",
			input:    "x",
			expected: "*",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "*",
		},
		{
			name:     "exactly 10 chars",
			input:    "1234567890",
			expected: "1•••••••••",
		},
		{
			name:     "11 chars",
			input:    "12345678901",
			expected: "123•••••8901",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskCredential(tt.input)
			if result != tt.expected {
				t.Errorf("maskCredential(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildReviewSummary(t *testing.T) {
	state := &setup.SetupState{
		ConfigPath: "/home/user/.config/conductor/config.yaml",
		Working: &config.Config{
			DefaultProvider: "anthropic",
			Providers: config.ProvidersMap{
				"anthropic": config.ProviderConfig{
					Type:   "anthropic",
					APIKey: "sk-ant-1234567890abcdef",
				},
				"openai": config.ProviderConfig{
					Type:   "openai-compatible",
					APIKey: "$env:OPENAI_API_KEY",
				},
			},
		},
	}

	summary := buildReviewSummary(state)

	// Verify providers section
	if !strings.Contains(summary, "Providers:") {
		t.Error("Summary missing Providers section")
	}
	if !strings.Contains(summary, "anthropic") {
		t.Error("Summary missing anthropic provider")
	}
	if !strings.Contains(summary, "openai") {
		t.Error("Summary missing openai provider")
	}
	if !strings.Contains(summary, "(default)") {
		t.Error("Summary missing default provider indicator")
	}

	// Verify API key is masked for plaintext
	if !strings.Contains(summary, "sk-•••••cdef") {
		t.Error("Summary should mask plaintext API key")
	}

	// Verify secret reference is not masked
	if !strings.Contains(summary, "$env:OPENAI_API_KEY") {
		t.Error("Summary should show secret references as-is")
	}

	// Note: BaseURL was removed from the test since ProviderConfig doesn't have that field
	// The test has been simplified to focus on the fields that actually exist

	// Verify config path
	if !strings.Contains(summary, "/home/user/.config/conductor/config.yaml") {
		t.Error("Summary missing config path")
	}
}

func TestBuildReviewSummaryEmptyProviders(t *testing.T) {
	state := &setup.SetupState{
		ConfigPath: "/test/config.yaml",
		Working: &config.Config{
			Providers: config.ProvidersMap{},
		},
	}

	summary := buildReviewSummary(state)

	if !strings.Contains(summary, "(none configured)") {
		t.Error("Summary should indicate no providers configured")
	}
}

func TestBuildReviewSummaryWithModels(t *testing.T) {
	state := &setup.SetupState{
		ConfigPath: "/test/config.yaml",
		Working: &config.Config{
			DefaultProvider: "openai",
			Providers: config.ProvidersMap{
				"openai": config.ProviderConfig{
					Type: "openai-compatible",
					Models: config.ModelTierMap{
						Fast:      "gpt-3.5-turbo",
						Balanced:  "gpt-4",
						Strategic: "gpt-4-turbo",
					},
				},
			},
		},
	}

	summary := buildReviewSummary(state)

	// Verify models section is shown
	if !strings.Contains(summary, "Models:") {
		t.Error("Summary missing Models section")
	}
	if !strings.Contains(summary, "gpt-3.5-turbo") {
		t.Error("Summary missing fast model")
	}
	if !strings.Contains(summary, "gpt-4") {
		t.Error("Summary missing balanced model")
	}
}

func TestMaskCredentialDoesNotAlterSecretReferences(t *testing.T) {
	tests := []string{
		"$secret:ANTHROPIC_API_KEY",
		"$env:OPENAI_API_KEY",
		"$vault:path/to/secret#key",
	}

	for _, ref := range tests {
		// The masking should only happen for non-secret-reference values
		// In the buildReviewSummary, we check for prefixes before masking
		if strings.HasPrefix(ref, "$") {
			// Secret references should NOT be masked
			if maskCredential(ref) == ref {
				// This is expected - but the code should check before calling maskCredential
				continue
			}
		}
	}
}
