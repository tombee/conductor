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

package secrets

import (
	"errors"
	"testing"

	"github.com/tombee/conductor/pkg/profile"
)

func TestValidateSecretReference(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		wantError bool
		category  profile.ErrorCategory
	}{
		{
			name:      "${VAR} syntax is no longer supported",
			reference: "${GITHUB_TOKEN}",
			wantError: true,
			category:  profile.ErrorCategoryInvalidSyntax,
		},
		{
			name:      "valid env: syntax",
			reference: "env:API_KEY",
			wantError: false,
		},
		{
			name:      "valid file: syntax",
			reference: "file:/etc/secrets/token",
			wantError: false,
		},
		{
			name:      "valid vault: syntax (future)",
			reference: "vault:secret/data/prod#token",
			wantError: false,
		},
		{
			name:      "empty reference",
			reference: "",
			wantError: true,
			category:  profile.ErrorCategoryInvalidSyntax,
		},
		{
			name:      "unclosed brace (invalid syntax)",
			reference: "${GITHUB_TOKEN",
			wantError: true,
			category:  profile.ErrorCategoryInvalidSyntax,
		},
		{
			name:      "uppercase scheme",
			reference: "ENV:API_KEY",
			wantError: true,
			category:  profile.ErrorCategoryInvalidSyntax,
		},
		{
			name:      "empty key",
			reference: "env:",
			wantError: true,
			category:  profile.ErrorCategoryInvalidSyntax,
		},
		{
			name:      "invalid scheme format",
			reference: "123:value",
			wantError: true,
			category:  profile.ErrorCategoryInvalidSyntax,
		},
		{
			name:      "plain value (allowed)",
			reference: "plain-text-value",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretReference(tt.reference)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error for reference %q", tt.reference)
				}

				var resErr *profile.SecretResolutionError
				if !errors.As(err, &resErr) {
					t.Fatalf("expected SecretResolutionError, got %T", err)
				}

				if resErr.Category != tt.category {
					t.Errorf("Category = %q, want %q", resErr.Category, tt.category)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for reference %q: %v", tt.reference, err)
				}
			}
		})
	}
}

func TestExtractReferences(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{
			name:  "${VAR} syntax",
			value: "${GITHUB_TOKEN}",
			want:  []string{"GITHUB_TOKEN"},
		},
		{
			name:  "env: syntax",
			value: "env:API_KEY",
			want:  []string{"API_KEY"},
		},
		{
			name:  "file: syntax (no reference extraction)",
			value: "file:/path/to/secret",
			want:  []string{},
		},
		{
			name:  "plain value",
			value: "https://api.example.com",
			want:  []string{},
		},
		{
			name:  "mixed ${VAR} in text",
			value: "token:${TOKEN}",
			want:  []string{"TOKEN"},
		},
		{
			name:  "multiple ${VAR} references",
			value: "${VAR1} and ${VAR2}",
			want:  []string{"VAR1", "VAR2"},
		},
		{
			name:  "no references",
			value: "plain text",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractReferences(tt.value)
			if len(got) != len(tt.want) {
				t.Errorf("extractReferences(%q) = %v, want %v", tt.value, got, tt.want)
				return
			}

			for i, ref := range got {
				if ref != tt.want[i] {
					t.Errorf("extractReferences(%q)[%d] = %q, want %q", tt.value, i, ref, tt.want[i])
				}
			}
		})
	}
}

