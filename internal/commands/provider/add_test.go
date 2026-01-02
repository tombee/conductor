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

package provider

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestAddCommand_InvalidProviderType(t *testing.T) {
	cmd := newAddCmd()

	// Set up command with invalid provider type
	cmd.SetArgs([]string{"test-provider", "--type", "invalid-type"})

	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for invalid provider type, got nil")
	}

	expectedMsg := "unsupported provider type"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected error containing %q, got: %v", expectedMsg, err)
	}
}

func TestAddCommand_MissingRequiredFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name:        "missing type flag",
			args:        []string{"test-provider"},
			expectedErr: "interactive TUI mode not yet implemented",
		},
		{
			name:        "missing provider name with type flag",
			args:        []string{"--type", "anthropic"},
			expectedErr: "provider name is required when using --type flag",
		},
		{
			name:        "missing api key for anthropic",
			args:        []string{"test-provider", "--type", "anthropic"},
			expectedErr: "API key is required for anthropic provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newAddCmd()
			cmd.SetArgs(tt.args)

			var stderr bytes.Buffer
			cmd.SetErr(&stderr)

			err := cmd.Execute()

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("expected error containing %q, got: %v", tt.expectedErr, err)
			}
		})
	}
}

func TestAddCommand_DryRunMode(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectErr   bool
		expectedMsg string
	}{
		{
			name:        "dry run requires type",
			args:        []string{"--dry-run"},
			expectErr:   true,
			expectedMsg: "--type is required in dry-run mode",
		},
		{
			name:        "dry run requires provider name",
			args:        []string{"--type", "ollama", "--dry-run"},
			expectErr:   true,
			expectedMsg: "provider name is required in dry-run mode",
		},
		{
			name:        "valid dry run for ollama",
			args:        []string{"test-ollama", "--type", "ollama", "--dry-run"},
			expectErr:   false,
			expectedMsg: "MODIFY:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newAddCmd()
			cmd.SetArgs(tt.args)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)

			err := cmd.Execute()

			if tt.expectErr && err == nil {
				t.Fatal("expected error, got nil")
			}

			if !tt.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := stdout.String()
			if tt.expectErr {
				output = err.Error()
			}

			if !strings.Contains(output, tt.expectedMsg) {
				t.Errorf("expected output containing %q, got: %s", tt.expectedMsg, output)
			}
		})
	}
}

func TestValidateBaseURL(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		url       string
		expectErr bool
	}{
		{
			name:      "valid https URL",
			url:       "https://api.example.com",
			expectErr: false,
		},
		{
			name:      "invalid scheme",
			url:       "ftp://example.com",
			expectErr: true,
		},
		{
			name:      "missing host",
			url:       "https://",
			expectErr: true,
		},
		{
			name:      "malformed URL",
			url:       "not a url",
			expectErr: true,
		},
		{
			name:      "blocked metadata endpoint",
			url:       "http://169.254.169.254/latest/meta-data/",
			expectErr: true,
		},
		{
			name:      "private network IP",
			url:       "http://192.168.1.1",
			expectErr: true,
		},
		{
			name:      "loopback address",
			url:       "http://127.0.0.1:8080",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBaseURL(ctx, tt.url)

			if tt.expectErr && err == nil {
				t.Errorf("expected error for URL %q, got nil", tt.url)
			}

			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error for URL %q: %v", tt.url, err)
			}
		})
	}
}
