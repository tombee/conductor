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

package docs

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tombee/conductor/internal/commands/shared"
)

func TestDocsCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantErr        bool
		wantJSONOutput bool
	}{
		{
			name:           "docs without flags shows human output",
			args:           []string{},
			wantErr:        false,
			wantJSONOutput: false,
		},
		{
			name:           "docs with --json shows JSON output",
			args:           []string{},
			wantErr:        false,
			wantJSONOutput: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set JSON output mode via shared package
			shared.SetJSONForTest(tt.wantJSONOutput)
			defer shared.SetJSONForTest(false)

			cmd := NewDocsCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			output := buf.String()

			if tt.wantJSONOutput {
				// Verify JSON output structure
				var resp DocsResponse
				decoder := json.NewDecoder(strings.NewReader(output))
				if err := decoder.Decode(&resp); err != nil {
					t.Errorf("Failed to parse JSON output: %v\nOutput: %s", err, output)
					return
				}

				if resp.Version != "1.0" {
					t.Errorf("Expected version 1.0, got %s", resp.Version)
				}
				if resp.Command != "docs" {
					t.Errorf("Expected command 'docs', got %s", resp.Command)
				}
				if !resp.Success {
					t.Errorf("Expected success true, got false")
				}
				if len(resp.Resources) == 0 {
					t.Errorf("Expected resources, got none")
				}

				// Verify all resources have required fields
				for _, r := range resp.Resources {
					if r.Name == "" {
						t.Errorf("Resource missing name")
					}
					if r.Description == "" {
						t.Errorf("Resource missing description")
					}
					if r.URL == "" {
						t.Errorf("Resource missing URL")
					}
					if !strings.HasPrefix(r.URL, "https://") {
						t.Errorf("Resource URL should start with https://, got %s", r.URL)
					}
				}
			} else {
				// Verify human output contains expected content
				if !strings.Contains(output, "Conductor Documentation:") {
					t.Errorf("Expected human output to contain 'Conductor Documentation:'")
				}
				if !strings.Contains(output, "https://") {
					t.Errorf("Expected human output to contain URLs")
				}
			}
		})
	}
}

func TestDocsSubcommands(t *testing.T) {
	subcommands := []struct {
		name        string
		subcommand  string
		expectedURL string
	}{
		{
			name:        "docs cli",
			subcommand:  "cli",
			expectedURL: docsBaseURL + "/reference/cli/",
		},
		{
			name:        "docs schema",
			subcommand:  "schema",
			expectedURL: docsBaseURL + "/reference/schema/",
		},
		{
			name:        "docs config",
			subcommand:  "config",
			expectedURL: docsBaseURL + "/reference/configuration/",
		},
		{
			name:        "docs workflows",
			subcommand:  "workflows",
			expectedURL: docsBaseURL + "/workflows/",
		},
	}

	for _, tt := range subcommands {
		t.Run(tt.name, func(t *testing.T) {
			// Set JSON output mode via shared package
			shared.SetJSONForTest(true)
			defer shared.SetJSONForTest(false)

			cmd := NewDocsCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{tt.subcommand})

			err := cmd.Execute()
			if err != nil {
				t.Errorf("Execute() error = %v", err)
				return
			}

			output := buf.String()

			// Parse JSON output
			var resp DocsResponse
			decoder := json.NewDecoder(strings.NewReader(output))
			if err := decoder.Decode(&resp); err != nil {
				t.Errorf("Failed to parse JSON output: %v\nOutput: %s", err, output)
				return
			}

			if len(resp.Resources) != 1 {
				t.Errorf("Expected 1 resource, got %d", len(resp.Resources))
				return
			}

			if resp.Resources[0].URL != tt.expectedURL {
				t.Errorf("Expected URL %s, got %s", tt.expectedURL, resp.Resources[0].URL)
			}

			if resp.Command != "docs "+tt.subcommand {
				t.Errorf("Expected command 'docs %s', got %s", tt.subcommand, resp.Command)
			}
		})
	}
}

func TestDocsJSONStructure(t *testing.T) {
	// Set JSON output mode via shared package
	shared.SetJSONForTest(true)
	defer shared.SetJSONForTest(false)

	cmd := NewDocsCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()

	// Verify it's valid JSON
	var resp DocsResponse
	decoder := json.NewDecoder(strings.NewReader(output))
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify all expected fields are present
	if resp.Version == "" {
		t.Errorf("Missing @version field")
	}
	if resp.Command == "" {
		t.Errorf("Missing command field")
	}
	if resp.Resources == nil {
		t.Errorf("Missing resources field")
	}

	// Verify resources array structure
	for i, r := range resp.Resources {
		if r.Name == "" {
			t.Errorf("Resource %d missing name", i)
		}
		if r.Description == "" {
			t.Errorf("Resource %d missing description", i)
		}
		if r.URL == "" {
			t.Errorf("Resource %d missing URL", i)
		}
	}
}
