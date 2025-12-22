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

package binding

import (
	"context"
	"testing"

	"github.com/tombee/conductor/internal/secrets"
	"github.com/tombee/conductor/pkg/profile"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestResolver_ResolveIntegrationRequirements(t *testing.T) {
	tests := []struct {
		name        string
		profile     *profile.Profile
		workflow    *workflow.Definition
		wantErr     bool
		wantSource  BindingSource
		wantBinding bool
	}{
		{
			name: "profile binding takes precedence",
			profile: &profile.Profile{
				Name: "test",
				Bindings: profile.Bindings{
					Integrations: map[string]profile.IntegrationBinding{
						"github": {
							Auth: profile.AuthBinding{
								Token: "env:PROFILE_TOKEN",
							},
						},
					},
				},
			},
			workflow: &workflow.Definition{
				Name: "test-workflow",
				Requires: &workflow.RequirementsDefinition{
					Integrations: []string{
						"github",
					},
				},
				Integrations: map[string]workflow.IntegrationDefinition{
					"github": {
						Auth: &workflow.AuthDefinition{
							Token: "env:INLINE_TOKEN",
						},
					},
				},
			},
			wantErr:     false,
			wantSource:  SourceProfile,
			wantBinding: true,
		},
		{
			name: "inline binding used when no profile",
			profile: &profile.Profile{
				Name: "test",
			},
			workflow: &workflow.Definition{
				Name: "test-workflow",
				Requires: &workflow.RequirementsDefinition{
					Integrations: []string{
						"github",
					},
				},
				Integrations: map[string]workflow.IntegrationDefinition{
					"github": {
						Auth: &workflow.AuthDefinition{
							Token: "env:INLINE_TOKEN",
						},
					},
				},
			},
			wantErr:     false,
			wantSource:  SourceInline,
			wantBinding: true,
		},
		{
			name: "missing required binding fails",
			profile: &profile.Profile{
				Name: "test",
			},
			workflow: &workflow.Definition{
				Name: "test-workflow",
				Requires: &workflow.RequirementsDefinition{
					Integrations: []string{
						"github",
					},
				},
			},
			wantErr:     true,
			wantBinding: false,
		},
		{
			name: "optional binding missing is ok",
			profile: &profile.Profile{
				Name: "test",
			},
			workflow: &workflow.Definition{
				Name: "test-workflow",
				Requires: &workflow.RequirementsDefinition{
					Integrations: []string{},
				},
			},
			wantErr:     false,
			wantBinding: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up secret registry with test env provider
			registry := secrets.NewRegistry()
			envProvider := secrets.NewTestEnvProvider(map[string]string{
				"PROFILE_TOKEN": "profile-token-value",
				"INLINE_TOKEN":  "inline-token-value",
			})
			if err := registry.Register(envProvider); err != nil {
				t.Fatalf("failed to register env provider: %v", err)
			}

			// Register plain provider for non-secret values
			plainProvider := secrets.NewPlainProvider()
			if err := registry.Register(plainProvider); err != nil {
				t.Fatalf("failed to register plain provider: %v", err)
			}

			inheritEnv := profile.InheritEnvConfig{Enabled: true}
			resolver := NewResolver(registry, inheritEnv)

			resCtx := &ResolutionContext{
				Profile:   tt.profile,
				Workflow:  tt.workflow,
				RunID:     "test-run",
				Workspace: "default",
			}

			resolved, err := resolver.Resolve(context.Background(), resCtx)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantBinding {
				binding, exists := resolved.IntegrationBindings["github"]
				if !exists {
					t.Error("expected github integration binding but got none")
					return
				}

				if binding.Source != tt.wantSource {
					t.Errorf("expected source %q but got %q", tt.wantSource, binding.Source)
				}

				// Verify token was resolved
				if binding.Auth.Token == "" {
					t.Error("expected resolved token but got empty string")
				}
			} else {
				if _, exists := resolved.IntegrationBindings["github"]; exists {
					t.Error("expected no github binding but got one")
				}
			}
		})
	}
}

