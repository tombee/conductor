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
	"testing"
)

func TestRequirementsDefinition_Validate(t *testing.T) {
	tests := []struct {
		name      string
		requires  RequirementsDefinition
		wantError bool
	}{
		{
			name: "valid with connectors and MCP servers",
			requires: RequirementsDefinition{
				Integrations: []IntegrationRequirement{
					{Name: "github", Capabilities: []string{"issues", "pull_requests"}},
					{Name: "slack", Optional: true},
				},
				MCPServers: []MCPServerRequirement{
					{Name: "code-analysis"},
				},
			},
			wantError: false,
		},
		{
			name: "empty connector name",
			requires: RequirementsDefinition{
				Integrations: []IntegrationRequirement{
					{Name: ""},
				},
			},
			wantError: true,
		},
		{
			name: "duplicate connector names",
			requires: RequirementsDefinition{
				Integrations: []IntegrationRequirement{
					{Name: "github"},
					{Name: "github"},
				},
			},
			wantError: true,
		},
		{
			name: "empty MCP server name",
			requires: RequirementsDefinition{
				MCPServers: []MCPServerRequirement{
					{Name: ""},
				},
			},
			wantError: true,
		},
		{
			name: "duplicate MCP server names",
			requires: RequirementsDefinition{
				MCPServers: []MCPServerRequirement{
					{Name: "analyzer"},
					{Name: "analyzer"},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.requires.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("RequirementsDefinition.Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestParseDefinition_WithRequires(t *testing.T) {
	yaml := `
name: test-workflow
version: "1.0"

requires:
  integrations:
    - name: github
      capabilities: [issues, pull_requests]
    - name: slack
      optional: true
  mcp_servers:
    - name: code-analysis

steps:
  - id: step1
    type: llm
    prompt: "test"
`

	def, err := ParseDefinition([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseDefinition() error = %v", err)
	}

	if def.Requires == nil {
		t.Fatal("Requires should not be nil")
	}

	if len(def.Requires.Integrations) != 2 {
		t.Errorf("Expected 2 integration requirements, got %d", len(def.Requires.Integrations))
	}

	if def.Requires.Integrations[0].Name != "github" {
		t.Errorf("Expected connector name 'github', got %q", def.Requires.Integrations[0].Name)
	}

	if len(def.Requires.Integrations[0].Capabilities) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(def.Requires.Integrations[0].Capabilities))
	}

	if !def.Requires.Integrations[1].Optional {
		t.Error("Second connector should be optional")
	}

	if len(def.Requires.MCPServers) != 1 {
		t.Errorf("Expected 1 MCP server requirement, got %d", len(def.Requires.MCPServers))
	}

	if def.Requires.MCPServers[0].Name != "code-analysis" {
		t.Errorf("Expected MCP server name 'code-analysis', got %q", def.Requires.MCPServers[0].Name)
	}
}
