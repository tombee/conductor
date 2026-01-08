package workflow

import (
	"testing"
)

func TestIntegrationDefinitionValidate(t *testing.T) {
	tests := []struct {
		name        string
		integration IntegrationDefinition
		wantErr     bool
		errMsg      string
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
			name: "integration step workspace integration (not defined in workflow)",
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
			wantErr: false, // Workspace integrations are allowed - resolved at runtime
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

func TestPollTriggerValidation(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "valid poll trigger",
			definition: `
name: test-workflow
trigger:
  poll:
    integration: pagerduty
    query:
      user_id: PUSER123
    interval: 30s
steps:
  - id: process
    type: llm
    prompt: test
`,
			wantErr: false,
		},
		{
			name: "missing integration",
			definition: `
name: test-workflow
trigger:
  poll:
    query:
      user_id: PUSER123
steps:
  - id: process
    type: llm
    prompt: test
`,
			wantErr: true,
			errMsg:  "integration is required",
		},
		{
			name: "invalid integration",
			definition: `
name: test-workflow
trigger:
  poll:
    integration: invalid
    query:
      user_id: PUSER123
steps:
  - id: process
    type: llm
    prompt: test
`,
			wantErr: true,
			errMsg:  "unsupported integration: invalid",
		},
		{
			name: "missing query",
			definition: `
name: test-workflow
trigger:
  poll:
    integration: pagerduty
steps:
  - id: process
    type: llm
    prompt: test
`,
			wantErr: true,
			errMsg:  "query parameters are required",
		},
		{
			name: "interval too small",
			definition: `
name: test-workflow
trigger:
  poll:
    integration: pagerduty
    query:
      user_id: PUSER123
    interval: 5s
steps:
  - id: process
    type: llm
    prompt: test
`,
			wantErr: true,
			errMsg:  "interval must be at least 10s",
		},
		{
			name: "invalid startup mode",
			definition: `
name: test-workflow
trigger:
  poll:
    integration: pagerduty
    query:
      user_id: PUSER123
    startup: invalid
steps:
  - id: process
    type: llm
    prompt: test
`,
			wantErr: true,
			errMsg:  "invalid startup mode",
		},
		{
			name: "backfill without duration",
			definition: `
name: test-workflow
trigger:
  poll:
    integration: pagerduty
    query:
      user_id: PUSER123
    startup: backfill
steps:
  - id: process
    type: llm
    prompt: test
`,
			wantErr: true,
			errMsg:  "backfill duration is required",
		},
		{
			name: "backfill exceeds 24h",
			definition: `
name: test-workflow
trigger:
  poll:
    integration: pagerduty
    query:
      user_id: PUSER123
    startup: backfill
    backfill: 48h
steps:
  - id: process
    type: llm
    prompt: test
`,
			wantErr: true,
			errMsg:  "backfill duration cannot exceed 24h",
		},
		{
			name: "invalid query parameter pattern",
			definition: `
name: test-workflow
trigger:
  poll:
    integration: pagerduty
    query:
      user_id: "PUSER123; DROP TABLE"
steps:
  - id: process
    type: llm
    prompt: test
`,
			wantErr: true,
			errMsg:  "invalid query parameter value",
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

func TestPollTriggerUnmarshal(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		wantErr    bool
		validate   func(t *testing.T, def *Definition)
	}{
		{
			name: "valid poll trigger with all fields",
			definition: `
name: test-workflow
version: "1.0"

trigger:
  poll:
    integration: pagerduty
    query:
      user_id: PUSER123
      statuses: [triggered, acknowledged]
    interval: 30s
    startup: since_last
    backfill: 1h
    input_mapping:
      incident_id: "{{.trigger.event.id}}"
      incident_title: "{{.trigger.event.title}}"

steps:
  - id: process
    type: llm
    prompt: Process incident
`,
			wantErr: false,
			validate: func(t *testing.T, def *Definition) {
				if def.Trigger == nil {
					t.Fatal("Trigger is nil")
				}
				if def.Trigger.Poll == nil {
					t.Fatal("Poll trigger is nil")
				}
				if def.Trigger.Poll.Integration != "pagerduty" {
					t.Errorf("Integration = %v, want pagerduty", def.Trigger.Poll.Integration)
				}
				if def.Trigger.Poll.Interval != "30s" {
					t.Errorf("Interval = %v, want 30s", def.Trigger.Poll.Interval)
				}
				if def.Trigger.Poll.Startup != "since_last" {
					t.Errorf("Startup = %v, want since_last", def.Trigger.Poll.Startup)
				}
				if def.Trigger.Poll.Backfill != "1h" {
					t.Errorf("Backfill = %v, want 1h", def.Trigger.Poll.Backfill)
				}
				if len(def.Trigger.Poll.Query) == 0 {
					t.Error("Query is empty")
				}
				if len(def.Trigger.Poll.InputMapping) != 2 {
					t.Errorf("InputMapping length = %v, want 2", len(def.Trigger.Poll.InputMapping))
				}
			},
		},
		{
			name: "valid poll trigger with minimal fields",
			definition: `
name: test-workflow
version: "1.0"

trigger:
  poll:
    integration: slack
    query:
      mentions: "@jsmith"
      channels: [engineering]

steps:
  - id: process
    type: llm
    prompt: Process mention
`,
			wantErr: false,
			validate: func(t *testing.T, def *Definition) {
				if def.Trigger == nil || def.Trigger.Poll == nil {
					t.Fatal("Poll trigger is nil")
				}
				if def.Trigger.Poll.Integration != "slack" {
					t.Errorf("Integration = %v, want slack", def.Trigger.Poll.Integration)
				}
				if def.Trigger.Poll.Interval != "" {
					t.Errorf("Interval should be empty (use default), got %v", def.Trigger.Poll.Interval)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.definition))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.validate != nil {
				tt.validate(t, def)
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

// TestIfFieldParsing tests that the 'if' field is correctly parsed from YAML
func TestIfFieldParsing(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		wantErr    bool
		validate   func(t *testing.T, def *Definition)
	}{
		{
			name: "valid if field",
			definition: `
name: test-workflow
steps:
  - id: check
    type: llm
    prompt: "Check something"

  - id: conditional_step
    type: llm
    if: "{{.steps.check.response}} == 'yes'"
    prompt: "Do something"
`,
			wantErr: false,
			validate: func(t *testing.T, def *Definition) {
				if len(def.Steps) < 2 {
					t.Fatal("Expected at least 2 steps")
				}
				step := def.Steps[1]
				if step.If != "{{.steps.check.response}} == 'yes'" {
					t.Errorf("If field not parsed correctly, got %v", step.If)
				}
				// Check normalization: If should be copied to Condition.Expression
				if step.Condition == nil {
					t.Fatal("Condition should not be nil after normalization")
				}
				if step.Condition.Expression != step.If {
					t.Errorf("Condition.Expression = %v, want %v", step.Condition.Expression, step.If)
				}
			},
		},
		{
			name: "if field with simple expression",
			definition: `
name: test-workflow
steps:
  - id: conditional_step
    type: llm
    if: "inputs.enabled"
    prompt: "Do something"
`,
			wantErr: false,
			validate: func(t *testing.T, def *Definition) {
				step := def.Steps[0]
				if step.If != "inputs.enabled" {
					t.Errorf("If field = %v, want 'inputs.enabled'", step.If)
				}
				if step.Condition == nil || step.Condition.Expression != "inputs.enabled" {
					t.Error("If field not normalized to Condition.Expression")
				}
			},
		},
		{
			name: "if field with boolean operators",
			definition: `
name: test-workflow
steps:
  - id: conditional_step
    type: llm
    if: "inputs.enabled && !inputs.dry_run"
    prompt: "Do something"
`,
			wantErr: false,
			validate: func(t *testing.T, def *Definition) {
				step := def.Steps[0]
				expectedIf := "inputs.enabled && !inputs.dry_run"
				if step.If != expectedIf {
					t.Errorf("If field = %v, want %v", step.If, expectedIf)
				}
				if step.Condition == nil || step.Condition.Expression != expectedIf {
					t.Error("If field not normalized to Condition.Expression")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.definition))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.validate != nil {
				tt.validate(t, def)
			}
		})
	}
}

// TestIfFieldNormalization tests that 'if' is normalized to 'condition.expression'
func TestIfFieldNormalization(t *testing.T) {
	tests := []struct {
		name               string
		definition         string
		wantErr            bool
		expectedExpression string
	}{
		{
			name: "if normalizes to condition.expression",
			definition: `
name: test-workflow
steps:
  - id: step1
    type: llm
    if: "true"
    prompt: "test"
`,
			wantErr:            false,
			expectedExpression: "true",
		},
		{
			name: "if with template syntax",
			definition: `
name: test-workflow
steps:
  - id: check
    type: llm
    prompt: "check"
  - id: step2
    type: llm
    if: "{{.steps.check.response}} == 'proceed'"
    prompt: "test"
`,
			wantErr:            false,
			expectedExpression: "{{.steps.check.response}} == 'proceed'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.definition))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Find the step with 'if' field (not the first step in some tests)
				var targetStep *StepDefinition
				for i := range def.Steps {
					if def.Steps[i].If != "" {
						targetStep = &def.Steps[i]
						break
					}
				}
				if targetStep == nil {
					t.Fatal("No step with 'if' field found")
				}
				if targetStep.Condition == nil {
					t.Fatal("Condition should be set after normalization")
				}
				if targetStep.Condition.Expression != tt.expectedExpression {
					t.Errorf("Condition.Expression = %v, want %v",
						targetStep.Condition.Expression, tt.expectedExpression)
				}
			}
		})
	}
}

// TestIfAndConditionMutualExclusivity tests that 'if' and 'condition' cannot both be set
func TestIfAndConditionMutualExclusivity(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "both if and condition.expression set",
			definition: `
name: test-workflow
steps:
  - id: step1
    type: llm
    if: "inputs.enabled"
    condition:
      expression: "inputs.mode == 'strict'"
    prompt: "test"
`,
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name: "if and condition with then_steps",
			definition: `
name: test-workflow
steps:
  - id: step1
    type: llm
    if: "inputs.enabled"
    condition:
      expression: "inputs.enabled"
      then_steps: ["step2"]
    prompt: "test"
  - id: step2
    type: llm
    prompt: "test2"
`,
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name: "only if field (valid)",
			definition: `
name: test-workflow
steps:
  - id: step1
    type: llm
    if: "inputs.enabled"
    prompt: "test"
`,
			wantErr: false,
		},
		{
			name: "only condition field (valid)",
			definition: `
name: test-workflow
steps:
  - id: step1
    type: llm
    condition:
      expression: "inputs.enabled"
    prompt: "test"
`,
			wantErr: false,
		},
		{
			name: "neither if nor condition (valid)",
			definition: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantErr: false,
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
