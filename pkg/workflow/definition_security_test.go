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

	"gopkg.in/yaml.v3"
)

func TestDefinition_SecurityParsing(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(*testing.T, *Definition)
	}{
		{
			name: "empty security section",
			yaml: `
name: test-workflow
security:
  filesystem:
  network:
  shell:
steps:
  - id: step1
    action: shell.run
    with:
      command: echo "hello"
`,
			validate: func(t *testing.T, def *Definition) {
				if def.Security == nil {
					t.Fatal("expected Security to be non-nil")
				}
			},
		},
		{
			name: "filesystem read/write patterns",
			yaml: `
name: test-workflow
security:
  filesystem:
    read:
      - "./src/**"
      - "./docs/**"
    write:
      - "./output/**"
    deny:
      - "**/.env"
      - "**/*.key"
steps:
  - id: step1
    action: shell.run
    with:
      command: echo "hello"
`,
			validate: func(t *testing.T, def *Definition) {
				if def.Security == nil {
					t.Fatal("expected Security to be non-nil")
				}
				if len(def.Security.Filesystem.Read) != 2 {
					t.Errorf("expected 2 read patterns, got %d", len(def.Security.Filesystem.Read))
				}
				if len(def.Security.Filesystem.Write) != 1 {
					t.Errorf("expected 1 write pattern, got %d", len(def.Security.Filesystem.Write))
				}
				if len(def.Security.Filesystem.Deny) != 2 {
					t.Errorf("expected 2 deny patterns, got %d", len(def.Security.Filesystem.Deny))
				}
			},
		},
		{
			name: "network allow/deny patterns",
			yaml: `
name: test-workflow
security:
  network:
    allow:
      - "api.github.com"
      - "api.anthropic.com"
      - "*.slack.com"
    deny:
      - "10.0.0.0/8"
      - "192.168.0.0/16"
steps:
  - id: step1
    action: http.get
    with:
      url: "https://api.github.com/users/test"
`,
			validate: func(t *testing.T, def *Definition) {
				if def.Security == nil {
					t.Fatal("expected Security to be non-nil")
				}
				if len(def.Security.Network.Allow) != 3 {
					t.Errorf("expected 3 allow patterns, got %d", len(def.Security.Network.Allow))
				}
				if len(def.Security.Network.Deny) != 2 {
					t.Errorf("expected 2 deny patterns, got %d", len(def.Security.Network.Deny))
				}
			},
		},
		{
			name: "shell commands and deny patterns",
			yaml: `
name: test-workflow
security:
  shell:
    commands:
      - "git"
      - "go"
      - "npm"
    deny_patterns:
      - "git push --force"
      - "rm -rf /"
steps:
  - id: step1
    action: shell.run
    with:
      command: "git status"
`,
			validate: func(t *testing.T, def *Definition) {
				if def.Security == nil {
					t.Fatal("expected Security to be non-nil")
				}
				if len(def.Security.Shell.Commands) != 3 {
					t.Errorf("expected 3 commands, got %d", len(def.Security.Shell.Commands))
				}
				if len(def.Security.Shell.DenyPatterns) != 2 {
					t.Errorf("expected 2 deny patterns, got %d", len(def.Security.Shell.DenyPatterns))
				}
			},
		},
		{
			name: "complete security configuration",
			yaml: `
name: code-review
description: Review code changes
security:
  filesystem:
    read:
      - "./src/**"
      - "./docs/**"
    write:
      - "./output/**"
    deny:
      - "**/.env"
  network:
    allow:
      - "api.github.com"
      - "api.anthropic.com"
  shell:
    commands:
      - "git"
      - "go"
    deny_patterns:
      - "git push --force"
steps:
  - id: analyze
    action: shell.run
    with:
      command: "git diff"
`,
			validate: func(t *testing.T, def *Definition) {
				if def.Security == nil {
					t.Fatal("expected Security to be non-nil")
				}

				// Validate filesystem
				if len(def.Security.Filesystem.Read) != 2 {
					t.Errorf("filesystem.read: expected 2, got %d", len(def.Security.Filesystem.Read))
				}
				if len(def.Security.Filesystem.Write) != 1 {
					t.Errorf("filesystem.write: expected 1, got %d", len(def.Security.Filesystem.Write))
				}
				if len(def.Security.Filesystem.Deny) != 1 {
					t.Errorf("filesystem.deny: expected 1, got %d", len(def.Security.Filesystem.Deny))
				}

				// Validate network
				if len(def.Security.Network.Allow) != 2 {
					t.Errorf("network.allow: expected 2, got %d", len(def.Security.Network.Allow))
				}

				// Validate shell
				if len(def.Security.Shell.Commands) != 2 {
					t.Errorf("shell.commands: expected 2, got %d", len(def.Security.Shell.Commands))
				}
				if len(def.Security.Shell.DenyPatterns) != 1 {
					t.Errorf("shell.deny_patterns: expected 1, got %d", len(def.Security.Shell.DenyPatterns))
				}
			},
		},
		{
			name: "workflow without security section",
			yaml: `
name: simple-workflow
steps:
  - id: step1
    action: shell.run
    with:
      command: echo "hello"
`,
			validate: func(t *testing.T, def *Definition) {
				if def.Security != nil {
					t.Error("expected Security to be nil when not specified")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var def Definition
			if err := yaml.Unmarshal([]byte(tt.yaml), &def); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			tt.validate(t, &def)
		})
	}
}

func TestDefinition_SecurityValidation(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		expectError bool
	}{
		{
			name: "valid security configuration",
			yaml: `
name: test-workflow
security:
  filesystem:
    read: ["./src/**"]
  network:
    allow: ["api.github.com"]
  shell:
    commands: ["git"]
steps:
  - id: step1
    type: llm
    model: balanced
    prompt: "Say hello"
`,
			expectError: false,
		},
		{
			name: "empty security is valid",
			yaml: `
name: test-workflow
security:
steps:
  - id: step1
    type: llm
    model: balanced
    prompt: "Say hello"
`,
			expectError: false,
		},
		{
			name: "missing security is valid",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    model: balanced
    prompt: "Say hello"
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var def Definition
			if err := yaml.Unmarshal([]byte(tt.yaml), &def); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			err := def.Validate()
			if tt.expectError && err == nil {
				t.Error("expected validation error, got nil")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}
