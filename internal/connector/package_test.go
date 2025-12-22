package connector

import (
	"testing"

	"github.com/tombee/conductor/pkg/workflow"
)

func TestLoadBundledPackage(t *testing.T) {
	tests := []struct {
		name        string
		from        string
		wantErr     bool
		wantName    string
		wantBaseURL string
	}{
		{
			name:        "load github connector",
			from:        "connectors/github",
			wantErr:     false,
			wantName:    "github",
			wantBaseURL: "https://api.github.com",
		},
		{
			name:        "load slack connector",
			from:        "connectors/slack",
			wantErr:     false,
			wantName:    "slack",
			wantBaseURL: "https://slack.com/api",
		},
		{
			name:        "load jira connector",
			from:        "connectors/jira",
			wantErr:     false,
			wantName:    "jira",
			wantBaseURL: "https://your-domain.atlassian.net",
		},
		{
			name:        "load discord connector",
			from:        "connectors/discord",
			wantErr:     false,
			wantName:    "discord",
			wantBaseURL: "https://discord.com/api/v10",
		},
		{
			name:        "load jenkins connector",
			from:        "connectors/jenkins",
			wantErr:     false,
			wantName:    "jenkins",
			wantBaseURL: "https://jenkins.example.com",
		},
		{
			name:    "invalid format",
			from:    "github",
			wantErr: true,
		},
		{
			name:    "nonexistent connector",
			from:    "connectors/nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, err := loadBundledPackage(tt.from)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadBundledPackage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if pkg.Name != tt.wantName {
				t.Errorf("loadBundledPackage() name = %v, want %v", pkg.Name, tt.wantName)
			}

			if pkg.BaseURL != tt.wantBaseURL {
				t.Errorf("loadBundledPackage() base_url = %v, want %v", pkg.BaseURL, tt.wantBaseURL)
			}

			if len(pkg.Operations) == 0 {
				t.Errorf("loadBundledPackage() has no operations")
			}
		})
	}
}

func TestMergePackageWithOverrides(t *testing.T) {
	// Create a mock package
	pkg := &PackageDefinition{
		Version:     "1.0",
		Name:        "test",
		Description: "Test connector",
		BaseURL:     "https://api.example.com",
		Headers: map[string]string{
			"Accept":     "application/json",
			"User-Agent": "conductor",
		},
		RateLimit: &workflow.RateLimitConfig{
			RequestsPerSecond: 10,
			RequestsPerMinute: 100,
		},
		Operations: map[string]workflow.OperationDefinition{
			"test_op": {
				Method: "GET",
				Path:   "/test",
			},
		},
	}

	tests := []struct {
		name        string
		userDef     *workflow.ConnectorDefinition
		wantBaseURL string
		wantHeaders int
	}{
		{
			name: "no overrides",
			userDef: &workflow.ConnectorDefinition{
				Name: "my_connector",
				From: "connectors/test",
			},
			wantBaseURL: "https://api.example.com",
			wantHeaders: 2, // package headers
		},
		{
			name: "override base_url",
			userDef: &workflow.ConnectorDefinition{
				Name:    "my_connector",
				From:    "connectors/test",
				BaseURL: "https://custom.example.com",
			},
			wantBaseURL: "https://custom.example.com",
			wantHeaders: 2,
		},
		{
			name: "add custom headers",
			userDef: &workflow.ConnectorDefinition{
				Name: "my_connector",
				From: "connectors/test",
				Headers: map[string]string{
					"Authorization": "Bearer token",
					"Accept":        "application/vnd.api+json", // override package header
				},
			},
			wantBaseURL: "https://api.example.com",
			wantHeaders: 3, // 2 package + 1 custom (Accept is overridden)
		},
		{
			name: "override rate limit",
			userDef: &workflow.ConnectorDefinition{
				Name: "my_connector",
				From: "connectors/test",
				RateLimit: &workflow.RateLimitConfig{
					RequestsPerSecond: 1,
					RequestsPerMinute: 20,
				},
			},
			wantBaseURL: "https://api.example.com",
			wantHeaders: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := mergePackageWithOverrides(pkg, tt.userDef)

			if merged.BaseURL != tt.wantBaseURL {
				t.Errorf("mergePackageWithOverrides() base_url = %v, want %v", merged.BaseURL, tt.wantBaseURL)
			}

			if len(merged.Headers) != tt.wantHeaders {
				t.Errorf("mergePackageWithOverrides() headers count = %v, want %v", len(merged.Headers), tt.wantHeaders)
			}

			// Operations should always come from package
			if len(merged.Operations) != len(pkg.Operations) {
				t.Errorf("mergePackageWithOverrides() operations count = %v, want %v", len(merged.Operations), len(pkg.Operations))
			}

			// Name and From should be preserved
			if merged.Name != tt.userDef.Name {
				t.Errorf("mergePackageWithOverrides() name = %v, want %v", merged.Name, tt.userDef.Name)
			}

			if merged.From != tt.userDef.From {
				t.Errorf("mergePackageWithOverrides() from = %v, want %v", merged.From, tt.userDef.From)
			}
		})
	}
}

func TestNewPackageConnector(t *testing.T) {
	tests := []struct {
		name    string
		def     *workflow.ConnectorDefinition
		wantErr bool
	}{
		{
			name: "github connector with auth",
			def: &workflow.ConnectorDefinition{
				Name: "github",
				From: "connectors/github",
				Auth: &workflow.AuthDefinition{
					Type:  "bearer",
					Token: "ghp_test123",
				},
			},
			wantErr: false,
		},
		{
			name: "github enterprise with custom base_url",
			def: &workflow.ConnectorDefinition{
				Name:    "github_enterprise",
				From:    "connectors/github",
				BaseURL: "https://github.mycompany.com/api/v3",
				Auth: &workflow.AuthDefinition{
					Type:  "bearer",
					Token: "ghp_test123",
				},
			},
			wantErr: false,
		},
		{
			name: "nonexistent connector",
			def: &workflow.ConnectorDefinition{
				Name: "invalid",
				From: "connectors/invalid",
			},
			wantErr: true,
		},
	}

	config := DefaultConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector, err := newPackageConnector(tt.def, config)
			if (err != nil) != tt.wantErr {
				t.Errorf("newPackageConnector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if connector == nil {
				t.Error("newPackageConnector() returned nil connector")
				return
			}

			if connector.Name() != tt.def.Name {
				t.Errorf("connector.Name() = %v, want %v", connector.Name(), tt.def.Name)
			}
		})
	}
}
