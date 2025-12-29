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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspacesYAMLParsing(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		checks  func(t *testing.T, cfg *Config)
	}{
		{
			name: "simple workspace with single profile",
			yaml: `
workspaces:
  default:
    profiles:
      default:
        name: default
        description: Default profile
        inherit_env: true
`,
			wantErr: false,
			checks: func(t *testing.T, cfg *Config) {
				require.NotNil(t, cfg.Workspaces)
				require.Contains(t, cfg.Workspaces, "default")
				ws := cfg.Workspaces["default"]
				require.Contains(t, ws.Profiles, "default")
				profile := ws.Profiles["default"]
				assert.Equal(t, "default", profile.Name)
				assert.Equal(t, "Default profile", profile.Description)
				assert.True(t, profile.InheritEnv.Enabled)
			},
		},
		{
			name: "workspace with multiple profiles",
			yaml: `
workspaces:
  frontend:
    description: Frontend team workspace
    default_profile: dev
    profiles:
      dev:
        name: dev
        description: Development environment
        inherit_env: true
        bindings:
          integrations:
            github:
              auth:
                token: ${GITHUB_DEV_TOKEN}
      prod:
        name: prod
        description: Production environment
        inherit_env: false
        bindings:
          integrations:
            github:
              auth:
                token: env:GITHUB_PROD_TOKEN
`,
			wantErr: false,
			checks: func(t *testing.T, cfg *Config) {
				require.NotNil(t, cfg.Workspaces)
				require.Contains(t, cfg.Workspaces, "frontend")
				ws := cfg.Workspaces["frontend"]
				assert.Equal(t, "Frontend team workspace", ws.Description)
				assert.Equal(t, "dev", ws.DefaultProfile)
				require.Contains(t, ws.Profiles, "dev")
				require.Contains(t, ws.Profiles, "prod")

				// Check dev profile
				devProfile := ws.Profiles["dev"]
				assert.Equal(t, "dev", devProfile.Name)
				assert.True(t, devProfile.InheritEnv.Enabled)
				require.NotNil(t, devProfile.Bindings.Integrations)
				require.Contains(t, devProfile.Bindings.Integrations, "github")
				assert.Equal(t, "${GITHUB_DEV_TOKEN}", devProfile.Bindings.Integrations["github"].Auth.Token)

				// Check prod profile
				prodProfile := ws.Profiles["prod"]
				assert.Equal(t, "prod", prodProfile.Name)
				assert.False(t, prodProfile.InheritEnv.Enabled)
				require.NotNil(t, prodProfile.Bindings.Integrations)
				require.Contains(t, prodProfile.Bindings.Integrations, "github")
				assert.Equal(t, "env:GITHUB_PROD_TOKEN", prodProfile.Bindings.Integrations["github"].Auth.Token)
			},
		},
		{
			name: "profile with inherit_env allowlist",
			yaml: `
workspaces:
  default:
    profiles:
      restricted:
        name: restricted
        inherit_env:
          enabled: true
          allowlist:
            - CONDUCTOR_*
            - CI
            - GITHUB_*
        bindings:
          integrations:
            github:
              auth:
                token: ${GITHUB_TOKEN}
`,
			wantErr: false,
			checks: func(t *testing.T, cfg *Config) {
				require.NotNil(t, cfg.Workspaces)
				require.Contains(t, cfg.Workspaces, "default")
				ws := cfg.Workspaces["default"]
				require.Contains(t, ws.Profiles, "restricted")
				profile := ws.Profiles["restricted"]
				assert.True(t, profile.InheritEnv.Enabled)
				require.NotNil(t, profile.InheritEnv.Allowlist)
				assert.Equal(t, []string{"CONDUCTOR_*", "CI", "GITHUB_*"}, profile.InheritEnv.Allowlist)
			},
		},
		{
			name: "profile with MCP server bindings",
			yaml: `
workspaces:
  default:
    profiles:
      default:
        name: default
        bindings:
          mcp_servers:
            code-analyzer:
              command: npx
              args: ["-y", "@acme/analyzer"]
              env:
                API_KEY: ${ANALYZER_KEY}
                TIMEOUT: "30"
              timeout: 60
`,
			wantErr: false,
			checks: func(t *testing.T, cfg *Config) {
				require.NotNil(t, cfg.Workspaces)
				require.Contains(t, cfg.Workspaces, "default")
				ws := cfg.Workspaces["default"]
				require.Contains(t, ws.Profiles, "default")
				profile := ws.Profiles["default"]
				require.NotNil(t, profile.Bindings.MCPServers)
				require.Contains(t, profile.Bindings.MCPServers, "code-analyzer")
				mcp := profile.Bindings.MCPServers["code-analyzer"]
				assert.Equal(t, "npx", mcp.Command)
				assert.Equal(t, []string{"-y", "@acme/analyzer"}, mcp.Args)
				assert.Equal(t, "${ANALYZER_KEY}", mcp.Env["API_KEY"])
				assert.Equal(t, "30", mcp.Env["TIMEOUT"])
				assert.Equal(t, 60, mcp.Timeout)
			},
		},
		{
			name: "profile with integration headers and base URL",
			yaml: `
workspaces:
  default:
    profiles:
      custom:
        name: custom
        bindings:
          integrations:
            api:
              base_url: https://api.example.com
              auth:
                header: X-API-Key
                value: ${API_KEY}
              headers:
                X-Custom-Header: "custom-value"
`,
			wantErr: false,
			checks: func(t *testing.T, cfg *Config) {
				require.NotNil(t, cfg.Workspaces)
				ws := cfg.Workspaces["default"]
				profile := ws.Profiles["custom"]
				integration := profile.Bindings.Integrations["api"]
				assert.Equal(t, "https://api.example.com", integration.BaseURL)
				assert.Equal(t, "X-API-Key", integration.Auth.Header)
				assert.Equal(t, "${API_KEY}", integration.Auth.Value)
				assert.Equal(t, "custom-value", integration.Headers["X-Custom-Header"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test-config.yaml")
			err := os.WriteFile(configPath, []byte(tt.yaml), 0600)
			require.NoError(t, err)

			// Load configuration
			cfg, err := Load(configPath)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)

			// Run custom checks
			if tt.checks != nil {
				tt.checks(t, cfg)
			}
		})
	}
}

