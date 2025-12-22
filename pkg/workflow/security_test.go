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

func TestCheckShellInjectionRisk(t *testing.T) {
	tests := []struct {
		name          string
		def           *Definition
		wantWarnings  int
		wantStepID    string
		wantType      string
	}{
		{
			name: "string command with template variable",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:               "run_cmd",
						Type:             StepTypeBuiltin,
						BuiltinConnector: "shell",
						BuiltinOperation: "run",
						Inputs: map[string]interface{}{
							"command": "git commit -m '{{.inputs.message}}'",
						},
					},
				},
			},
			wantWarnings: 1,
			wantStepID:   "run_cmd",
			wantType:     "shell_injection",
		},
		{
			name: "array command is safe",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:               "run_cmd",
						Type:             StepTypeBuiltin,
						BuiltinConnector: "shell",
						BuiltinOperation: "run",
						Inputs: map[string]interface{}{
							"command": []interface{}{"git", "commit", "-m", "{{.inputs.message}}"},
						},
					},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "string command without variables is safe",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:               "run_cmd",
						Type:             StepTypeBuiltin,
						BuiltinConnector: "shell",
						BuiltinOperation: "run",
						Inputs: map[string]interface{}{
							"command": "git status",
						},
					},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "nested step with shell injection",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:   "parallel_work",
						Type: StepTypeParallel,
						Steps: []StepDefinition{
							{
								ID:               "run_cmd",
								Type:             StepTypeBuiltin,
								BuiltinConnector: "shell",
								BuiltinOperation: "run",
								Inputs: map[string]interface{}{
									"command": "echo {{.item}}",
								},
							},
						},
					},
				},
			},
			wantWarnings: 1,
			wantStepID:   "run_cmd",
			wantType:     "shell_injection",
		},
		{
			name: "non-shell step is ignored",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:               "read_file",
						Type:             StepTypeBuiltin,
						BuiltinConnector: "file",
						BuiltinOperation: "read",
						Inputs: map[string]interface{}{
							"path": "{{.inputs.path}}",
						},
					},
				},
			},
			wantWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSecurity(tt.def)

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
			}

			if tt.wantWarnings > 0 && len(result.Warnings) > 0 {
				warning := result.Warnings[0]
				if warning.StepID != tt.wantStepID {
					t.Errorf("warning StepID = %q, want %q", warning.StepID, tt.wantStepID)
				}
				if warning.Type != tt.wantType {
					t.Errorf("warning Type = %q, want %q", warning.Type, tt.wantType)
				}
				if warning.Severity != "warning" {
					t.Errorf("warning Severity = %q, want %q", warning.Severity, "warning")
				}
				if warning.Suggestion == "" {
					t.Error("warning Suggestion is empty")
				}
			}
		})
	}
}

