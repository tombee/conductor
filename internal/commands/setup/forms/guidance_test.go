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
	"testing"
)

func TestGetAPIKeyGuidance(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		wantURL      string
		wantHint     string
		wantNil      bool
	}{
		{
			name:         "anthropic guidance",
			providerType: "anthropic",
			wantURL:      "console.anthropic.com/settings/keys",
			wantHint:     "Starts with sk-ant-api03-...",
			wantNil:      false,
		},
		{
			name:         "openai guidance",
			providerType: "openai",
			wantURL:      "platform.openai.com/api-keys",
			wantHint:     "Starts with sk-...",
			wantNil:      false,
		},
		{
			name:         "google guidance",
			providerType: "google",
			wantURL:      "aistudio.google.com/app/apikey",
			wantHint:     "Starts with AIza...",
			wantNil:      false,
		},
		{
			name:         "openrouter guidance",
			providerType: "openrouter",
			wantURL:      "openrouter.ai/keys",
			wantHint:     "Starts with sk-or-...",
			wantNil:      false,
		},
		{
			name:         "groq guidance",
			providerType: "groq",
			wantURL:      "console.groq.com/keys",
			wantHint:     "Starts with gsk_...",
			wantNil:      false,
		},
		{
			name:         "openai-compatible guidance",
			providerType: "openai-compatible",
			wantURL:      "",
			wantHint:     "Format varies by provider",
			wantNil:      false,
		},
		{
			name:         "unknown provider returns nil",
			providerType: "unknown-provider",
			wantNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAPIKeyGuidance(tt.providerType)

			if tt.wantNil {
				if got != nil {
					t.Errorf("GetAPIKeyGuidance() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("GetAPIKeyGuidance() = nil, want guidance")
			}

			if got.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", got.URL, tt.wantURL)
			}

			if got.FormatHint != tt.wantHint {
				t.Errorf("FormatHint = %q, want %q", got.FormatHint, tt.wantHint)
			}

			if got.ValidationRegex == nil {
				t.Error("ValidationRegex is nil, want regex")
			}
		})
	}
}

func TestValidateAPIKeyFormat(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		apiKey       string
		want         bool
	}{
		// Anthropic
		{
			name:         "valid anthropic key",
			providerType: "anthropic",
			apiKey:       "sk-ant-api03-" + generateString(95, 'a'),
			want:         true,
		},
		{
			name:         "invalid anthropic key - wrong prefix",
			providerType: "anthropic",
			apiKey:       "sk-ant-" + generateString(95, 'a'),
			want:         false,
		},
		{
			name:         "invalid anthropic key - too short",
			providerType: "anthropic",
			apiKey:       "sk-ant-api03-abc",
			want:         false,
		},
		// OpenAI
		{
			name:         "valid openai key",
			providerType: "openai",
			apiKey:       "sk-" + generateString(48, 'A'),
			want:         true,
		},
		{
			name:         "invalid openai key - wrong prefix",
			providerType: "openai",
			apiKey:       "pk-" + generateString(48, 'A'),
			want:         false,
		},
		{
			name:         "invalid openai key - too short",
			providerType: "openai",
			apiKey:       "sk-abc",
			want:         false,
		},
		// Google
		{
			name:         "valid google key",
			providerType: "google",
			apiKey:       "AIza" + generateString(35, 'B'),
			want:         true,
		},
		{
			name:         "invalid google key - wrong prefix",
			providerType: "google",
			apiKey:       "AIzb" + generateString(35, 'B'),
			want:         false,
		},
		// OpenRouter
		{
			name:         "valid openrouter key",
			providerType: "openrouter",
			apiKey:       "sk-or-v1-" + generateString(64, 'C'),
			want:         true,
		},
		{
			name:         "invalid openrouter key - wrong format",
			providerType: "openrouter",
			apiKey:       "sk-or-" + generateString(64, 'C'),
			want:         false,
		},
		// Groq
		{
			name:         "valid groq key",
			providerType: "groq",
			apiKey:       "gsk_" + generateString(52, 'D'),
			want:         true,
		},
		{
			name:         "invalid groq key - wrong prefix",
			providerType: "groq",
			apiKey:       "gsk" + generateString(52, 'D'),
			want:         false,
		},
		// OpenAI-compatible (lenient)
		{
			name:         "valid openai-compatible key - any 10+ chars",
			providerType: "openai-compatible",
			apiKey:       "any-key-here-123",
			want:         true,
		},
		{
			name:         "invalid openai-compatible key - too short",
			providerType: "openai-compatible",
			apiKey:       "short",
			want:         false,
		},
		// Unknown provider
		{
			name:         "unknown provider accepts non-empty",
			providerType: "unknown-provider",
			apiKey:       "anything",
			want:         true,
		},
		{
			name:         "unknown provider rejects empty",
			providerType: "unknown-provider",
			apiKey:       "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateAPIKeyFormat(tt.providerType, tt.apiKey)
			if got != tt.want {
				t.Errorf("ValidateAPIKeyFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIKeyGuidanceCompleteness(t *testing.T) {
	// Verify all providers in guidance map have complete data
	for providerType, guidance := range apiKeyGuidanceMap {
		t.Run(providerType, func(t *testing.T) {
			if guidance.FormatHint == "" {
				t.Error("FormatHint is empty")
			}

			if guidance.ValidationRegex == nil {
				t.Error("ValidationRegex is nil")
			}

			// URL can be empty for some providers (like openai-compatible)
			// so we don't validate it
		})
	}
}

// Helper function to generate a string of specified length filled with a character
func generateString(length int, char rune) string {
	result := make([]rune, length)
	for i := range result {
		result[i] = char
	}
	return string(result)
}
