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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tombee/conductor/pkg/profile"
)

func TestDetectPlaintextCredentials(t *testing.T) {
	tests := []struct {
		name         string
		profile      profile.Profile
		wantWarnings int
		wantPatterns []string
	}{
		{
			name: "GitHub token in integration",
			profile: profile.Profile{
				Name: "test",
				Bindings: profile.Bindings{
					Integrations: map[string]profile.IntegrationBinding{
						"github": {
							Auth: profile.AuthBinding{
								Token: "ghp_FAKEtestTOKENnotREAL000000000000000000",
							},
						},
					},
				},
			},
			wantWarnings: 1,
			wantPatterns: []string{"GitHub Token"},
		},
		{
			name: "Anthropic API key in integration",
			profile: profile.Profile{
				Name: "test",
				Bindings: profile.Bindings{
					Integrations: map[string]profile.IntegrationBinding{
						"api": {
							Auth: profile.AuthBinding{
								Token: "sk-ant-api03-FAKEtestKEYnotREAL000000000000000000000000000000000000000000000000000000000000000000000000",
							},
						},
					},
				},
			},
			wantWarnings: 1,
			wantPatterns: []string{"Anthropic API Key"},
		},
		{
			name: "AWS access key in password",
			profile: profile.Profile{
				Name: "test",
				Bindings: profile.Bindings{
					Integrations: map[string]profile.IntegrationBinding{
						"aws": {
							Auth: profile.AuthBinding{
								Username: "admin",
								Password: "AKIAIOSFODNN7EXAMPLE",
							},
						},
					},
				},
			},
			wantWarnings: 1,
			wantPatterns: []string{"AWS Access Key"},
		},
		{
			name: "Slack token in MCP server env",
			profile: profile.Profile{
				Name: "test",
				Bindings: profile.Bindings{
					MCPServers: map[string]profile.MCPServerBinding{
						"slack-bot": {
							Command: "node",
							Env: map[string]string{
								"SLACK_TOKEN": "xoxb-1234567890-1234567890-NOTrealTESTtoken12345678",
							},
						},
					},
				},
			},
			wantWarnings: 1,
			wantPatterns: []string{"Slack Token"},
		},
		{
			name: "secret reference not flagged",
			profile: profile.Profile{
				Name: "test",
				Bindings: profile.Bindings{
					Integrations: map[string]profile.IntegrationBinding{
						"github": {
							Auth: profile.AuthBinding{
								Token: "${GITHUB_TOKEN}",
							},
						},
					},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "env: reference not flagged",
			profile: profile.Profile{
				Name: "test",
				Bindings: profile.Bindings{
					Integrations: map[string]profile.IntegrationBinding{
						"api": {
							Auth: profile.AuthBinding{
								Token: "env:API_TOKEN",
							},
						},
					},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "file: reference not flagged",
			profile: profile.Profile{
				Name: "test",
				Bindings: profile.Bindings{
					Integrations: map[string]profile.IntegrationBinding{
						"api": {
							Auth: profile.AuthBinding{
								Token: "file:/etc/secrets/token",
							},
						},
					},
				},
			},
			wantWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := detectPlaintextCredentials("test.profile", tt.profile)
			assert.Len(t, warnings, tt.wantWarnings)

			for _, pattern := range tt.wantPatterns {
				found := false
				for _, warning := range warnings {
					if assert.Contains(t, warning, pattern) {
						found = true
						break
					}
				}
				if !found && len(tt.wantPatterns) > 0 {
					t.Errorf("expected warning containing %q, got warnings: %v", pattern, warnings)
				}
			}
		})
	}
}

func TestValidateAllowlistPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "valid simple pattern",
			pattern: "CONDUCTOR_TOKEN",
			wantErr: false,
		},
		{
			name:    "valid wildcard pattern",
			pattern: "CONDUCTOR_*",
			wantErr: false,
		},
		{
			name:    "valid pattern with numbers",
			pattern: "VAR_123",
			wantErr: false,
		},
		{
			name:    "empty pattern",
			pattern: "",
			wantErr: true,
		},
		{
			name:    "lowercase not allowed",
			pattern: "conductor_token",
			wantErr: true,
		},
		{
			name:    "wildcard in middle",
			pattern: "CONDUCTOR_*_TOKEN",
			wantErr: true,
		},
		{
			name:    "multiple wildcards",
			pattern: "CONDUCTOR_**",
			wantErr: true,
		},
		{
			name:    "special characters not allowed",
			pattern: "CONDUCTOR-TOKEN",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAllowlistPattern(tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateIntegrationBinding(t *testing.T) {
	tests := []struct {
		name    string
		binding profile.IntegrationBinding
		wantErr bool
	}{
		{
			name: "valid token auth",
			binding: profile.IntegrationBinding{
				Auth: profile.AuthBinding{
					Token: "${TOKEN}",
				},
			},
			wantErr: false,
		},
		{
			name: "valid basic auth",
			binding: profile.IntegrationBinding{
				Auth: profile.AuthBinding{
					Username: "user",
					Password: "${PASSWORD}",
				},
			},
			wantErr: false,
		},
		{
			name: "valid header auth",
			binding: profile.IntegrationBinding{
				Auth: profile.AuthBinding{
					Header: "X-API-Key",
					Value:  "${API_KEY}",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - username without password",
			binding: profile.IntegrationBinding{
				Auth: profile.AuthBinding{
					Username: "user",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - password without username",
			binding: profile.IntegrationBinding{
				Auth: profile.AuthBinding{
					Password: "${PASSWORD}",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - header without value",
			binding: profile.IntegrationBinding{
				Auth: profile.AuthBinding{
					Header: "X-API-Key",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - value without header",
			binding: profile.IntegrationBinding{
				Auth: profile.AuthBinding{
					Value: "${API_KEY}",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - mixed auth methods",
			binding: profile.IntegrationBinding{
				Auth: profile.AuthBinding{
					Token:    "${TOKEN}",
					Username: "user",
					Password: "${PASSWORD}",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIntegrationBinding("test.path", tt.binding)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMCPServerBinding(t *testing.T) {
	tests := []struct {
		name    string
		binding profile.MCPServerBinding
		wantErr bool
	}{
		{
			name: "valid binding",
			binding: profile.MCPServerBinding{
				Command: "npx",
				Args:    []string{"-y", "server"},
				Timeout: 30,
			},
			wantErr: false,
		},
		{
			name: "valid binding without timeout",
			binding: profile.MCPServerBinding{
				Command: "node",
				Args:    []string{"server.js"},
			},
			wantErr: false,
		},
		{
			name: "invalid - missing command",
			binding: profile.MCPServerBinding{
				Args: []string{"server.js"},
			},
			wantErr: true,
		},
		{
			name: "invalid - negative timeout",
			binding: profile.MCPServerBinding{
				Command: "node",
				Timeout: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMCPServerBinding("test.path", tt.binding)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateProfiles(t *testing.T) {
	tests := []struct {
		name         string
		workspaces   map[string]Workspace
		wantErrs     int
		wantWarnings int
		wantErrMsg   string
	}{
		{
			name: "valid workspace with profiles",
			workspaces: map[string]Workspace{
				"default": {
					Name: "default",
					Profiles: map[string]profile.Profile{
						"default": {
							Name: "default",
							InheritEnv: profile.InheritEnvConfig{
								Enabled: true,
							},
						},
					},
					DefaultProfile: "default",
				},
			},
			wantErrs:     0,
			wantWarnings: 0,
		},
		{
			name: "workspace with invalid default profile",
			workspaces: map[string]Workspace{
				"test": {
					Name: "test",
					Profiles: map[string]profile.Profile{
						"dev": {
							Name: "dev",
						},
					},
					DefaultProfile: "nonexistent",
				},
			},
			wantErrs:   1,
			wantErrMsg: "default_profile \"nonexistent\" not found",
		},
		{
			name: "profile with invalid name",
			workspaces: map[string]Workspace{
				"default": {
					Profiles: map[string]profile.Profile{
						"INVALID-NAME": {
							Name: "INVALID-NAME",
						},
					},
				},
			},
			wantErrs:   1,
			wantErrMsg: "profile name must contain only lowercase",
		},
		{
			name: "profile with plaintext credentials",
			workspaces: map[string]Workspace{
				"default": {
					Profiles: map[string]profile.Profile{
						"test": {
							Name: "test",
							Bindings: profile.Bindings{
								Integrations: map[string]profile.IntegrationBinding{
									"github": {
										Auth: profile.AuthBinding{
											Token: "ghp_FAKEtestTOKENnotREAL000000000000000000",
										},
									},
								},
							},
						},
					},
				},
			},
			wantErrs:     0,
			wantWarnings: 1,
		},
		{
			name: "profile with invalid allowlist pattern",
			workspaces: map[string]Workspace{
				"default": {
					Profiles: map[string]profile.Profile{
						"test": {
							Name: "test",
							InheritEnv: profile.InheritEnvConfig{
								Enabled:   true,
								Allowlist: []string{"VALID_*", "invalid-pattern"},
							},
						},
					},
				},
			},
			wantErrs:   1,
			wantErrMsg: "invalid pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs, warnings, err := ValidateProfiles(tt.workspaces)

			if tt.wantErrs > 0 {
				require.Error(t, err)
				assert.Len(t, errs, tt.wantErrs)
				if tt.wantErrMsg != "" {
					assert.Contains(t, errs[0], tt.wantErrMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Empty(t, errs)
			}

			if tt.wantWarnings > 0 {
				assert.Len(t, warnings, tt.wantWarnings)
			}
		})
	}
}

func TestIsSecretReference(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "environment variable syntax",
			value: "${GITHUB_TOKEN}",
			want:  true,
		},
		{
			name:  "env: scheme",
			value: "env:API_KEY",
			want:  true,
		},
		{
			name:  "file: scheme",
			value: "file:/etc/secrets/token",
			want:  true,
		},
		{
			name:  "vault: scheme",
			value: "vault:secret/data/github#token",
			want:  true,
		},
		{
			name:  "1password: scheme",
			value: "1password:vault/item/field",
			want:  true,
		},
		{
			name:  "aws-secrets: scheme",
			value: "aws-secrets:conductor/github-token",
			want:  true,
		},
		{
			name:  "ref: scheme",
			value: "ref:vault/frontend/github",
			want:  true,
		},
		{
			name:  "plaintext value",
			value: "ghp_FAKEtestTOKENnotREAL000000000000000000",
			want:  false,
		},
		{
			name:  "URL is not secret reference",
			value: "https://api.example.com",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSecretReference(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}
