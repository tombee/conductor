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
)

func TestMakeHyperlink(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "console.anthropic.com",
			url:  "console.anthropic.com/settings/keys",
		},
		{
			name: "platform.openai.com",
			url:  "platform.openai.com/api-keys",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeHyperlink(tt.url)

			// Should contain the URL with https://
			if !strings.Contains(result, "https://"+tt.url) {
				t.Errorf("makeHyperlink() missing expected URL: got %q", result)
			}

			// Should not be empty
			if result == "" {
				t.Error("makeHyperlink() returned empty string")
			}
		})
	}
}

func TestGetStorageBackendDisplay(t *testing.T) {
	tests := []struct {
		name     string
		backend  string
		expected string
	}{
		{
			name:     "keychain",
			backend:  "keychain",
			expected: "Will store in: macOS Keychain",
		},
		{
			name:     "env",
			backend:  "env",
			expected: "Will store in: .env file",
		},
		{
			name:     "custom backend",
			backend:  "vault",
			expected: "Will store in: vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStorageBackendDisplay(tt.backend)
			if result != tt.expected {
				t.Errorf("GetStorageBackendDisplay(%q) = %q, want %q",
					tt.backend, result, tt.expected)
			}
		})
	}
}
