package polltrigger

import (
	"errors"
	"testing"
)

func TestStripSensitiveFields(t *testing.T) {
	tests := []struct {
		name        string
		event       map[string]interface{}
		integration string
		want        map[string]interface{}
	}{
		{
			name: "strips token fields",
			event: map[string]interface{}{
				"id":          "123",
				"title":       "Test Event",
				"api_token":   "secret",
				"auth_header": "Bearer secret",
			},
			integration: "pagerduty",
			want: map[string]interface{}{
				"id":    "123",
				"title": "Test Event",
			},
		},
		{
			name: "strips nested sensitive fields",
			event: map[string]interface{}{
				"id": "456",
				"user": map[string]interface{}{
					"name":     "John Doe",
					"password": "secret123",
					"email":    "john@example.com",
				},
			},
			integration: "jira",
			want: map[string]interface{}{
				"id": "456",
				"user": map[string]interface{}{
					"name":  "John Doe",
					"email": "john@example.com",
				},
			},
		},
		{
			name: "strips integration-specific fields - pagerduty",
			event: map[string]interface{}{
				"id":                "789",
				"title":             "Alert",
				"conference_bridge": "secret-bridge",
			},
			integration: "pagerduty",
			want: map[string]interface{}{
				"id":    "789",
				"title": "Alert",
			},
		},
		{
			name: "strips integration-specific fields - slack",
			event: map[string]interface{}{
				"id":          "slack-123",
				"text":        "Hello world",
				"bot_profile": "sensitive-data",
			},
			integration: "slack",
			want: map[string]interface{}{
				"id":   "slack-123",
				"text": "Hello world",
			},
		},
		{
			name: "handles arrays with nested maps",
			event: map[string]interface{}{
				"id": "array-test",
				"items": []interface{}{
					map[string]interface{}{
						"name":   "Item 1",
						"secret": "hidden",
					},
					map[string]interface{}{
						"name": "Item 2",
						"key":  "also-hidden",
					},
				},
			},
			integration: "datadog",
			want: map[string]interface{}{
				"id": "array-test",
				"items": []interface{}{
					map[string]interface{}{
						"name": "Item 1",
					},
					map[string]interface{}{
						"name": "Item 2",
					},
				},
			},
		},
		{
			name: "preserves non-sensitive fields",
			event: map[string]interface{}{
				"id":          "preserve-test",
				"title":       "Normal Event",
				"description": "This should be kept",
				"status":      "active",
			},
			integration: "pagerduty",
			want: map[string]interface{}{
				"id":          "preserve-test",
				"title":       "Normal Event",
				"description": "This should be kept",
				"status":      "active",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripSensitiveFields(tt.event, tt.integration)

			// Compare the maps
			if !mapsEqual(got, tt.want) {
				t.Errorf("StripSensitiveFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateQueryParameters(t *testing.T) {
	tests := []struct {
		name    string
		query   map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid alphanumeric values",
			query: map[string]interface{}{
				"status":     "open",
				"project_id": "proj-123",
				"label":      "bug-fix",
			},
			wantErr: false,
		},
		{
			name: "valid username with spaces",
			query: map[string]interface{}{
				"assignee": "John Doe",
			},
			wantErr: false,
		},
		{
			name: "valid username with periods",
			query: map[string]interface{}{
				"user_email": "john.doe",
			},
			wantErr: false,
		},
		{
			name: "invalid characters - semicolon",
			query: map[string]interface{}{
				"status": "open; DROP TABLE users",
			},
			wantErr: true,
		},
		{
			name: "invalid characters - quotes",
			query: map[string]interface{}{
				"label": "bug\"OR\"1=1",
			},
			wantErr: true,
		},
		{
			name: "invalid characters - backticks",
			query: map[string]interface{}{
				"project": "`malicious`",
			},
			wantErr: true,
		},
		{
			name: "empty value",
			query: map[string]interface{}{
				"status": "",
			},
			wantErr: true,
		},
		{
			name: "valid array values",
			query: map[string]interface{}{
				"statuses": []interface{}{"open", "in-progress", "closed"},
			},
			wantErr: false,
		},
		{
			name: "invalid array value",
			query: map[string]interface{}{
				"statuses": []interface{}{"open", "DROP TABLE"},
			},
			wantErr: true,
		},
		{
			name: "non-string values ignored",
			query: map[string]interface{}{
				"limit":  100,
				"active": true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQueryParameters(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateQueryParameters() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "redacts Bearer token",
			err:  errors.New("API error: Bearer abc123xyz failed"),
			want: "API error: Bearer [REDACTED] failed",
		},
		{
			name: "redacts PagerDuty token",
			err:  errors.New("Auth failed with Token token=upKxyz123"),
			want: "Auth failed with Token token=[REDACTED]",
		},
		{
			name: "redacts Basic auth",
			err:  errors.New("Authorization: Basic dXNlcjpwYXNz"),
			want: "Authorization: Basic [REDACTED]",
		},
		{
			name: "redacts Datadog API key",
			err:  errors.New("Request failed: DD-API-KEY: abcd1234efgh5678"),
			want: "Request failed: DD-API-KEY: [REDACTED]",
		},
		{
			name: "redacts Datadog APP key",
			err:  errors.New("Request failed: DD-APPLICATION-KEY: app1234key5678"),
			want: "Request failed: DD-APPLICATION-KEY: [REDACTED]",
		},
		{
			name: "redacts Slack bot token",
			err:  errors.New("Slack API error with xoxb-1234-5678-abcd"),
			want: "Slack API error with [REDACTED-SLACK-TOKEN]",
		},
		{
			name: "redacts Slack user token",
			err:  errors.New("Token xoxp-9876-5432-xyz failed"),
			want: "Token [REDACTED-SLACK-TOKEN] failed",
		},
		{
			name: "handles multiple tokens",
			err:  errors.New("Bearer token123 and xoxb-slack-token both invalid"),
			want: "Bearer [REDACTED] and [REDACTED-SLACK-TOKEN] both invalid",
		},
		{
			name: "nil error returns empty string",
			err:  nil,
			want: "",
		},
		{
			name: "error with no sensitive data unchanged",
			err:  errors.New("Connection timeout"),
			want: "Connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeErrorMessage(tt.err)
			if got != tt.want {
				t.Errorf("SanitizeErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "valid alphanumeric",
			value:   "project123",
			wantErr: false,
		},
		{
			name:    "valid with underscores",
			value:   "my_project_id",
			wantErr: false,
		},
		{
			name:    "valid with hyphens",
			value:   "bug-fix-123",
			wantErr: false,
		},
		{
			name:    "invalid with spaces",
			value:   "project 123",
			wantErr: true,
		},
		{
			name:    "invalid with special chars",
			value:   "project@123",
			wantErr: true,
		},
		{
			name:    "empty string",
			value:   "",
			wantErr: true,
		},
		{
			name:    "invalid with semicolon",
			value:   "id;DROP",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIdentifier(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIdentifier() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateExtendedIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "valid with spaces",
			value:   "John Doe",
			wantErr: false,
		},
		{
			name:    "valid with periods",
			value:   "john.doe",
			wantErr: false,
		},
		{
			name:    "valid alphanumeric",
			value:   "user123",
			wantErr: false,
		},
		{
			name:    "invalid with special chars",
			value:   "user@example.com",
			wantErr: true,
		},
		{
			name:    "invalid with semicolon",
			value:   "name;DROP",
			wantErr: true,
		},
		{
			name:    "empty string",
			value:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExtendedIdentifier(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExtendedIdentifier() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// mapsEqual compares two maps recursively for equality.
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for key, aVal := range a {
		bVal, exists := b[key]
		if !exists {
			return false
		}

		if !valuesEqual(aVal, bVal) {
			return false
		}
	}

	return true
}

// valuesEqual compares two interface{} values recursively.
func valuesEqual(a, b interface{}) bool {
	switch aTyped := a.(type) {
	case map[string]interface{}:
		bTyped, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		return mapsEqual(aTyped, bTyped)
	case []interface{}:
		bTyped, ok := b.([]interface{})
		if !ok {
			return false
		}
		return arraysEqual(aTyped, bTyped)
	default:
		return a == b
	}
}

// arraysEqual compares two slices recursively.
func arraysEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !valuesEqual(a[i], b[i]) {
			return false
		}
	}

	return true
}
