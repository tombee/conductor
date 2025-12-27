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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePublicAPIRequirements(t *testing.T) {
	tests := []struct {
		name          string
		publicEnabled bool
		workflows     map[string]string
		wantErr       bool
		errContains   string
	}{
		{
			name:          "public API enabled - no validation error",
			publicEnabled: true,
			workflows: map[string]string{
				"webhook.yaml": `
name: webhook-workflow
listen:
  webhook:
    path: /test
    secret: test
steps:
  - id: step1
    type: llm
    prompt: test
`,
			},
			wantErr: false,
		},
		{
			name:          "no workflows dir - no validation error",
			publicEnabled: false,
			workflows:     nil,
			wantErr:       false,
		},
		{
			name:          "public API disabled with webhook listener",
			publicEnabled: false,
			workflows: map[string]string{
				"webhook.yaml": `
name: webhook-workflow
listen:
  webhook:
    path: /test
    secret: test
steps:
  - id: step1
    type: llm
    prompt: test
`,
			},
			wantErr:     true,
			errContains: "webhook-workflow (has listen.webhook)",
		},
		{
			name:          "public API disabled with API listener",
			publicEnabled: false,
			workflows: map[string]string{
				"api.yaml": `
name: api-workflow
listen:
  api:
    secret: test-secret-123
steps:
  - id: step1
    type: llm
    prompt: test
`,
			},
			wantErr:     true,
			errContains: "api-workflow (has listen.api)",
		},
		{
			name:          "public API disabled with both listeners",
			publicEnabled: false,
			workflows: map[string]string{
				"both.yaml": `
name: both-workflow
listen:
  webhook:
    path: /test
    secret: test
  api:
    secret: test-secret-123
steps:
  - id: step1
    type: llm
    prompt: test
`,
			},
			wantErr:     true,
			errContains: "both-workflow",
		},
		{
			name:          "public API disabled with schedule only - no error",
			publicEnabled: false,
			workflows: map[string]string{
				"schedule.yaml": `
name: schedule-workflow
listen:
  schedule:
    cron: "0 * * * *"
steps:
  - id: step1
    type: llm
    prompt: test
`,
			},
			wantErr: false,
		},
		{
			name:          "public API disabled with no listeners - no error",
			publicEnabled: false,
			workflows: map[string]string{
				"manual.yaml": `
name: manual-workflow
steps:
  - id: step1
    type: llm
    prompt: test
`,
			},
			wantErr: false,
		},
		{
			name:          "invalid workflow - skipped",
			publicEnabled: false,
			workflows: map[string]string{
				"invalid.yaml": `
this is not valid yaml: [
`,
			},
			wantErr: false,
		},
		{
			name:          "multiple workflows requiring public API",
			publicEnabled: false,
			workflows: map[string]string{
				"webhook1.yaml": `
name: webhook-1
listen:
  webhook:
    path: /test1
steps:
  - id: step1
    type: llm
    prompt: test
`,
				"webhook2.yaml": `
name: webhook-2
listen:
  webhook:
    path: /test2
steps:
  - id: step1
    type: llm
    prompt: test
`,
			},
			wantErr:     true,
			errContains: "webhook-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := ""
			if tt.workflows != nil {
				tmpDir = t.TempDir()
				for filename, content := range tt.workflows {
					path := filepath.Join(tmpDir, filename)
					if err := os.WriteFile(path, []byte(content), 0644); err != nil {
						t.Fatalf("Failed to write test workflow: %v", err)
					}
				}
			}

			cfg := &Config{
				Daemon: DaemonConfig{
					WorkflowsDir: tmpDir,
					Listen: DaemonListenConfig{
						PublicAPI: PublicAPIConfig{
							Enabled: tt.publicEnabled,
						},
					},
				},
			}

			err := ValidatePublicAPIRequirements(cfg)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Error should contain %q, got: %v", tt.errContains, err)
				}
			}
		})
	}
}
