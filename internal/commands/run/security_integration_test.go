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

package run

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSecurityOptionsIntegration verifies that security options are properly
// passed through the execution chain and validated.
func TestSecurityOptionsIntegration(t *testing.T) {
	tests := []struct {
		name        string
		opts        SecurityOptions
		wantErr     bool
		errContains string
	}{
		{
			name: "valid standard profile",
			opts: SecurityOptions{
				Mode: "standard",
			},
			wantErr: false,
		},
		{
			name: "valid standard profile with additional hosts",
			opts: SecurityOptions{
				Mode:       "standard",
				AllowHosts: []string{"api.example.com", "*.cdn.example.com"},
			},
			wantErr: false,
		},
		{
			name: "valid standard profile with additional paths",
			opts: SecurityOptions{
				Mode:       "standard",
				AllowPaths: []string{"/tmp/output", "./workspace"},
			},
			wantErr: false,
		},
		{
			name: "invalid security profile",
			opts: SecurityOptions{
				Mode: "nonexistent",
			},
			wantErr:     true,
			errContains: "invalid security profile",
		},
		{
			name: "invalid host with protocol",
			opts: SecurityOptions{
				Mode:       "standard",
				AllowHosts: []string{"https://api.example.com"},
			},
			wantErr:     true,
			errContains: "should not include protocol",
		},
		{
			name: "invalid empty path",
			opts: SecurityOptions{
				Mode:       "standard",
				AllowPaths: []string{""},
			},
			wantErr:     true,
			errContains: "path cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build security profile
			profile, err := buildSecurityProfile(tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if tt.errContains != "" {
					if !contains(err.Error(), tt.errContains) {
						t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
					}
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// For non-empty mode, verify profile is created
			if tt.opts.Mode != "" && profile == nil {
				t.Error("expected security profile to be created, got nil")
			}

			// For empty mode, verify no profile is created
			if tt.opts.Mode == "" && profile != nil {
				t.Error("expected nil security profile for empty mode, got profile")
			}

			// Verify additional hosts were merged
			if len(tt.opts.AllowHosts) > 0 && profile != nil {
				for _, host := range tt.opts.AllowHosts {
					found := false
					for _, allowed := range profile.Network.Allow {
						if allowed == host {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected host %q to be in allowed list, got %v", host, profile.Network.Allow)
					}
				}
			}

			// Verify additional paths were merged
			if len(tt.opts.AllowPaths) > 0 && profile != nil {
				for _, path := range tt.opts.AllowPaths {
					found := false
					for _, allowed := range profile.Filesystem.Read {
						if allowed == path {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected path %q to be in allowed read list, got %v", path, profile.Filesystem.Read)
					}
				}
			}
		})
	}
}

// TestSecurityFlagParsing tests that command flags are properly parsed.
func TestSecurityFlagParsing(t *testing.T) {
	// Create a temporary workflow file
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "test.yaml")

	workflow := `name: test
description: Test workflow
version: "1.0"

steps:
  - id: step1
    type: llm
    prompt: "test"
    inputs:
      model: fast
`

	if err := os.WriteFile(workflowPath, []byte(workflow), 0644); err != nil {
		t.Fatalf("failed to create test workflow: %v", err)
	}

	cmd := NewCommand()

	// Test that security flags are registered
	securityFlag := cmd.Flags().Lookup("security")
	if securityFlag == nil {
		t.Error("--security flag not registered")
	}

	allowHostsFlag := cmd.Flags().Lookup("allow-hosts")
	if allowHostsFlag == nil {
		t.Error("--allow-hosts flag not registered")
	}

	allowPathsFlag := cmd.Flags().Lookup("allow-paths")
	if allowPathsFlag == nil {
		t.Error("--allow-paths flag not registered")
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
