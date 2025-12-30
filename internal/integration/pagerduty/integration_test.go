package pagerduty

import (
	"testing"

	"github.com/tombee/conductor/internal/operation/api"
)

func TestNewPagerDutyIntegration(t *testing.T) {
	tests := []struct {
		name    string
		config  *api.ProviderConfig
		wantErr bool
	}{
		{
			name: "default config",
			config: &api.ProviderConfig{
				Token: "test-token",
			},
			wantErr: false,
		},
		{
			name: "custom base URL",
			config: &api.ProviderConfig{
				BaseURL: "https://custom.pagerduty.com",
				Token:   "test-token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewPagerDutyIntegration(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPagerDutyIntegration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if provider == nil {
				t.Error("NewPagerDutyIntegration() returned nil provider")
			}
		})
	}
}

func TestPagerDutyIntegration_Operations(t *testing.T) {
	config := &api.ProviderConfig{
		Token: "test-token",
	}
	provider, err := NewPagerDutyIntegration(config)
	if err != nil {
		t.Fatalf("NewPagerDutyIntegration() error = %v", err)
	}

	pd := provider.(*PagerDutyIntegration)
	ops := pd.Operations()

	expectedOps := []string{
		"list_incidents",
		"get_incident",
		"update_incident",
		"acknowledge_incident",
		"resolve_incident",
		"list_incident_notes",
		"create_incident_note",
		"list_incident_log_entries",
		"get_current_user",
		"list_services",
		"list_oncalls",
	}

	if len(ops) != len(expectedOps) {
		t.Errorf("Operations() returned %d operations, want %d", len(ops), len(expectedOps))
	}

	opNames := make(map[string]bool)
	for _, op := range ops {
		opNames[op.Name] = true
	}

	for _, expected := range expectedOps {
		if !opNames[expected] {
			t.Errorf("Operations() missing expected operation %q", expected)
		}
	}
}

func TestPagerDutyError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *PagerDutyError
		want string
	}{
		{
			name: "with errors list",
			err: &PagerDutyError{
				StatusCode: 400,
				Code:       2001,
				Message:    "Invalid input",
				Errors:     []string{"field1 required", "field2 invalid"},
			},
			want: "PagerDuty API error (HTTP 400, code 2001): Invalid input - field1 required; field2 invalid",
		},
		{
			name: "with code no errors",
			err: &PagerDutyError{
				StatusCode: 404,
				Code:       2100,
				Message:    "Not found",
			},
			want: "PagerDuty API error (HTTP 404, code 2100): Not found",
		},
		{
			name: "simple HTTP error",
			err: &PagerDutyError{
				StatusCode: 500,
				Message:    "Internal server error",
			},
			want: "PagerDuty API error (HTTP 500): Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("PagerDutyError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPagerDutyError_StatusChecks(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		isNotFound   bool
		isRateLimited bool
		isAuthError  bool
	}{
		{name: "404", statusCode: 404, isNotFound: true, isRateLimited: false, isAuthError: false},
		{name: "429", statusCode: 429, isNotFound: false, isRateLimited: true, isAuthError: false},
		{name: "401", statusCode: 401, isNotFound: false, isRateLimited: false, isAuthError: true},
		{name: "403", statusCode: 403, isNotFound: false, isRateLimited: false, isAuthError: true},
		{name: "200", statusCode: 200, isNotFound: false, isRateLimited: false, isAuthError: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &PagerDutyError{StatusCode: tt.statusCode}
			if err.IsNotFound() != tt.isNotFound {
				t.Errorf("IsNotFound() = %v, want %v", err.IsNotFound(), tt.isNotFound)
			}
			if err.IsRateLimited() != tt.isRateLimited {
				t.Errorf("IsRateLimited() = %v, want %v", err.IsRateLimited(), tt.isRateLimited)
			}
			if err.IsAuthError() != tt.isAuthError {
				t.Errorf("IsAuthError() = %v, want %v", err.IsAuthError(), tt.isAuthError)
			}
		})
	}
}
