package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tombee/conductor/pkg/workflow"
)

// TestMerge tests the full permission merge logic
func TestMerge(t *testing.T) {
	tests := []struct {
		name          string
		workflowPerms *workflow.PermissionDefinition
		stepPerms     *workflow.PermissionDefinition
		validate      func(t *testing.T, result *PermissionContext)
	}{
		{
			name:          "nil workflow and step permissions use defaults",
			workflowPerms: nil,
			stepPerms:     nil,
			validate: func(t *testing.T, result *PermissionContext) {
				require.NotNil(t, result)
				assert.Equal(t, []string{"**/*"}, result.Paths.Read)
				assert.Equal(t, []string{"**/*"}, result.Paths.Write)
				assert.Equal(t, []string{"*"}, result.Secrets.Allowed)
			},
		},
		{
			name: "step permissions narrow workflow permissions",
			workflowPerms: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{
					Read:  []string{"src/**", "docs/**"},
					Write: []string{"$out/**"},
				},
			},
			stepPerms: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{
					Read:  []string{"src/**"},
					Write: []string{},
				},
			},
			validate: func(t *testing.T, result *PermissionContext) {
				// Intersection: only "src/**" is in both
				assert.Equal(t, []string{"src/**"}, result.Paths.Read)
				// Empty step write = no writes allowed (empty intersection)
				assert.Equal(t, []string{}, result.Paths.Write)
			},
		},
		{
			name: "network allowed_hosts intersection",
			workflowPerms: &workflow.PermissionDefinition{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"*.github.com", "api.openai.com"},
				},
			},
			stepPerms: &workflow.PermissionDefinition{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"api.openai.com"},
				},
			},
			validate: func(t *testing.T, result *PermissionContext) {
				assert.Equal(t, []string{"api.openai.com"}, result.Network.AllowedHosts)
			},
		},
		{
			name: "network blocked_hosts union",
			workflowPerms: &workflow.PermissionDefinition{
				Network: &workflow.NetworkPermissions{
					BlockedHosts: []string{"*.internal"},
				},
			},
			stepPerms: &workflow.PermissionDefinition{
				Network: &workflow.NetworkPermissions{
					BlockedHosts: []string{"metadata"},
				},
			},
			validate: func(t *testing.T, result *PermissionContext) {
				assert.Contains(t, result.Network.BlockedHosts, "*.internal")
				assert.Contains(t, result.Network.BlockedHosts, "metadata")
			},
		},
		{
			name: "tools allowed intersection and blocked union",
			workflowPerms: &workflow.PermissionDefinition{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"file.*", "transform.*"},
					Blocked: []string{"shell.*"},
				},
			},
			stepPerms: &workflow.PermissionDefinition{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"file.*"},
					Blocked: []string{"http.*"},
				},
			},
			validate: func(t *testing.T, result *PermissionContext) {
				assert.Equal(t, []string{"file.*"}, result.Tools.Allowed)
				assert.Contains(t, result.Tools.Blocked, "shell.*")
				assert.Contains(t, result.Tools.Blocked, "http.*")
			},
		},
		{
			name: "shell enabled: most restrictive wins",
			workflowPerms: &workflow.PermissionDefinition{
				Shell: &workflow.ShellPermissions{
					Enabled: boolPtr(true),
				},
			},
			stepPerms: &workflow.PermissionDefinition{
				Shell: &workflow.ShellPermissions{
					Enabled: boolPtr(false),
				},
			},
			validate: func(t *testing.T, result *PermissionContext) {
				assert.False(t, result.Shell.IsShellEnabled())
			},
		},
		{
			name: "shell commands intersection",
			workflowPerms: &workflow.PermissionDefinition{
				Shell: &workflow.ShellPermissions{
					Enabled:         boolPtr(true),
					AllowedCommands: []string{"git", "npm", "node"},
				},
			},
			stepPerms: &workflow.PermissionDefinition{
				Shell: &workflow.ShellPermissions{
					Enabled:         boolPtr(true),
					AllowedCommands: []string{"git", "npm"},
				},
			},
			validate: func(t *testing.T, result *PermissionContext) {
				assert.True(t, result.Shell.IsShellEnabled())
				assert.ElementsMatch(t, []string{"git", "npm"}, result.Shell.AllowedCommands)
			},
		},
		{
			name: "env inherit: most restrictive wins",
			workflowPerms: &workflow.PermissionDefinition{
				Env: &workflow.EnvPermissions{
					Inherit: true,
					Allowed: []string{"CI", "PATH", "HOME"},
				},
			},
			stepPerms: &workflow.PermissionDefinition{
				Env: &workflow.EnvPermissions{
					Inherit: false,
					Allowed: []string{"CI"},
				},
			},
			validate: func(t *testing.T, result *PermissionContext) {
				assert.False(t, result.Env.Inherit)
				assert.Equal(t, []string{"CI"}, result.Env.Allowed)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Merge(tt.workflowPerms, tt.stepPerms)
			require.NotNil(t, result)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestMergeTruthTable tests the merge truth table from FR2
func TestMergeTruthTable(t *testing.T) {
	tests := []struct {
		name     string
		baseline *workflow.PermissionDefinition
		workflow *workflow.PermissionDefinition
		step     *workflow.PermissionDefinition
		expected []string // expected read paths
	}{
		{
			name: "TC1: Normal restriction",
			baseline: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{Read: []string{"src/**"}},
			},
			workflow: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{Read: []string{"src/**"}},
			},
			step: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{Read: []string{"src/**"}},
			},
			expected: []string{"src/**"},
		},
		{
			name: "TC2: Workflow overly permissive (intersection still applies)",
			baseline: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{Read: []string{"src/**"}},
			},
			workflow: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{Read: []string{"**/*"}},
			},
			step: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{Read: []string{"**/*"}},
			},
			// Note: In the full implementation, baseline would be applied first.
			// For this test, we're testing workflow+step merge only.
			expected: []string{"**/*"},
		},
		{
			name: "TC3: Step exceeds workflow (no overlap = empty)",
			workflow: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{Read: []string{"src/**"}},
			},
			step: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{Read: []string{"/etc/**"}},
			},
			expected: []string{}, // No intersection
		},
		{
			name: "TC5: Empty step inherits workflow",
			workflow: &workflow.PermissionDefinition{
				Paths: &workflow.PathPermissions{Read: []string{"src/**"}},
			},
			step:     nil, // Not specified
			expected: []string{"src/**"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Merge(tt.workflow, tt.step)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, result.Paths.Read)
		})
	}
}

