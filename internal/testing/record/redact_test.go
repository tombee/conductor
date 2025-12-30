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

package record

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestRedactString_APIKeys(t *testing.T) {
	redactor := NewRedactor()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "OpenAI API key",
			input: "My key is sk-1234567890abcdefghij",
			want:  "My key is [REDACTED-OPENAI-KEY]",
		},
		{
			name:  "GitHub token",
			input: "Token: ghp_1234567890abcdefghijklmnopqrstuvwx",
			want:  "Token: [REDACTED-GITHUB-TOKEN]",
		},
		{
			name:  "Slack token",
			input: "xoxb-1234567890-1234567890-abcdefghijklmnopqrstuvwx",
			want:  "[REDACTED-SLACK-TOKEN]",
		},
		{
			name:  "AWS access key",
			input: "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			want:  "AWS_ACCESS_KEY_ID=[REDACTED-AWS-KEY]",
		},
		{
			name:  "GitLab token",
			input: "glpat_abcdefghijklmnopqrst",
			want:  "[REDACTED-GITLAB-TOKEN]",
		},
		{
			name:  "Auth0 token",
			input: "auth0-12345678901234567890123456789012",
			want:  "[REDACTED-AUTH0-TOKEN]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactor.RedactString(tt.input)
			if got != tt.want {
				t.Errorf("RedactString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactString_JWT(t *testing.T) {
	redactor := NewRedactor()

	input := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	result := redactor.RedactString(input)

	if !strings.Contains(result, "[REDACTED-JWT]") {
		t.Errorf("Expected JWT to be redacted, got: %s", result)
	}
}

func TestRedactString_BearerToken(t *testing.T) {
	redactor := NewRedactor()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "standard bearer",
			input: "Authorization: Bearer abc123def456ghi789jkl012mno345pqr",
		},
		{
			name:  "case insensitive",
			input: "Authorization: bearer abc123def456ghi789jkl012mno345pqr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactor.RedactString(tt.input)
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("Expected bearer token to be redacted, got: %s", got)
			}
		})
	}
}

