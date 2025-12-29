package workflow

import (
	"testing"
)

func TestIntegrationDefinitionValidate(t *testing.T) {
	tests := []struct {
		name      string
		integration IntegrationDefinition
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid inline integration",
			integration: IntegrationDefinition{
				Name:    "github",
				BaseURL: "https://api.github.com",
				Auth: &AuthDefinition{
					Token: "${GITHUB_TOKEN}",
				},
				Operations: map[string]OperationDefinition{
					"create_issue": {
						Method: "POST",
						Path:   "/repos/{owner}/{repo}/issues",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid package integration",
			integration: IntegrationDefinition{
				Name: "github",
				From: "integrations/github",
				Auth: &AuthDefinition{
					Token: "${GITHUB_TOKEN}",
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			integration: IntegrationDefinition{
				BaseURL: "https://api.github.com",
				Operations: map[string]OperationDefinition{
					"test": {Method: "GET", Path: "/test"},
				},
			},
			wantErr: true,
			errMsg:  "integration name is required",
		},
		{
			name: "missing from and inline definition",
			integration: IntegrationDefinition{
				Name: "test",
			},
			wantErr: true,
			errMsg:  "integration must specify either 'from' (package import) or inline definition (base_url + operations)",
		},
		{
			name: "both from and inline definition",
			integration: IntegrationDefinition{
				Name:    "test",
				From:    "integrations/test",
				BaseURL: "https://api.test.com",
				Operations: map[string]OperationDefinition{
					"test": {Method: "GET", Path: "/test"},
				},
			},
			wantErr: true,
			errMsg:  "integration cannot specify both 'from' and inline definition (base_url/operations)",
		},
		{
			name: "inline integration missing base_url",
			integration: IntegrationDefinition{
				Name: "test",
				Operations: map[string]OperationDefinition{
					"test": {Method: "GET", Path: "/test"},
				},
			},
			wantErr: true,
			errMsg:  "base_url is required for inline integration definition",
		},
		{
			name: "inline integration missing operations",
			integration: IntegrationDefinition{
				Name:    "test",
				BaseURL: "https://api.test.com",
			},
			wantErr: true,
			errMsg:  "inline integration must define at least one operation",
		},
		{
			name: "invalid auth",
			integration: IntegrationDefinition{
				Name:    "test",
				BaseURL: "https://api.test.com",
				Auth: &AuthDefinition{
					Type: "invalid",
				},
				Operations: map[string]OperationDefinition{
					"test": {Method: "GET", Path: "/test"},
				},
			},
			wantErr: true,
			errMsg:  "invalid auth:",
		},
		{
			name: "invalid rate limit",
			integration: IntegrationDefinition{
				Name:    "test",
				BaseURL: "https://api.test.com",
				RateLimit: &RateLimitConfig{
					RequestsPerSecond: -1,
				},
				Operations: map[string]OperationDefinition{
					"test": {Method: "GET", Path: "/test"},
				},
			},
			wantErr: true,
			errMsg:  "invalid rate_limit:",
		},
		{
			name: "invalid operation",
			integration: IntegrationDefinition{
				Name:    "test",
				BaseURL: "https://api.test.com",
				Operations: map[string]OperationDefinition{
					"test": {
						Method: "INVALID",
						Path:   "/test",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid operation test:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.integration.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("IntegrationDefinition.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("IntegrationDefinition.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestOperationDefinitionValidate(t *testing.T) {
	tests := []struct {
		name      string
		operation OperationDefinition
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid operation",
			operation: OperationDefinition{
				Method: "POST",
				Path:   "/repos/{owner}/{repo}/issues",
			},
			wantErr: false,
		},
		{
			name: "missing method",
			operation: OperationDefinition{
				Path: "/test",
			},
			wantErr: true,
			errMsg:  "method is required",
		},
		{
			name: "invalid method",
			operation: OperationDefinition{
				Method: "INVALID",
				Path:   "/test",
			},
			wantErr: true,
			errMsg:  "invalid method:",
		},
		{
			name: "missing path",
			operation: OperationDefinition{
				Method: "GET",
			},
			wantErr: true,
			errMsg:  "path is required",
		},
		{
			name: "negative timeout",
			operation: OperationDefinition{
				Method:  "GET",
				Path:    "/test",
				Timeout: -1,
			},
			wantErr: true,
			errMsg:  "timeout must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("OperationDefinition.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("OperationDefinition.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestAuthDefinitionValidate(t *testing.T) {
	tests := []struct {
		name    string
		auth    AuthDefinition
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid bearer auth",
			auth: AuthDefinition{
				Type:  "bearer",
				Token: "${GITHUB_TOKEN}",
			},
			wantErr: false,
		},
		{
			name: "valid bearer auth shorthand",
			auth: AuthDefinition{
				Token: "${GITHUB_TOKEN}",
			},
			wantErr: false,
		},
		{
			name: "valid basic auth",
			auth: AuthDefinition{
				Type:     "basic",
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name: "valid api_key auth",
			auth: AuthDefinition{
				Type:   "api_key",
				Header: "X-API-Key",
				Value:  "${API_KEY}",
			},
			wantErr: false,
		},
		{
			name: "bearer missing token",
			auth: AuthDefinition{
				Type: "bearer",
			},
			wantErr: true,
			errMsg:  "token is required for bearer auth",
		},
		{
			name: "basic missing username",
			auth: AuthDefinition{
				Type:     "basic",
				Password: "pass",
			},
			wantErr: true,
			errMsg:  "username is required for basic auth",
		},
		{
			name: "basic missing password",
			auth: AuthDefinition{
				Type:     "basic",
				Username: "user",
			},
			wantErr: true,
			errMsg:  "password is required for basic auth",
		},
		{
			name: "api_key missing header",
			auth: AuthDefinition{
				Type:  "api_key",
				Value: "${API_KEY}",
			},
			wantErr: true,
			errMsg:  "header is required for api_key auth",
		},
		{
			name: "api_key missing value",
			auth: AuthDefinition{
				Type:   "api_key",
				Header: "X-API-Key",
			},
			wantErr: true,
			errMsg:  "value is required for api_key auth",
		},
		{
			name: "oauth2 not implemented",
			auth: AuthDefinition{
				Type:         "oauth2_client",
				ClientID:     "client",
				ClientSecret: "secret",
				TokenURL:     "https://oauth.example.com/token",
			},
			wantErr: true,
			errMsg:  "oauth2_client auth type is not yet implemented",
		},
		{
			name: "invalid auth type",
			auth: AuthDefinition{
				Type: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid auth type:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.auth.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("AuthDefinition.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("AuthDefinition.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestRateLimitConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		rateLimit RateLimitConfig
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid with requests_per_second",
			rateLimit: RateLimitConfig{
				RequestsPerSecond: 10,
			},
			wantErr: false,
		},
		{
			name: "valid with requests_per_minute",
			rateLimit: RateLimitConfig{
				RequestsPerMinute: 100,
			},
			wantErr: false,
		},
		{
			name: "valid with both limits",
			rateLimit: RateLimitConfig{
				RequestsPerSecond: 10,
				RequestsPerMinute: 100,
			},
			wantErr: false,
		},
		{
			name:      "missing both limits",
			rateLimit: RateLimitConfig{},
			wantErr:   true,
			errMsg:    "at least one of requests_per_second or requests_per_minute must be specified",
		},
		{
			name: "negative requests_per_second",
			rateLimit: RateLimitConfig{
				RequestsPerSecond: -1,
			},
			wantErr: true,
			errMsg:  "requests_per_second must be non-negative",
		},
		{
			name: "negative requests_per_minute",
			rateLimit: RateLimitConfig{
				RequestsPerMinute: -1,
			},
			wantErr: true,
			errMsg:  "requests_per_minute must be non-negative",
		},
		{
			name: "negative timeout",
			rateLimit: RateLimitConfig{
				RequestsPerSecond: 10,
				Timeout:           -1,
			},
			wantErr: true,
			errMsg:  "timeout must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rateLimit.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RateLimitConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("RateLimitConfig.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestIntegrationStepValidation(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "valid integration step with inline integration",
			definition: `
name: test-workflow
version: "1.0"

integrations:
  github:
    base_url: https://api.github.com
    auth:
      token: ${GITHUB_TOKEN}
    operations:
      create_issue:
        method: POST
        path: /repos/{owner}/{repo}/issues

steps:
  - id: create_issue
    type: integration
    integration: github.create_issue
    inputs:
      owner: test
      repo: test
      title: Test Issue
`,
			wantErr: false,
		},
		{
			name: "valid integration step with package integration",
			definition: `
name: test-workflow
version: "1.0"

integrations:
  github:
    from: integrations/github
    auth:
      token: ${GITHUB_TOKEN}

steps:
  - id: create_issue
    type: integration
    integration: github.create_issue
    inputs:
      owner: test
      repo: test
`,
			wantErr: false,
		},
		{
			name: "integration step missing integration field",
			definition: `
name: test-workflow
version: "1.0"

integrations:
  github:
    from: integrations/github
    auth:
      token: ${GITHUB_TOKEN}

steps:
  - id: create_issue
    type: integration
    inputs:
      owner: test
`,
			wantErr: true,
			errMsg:  "integration step requires either 'integration' field or 'action'+'operation' fields",
		},
		{
			name: "integration step invalid format",
			definition: `
name: test-workflow
version: "1.0"

integrations:
  github:
    from: integrations/github
    auth:
      token: ${GITHUB_TOKEN}

steps:
  - id: create_issue
    type: integration
    integration: invalid_format
    inputs:
      owner: test
`,
			wantErr: true,
			errMsg:  "integration must be in format 'integration_name.operation_name'",
		},
		{
			name: "integration step undefined integration",
			definition: `
name: test-workflow
version: "1.0"

integrations:
  github:
    from: integrations/github
    auth:
      token: ${GITHUB_TOKEN}

steps:
  - id: create_issue
    type: integration
    integration: slack.post_message
    inputs:
      channel: test
`,
			wantErr: true,
			errMsg:  "references undefined integration: slack",
		},
		{
			name: "integration step undefined operation in inline integration",
			definition: `
name: test-workflow
version: "1.0"

integrations:
  github:
    base_url: https://api.github.com
    auth:
      token: ${GITHUB_TOKEN}
    operations:
      create_issue:
        method: POST
        path: /repos/{owner}/{repo}/issues

steps:
  - id: update_issue
    type: integration
    integration: github.update_issue
    inputs:
      owner: test
`,
			wantErr: true,
			errMsg:  "references undefined operation update_issue in integration github",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDefinition([]byte(tt.definition))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseDefinition() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