// TestIntersectPatterns tests pattern intersection logic
func TestIntersectPatterns(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "both empty",
			a:        []string{},
			b:        []string{},
			expected: []string{},
		},
		{
			name:     "a empty, b has values - intersection is empty",
			a:        []string{},
			b:        []string{"src/**"},
			expected: []string{},
		},
		{
			name:     "b empty, a has values - intersection is empty",
			a:        []string{"src/**"},
			b:        []string{},
			expected: []string{},
		},
		{
			name:     "exact match",
			a:        []string{"src/**", "docs/**"},
			b:        []string{"src/**"},
			expected: []string{"src/**"},
		},
		{
			name:     "no overlap",
			a:        []string{"src/**"},
			b:        []string{"/etc/**"},
			expected: []string{},
		},
		{
			name:     "partial overlap",
			a:        []string{"src/**", "docs/**", "test/**"},
			b:        []string{"docs/**", "test/**"},
			expected: []string{"docs/**", "test/**"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intersectPatterns(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestUnionPatterns tests pattern union logic
func TestUnionPatterns(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "both empty",
			a:        []string{},
			b:        []string{},
			expected: []string{},
		},
		{
			name:     "a empty, b has values",
			a:        []string{},
			b:        []string{"*.internal"},
			expected: []string{"*.internal"},
		},
		{
			name:     "no duplicates",
			a:        []string{"*.internal"},
			b:        []string{"metadata"},
			expected: []string{"*.internal", "metadata"},
		},
		{
			name:     "with duplicates",
			a:        []string{"*.internal", "metadata"},
			b:        []string{"metadata", "169.254.169.254"},
			expected: []string{"*.internal", "metadata", "169.254.169.254"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unionPatterns(tt.a, tt.b)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}
