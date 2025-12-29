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

package profile

import (
	"strings"
	"testing"
)

func TestValidateProfileName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{"valid lowercase", "prod", false},
		{"valid with numbers", "team1", false},
		{"valid with underscore", "team_prod", false},
		{"valid with hyphen", "team-prod", false},
		{"valid complex", "frontend-team_1", false},
		{"empty name", "", true},
		{"uppercase", "Prod", true},
		{"contains space", "team prod", true},
		{"contains dot", "team.prod", true},
		{"reserved default", "default", true},
		{"reserved system", "system", true},
		{"too long", strings.Repeat("a", 65), true},
		{"max length ok", strings.Repeat("a", 64), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfileName(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateProfileName(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
			}
		})
	}
}

func TestValidateWorkspaceName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{"valid lowercase", "frontend", false},
		{"valid with numbers", "team1", false},
		{"valid with underscore", "team_frontend", false},
		{"valid with hyphen", "team-frontend", false},
		{"empty name", "", true},
		{"uppercase", "Frontend", true},
		{"reserved default", "default", true},
		{"reserved system", "system", true},
		{"too long", strings.Repeat("a", 65), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkspaceName(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateWorkspaceName(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
			}
		})
	}
}

func TestProfile_Validate(t *testing.T) {
	tests := []struct {
		name      string
		profile   Profile
		wantError bool
	}{
		{
			name: "valid profile",
			profile: Profile{
				Name:        "prod",
				Description: "Production environment",
				Bindings: Bindings{
					Connectors: map[string]IntegrationBinding{
						"github": {
							Auth: AuthBinding{
								Token: "${GITHUB_TOKEN}",
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid profile name",
			profile: Profile{
				Name: "PROD",
			},
			wantError: true,
		},
		{
			name: "empty allowlist pattern",
			profile: Profile{
				Name: "prod",
				InheritEnv: InheritEnvConfig{
					Enabled:   true,
					Allowlist: []string{""},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Profile.Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestAuthBinding_Validate(t *testing.T) {
	tests := []struct {
		name      string
		auth      AuthBinding
		wantError bool
	}{
		{
			name: "valid bearer token",
			auth: AuthBinding{
				Token: "${GITHUB_TOKEN}",
			},
			wantError: false,
		},
		{
			name: "valid basic auth",
			auth: AuthBinding{
				Username: "user",
				Password: "${PASSWORD}",
			},
			wantError: false,
		},
		{
			name: "valid API key",
			auth: AuthBinding{
				Header: "X-API-Key",
				Value:  "${API_KEY}",
			},
			wantError: false,
		},
		{
			name:      "no auth provided",
			auth:      AuthBinding{},
			wantError: true,
		},
		{
			name: "incomplete basic auth - missing password",
			auth: AuthBinding{
				Username: "user",
			},
			wantError: true,
		},
		{
			name: "incomplete API key - missing value",
			auth: AuthBinding{
				Header: "X-API-Key",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.auth.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("AuthBinding.Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestMCPServerBinding_Validate(t *testing.T) {
	tests := []struct {
		name      string
		binding   MCPServerBinding
		wantError bool
	}{
		{
			name: "valid binding",
			binding: MCPServerBinding{
				Command: "npx",
				Args:    []string{"-y", "@acme/analyzer"},
				Timeout: 30,
			},
			wantError: false,
		},
		{
			name: "missing command",
			binding: MCPServerBinding{
				Args: []string{"arg"},
			},
			wantError: true,
		},
		{
			name: "negative timeout",
			binding: MCPServerBinding{
				Command: "npx",
				Timeout: -1,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.binding.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("MCPServerBinding.Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
