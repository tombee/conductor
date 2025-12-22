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
	"strings"
	"testing"

	"github.com/tombee/conductor/internal/config"
)

func TestValidateProviderName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		existing  config.ProvidersMap
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid simple name",
			input:     "my-provider",
			expectErr: false,
		},
		{
			name:      "valid with underscore",
			input:     "my_provider",
			expectErr: false,
		},
		{
			name:      "valid starts with underscore",
			input:     "_provider",
			expectErr: false,
		},
		{
			name:      "valid alphanumeric",
			input:     "provider123",
			expectErr: false,
		},
		{
			name:      "empty name",
			input:     "",
			expectErr: true,
			errMsg:    "cannot be empty",
		},
		{
			name:      "too long",
			input:     strings.Repeat("a", 65),
			expectErr: true,
			errMsg:    "too long",
		},
		{
			name:      "starts with number",
			input:     "123provider",
			expectErr: true,
			errMsg:    "must start with",
		},
		{
			name:      "contains special chars",
			input:     "my@provider",
			expectErr: true,
			errMsg:    "must start with",
		},
		{
			name:      "reserved name add",
			input:     "add",
			expectErr: true,
			errMsg:    "reserved name",
		},
		{
			name:      "reserved name list",
			input:     "list",
			expectErr: true,
			errMsg:    "reserved name",
		},
		{
			name:      "reserved name test",
			input:     "test",
			expectErr: true,
			errMsg:    "reserved name",
		},
		{
			name:  "already exists",
			input: "existing",
			existing: config.ProvidersMap{
				"existing": config.ProviderConfig{Type: "anthropic"},
			},
			expectErr: true,
			errMsg:    "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProviderName(tt.input, tt.existing)

			if tt.expectErr && err == nil {
				t.Errorf("expected error for input %q, got nil", tt.input)
			}

			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
			}

			if tt.expectErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			}
		})
	}
}

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid api key",
			input:     "sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
			expectErr: false,
		},
		{
			name:      "empty key",
			input:     "",
			expectErr: true,
			errMsg:    "cannot be empty",
		},
		{
			name:      "too short",
			input:     "short",
			expectErr: true,
			errMsg:    "too short",
		},
		{
			name:      "placeholder XXX",
			input:     "XXX-placeholder-key",
			expectErr: true,
			errMsg:    "placeholder",
		},
		{
			name:      "placeholder test",
			input:     "sk-test-1234567890",
			expectErr: true,
			errMsg:    "placeholder",
		},
		{
			name:      "placeholder DUMMY",
			input:     "DUMMY-api-key-here",
			expectErr: true,
			errMsg:    "placeholder",
		},
		{
			name:      "maximum length api key (8192 chars)",
			input:     strings.Repeat("a", 8192),
			expectErr: false,
		},
		{
			name:      "exceeds maximum length (8193 chars)",
			input:     strings.Repeat("a", 8193),
			expectErr: true,
			errMsg:    "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPIKey(tt.input)

			if tt.expectErr && err == nil {
				t.Errorf("expected error for input %q, got nil", tt.input)
			}

			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
			}

			if tt.expectErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			}
		})
	}
}

func TestValidateEnvVarName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "valid uppercase",
			input:     "ANTHROPIC_API_KEY",
			expectErr: false,
		},
		{
			name:      "valid lowercase",
			input:     "api_key",
			expectErr: false,
		},
		{
			name:      "valid with numbers",
			input:     "API_KEY_2",
			expectErr: false,
		},
		{
			name:      "starts with underscore",
			input:     "_API_KEY",
			expectErr: false,
		},
		{
			name:      "empty",
			input:     "",
			expectErr: true,
		},
		{
			name:      "starts with number",
			input:     "2API_KEY",
			expectErr: true,
		},
		{
			name:      "contains dash",
			input:     "API-KEY",
			expectErr: true,
		},
		{
			name:      "contains space",
			input:     "API KEY",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvVarName(tt.input)

			if tt.expectErr && err == nil {
				t.Errorf("expected error for input %q, got nil", tt.input)
			}

			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
			}
		})
	}
}

func TestValidateProviderNameFunc(t *testing.T) {
	existing := config.ProvidersMap{
		"existing": config.ProviderConfig{Type: "anthropic"},
	}

	validateFunc := ValidateProviderNameFunc(existing)

	// Test that it returns a valid function
	if validateFunc == nil {
		t.Fatal("ValidateProviderNameFunc returned nil")
	}

	// Test valid name
	if err := validateFunc("new-provider"); err != nil {
		t.Errorf("unexpected error for valid name: %v", err)
	}

	// Test existing name
	if err := validateFunc("existing"); err == nil {
		t.Error("expected error for existing name, got nil")
	}
}