func TestRedactString_PrivateKey(t *testing.T) {
	redactor := NewRedactor()

	input := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA1234567890abcdefghijklmnop
qrstuvwxyzABCDEFGHIJKLMNOP
-----END RSA PRIVATE KEY-----`

	result := redactor.RedactString(input)

	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("Expected private key to be redacted, got: %s", result)
	}
	if !strings.Contains(result, "BEGIN RSA PRIVATE KEY") {
		t.Errorf("Expected key markers to be preserved, got: %s", result)
	}
}

func TestRedactMap_SensitiveKeys(t *testing.T) {
	redactor := NewRedactor()

	tests := []struct {
		name  string
		input map[string]interface{}
		check func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "password field",
			input: map[string]interface{}{
				"username": "alice",
				"password": "secret123",
			},
			check: func(t *testing.T, result map[string]interface{}) {
				if result["password"] != "[REDACTED]" {
					t.Errorf("Expected password to be redacted, got: %v", result["password"])
				}
				if result["username"] != "alice" {
					t.Errorf("Expected username to be preserved, got: %v", result["username"])
				}
			},
		},
		{
			name: "api_key field",
			input: map[string]interface{}{
				"api_key": "sk-1234567890",
				"user_id": "123",
			},
			check: func(t *testing.T, result map[string]interface{}) {
				if result["api_key"] != "[REDACTED]" {
					t.Errorf("Expected api_key to be redacted, got: %v", result["api_key"])
				}
			},
		},
		{
			name: "access_token field",
			input: map[string]interface{}{
				"access_token":  "token123",
				"refresh_token": "refresh456",
			},
			check: func(t *testing.T, result map[string]interface{}) {
				if result["access_token"] != "[REDACTED]" {
					t.Errorf("Expected access_token to be redacted, got: %v", result["access_token"])
				}
				if result["refresh_token"] != "[REDACTED]" {
					t.Errorf("Expected refresh_token to be redacted, got: %v", result["refresh_token"])
				}
			},
		},
		{
			name: "case insensitive matching",
			input: map[string]interface{}{
				"Password": "secret",
				"API_KEY":  "key123",
			},
			check: func(t *testing.T, result map[string]interface{}) {
				if result["Password"] != "[REDACTED]" {
					t.Errorf("Expected Password to be redacted (case insensitive), got: %v", result["Password"])
				}
				if result["API_KEY"] != "[REDACTED]" {
					t.Errorf("Expected API_KEY to be redacted (case insensitive), got: %v", result["API_KEY"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.RedactMap(tt.input)
			tt.check(t, result)
		})
	}
}

func TestRedactMap_Nested(t *testing.T) {
	redactor := NewRedactor()

	input := map[string]interface{}{
		"user": map[string]interface{}{
			"name":     "alice",
			"password": "secret123",
		},
		"config": map[string]interface{}{
			"api_key": "sk-1234567890",
			"timeout": 30,
		},
	}

	result := redactor.RedactMap(input)

	user := result["user"].(map[string]interface{})
	if user["password"] != "[REDACTED]" {
		t.Errorf("Expected nested password to be redacted, got: %v", user["password"])
	}

	config := result["config"].(map[string]interface{})
	if config["api_key"] != "[REDACTED]" {
		t.Errorf("Expected nested api_key to be redacted, got: %v", config["api_key"])
	}
	if config["timeout"] != 30 {
		t.Errorf("Expected timeout to be preserved, got: %v", config["timeout"])
	}
}

func TestRedactMap_Array(t *testing.T) {
	redactor := NewRedactor()

	input := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"name":     "alice",
				"password": "secret1",
			},
			map[string]interface{}{
				"name":     "bob",
				"password": "secret2",
			},
		},
	}

	result := redactor.RedactMap(input)

	users := result["users"].([]interface{})
	user1 := users[0].(map[string]interface{})
	user2 := users[1].(map[string]interface{})

	if user1["password"] != "[REDACTED]" {
		t.Errorf("Expected password in array to be redacted, got: %v", user1["password"])
	}
	if user2["password"] != "[REDACTED]" {
		t.Errorf("Expected password in array to be redacted, got: %v", user2["password"])
	}
}

func TestRedactHeaders(t *testing.T) {
	redactor := NewRedactor()

	tests := []struct {
		name  string
		input map[string]string
		check func(t *testing.T, result map[string]string)
	}{
		{
			name: "authorization header",
			input: map[string]string{
				"Authorization": "Bearer token123",
				"Content-Type":  "application/json",
			},
			check: func(t *testing.T, result map[string]string) {
				if result["Authorization"] != "[REDACTED]" {
					t.Errorf("Expected Authorization to be redacted, got: %v", result["Authorization"])
				}
				if result["Content-Type"] != "application/json" {
					t.Errorf("Expected Content-Type to be preserved, got: %v", result["Content-Type"])
				}
			},
		},
		{
			name: "x-api-key header",
			input: map[string]string{
				"X-API-Key":    "secret123",
				"Content-Type": "application/json",
			},
			check: func(t *testing.T, result map[string]string) {
				if result["X-API-Key"] != "[REDACTED]" {
					t.Errorf("Expected X-API-Key to be redacted, got: %v", result["X-API-Key"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.RedactHeaders(tt.input)
			tt.check(t, result)
		})
	}
}

func TestRedactJSON(t *testing.T) {
	redactor := NewRedactor()

	tests := []struct {
		name    string
		input   string
		check   func(t *testing.T, result string)
		wantErr bool
	}{
		{
			name:  "simple JSON with password",
			input: `{"username":"alice","password":"secret123"}`,
			check: func(t *testing.T, result string) {
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(result), &data); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}
				if data["password"] != "[REDACTED]" {
					t.Errorf("Expected password to be redacted, got: %v", data["password"])
				}
				if data["username"] != "alice" {
					t.Errorf("Expected username to be preserved, got: %v", data["username"])
				}
			},
		},
		{
			name:  "JSON with API key pattern",
			input: `{"key":"sk-1234567890abcdefghij","value":"test"}`,
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "[REDACTED-OPENAI-KEY]") {
					t.Errorf("Expected API key to be redacted, got: %s", result)
				}
			},
		},
		{
			name:  "invalid JSON falls back to string redaction",
			input: `not valid json with sk-1234567890abcdefghij`,
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "[REDACTED-OPENAI-KEY]") {
					t.Errorf("Expected API key to be redacted in fallback, got: %s", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := redactor.RedactJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("RedactJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestRedactURL(t *testing.T) {
	redactor := NewRedactor()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "URL with API key in query param",
			input: "https://api.example.com/data?api_key=sk-1234567890abcdefghij",
			want:  "https://api.example.com/data?api_key=[REDACTED-OPENAI-KEY]",
		},
		{
			name:  "URL with token in query param",
			input: "https://example.com?token=ghp_1234567890abcdefghijklmnopqrstuvwx",
			want:  "https://example.com?token=[REDACTED-GITHUB-TOKEN]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactor.RedactURL(tt.input)
			if got != tt.want {
				t.Errorf("RedactURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAddPattern(t *testing.T) {
	redactor := NewRedactor()

	customPattern := RedactionPattern{
		Name:        "custom_id",
		Regex:       regexp.MustCompile(`CUST-\d{6}`),
		Replacement: "[REDACTED-CUSTOMER-ID]",
	}

	redactor.AddPattern(customPattern)

	input := "Customer ID: CUST-123456"
	result := redactor.RedactString(input)

	if !strings.Contains(result, "[REDACTED-CUSTOMER-ID]") {
		t.Errorf("Expected custom pattern to be applied, got: %s", result)
	}
}

func TestNewRedactorWithPatterns(t *testing.T) {
	customPatterns := []RedactionPattern{
		{
			Name:        "test_pattern",
			Regex:       regexp.MustCompile(`TEST-\d+`),
			Replacement: "[REDACTED-TEST]",
		},
	}

	redactor := NewRedactorWithPatterns(customPatterns)

	input := "Test ID: TEST-123"
	result := redactor.RedactString(input)

	if !strings.Contains(result, "[REDACTED-TEST]") {
		t.Errorf("Expected custom pattern to be applied, got: %s", result)
	}
}
