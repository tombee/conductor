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

package workflow

import (
	"strings"
	"testing"
)

func TestDetectEmbeddedCredentials(t *testing.T) {
	tests := []struct {
		name     string
		def      *Definition
		wantWarn bool
		contains string
	}{
		{
			name: "no credentials",
			def: &Definition{
				Name: "test",
				Integrations: map[string]IntegrationDefinition{
					"github": {
						Name: "github",
						Auth: &AuthDefinition{
							Token: "${GITHUB_TOKEN}", // Template, not plaintext
						},
					},
				},
			},
			wantWarn: false,
		},
		{
			name: "GitHub token in integration",
			def: &Definition{
				Name: "test",
				Integrations: map[string]IntegrationDefinition{
					"github": {
						Name: "github",
						Auth: &AuthDefinition{
							Token: "ghp_FAKEtestTOKENnotREAL000000000000000000",
						},
					},
				},
			},
			wantWarn: true,
			contains: "GitHub Token",
		},
		{
			name: "Anthropic API key in integration",
			def: &Definition{
				Name: "test",
				Integrations: map[string]IntegrationDefinition{
					"anthropic": {
						Name: "anthropic",
						Auth: &AuthDefinition{
							Value: "sk-ant-api03-FAKEtestKEYnotREAL000000000000000000000000000000000000000000000000000000000000000000000000000000",
						},
					},
				},
			},
			wantWarn: true,
			contains: "Anthropic API Key",
		},
		{
			name: "AWS access key in integration",
			def: &Definition{
				Name: "test",
				Integrations: map[string]IntegrationDefinition{
					"aws": {
						Name: "aws",
						Auth: &AuthDefinition{
							Username: "AKIAIOSFODNN7EXAMPLE",
						},
					},
				},
			},
			wantWarn: true,
			contains: "AWS Access Key",
		},
		{
			name: "Slack token in MCP server env",
			def: &Definition{
				Name: "test",
				MCPServers: []MCPServerConfig{
					{
						Name:    "slack-server",
						Command: "slack-mcp",
						Env: []string{
							"SLACK_TOKEN=xoxb-1234567890-1234567890-NOTrealTESTtoken12345678",
						},
					},
				},
			},
			wantWarn: true,
			contains: "Slack Token",
		},
		{
			name: "multiple credentials",
			def: &Definition{
				Name: "test",
				Integrations: map[string]IntegrationDefinition{
					"github": {
						Name: "github",
						Auth: &AuthDefinition{
							Token: "ghp_FAKEtestTOKENnotREAL000000000000000000",
						},
					},
					"slack": {
						Name: "slack",
						Auth: &AuthDefinition{
							Token: "xoxb-1234567890-1234567890-NOTrealTESTtoken12345678",
						},
					},
				},
			},
			wantWarn: true,
			contains: "GitHub Token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := DetectEmbeddedCredentials(tt.def)

			if tt.wantWarn && len(warnings) == 0 {
				t.Errorf("Expected warnings, got none")
			}

			if !tt.wantWarn && len(warnings) > 0 {
				t.Errorf("Expected no warnings, got: %v", warnings)
			}

			if tt.contains != "" {
				found := false
				for _, w := range warnings {
					if strings.Contains(w, tt.contains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q, got warnings: %v", tt.contains, warnings)
				}
			}
		})
	}
}

func TestRequirementsDefinitionValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     *RequirementsDefinition
		wantErr bool
	}{
		{
			name: "valid requirements",
			req: &RequirementsDefinition{
				Integrations: []IntegrationRequirement{
					{Name: "github", Capabilities: []string{"issues", "pull_requests"}},
					{Name: "slack", Optional: true},
				},
				MCPServers: []MCPServerRequirement{
					{Name: "code-analysis"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing integration name",
			req: &RequirementsDefinition{
				Integrations: []IntegrationRequirement{
					{Name: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate integration",
			req: &RequirementsDefinition{
				Integrations: []IntegrationRequirement{
					{Name: "github"},
					{Name: "github"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing MCP server name",
			req: &RequirementsDefinition{
				MCPServers: []MCPServerRequirement{
					{Name: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate MCP server",
			req: &RequirementsDefinition{
				MCPServers: []MCPServerRequirement{
					{Name: "code-analysis"},
					{Name: "code-analysis"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