func TestDefaultWorkspace(t *testing.T) {
	cfg := Default()
	require.NotNil(t, cfg.Workspaces)
	require.Contains(t, cfg.Workspaces, "default")

	ws := cfg.Workspaces["default"]
	assert.Equal(t, "default", ws.Name)
	assert.Equal(t, "default", ws.DefaultProfile)
	require.Contains(t, ws.Profiles, "default")

	profile := ws.Profiles["default"]
	assert.Equal(t, "default", profile.Name)
	assert.True(t, profile.InheritEnv.Enabled)
	assert.Nil(t, profile.InheritEnv.Allowlist)
}

func TestApplyDefaultsCreatesWorkspace(t *testing.T) {
	// Create minimal config without workspaces
	cfg := &Config{
		Log: LogConfig{Level: "info"},
	}

	cfg.applyDefaults()

	// Should have default workspace
	require.NotNil(t, cfg.Workspaces)
	require.Contains(t, cfg.Workspaces, "default")
}

func TestLoadMinimalConfigCreatesDefaultWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "minimal.yaml")
	yaml := `
log:
  level: info
`
	err := os.WriteFile(configPath, []byte(yaml), 0600)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Should have default workspace
	require.NotNil(t, cfg.Workspaces)
	require.Contains(t, cfg.Workspaces, "default")
	ws := cfg.Workspaces["default"]
	require.Contains(t, ws.Profiles, "default")
}
