package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermissionDefinitionParsing tests YAML unmarshaling of permission definitions
func TestPermissionDefinitionParsing(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantError bool
		validate  func(t *testing.T, def *Definition)
	}{
		{
			name: "workflow-level permissions with all fields",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  paths:
    read: ["src/**", "docs/**"]
    write: ["$out/**"]
  network:
    allowed_hosts: ["api.github.com", "*.openai.com"]
    blocked_hosts: ["169.254.169.254", "*.internal"]
  secrets:
    allowed: ["GITHUB_TOKEN", "OPENAI_API_KEY"]
  tools:
    allowed: ["file.*", "transform.*"]
    blocked: ["shell.*"]
  shell:
    enabled: false
    allowed_commands: ["git", "npm"]
  env:
    inherit: false
    allowed: ["CI", "PATH", "HOME"]
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: false,
			validate: func(t *testing.T, def *Definition) {
				require.NotNil(t, def.Permissions)
				require.NotNil(t, def.Permissions.Paths)
				assert.Equal(t, []string{"src/**", "docs/**"}, def.Permissions.Paths.Read)
				assert.Equal(t, []string{"$out/**"}, def.Permissions.Paths.Write)

				require.NotNil(t, def.Permissions.Network)
				assert.Equal(t, []string{"api.github.com", "*.openai.com"}, def.Permissions.Network.AllowedHosts)
				assert.Equal(t, []string{"169.254.169.254", "*.internal"}, def.Permissions.Network.BlockedHosts)

				require.NotNil(t, def.Permissions.Secrets)
				assert.Equal(t, []string{"GITHUB_TOKEN", "OPENAI_API_KEY"}, def.Permissions.Secrets.Allowed)

				require.NotNil(t, def.Permissions.Tools)
				assert.Equal(t, []string{"file.*", "transform.*"}, def.Permissions.Tools.Allowed)
				assert.Equal(t, []string{"shell.*"}, def.Permissions.Tools.Blocked)

				require.NotNil(t, def.Permissions.Shell)
				assert.False(t, def.Permissions.Shell.IsShellEnabled())
				assert.Equal(t, []string{"git", "npm"}, def.Permissions.Shell.AllowedCommands)

				require.NotNil(t, def.Permissions.Env)
				assert.False(t, def.Permissions.Env.Inherit)
				assert.Equal(t, []string{"CI", "PATH", "HOME"}, def.Permissions.Env.Allowed)
			},
		},
		{
			name: "step-level permissions",
			yaml: `
name: test-workflow
version: "1.0"
steps:
  - id: restricted_step
    type: llm
    prompt: "test"
    permissions:
      paths:
        read: ["src/**"]
        write: []
      network:
        allowed_hosts: ["api.openai.com"]
      tools:
        allowed: ["file.read"]
        blocked: ["shell.*", "http.*"]
`,
			wantError: false,
			validate: func(t *testing.T, def *Definition) {
				require.Len(t, def.Steps, 1)
				step := def.Steps[0]
				require.NotNil(t, step.Permissions)
				require.NotNil(t, step.Permissions.Paths)
				assert.Equal(t, []string{"src/**"}, step.Permissions.Paths.Read)
				assert.Equal(t, []string{}, step.Permissions.Paths.Write)

				require.NotNil(t, step.Permissions.Network)
				assert.Equal(t, []string{"api.openai.com"}, step.Permissions.Network.AllowedHosts)

				require.NotNil(t, step.Permissions.Tools)
				assert.Equal(t, []string{"file.read"}, step.Permissions.Tools.Allowed)
				assert.Equal(t, []string{"shell.*", "http.*"}, step.Permissions.Tools.Blocked)
			},
		},
		{
			name: "shell enabled true",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  shell:
    enabled: true
    allowed_commands: ["git"]
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: false,
			validate: func(t *testing.T, def *Definition) {
				require.NotNil(t, def.Permissions)
				require.NotNil(t, def.Permissions.Shell)
				assert.True(t, def.Permissions.Shell.IsShellEnabled())
				assert.Equal(t, []string{"git"}, def.Permissions.Shell.AllowedCommands)
			},
		},
		{
			name: "accept_unenforceable flag",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  paths:
    write: ["$out/**"]
  accept_unenforceable: true
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: false,
			validate: func(t *testing.T, def *Definition) {
				require.NotNil(t, def.Permissions)
				assert.True(t, def.Permissions.AcceptUnenforceable)
			},
		},
		{
			name: "accept_unenforceable_for providers",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  paths:
    write: ["$out/**"]
  accept_unenforceable_for: ["claude-code", "aider"]
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: false,
			validate: func(t *testing.T, def *Definition) {
				require.NotNil(t, def.Permissions)
				assert.Equal(t, []string{"claude-code", "aider"}, def.Permissions.AcceptUnenforceableFor)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, def)
			if tt.validate != nil {
				tt.validate(t, def)
			}
		})
	}
}

// TestPermissionValidation tests permission validation logic
func TestPermissionValidation(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantError bool
		errorMsg  string
	}{
		{
			name: "empty glob pattern",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  paths:
    read: [""]
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: true,
			errorMsg:  "empty pattern not allowed",
		},
		{
			name: "empty host pattern",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  network:
    allowed_hosts: [""]
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: true,
			errorMsg:  "empty pattern not allowed",
		},
		{
			name: "host pattern with path separator",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  network:
    allowed_hosts: ["api.github.com/v1"]
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: true,
			errorMsg:  "host pattern should not contain path separators",
		},
		{
			name: "command with path separator",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  shell:
    enabled: true
    allowed_commands: ["/usr/bin/git"]
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: true,
			errorMsg:  "should not contain path separators",
		},
		{
			name: "empty tool pattern",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  tools:
    allowed: [""]
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: true,
			errorMsg:  "empty pattern not allowed",
		},
		{
			name: "negation-only tool pattern",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  tools:
    blocked: ["!"]
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: true,
			errorMsg:  "empty pattern after negation not allowed",
		},
		{
			name: "valid permissions",
			yaml: `
name: test-workflow
version: "1.0"
permissions:
  paths:
    read: ["src/**/*.go", "!src/**/test/**"]
    write: ["$out/**"]
  network:
    allowed_hosts: ["*.github.com", "api.openai.com"]
    blocked_hosts: ["169.254.169.254"]
  secrets:
    allowed: ["GITHUB_*", "OPENAI_API_KEY"]
  tools:
    allowed: ["file.*", "transform.*"]
    blocked: ["!shell.run"]
  shell:
    enabled: false
    allowed_commands: ["git", "npm", "node"]
  env:
    inherit: true
    allowed: ["CI", "PATH"]
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, def)
		})
	}
}

// TestShellPermissionsIsEnabled tests the IsShellEnabled helper
func TestShellPermissionsIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		perms    *ShellPermissions
		expected bool
	}{
		{
			name:     "nil permissions",
			perms:    nil,
			expected: false,
		},
		{
			name:     "nil enabled field",
			perms:    &ShellPermissions{},
			expected: false,
		},
		{
			name: "enabled false",
			perms: &ShellPermissions{
				Enabled: boolPtr(false),
			},
			expected: false,
		},
		{
			name: "enabled true",
			perms: &ShellPermissions{
				Enabled: boolPtr(true),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.perms.IsShellEnabled())
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
