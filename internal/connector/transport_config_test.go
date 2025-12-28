package connector

import (
	"os"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

func TestToHTTPTransportConfig(t *testing.T) {
	tests := []struct {
		name     string
		def      *workflow.ConnectorDefinition
		wantAuth bool
	}{
		{
			name: "with environment variable auth",
			def: &workflow.ConnectorDefinition{
				BaseURL: "https://api.example.com",
				Auth: &workflow.AuthDefinition{
					Type:  "bearer",
					Token: "${API_TOKEN}",
				},
				Headers: map[string]string{
					"X-Custom": "value",
				},
			},
			wantAuth: true,
		},
		{
			name: "with plain token (backward compat)",
			def: &workflow.ConnectorDefinition{
				BaseURL: "https://api.example.com",
				Auth: &workflow.AuthDefinition{
					Type:  "bearer",
					Token: "plain-token",
				},
			},
			wantAuth: false,
		},
		{
			name: "without auth",
			def: &workflow.ConnectorDefinition{
				BaseURL: "https://api.example.com",
			},
			wantAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := toHTTPTransportConfig(tt.def)

			if config.BaseURL != tt.def.BaseURL {
				t.Errorf("BaseURL = %q, want %q", config.BaseURL, tt.def.BaseURL)
			}

			if config.Timeout != 30*time.Second {
				t.Errorf("Timeout = %v, want 30s", config.Timeout)
			}

			if tt.wantAuth && config.Auth == nil {
				t.Error("Expected auth config, got nil")
			}

			if !tt.wantAuth && config.Auth != nil {
				t.Error("Expected no auth config, got non-nil")
			}
		})
	}
}

func TestToAWSTransportConfig(t *testing.T) {
	tests := []struct {
		name    string
		def     *workflow.ConnectorDefinition
		wantErr bool
	}{
		{
			name: "valid AWS config",
			def: &workflow.ConnectorDefinition{
				BaseURL: "https://s3.amazonaws.com",
				AWS: &workflow.AWSConfig{
					Service: "s3",
					Region:  "us-east-1",
				},
			},
			wantErr: false,
		},
		{
			name: "missing AWS config",
			def: &workflow.ConnectorDefinition{
				BaseURL: "https://s3.amazonaws.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := toAWSTransportConfig(tt.def)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if config.Service != tt.def.AWS.Service {
				t.Errorf("Service = %q, want %q", config.Service, tt.def.AWS.Service)
			}

			if config.Region != tt.def.AWS.Region {
				t.Errorf("Region = %q, want %q", config.Region, tt.def.AWS.Region)
			}
		})
	}
}

func TestToOAuth2TransportConfig(t *testing.T) {
	// Set up environment variables for testing
	os.Setenv("TEST_CLIENT_ID", "test-client-id")
	os.Setenv("TEST_CLIENT_SECRET", "test-client-secret")
	defer os.Unsetenv("TEST_CLIENT_ID")
	defer os.Unsetenv("TEST_CLIENT_SECRET")

	tests := []struct {
		name    string
		def     *workflow.ConnectorDefinition
		wantErr bool
	}{
		{
			name: "valid OAuth2 config with explicit flow",
			def: &workflow.ConnectorDefinition{
				BaseURL: "https://api.example.com",
				OAuth2: &workflow.OAuth2Config{
					ClientID:     "${TEST_CLIENT_ID}",
					ClientSecret: "${TEST_CLIENT_SECRET}",
					TokenURL:     "https://auth.example.com/token",
					Flow:         "client_credentials",
					Scopes:       []string{"read", "write"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid OAuth2 config with default flow",
			def: &workflow.ConnectorDefinition{
				BaseURL: "https://api.example.com",
				OAuth2: &workflow.OAuth2Config{
					ClientID:     "${TEST_CLIENT_ID}",
					ClientSecret: "${TEST_CLIENT_SECRET}",
					TokenURL:     "https://auth.example.com/token",
				},
			},
			wantErr: false,
		},
		{
			name: "missing OAuth2 config",
			def: &workflow.ConnectorDefinition{
				BaseURL: "https://api.example.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := toOAuth2TransportConfig(tt.def)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if config.ClientID != tt.def.OAuth2.ClientID {
				t.Errorf("ClientID = %q, want %q", config.ClientID, tt.def.OAuth2.ClientID)
			}

			if config.TokenURL != tt.def.OAuth2.TokenURL {
				t.Errorf("TokenURL = %q, want %q", config.TokenURL, tt.def.OAuth2.TokenURL)
			}

			// Check flow defaults to client_credentials
			expectedFlow := tt.def.OAuth2.Flow
			if expectedFlow == "" {
				expectedFlow = "client_credentials"
			}
			if config.Flow != expectedFlow {
				t.Errorf("Flow = %q, want %q", config.Flow, expectedFlow)
			}
		})
	}
}

func TestUsesEnvVarSyntax(t *testing.T) {
	tests := []struct {
		name string
		auth *workflow.AuthDefinition
		want bool
	}{
		{
			name: "bearer with env var",
			auth: &workflow.AuthDefinition{
				Type:  "bearer",
				Token: "${API_TOKEN}",
			},
			want: true,
		},
		{
			name: "bearer with plain token",
			auth: &workflow.AuthDefinition{
				Type:  "bearer",
				Token: "plain-token",
			},
			want: false,
		},
		{
			name: "basic with env var password",
			auth: &workflow.AuthDefinition{
				Type:     "basic",
				Username: "user",
				Password: "${PASSWORD}",
			},
			want: true,
		},
		{
			name: "basic with plain password",
			auth: &workflow.AuthDefinition{
				Type:     "basic",
				Username: "user",
				Password: "plain-password",
			},
			want: false,
		},
		{
			name: "api_key with env var",
			auth: &workflow.AuthDefinition{
				Type:   "api_key",
				Header: "X-API-Key",
				Value:  "${API_KEY}",
			},
			want: true,
		},
		{
			name: "api_key with plain value",
			auth: &workflow.AuthDefinition{
				Type:   "api_key",
				Header: "X-API-Key",
				Value:  "plain-key",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usesEnvVarSyntax(tt.auth)
			if got != tt.want {
				t.Errorf("usesEnvVarSyntax() = %v, want %v", got, tt.want)
			}
		})
	}
}
