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

package secrets

import (
	"context"
	"os"
	"testing"

	"github.com/tombee/conductor/pkg/profile"
)

func TestEnvProvider_Scheme(t *testing.T) {
	provider := NewEnvProvider(profile.InheritEnvConfig{Enabled: true})
	if got := provider.Scheme(); got != "env" {
		t.Errorf("Scheme() = %v, want env", got)
	}
}

func TestEnvProvider_Resolve(t *testing.T) {
	// Set up test environment variables
	testEnvVars := map[string]string{
		"TEST_VAR_1":     "value1",
		"CONDUCTOR_TEST": "value2",
		"GITHUB_TOKEN":   "ghp_test",
	}
	for key, value := range testEnvVars {
		os.Setenv(key, value)
		defer os.Unsetenv(key)
	}

	tests := []struct {
		name       string
		inheritEnv profile.InheritEnvConfig
		reference  string
		want       string
		wantError  bool
	}{
		{
			name: "resolve existing variable with inherit_env enabled",
			inheritEnv: profile.InheritEnvConfig{
				Enabled: true,
			},
			reference: "TEST_VAR_1",
			want:      "value1",
			wantError: false,
		},
		{
			name: "resolve with allowlist match",
			inheritEnv: profile.InheritEnvConfig{
				Enabled:   true,
				Allowlist: []string{"CONDUCTOR_*"},
			},
			reference: "CONDUCTOR_TEST",
			want:      "value2",
			wantError: false,
		},
		{
			name: "reject variable not in allowlist",
			inheritEnv: profile.InheritEnvConfig{
				Enabled:   true,
				Allowlist: []string{"CONDUCTOR_*"},
			},
			reference: "TEST_VAR_1",
			wantError: true,
		},
		{
			name: "reject when inherit_env disabled",
			inheritEnv: profile.InheritEnvConfig{
				Enabled: false,
			},
			reference: "TEST_VAR_1",
			wantError: true,
		},
		{
			name: "not found variable",
			inheritEnv: profile.InheritEnvConfig{
				Enabled: true,
			},
			reference: "NONEXISTENT_VAR",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewEnvProvider(tt.inheritEnv)
			got, err := provider.Resolve(context.Background(), tt.reference)
			if (err != nil) != tt.wantError {
				t.Errorf("Resolve() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && got != tt.want {
				t.Errorf("Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		value   string
		pattern string
		want    bool
	}{
		{"FOO", "FOO", true},
		{"FOO_BAR", "FOO_*", true},
		{"FOO_BAR_BAZ", "FOO_*", true},
		{"API_KEY", "*_KEY", true},
		{"SECRET_KEY", "*_KEY", true},
		{"FOO", "BAR", false},
		{"FOO_BAR", "BAZ_*", false},
		{"FOO", "*_KEY", false},
		{"CONDUCTOR_TEST", "CONDUCTOR_*", true},
		{"GITHUB_TOKEN", "GITHUB_*", true},
		{"GITHUB_TOKEN", "*_TOKEN", true},
	}

	for _, tt := range tests {
		t.Run(tt.value+"_vs_"+tt.pattern, func(t *testing.T) {
			if got := matchesPattern(tt.value, tt.pattern); got != tt.want {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.value, tt.pattern, got, tt.want)
			}
		})
	}
}