func TestDetectCircularReferences(t *testing.T) {
	tests := []struct {
		name      string
		bindings  map[string]string
		wantError bool
		errorType string // "circular" or "depth"
	}{
		{
			name: "no circular references",
			bindings: map[string]string{
				"github_token": "env:GITHUB_TOKEN",
				"api_key":      "env:API_KEY",
			},
			wantError: false,
		},
		{
			name: "simple circular reference A->B->A",
			bindings: map[string]string{
				"A": "env:B",
				"B": "env:A",
			},
			wantError: true,
			errorType: "circular",
		},
		{
			name: "self reference A->A",
			bindings: map[string]string{
				"A": "env:A",
			},
			wantError: true,
			errorType: "circular",
		},
		{
			name: "three-way circular A->B->C->A",
			bindings: map[string]string{
				"A": "env:B",
				"B": "env:C",
				"C": "env:A",
			},
			wantError: true,
			errorType: "circular",
		},
		{
			name: "valid chain",
			bindings: map[string]string{
				"A": "env:B",
				"B": "env:C",
				"C": "env:D",
				"D": "env:FINAL",
			},
			wantError: false,
		},
		{
			name: "max depth exceeded",
			bindings: map[string]string{
				"A": "env:B",
				"B": "env:C",
				"C": "env:D",
				"D": "env:E",
				"E": "env:F",
				"F": "env:G",
				"G": "env:H",
				"H": "env:I",
				"I": "env:J",
				"J": "env:K",
				"K": "env:L", // Depth 11, exceeds limit of 10
			},
			wantError: true,
			errorType: "depth",
		},
		{
			name: "file references don't create dependencies",
			bindings: map[string]string{
				"A": "file:/path/to/B",
				"B": "file:/path/to/A",
			},
			wantError: false, // file: doesn't create cross-binding dependencies
		},
		{
			name: "plain values don't create dependencies",
			bindings: map[string]string{
				"A": "plain-value-B",
				"B": "plain-value-A",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DetectCircularReferences(tt.bindings)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error for bindings %v", tt.bindings)
				}

				switch tt.errorType {
				case "circular":
					var circErr *profile.CircularReferenceError
					if !errors.As(err, &circErr) {
						t.Errorf("expected CircularReferenceError, got %T: %v", err, err)
					} else {
						t.Logf("Circular reference detected: %v", circErr.Chain)
					}
				case "depth":
					var resErr *profile.SecretResolutionError
					if !errors.As(err, &resErr) {
						t.Errorf("expected SecretResolutionError, got %T: %v", err, err)
					} else if resErr.Category != profile.ErrorCategoryCircularRef {
						t.Errorf("Category = %q, want %q", resErr.Category, profile.ErrorCategoryCircularRef)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for bindings %v: %v", tt.bindings, err)
				}
			}
		})
	}
}

func TestValidateSecretReferences(t *testing.T) {
	tests := []struct {
		name      string
		bindings  map[string]string
		wantError bool
	}{
		{
			name: "all valid references",
			bindings: map[string]string{
				"github_token": "env:GITHUB_TOKEN",
				"api_key":      "env:API_KEY",
				"secret_file":  "file:/etc/secrets/token",
			},
			wantError: false,
		},
		{
			name: "invalid ${VAR} syntax",
			bindings: map[string]string{
				"github_token": "${GITHUB_TOKEN}",
			},
			wantError: true,
		},
		{
			name: "circular reference",
			bindings: map[string]string{
				"A": "env:B",
				"B": "env:A",
			},
			wantError: true,
		},
		{
			name: "mixed valid and plain values",
			bindings: map[string]string{
				"token":    "env:GITHUB_TOKEN",
				"base_url": "https://api.github.com",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretReferences(tt.bindings)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error for bindings %v", tt.bindings)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for bindings %v: %v", tt.bindings, err)
				}
			}
		})
	}
}

func TestIsPlainValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "${VAR} is not plain",
			value: "${VAR}",
			want:  false,
		},
		{
			name:  "env:VAR is not plain",
			value: "env:VAR",
			want:  false,
		},
		{
			name:  "https://example.com is plain (contains :)",
			value: "https://example.com",
			want:  false,
		},
		{
			name:  "plain text is plain",
			value: "plain-text",
			want:  true,
		},
		{
			name:  "number is plain",
			value: "12345",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPlainValue(tt.value)
			if got != tt.want {
				t.Errorf("isPlainValue(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
