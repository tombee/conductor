package operation_test

import (
	"testing"

	// Import integration package to trigger init() registration
	_ "github.com/tombee/conductor/internal/integration"
	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestNewPackageConnector(t *testing.T) {
	tests := []struct {
		name    string
		def     *workflow.IntegrationDefinition
		wantErr bool
	}{
		{
			name: "github connector with auth",
			def: &workflow.IntegrationDefinition{
				Name: "github",
				From: "integrations/github",
				Auth: &workflow.AuthDefinition{
					Type:  "bearer",
					Token: "ghp_test123",
				},
			},
			wantErr: false,
		},
		{
			name: "github enterprise with custom base_url",
			def: &workflow.IntegrationDefinition{
				Name:    "github", // Note: builtin connectors use their own name
				From:    "integrations/github",
				BaseURL: "https://github.mycompany.com/api/v3",
				Auth: &workflow.AuthDefinition{
					Type:  "bearer",
					Token: "ghp_test123",
				},
			},
			wantErr: false,
		},
		{
			name: "slack connector",
			def: &workflow.IntegrationDefinition{
				Name: "slack",
				From: "integrations/slack",
				Auth: &workflow.AuthDefinition{
					Type:  "bearer",
					Token: "xoxb-test123",
				},
			},
			wantErr: false,
		},
		{
			name: "jira connector",
			def: &workflow.IntegrationDefinition{
				Name:    "jira",
				From:    "integrations/jira",
				BaseURL: "https://mycompany.atlassian.net",
				Auth: &workflow.AuthDefinition{
					Type:     "basic",
					Username: "user@example.com",
					Password: "api-token",
				},
			},
			wantErr: false,
		},
		{
			name: "discord connector",
			def: &workflow.IntegrationDefinition{
				Name: "discord",
				From: "integrations/discord",
				Auth: &workflow.AuthDefinition{
					Type:  "bearer",
					Token: "Bot MjM4NDk0NzU2NTIxMjY",
				},
			},
			wantErr: false,
		},
		{
			name: "jenkins connector",
			def: &workflow.IntegrationDefinition{
				Name:    "jenkins",
				From:    "integrations/jenkins",
				BaseURL: "https://jenkins.example.com",
				Auth: &workflow.AuthDefinition{
					Type:     "basic",
					Username: "admin",
					Password: "api-token",
				},
			},
			wantErr: false,
		},
		{
			name: "nonexistent connector",
			def: &workflow.IntegrationDefinition{
				Name: "invalid",
				From: "integrations/invalid",
			},
			wantErr: true,
		},
	}

	config := operation.DefaultConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := operation.New(tt.def, config)
			if (err != nil) != tt.wantErr {
				t.Errorf("connector.New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if conn == nil {
				t.Error("connector.New() returned nil connector")
				return
			}

			if conn.Name() != tt.def.Name {
				t.Errorf("connector.Name() = %v, want %v", conn.Name(), tt.def.Name)
			}
		})
	}
}
