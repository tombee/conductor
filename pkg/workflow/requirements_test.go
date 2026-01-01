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
			name: "valid simple integrations",
			requires: RequirementsDefinition{
				Integrations: []string{"github", "slack"},
				MCPServers: []MCPServerRequirement{
					{Name: "code-analysis"},
				},
			},
			wantError: false,
		},
		{
			name: "valid aliased integrations",
			requires: RequirementsDefinition{
				Integrations: []string{"github as source", "github as target"},
			},
			wantError: false,
		},
		{
			name: "valid mixed simple and aliased",
			requires: RequirementsDefinition{
				Integrations: []string{"slack", "github as source"},
			},
			wantError: false,
		},
		{
			name: "empty integration string",
			requires: RequirementsDefinition{
				Integrations: []string{""},
			},
			wantError: true,
		},
		{
			name: "duplicate simple integration types",
			requires: RequirementsDefinition{
				Integrations: []string{"github", "github"},
			},
			wantError: true,
		},
		{
			name: "duplicate aliases",
			requires: RequirementsDefinition{
				Integrations: []string{"github as source", "slack as source"},
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

func TestParseIntegrationRequirement(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  string
		wantAlias string
	}{
		{
			name:      "simple requirement",
			input:     "github",
			wantType:  "github",
			wantAlias: "",
		},
		{
			name:      "aliased requirement",
			input:     "github as source",
			wantType:  "github",
			wantAlias: "source",
		},
		{
			name:      "aliased with extra spaces",
			input:     "  github   as   source  ",
			wantType:  "github",
			wantAlias: "source",
		},
		{
			name:      "simple with spaces",
			input:     "  slack  ",
			wantType:  "slack",
			wantAlias: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseIntegrationRequirement(tt.input)
			if parsed.Type != tt.wantType {
				t.Errorf("ParseIntegrationRequirement() Type = %q, want %q", parsed.Type, tt.wantType)
			}
			if parsed.Alias != tt.wantAlias {
				t.Errorf("ParseIntegrationRequirement() Alias = %q, want %q", parsed.Alias, tt.wantAlias)
			}
		})
	}
}

func TestParseDefinition_WithRequires(t *testing.T) {
	tests := []struct {
		name             string
		yaml             string
		wantIntegrations int
		wantMCPServers   int
	}{
		{
			name: "simple integrations",
			yaml: `
name: test-workflow
version: "1.0"

requires:
  integrations:
    - github
    - slack
  mcp_servers:
    - name: code-analysis

steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantIntegrations: 2,
			wantMCPServers:   1,
		},
		{
			name: "aliased integrations",
			yaml: `
name: test-workflow
version: "1.0"

requires:
  integrations:
    - github as source
    - github as target

steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantIntegrations: 2,
			wantMCPServers:   0,
		},
		{
			name: "mixed simple and aliased",
			yaml: `
name: test-workflow
version: "1.0"

requires:
  integrations:
    - slack
    - github as source

steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantIntegrations: 2,
			wantMCPServers:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("ParseDefinition() error = %v", err)
			}

			if def.Requires == nil {
				t.Fatal("Requires should not be nil")
			}

			if len(def.Requires.Integrations) != tt.wantIntegrations {
				t.Errorf("Expected %d integration requirements, got %d", tt.wantIntegrations, len(def.Requires.Integrations))
			}

			if len(def.Requires.MCPServers) != tt.wantMCPServers {
				t.Errorf("Expected %d MCP server requirements, got %d", tt.wantMCPServers, len(def.Requires.MCPServers))
			}

			// Validate the parsed requirements
			if err := def.Requires.Validate(); err != nil {
				t.Errorf("Requires.Validate() error = %v", err)
			}
		})
	}
}
