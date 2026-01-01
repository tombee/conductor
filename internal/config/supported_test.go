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
	"os"
	"testing"
)

func TestIsSupportedProvider(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		want         bool
	}{
		{
			name:         "claude-code is supported",
			providerType: "claude-code",
			want:         true,
		},
		{
			name:         "anthropic is not supported",
			providerType: "anthropic",
			want:         false,
		},
		{
			name:         "openai is not supported",
			providerType: "openai",
			want:         false,
		},
		{
			name:         "ollama is not supported",
			providerType: "ollama",
			want:         false,
		},
		{
			name:         "unknown provider is not supported",
			providerType: "unknown",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSupportedProvider(tt.providerType)
			if got != tt.want {
				t.Errorf("IsSupportedProvider(%q) = %v, want %v", tt.providerType, got, tt.want)
			}
		})
	}
}

func TestAllProvidersEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "enabled when CONDUCTOR_ALL_PROVIDERS=1",
			envValue: "1",
			want:     true,
		},
		{
			name:     "disabled when CONDUCTOR_ALL_PROVIDERS is empty",
			envValue: "",
			want:     false,
		},
		{
			name:     "disabled when CONDUCTOR_ALL_PROVIDERS=0",
			envValue: "0",
			want:     false,
		},
		{
			name:     "disabled when CONDUCTOR_ALL_PROVIDERS=true",
			envValue: "true",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("CONDUCTOR_ALL_PROVIDERS", tt.envValue)
			} else {
				os.Unsetenv("CONDUCTOR_ALL_PROVIDERS")
			}
			defer os.Unsetenv("CONDUCTOR_ALL_PROVIDERS")

			got := AllProvidersEnabled()
			if got != tt.want {
				t.Errorf("AllProvidersEnabled() = %v, want %v (env=%q)", got, tt.want, tt.envValue)
			}
		})
	}
}

func TestGetVisibleProviderTypes(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     []string
	}{
		{
			name:     "returns only supported types by default",
			envValue: "",
			want:     []string{"claude-code"},
		},
		{
			name:     "returns all types when CONDUCTOR_ALL_PROVIDERS=1",
			envValue: "1",
			want:     []string{"claude-code", "anthropic", "openai", "ollama"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("CONDUCTOR_ALL_PROVIDERS", tt.envValue)
			} else {
				os.Unsetenv("CONDUCTOR_ALL_PROVIDERS")
			}
			defer os.Unsetenv("CONDUCTOR_ALL_PROVIDERS")

			got := GetVisibleProviderTypes()
			if len(got) != len(tt.want) {
				t.Errorf("GetVisibleProviderTypes() returned %d types, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if i >= len(got) || got[i] != want {
					t.Errorf("GetVisibleProviderTypes()[%d] = %v, want %v", i, got, tt.want)
					break
				}
			}
		})
	}
}

func TestWarnUnsupportedProvider(t *testing.T) {
	// This is a simple test to ensure the function doesn't panic
	// Testing stderr output would require capturing stderr which is complex
	tests := []struct {
		name         string
		providerType string
	}{
		{
			name:         "warn for anthropic",
			providerType: "anthropic",
		},
		{
			name:         "no warn for claude-code",
			providerType: "claude-code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just ensure it doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("WarnUnsupportedProvider(%q) panicked: %v", tt.providerType, r)
				}
			}()
			WarnUnsupportedProvider(tt.providerType)
		})
	}
}