func TestResolver_ResolveMCPServerRequirements(t *testing.T) {
	tests := []struct {
		name        string
		profile     *profile.Profile
		workflow    *workflow.Definition
		wantErr     bool
		wantSource  BindingSource
		wantBinding bool
	}{
		{
			name: "profile binding takes precedence",
			profile: &profile.Profile{
				Name: "test",
				Bindings: profile.Bindings{
					MCPServers: map[string]profile.MCPServerBinding{
						"analyzer": {
							Command: "npx",
							Args:    []string{"-y", "@acme/analyzer"},
							Env: map[string]string{
								"API_KEY": "env:PROFILE_KEY",
							},
						},
					},
				},
			},
			workflow: &workflow.Definition{
				Name: "test-workflow",
				Requires: &workflow.RequirementsDefinition{
					MCPServers: []workflow.MCPServerRequirement{
						{Name: "analyzer"},
					},
				},
			},
			wantErr:     false,
			wantSource:  SourceProfile,
			wantBinding: true,
		},
		{
			name: "inline binding used when no profile",
			profile: &profile.Profile{
				Name: "test",
			},
			workflow: &workflow.Definition{
				Name: "test-workflow",
				Requires: &workflow.RequirementsDefinition{
					MCPServers: []workflow.MCPServerRequirement{
						{Name: "analyzer"},
					},
				},
				MCPServers: []workflow.MCPServerConfig{
					{
						Name:    "analyzer",
						Command: "python",
						Args:    []string{"-m", "analyzer"},
						Env: []string{
							"API_KEY=env:INLINE_KEY",
						},
					},
				},
			},
			wantErr:     false,
			wantSource:  SourceInline,
			wantBinding: true,
		},
		{
			name: "missing required MCP server fails",
			profile: &profile.Profile{
				Name: "test",
			},
			workflow: &workflow.Definition{
				Name: "test-workflow",
				Requires: &workflow.RequirementsDefinition{
					MCPServers: []workflow.MCPServerRequirement{
						{Name: "analyzer"},
					},
				},
			},
			wantErr:     true,
			wantBinding: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up secret registry with test env provider
			registry := secrets.NewRegistry()
			envProvider := secrets.NewTestEnvProvider(map[string]string{
				"PROFILE_KEY": "profile-key-value",
				"INLINE_KEY":  "inline-key-value",
			})
			if err := registry.Register(envProvider); err != nil {
				t.Fatalf("failed to register env provider: %v", err)
			}

			// Register plain provider for non-secret values
			plainProvider := secrets.NewPlainProvider()
			if err := registry.Register(plainProvider); err != nil {
				t.Fatalf("failed to register plain provider: %v", err)
			}

			inheritEnv := profile.InheritEnvConfig{Enabled: true}
			resolver := NewResolver(registry, inheritEnv)

			resCtx := &ResolutionContext{
				Profile:   tt.profile,
				Workflow:  tt.workflow,
				RunID:     "test-run",
				Workspace: "default",
			}

			resolved, err := resolver.Resolve(context.Background(), resCtx)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantBinding {
				binding, exists := resolved.MCPServerBindings["analyzer"]
				if !exists {
					t.Error("expected analyzer MCP server binding but got none")
					return
				}

				if binding.Source != tt.wantSource {
					t.Errorf("expected source %q but got %q", tt.wantSource, binding.Source)
				}

				// Verify env variable was resolved
				if apiKey, exists := binding.Env["API_KEY"]; !exists || apiKey == "" {
					t.Error("expected resolved API_KEY but got empty or missing")
				}
			} else {
				if _, exists := resolved.MCPServerBindings["analyzer"]; exists {
					t.Error("expected no analyzer binding but got one")
				}
			}
		})
	}
}

func TestResolver_BackwardCompatibility(t *testing.T) {
	// Workflow with no requires section should use inline definitions
	workflow := &workflow.Definition{
		Name: "legacy-workflow",
		Integrations: map[string]workflow.IntegrationDefinition{
			"github": {
				Auth: &workflow.AuthDefinition{
					Token: "env:GITHUB_TOKEN",
				},
			},
		},
		MCPServers: []workflow.MCPServerConfig{
			{
				Name:    "analyzer",
				Command: "python",
				Args:    []string{"-m", "analyzer"},
				Env: []string{
					"API_KEY=env:API_KEY",
				},
			},
		},
	}

	testProfile := &profile.Profile{
		Name: "default",
	}

	// Set up secret registry with test env provider
	registry := secrets.NewRegistry()
	envProvider := secrets.NewTestEnvProvider(map[string]string{
		"GITHUB_TOKEN": "github-token-value",
		"API_KEY":      "api-key-value",
	})
	if err := registry.Register(envProvider); err != nil {
		t.Fatalf("failed to register env provider: %v", err)
	}

	// Register plain provider for non-secret values
	plainProvider := secrets.NewPlainProvider()
	if err := registry.Register(plainProvider); err != nil {
		t.Fatalf("failed to register plain provider: %v", err)
	}

	inheritEnv := profile.InheritEnvConfig{Enabled: true}
	resolver := NewResolver(registry, inheritEnv)

	resCtx := &ResolutionContext{
		Profile:   testProfile,
		Workflow:  workflow,
		RunID:     "test-run",
		Workspace: "default",
	}

	resolved, err := resolver.Resolve(context.Background(), resCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify integration was resolved
	githubBinding, exists := resolved.IntegrationBindings["github"]
	if !exists {
		t.Error("expected github integration binding")
	}
	if githubBinding.Source != SourceInline {
		t.Errorf("expected source %q but got %q", SourceInline, githubBinding.Source)
	}
	if githubBinding.Auth.Token != "github-token-value" {
		t.Errorf("expected token %q but got %q", "github-token-value", githubBinding.Auth.Token)
	}

	// Verify MCP server was resolved
	analyzerBinding, exists := resolved.MCPServerBindings["analyzer"]
	if !exists {
		t.Error("expected analyzer MCP server binding")
	}
	if analyzerBinding.Source != SourceInline {
		t.Errorf("expected source %q but got %q", SourceInline, analyzerBinding.Source)
	}
	if apiKey := analyzerBinding.Env["API_KEY"]; apiKey != "api-key-value" {
		t.Errorf("expected API_KEY %q but got %q", "api-key-value", apiKey)
	}
}

func TestIsSecretReference(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"${GITHUB_TOKEN}", true},
		{"env:API_KEY", true},
		{"file:/etc/secrets/token", true},
		{"vault:secret/data/prod", true},
		{"plain-text-value", false},
		{"http://example.com", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := isSecretReference(tt.value)
			if got != tt.want {
				t.Errorf("isSecretReference(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
