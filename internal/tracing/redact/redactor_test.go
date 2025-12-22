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

package redact

import (
	"fmt"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestRedactor_RedactString_StandardMode(t *testing.T) {
	r := NewRedactor(ModeStandard)

	tests := []struct {
		name     string
		input    string
		contains string // What should remain or be redacted
		notContains string // What should be removed
	}{
		{
			name:        "API key",
			input:       `api_key="sk-1234567890abcdef"`,
			contains:    "api_key=[REDACTED]",
			notContains: "sk-1234567890abcdef",
		},
		{
			name:        "Bearer token",
			input:       "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			contains:    "Bearer [REDACTED]",
			notContains: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:        "Password",
			input:       `password="mysecretpass123"`,
			contains:    "password=[REDACTED]",
			notContains: "mysecretpass123",
		},
		{
			name:        "AWS access key",
			input:       "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			contains:    "[REDACTED-AWS-KEY]",
			notContains: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:        "Email",
			input:       "Contact: user@example.com for support",
			contains:    "[REDACTED-EMAIL]",
			notContains: "user@example.com",
		},
		{
			name:        "SSN",
			input:       "SSN: 123-45-6789",
			contains:    "[REDACTED-SSN]",
			notContains: "123-45-6789",
		},
		{
			name:        "Credit card",
			input:       "Card: 4532-1234-5678-9010",
			contains:    "[REDACTED-CC]",
			notContains: "4532-1234-5678-9010",
		},
		{
			name:     "JWT token",
			input:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			contains: "[REDACTED-JWT]",
		},
		{
			name:     "Normal text",
			input:    "This is normal text without secrets",
			contains: "This is normal text without secrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.RedactString(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected result to contain %q, got %q", tt.contains, result)
			}
			if tt.notContains != "" && strings.Contains(result, tt.notContains) {
				t.Errorf("expected result to NOT contain %q, got %q", tt.notContains, result)
			}
		})
	}
}

func TestRedactor_RedactString_StrictMode(t *testing.T) {
	r := NewRedactor(ModeStrict)

	tests := []struct {
		name  string
		input string
	}{
		{"any text", "This is any text"},
		{"secret", "api_key=secret123"},
		{"normal", "normal value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.RedactString(tt.input)
			if result != "[REDACTED]" {
				t.Errorf("expected [REDACTED], got %q", result)
			}
		})
	}
}

func TestRedactor_RedactString_NoneMode(t *testing.T) {
	r := NewRedactor(ModeNone)

	input := "api_key=secret123 password=test"
	result := r.RedactString(input)
	if result != input {
		t.Errorf("expected no redaction, got %q", result)
	}
}

func TestRedactor_RedactAttributes(t *testing.T) {
	r := NewRedactor(ModeStandard)

	attrs := []attribute.KeyValue{
		attribute.String("normal_key", "normal value"),
		attribute.String("api_key", "sk-1234567890abcdef"),
		attribute.String("user.email", "user@example.com"),
		attribute.String("password", "secretpass"),
		attribute.Int("count", 42),
	}

	redacted := r.RedactAttributes(attrs)

	tests := []struct {
		key         string
		shouldMatch bool
		expected    string
	}{
		{"normal_key", true, "normal value"},
		{"api_key", true, "[REDACTED]"},
		{"user.email", false, "user@example.com"}, // Should be redacted by pattern
		{"password", true, "[REDACTED]"},
		{"count", true, "42"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			var found bool
			var attr attribute.KeyValue
			for _, a := range redacted {
				if string(a.Key) == tt.key {
					found = true
					attr = a
					break
				}
			}

			if !found {
				t.Fatalf("attribute %q not found", tt.key)
			}

			// Handle different attribute types
			var value string
			switch attr.Value.Type() {
			case attribute.STRING:
				value = attr.Value.AsString()
			case attribute.INT64:
				value = fmt.Sprintf("%d", attr.Value.AsInt64())
			case attribute.BOOL:
				value = fmt.Sprintf("%t", attr.Value.AsBool())
			case attribute.FLOAT64:
				value = fmt.Sprintf("%f", attr.Value.AsFloat64())
			default:
				value = fmt.Sprintf("%v", attr.Value.AsInterface())
			}

			if tt.shouldMatch {
				if value != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, value)
				}
			} else {
				if value == tt.expected {
					t.Errorf("expected value to be redacted, got %q", value)
				}
			}
		})
	}
}

func TestRedactor_RedactAttributes_StrictMode(t *testing.T) {
	r := NewRedactor(ModeStrict)

	attrs := []attribute.KeyValue{
		attribute.String("key1", "value1"),
		attribute.Int("key2", 42),
		attribute.Bool("key3", true),
	}

	redacted := r.RedactAttributes(attrs)

	for _, attr := range redacted {
		value := attr.Value.AsString()
		if value != "[REDACTED]" {
			t.Errorf("expected all values to be [REDACTED], got %q for key %q", value, attr.Key)
		}
	}
}

func TestRedactor_ShouldRedactKey(t *testing.T) {
	r := NewRedactor(ModeStandard)

	tests := []struct {
		key      string
		expected bool
	}{
		{"password", true},
		{"user_password", true},
		{"PASSWORD", true},
		{"api_key", true},
		{"apikey", true},
		{"secret", true},
		{"my_secret_value", true},
		{"token", true},
		{"auth_token", true},
		{"authorization", true},
		{"cookie", true},
		{"session", true},
		{"private_key", true},
		{"normal_field", false},
		{"username", false},
		{"count", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := r.shouldRedactKey(tt.key)
			if result != tt.expected {
				t.Errorf("shouldRedactKey(%q) = %v, expected %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestStandardPatterns(t *testing.T) {
	patterns := StandardPatterns()

	// Verify we have expected patterns
	expectedPatterns := []string{
		"api_key",
		"bearer_token",
		"password",
		"aws_key",
		"private_key",
		"email",
		"ssn",
		"credit_card",
		"jwt",
		"generic_secret",
	}

	if len(patterns) != len(expectedPatterns) {
		t.Errorf("expected %d patterns, got %d", len(expectedPatterns), len(patterns))
	}

	for _, expected := range expectedPatterns {
		found := false
		for _, pattern := range patterns {
			if pattern.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected pattern %q not found", expected)
		}
	}
}
