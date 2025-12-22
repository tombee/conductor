package connector

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/tombee/conductor/pkg/workflow"
)

// Test helpers
func newTestRequest() *http.Request {
	req := httptest.NewRequest("GET", "http://example.com", nil)
	return req
}

type testAuth struct {
	Type     string
	Token    string
	Username string
	Password string
	Header   string
	Value    string
}

func (ta *testAuth) toWorkflowAuth() *workflow.AuthDefinition {
	return &workflow.AuthDefinition{
		Type:     ta.Type,
		Token:    ta.Token,
		Username: ta.Username,
		Password: ta.Password,
		Header:   ta.Header,
		Value:    ta.Value,
	}
}

func TestExpandEnvVar(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_VAR", "test_value")
	os.Setenv("API_KEY", "secret123")
	defer os.Unsetenv("TEST_VAR")
	defer os.Unsetenv("API_KEY")

	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{
			name:      "no expansion needed",
			input:     "plain_value",
			want:      "plain_value",
			wantError: false,
		},
		{
			name:      "empty string",
			input:     "",
			want:      "",
			wantError: false,
		},
		{
			name:      "valid expansion",
			input:     "${TEST_VAR}",
			want:      "test_value",
			wantError: false,
		},
		{
			name:      "valid expansion with prefix",
			input:     "Bearer ${API_KEY}",
			want:      "Bearer secret123",
			wantError: false,
		},
		{
			name:      "multiple expansions",
			input:     "${TEST_VAR}_${API_KEY}",
			want:      "test_value_secret123",
			wantError: false,
		},
		{
			name:      "invalid variable name - special chars",
			input:     "${TEST-VAR}",
			want:      "",
			wantError: true,
		},
		{
			name:      "invalid variable name - dots",
			input:     "${test.var}",
			want:      "",
			wantError: true,
		},
		{
			name:      "invalid variable name - spaces",
			input:     "${TEST VAR}",
			want:      "",
			wantError: true,
		},
		{
			name:      "invalid variable name - path traversal attempt",
			input:     "${../etc/passwd}",
			want:      "",
			wantError: true,
		},
		{
			name:      "variable not found",
			input:     "${NONEXISTENT_VAR}",
			want:      "",
			wantError: true,
		},
		{
			name:      "malformed - unclosed",
			input:     "${TEST_VAR",
			want:      "",
			wantError: true,
		},
		{
			name:      "empty variable name",
			input:     "${}",
			want:      "",
			wantError: true,
		},
		{
			name:      "valid variable name with underscore",
			input:     "${_PRIVATE_VAR}",
			want:      "",
			wantError: true, // variable doesn't exist
		},
		{
			name:      "invalid variable name - starts with number",
			input:     "${123VAR}",
			want:      "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandEnvVar(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("expandEnvVar(%q) = %q, want %q", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestApplyBearerAuth_EnvVarValidation(t *testing.T) {
	os.Setenv("VALID_TOKEN", "token123")
	defer os.Unsetenv("VALID_TOKEN")

	tests := []struct {
		name      string
		token     string
		wantError bool
	}{
		{
			name:      "valid static token",
			token:     "static_token",
			wantError: false,
		},
		{
			name:      "valid env var expansion",
			token:     "${VALID_TOKEN}",
			wantError: false,
		},
		{
			name:      "invalid env var name",
			token:     "${INVALID-TOKEN}",
			wantError: true,
		},
		{
			name:      "nonexistent env var",
			token:     "${NONEXISTENT}",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest()
			auth := &testAuth{Token: tt.token}

			err := applyBearerAuth(req, auth.toWorkflowAuth())

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestApplyBasicAuth_EnvVarValidation(t *testing.T) {
	os.Setenv("VALID_USER", "user123")
	os.Setenv("VALID_PASS", "pass123")
	defer os.Unsetenv("VALID_USER")
	defer os.Unsetenv("VALID_PASS")

	tests := []struct {
		name      string
		username  string
		password  string
		wantError bool
	}{
		{
			name:      "valid static credentials",
			username:  "user",
			password:  "pass",
			wantError: false,
		},
		{
			name:      "valid env var expansion",
			username:  "${VALID_USER}",
			password:  "${VALID_PASS}",
			wantError: false,
		},
		{
			name:      "invalid username env var",
			username:  "${INVALID-USER}",
			password:  "pass",
			wantError: true,
		},
		{
			name:      "invalid password env var",
			username:  "user",
			password:  "${INVALID-PASS}",
			wantError: true,
		},
		{
			name:      "nonexistent username",
			username:  "${NONEXISTENT}",
			password:  "pass",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest()
			auth := &testAuth{
				Username: tt.username,
				Password: tt.password,
			}

			err := applyBasicAuth(req, auth.toWorkflowAuth())

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestApplyAPIKeyAuth_EnvVarValidation(t *testing.T) {
	os.Setenv("VALID_KEY", "key123")
	defer os.Unsetenv("VALID_KEY")

	tests := []struct {
		name      string
		header    string
		value     string
		wantError bool
	}{
		{
			name:      "valid static key",
			header:    "X-API-Key",
			value:     "static_key",
			wantError: false,
		},
		{
			name:      "valid env var expansion",
			header:    "X-API-Key",
			value:     "${VALID_KEY}",
			wantError: false,
		},
		{
			name:      "invalid env var name",
			header:    "X-API-Key",
			value:     "${INVALID-KEY}",
			wantError: true,
		},
		{
			name:      "nonexistent env var",
			header:    "X-API-Key",
			value:     "${NONEXISTENT}",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest()
			auth := &testAuth{
				Header: tt.header,
				Value:  tt.value,
			}

			err := applyAPIKeyAuth(req, auth.toWorkflowAuth())

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
