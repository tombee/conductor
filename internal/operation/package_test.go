package operation_test

import (
	"testing"

	// Import integration package to trigger init() registration
	_ "github.com/tombee/conductor/internal/integration"
	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestNewPackageIntegration(t *testing.T) {
	tests := []struct {
		name    string
		def     *workflow.IntegrationDefinition
		wantErr bool
	}{
		{
			name: "github integration with auth",
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
				Name:    "github", // Note: builtin integrations use their own name
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
			name: "slack integration",
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
			name: "jira integration",
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
			name: "discord integration",
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
			name: "jenkins integration",
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
			name: "nonexistent integration",
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
			op, err := operation.New(tt.def, config)
			if (err != nil) != tt.wantErr {
				t.Errorf("operation.New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if op == nil {
				t.Error("operation.New() returned nil operation")
				return
			}

			if op.Name() != tt.def.Name {
				t.Errorf("operation.Name() = %v, want %v", op.Name(), tt.def.Name)
			}
		})
	}
}