func TestDetectPlaintextCredentials(t *testing.T) {
	tests := []struct {
		name         string
		def          *Definition
		wantErrors   int
		wantStepID   string
		wantType     string
		wantSeverity string
	}{
		{
			name: "GitHub token plaintext",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"github": {
						Auth: &AuthDefinition{
							Token: "ghp_1234567890abcdefghijklmnopqrstuvwxyz",
						},
					},
				},
			},
			wantErrors:   1,
			wantStepID:   "connectors.github",
			wantType:     "plaintext_credential",
			wantSeverity: "error",
		},
		{
			name: "OpenAI key plaintext",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"openai": {
						Auth: &AuthDefinition{
							Token: "sk-1234567890abcdefghijklmnopqrstuvwxyz",
						},
					},
				},
			},
			wantErrors:   1,
			wantStepID:   "connectors.openai",
			wantType:     "plaintext_credential",
			wantSeverity: "error",
		},
		{
			name: "Anthropic key plaintext",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"anthropic": {
						Auth: &AuthDefinition{
							Token: "sk-ant-api03-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
						},
					},
				},
			},
			wantErrors:   1,
			wantStepID:   "connectors.anthropic",
			wantType:     "plaintext_credential",
			wantSeverity: "error",
		},
		{
			name: "Slack bot token plaintext",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"slack": {
						Auth: &AuthDefinition{
							Token: "xoxb-fake-test-token-for-unit-tests",
						},
					},
				},
			},
			wantErrors:   1,
			wantStepID:   "connectors.slack",
			wantType:     "plaintext_credential",
			wantSeverity: "error",
		},
		{
			name: "environment variable reference is safe",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"github": {
						Auth: &AuthDefinition{
							Token: "${GITHUB_TOKEN}",
						},
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "secret backend reference is safe",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"github": {
						Auth: &AuthDefinition{
							Token: "$secret:github_token",
						},
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "password field with plaintext",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"database": {
						Auth: &AuthDefinition{
							Password: "supersecret123",
						},
					},
				},
			},
			wantErrors:   1,
			wantStepID:   "connectors.database",
			wantType:     "plaintext_credential",
			wantSeverity: "error",
		},
		{
			name: "client_secret field with plaintext",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"oauth_app": {
						Auth: &AuthDefinition{
							ClientSecret: "secret123",
						},
					},
				},
			},
			wantErrors:   1,
			wantStepID:   "connectors.oauth_app",
			wantType:     "plaintext_credential",
			wantSeverity: "error",
		},
		{
			name: "no auth definition",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"public_api": {},
				},
			},
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSecurity(tt.def)

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d", len(result.Errors), tt.wantErrors)
				for _, err := range result.Errors {
					t.Logf("  error: %s - %s", err.StepID, err.Message)
				}
			}

			if tt.wantErrors > 0 && len(result.Errors) > 0 {
				error := result.Errors[0]
				if error.StepID != tt.wantStepID {
					t.Errorf("error StepID = %q, want %q", error.StepID, tt.wantStepID)
				}
				if error.Type != tt.wantType {
					t.Errorf("error Type = %q, want %q", error.Type, tt.wantType)
				}
				if error.Severity != tt.wantSeverity {
					t.Errorf("error Severity = %q, want %q", error.Severity, tt.wantSeverity)
				}
				if error.Suggestion == "" {
					t.Error("error Suggestion is empty")
				}
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
			name:  "environment variable",
			value: "${GITHUB_TOKEN}",
			want:  true,
		},
		{
			name:  "secret backend",
			value: "$secret:github_token",
			want:  true,
		},
		{
			name:  "plaintext value",
			value: "ghp_1234567890",
			want:  false,
		},
		{
			name:  "partial env var syntax",
			value: "${INCOMPLETE",
			want:  false,
		},
		{
			name:  "empty string",
			value: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSecretReference(tt.value)
			if got != tt.want {
				t.Errorf("isSecretReference(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestCheckOverlyPermissivePaths(t *testing.T) {
	tests := []struct {
		name         string
		def          *Definition
		wantWarnings int
		wantStepID   string
		wantType     string
	}{
		{
			name: "root directory path",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:               "read_all",
						Type:             StepTypeBuiltin,
						BuiltinConnector: "file",
						BuiltinOperation: "read",
						Inputs: map[string]interface{}{
							"path": "/",
						},
					},
				},
			},
			wantWarnings: 1,
			wantStepID:   "read_all",
			wantType:     "overly_permissive_path",
		},
		{
			name: "home directory without subdirectory",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:               "read_home",
						Type:             StepTypeBuiltin,
						BuiltinConnector: "file",
						BuiltinOperation: "read",
						Inputs: map[string]interface{}{
							"path": "~",
						},
					},
				},
			},
			wantWarnings: 1,
			wantStepID:   "read_home",
			wantType:     "overly_permissive_path",
		},
		{
			name: "output directory without subdirectory",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:               "read_out",
						Type:             StepTypeBuiltin,
						BuiltinConnector: "file",
						BuiltinOperation: "write",
						Inputs: map[string]interface{}{
							"path": "$out/",
						},
					},
				},
			},
			wantWarnings: 1,
			wantStepID:   "read_out",
			wantType:     "overly_permissive_path",
		},
		{
			name: "specific path is safe",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:               "read_config",
						Type:             StepTypeBuiltin,
						BuiltinConnector: "file",
						BuiltinOperation: "read",
						Inputs: map[string]interface{}{
							"path": "~/projects/myapp/config.json",
						},
					},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "non-file step is ignored",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:               "run_shell",
						Type:             StepTypeBuiltin,
						BuiltinConnector: "shell",
						BuiltinOperation: "run",
						Inputs: map[string]interface{}{
							"command": "ls /",
						},
					},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "nested step with permissive path",
			def: &Definition{
				Steps: []StepDefinition{
					{
						ID:   "parallel_work",
						Type: StepTypeParallel,
						Steps: []StepDefinition{
							{
								ID:               "read_root",
								Type:             StepTypeBuiltin,
								BuiltinConnector: "file",
								BuiltinOperation: "list",
								Inputs: map[string]interface{}{
									"path": "/",
								},
							},
						},
					},
				},
			},
			wantWarnings: 1,
			wantStepID:   "read_root",
			wantType:     "overly_permissive_path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSecurity(tt.def)

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
				for _, w := range result.Warnings {
					t.Logf("  warning: %s - %s", w.StepID, w.Message)
				}
			}

			if tt.wantWarnings > 0 && len(result.Warnings) > 0 {
				warning := result.Warnings[0]
				if warning.StepID != tt.wantStepID {
					t.Errorf("warning StepID = %q, want %q", warning.StepID, tt.wantStepID)
				}
				if warning.Type != tt.wantType {
					t.Errorf("warning Type = %q, want %q", warning.Type, tt.wantType)
				}
				if warning.Severity != "warning" {
					t.Errorf("warning Severity = %q, want %q", warning.Severity, "warning")
				}
			}
		})
	}
}

func TestIsOverlyPermissivePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "root directory", path: "/", want: true},
		{name: "home directory", path: "~", want: true},
		{name: "home directory with slash", path: "~/", want: true},
		{name: "out directory", path: "$out", want: true},
		{name: "out directory with slash", path: "$out/", want: true},
		{name: "temp directory", path: "$temp", want: true},
		{name: "temp directory with slash", path: "$temp/", want: true},
		{name: "specific path", path: "/etc/config.json", want: false},
		{name: "home subdirectory", path: "~/projects", want: false},
		{name: "out subdirectory", path: "$out/results", want: false},
		{name: "relative path", path: "./config.json", want: false},
		{name: "empty path", path: "", want: false},
		{name: "whitespace only", path: "  ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOverlyPermissivePath(tt.path)
			if got != tt.want {
				t.Errorf("isOverlyPermissivePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestCheckMissingAuth(t *testing.T) {
	tests := []struct {
		name         string
		def          *Definition
		wantWarnings int
		wantStepID   string
		wantType     string
	}{
		{
			name: "external connector without auth",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"github": {
						BaseURL: "https://api.github.com",
					},
				},
			},
			wantWarnings: 1,
			wantStepID:   "connectors.github",
			wantType:     "missing_auth",
		},
		{
			name: "external connector with auth",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"github": {
						BaseURL: "https://api.github.com",
						Auth: &AuthDefinition{
							Token: "${GITHUB_TOKEN}",
						},
					},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "builtin connector without auth is okay",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"file": {
						BaseURL: "file://",
					},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "multiple connectors, one missing auth",
			def: &Definition{
				Connectors: map[string]ConnectorDefinition{
					"github": {
						BaseURL: "https://api.github.com",
						Auth: &AuthDefinition{
							Token: "${GITHUB_TOKEN}",
						},
					},
					"gitlab": {
						BaseURL: "https://gitlab.com/api/v4",
					},
				},
			},
			wantWarnings: 1,
			wantStepID:   "connectors.gitlab",
			wantType:     "missing_auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSecurity(tt.def)

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
				for _, w := range result.Warnings {
					t.Logf("  warning: %s - %s", w.StepID, w.Message)
				}
			}

			if tt.wantWarnings > 0 && len(result.Warnings) > 0 {
				warning := result.Warnings[0]
				if warning.StepID != tt.wantStepID {
					t.Errorf("warning StepID = %q, want %q", warning.StepID, tt.wantStepID)
				}
				if warning.Type != tt.wantType {
					t.Errorf("warning Type = %q, want %q", warning.Type, tt.wantType)
				}
				if warning.Severity != "warning" {
					t.Errorf("warning Severity = %q, want %q", warning.Severity, "warning")
				}
			}
		})
	}
}

func TestValidateSecurity_Integration(t *testing.T) {
	// Test multiple security issues in one workflow
	def := &Definition{
		Connectors: map[string]ConnectorDefinition{
			"github": {
				BaseURL: "https://api.github.com",
				Auth: &AuthDefinition{
					Token: "ghp_1234567890abcdefghijklmnopqrstuvwxyz",
				},
			},
			"slack": {
				BaseURL: "https://slack.com/api",
				// Missing auth - should warn
			},
		},
		Steps: []StepDefinition{
			{
				ID:               "run_dangerous",
				Type:             StepTypeBuiltin,
				BuiltinConnector: "shell",
				BuiltinOperation: "run",
				Inputs: map[string]interface{}{
					"command": "rm -rf {{.inputs.path}}",
				},
			},
			{
				ID:               "read_root",
				Type:             StepTypeBuiltin,
				BuiltinConnector: "file",
				BuiltinOperation: "list",
				Inputs: map[string]interface{}{
					"path": "/",
				},
			},
		},
	}

	result := ValidateSecurity(def)

	// Should have 1 error (plaintext credential)
	if len(result.Errors) != 1 {
		t.Errorf("got %d errors, want 1", len(result.Errors))
		for _, e := range result.Errors {
			t.Logf("  error: %s - %s", e.StepID, e.Message)
		}
	}

	// Should have 3 warnings (shell injection, overly permissive path, missing auth)
	if len(result.Warnings) != 3 {
		t.Errorf("got %d warnings, want 3", len(result.Warnings))
		for _, w := range result.Warnings {
			t.Logf("  warning: %s - %s", w.StepID, w.Message)
		}
	}

	// Verify we have the expected warning types
	warningTypes := make(map[string]bool)
	for _, w := range result.Warnings {
		warningTypes[w.Type] = true
	}

	expectedTypes := []string{"shell_injection", "overly_permissive_path", "missing_auth"}
	for _, expectedType := range expectedTypes {
		if !warningTypes[expectedType] {
			t.Errorf("missing expected warning type: %s", expectedType)
		}
	}
}
