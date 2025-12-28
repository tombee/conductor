package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestCheckTool(t *testing.T) {
	tests := []struct {
		name      string
		permCtx   *PermissionContext
		toolName  string
		wantError bool
		errorType string
	}{
		{
			name:      "nil context allows everything",
			permCtx:   nil,
			toolName:  "file.read",
			wantError: false,
		},
		{
			name: "exact match allowed",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"file.read"},
				},
			},
			toolName:  "file.read",
			wantError: false,
		},
		{
			name: "wildcard pattern matches",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"file.*"},
				},
			},
			toolName:  "file.read",
			wantError: false,
		},
		{
			name: "wildcard pattern matches write",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"file.*"},
				},
			},
			toolName:  "file.write",
			wantError: false,
		},
		{
			name: "tool not in allowed list",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"file.*"},
				},
			},
			toolName:  "shell.run",
			wantError: true,
			errorType: "tools.denied",
		},
		{
			name: "blocked tool with ! prefix",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"*"},
					Blocked: []string{"!shell.run"},
				},
			},
			toolName:  "shell.run",
			wantError: true,
			errorType: "tools.blocked",
		},
		{
			name: "blocked tool without ! prefix",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"*"},
					Blocked: []string{"shell.run"},
				},
			},
			toolName:  "shell.run",
			wantError: true,
			errorType: "tools.blocked",
		},
		{
			name: "blocked takes precedence over allowed",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"shell.*"},
					Blocked: []string{"shell.run"},
				},
			},
			toolName:  "shell.run",
			wantError: true,
			errorType: "tools.blocked",
		},
		{
			name: "blocked wildcard pattern",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"*"},
					Blocked: []string{"shell.*"},
				},
			},
			toolName:  "shell.run",
			wantError: true,
			errorType: "tools.blocked",
		},
		{
			name: "empty allowed list allows all (except blocked)",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{},
					Blocked: []string{"shell.run"},
				},
			},
			toolName:  "file.read",
			wantError: false,
		},
		{
			name: "multiple allowed patterns",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"file.*", "http.*"},
				},
			},
			toolName:  "http.request",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckTool(tt.permCtx, tt.toolName)
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorType != "" {
					permErr, ok := err.(*PermissionError)
					assert.True(t, ok, "expected PermissionError")
					assert.Equal(t, tt.errorType, permErr.Type)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMatchesToolPattern(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		pattern  string
		expected bool
	}{
		{
			name:     "exact match",
			toolName: "file.read",
			pattern:  "file.read",
			expected: true,
		},
		{
			name:     "exact match fails",
			toolName: "file.read",
			pattern:  "file.write",
			expected: false,
		},
		{
			name:     "wildcard namespace",
			toolName: "file.read",
			pattern:  "file.*",
			expected: true,
		},
		{
			name:     "wildcard all",
			toolName: "anything",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "wildcard mismatch",
			toolName: "shell.run",
			pattern:  "file.*",
			expected: false,
		},
		{
			name:     "nested namespace",
			toolName: "mcp.server.tool",
			pattern:  "mcp.*",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesToolPattern(tt.toolName, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterAllowedTools(t *testing.T) {
	tests := []struct {
		name      string
		permCtx   *PermissionContext
		toolNames []string
		expected  []string
	}{
		{
			name:      "nil context returns all tools",
			permCtx:   nil,
			toolNames: []string{"file.read", "file.write", "shell.run"},
			expected:  []string{"file.read", "file.write", "shell.run"},
		},
		{
			name: "filter by exact match",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"file.read"},
				},
			},
			toolNames: []string{"file.read", "file.write", "shell.run"},
			expected:  []string{"file.read"},
		},
		{
			name: "filter by wildcard pattern",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"file.*"},
				},
			},
			toolNames: []string{"file.read", "file.write", "shell.run", "http.request"},
			expected:  []string{"file.read", "file.write"},
		},
		{
			name: "filter with blocked list",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"*"},
					Blocked: []string{"shell.*"},
				},
			},
			toolNames: []string{"file.read", "shell.run", "http.request"},
			expected:  []string{"file.read", "http.request"},
		},
		{
			name: "filter with multiple patterns",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{"file.*", "http.*"},
				},
			},
			toolNames: []string{"file.read", "http.request", "shell.run", "db.query"},
			expected:  []string{"file.read", "http.request"},
		},
		{
			name: "empty allowed list returns all (except blocked)",
			permCtx: &PermissionContext{
				Tools: &workflow.ToolPermissions{
					Allowed: []string{},
					Blocked: []string{"shell.run"},
				},
			},
			toolNames: []string{"file.read", "shell.run", "http.request"},
			expected:  []string{"file.read", "http.request"},
		},
		{
			name:      "empty tool list returns empty",
			permCtx:   nil,
			toolNames: []string{},
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterAllowedTools(tt.permCtx, tt.toolNames)
			assert.Equal(t, tt.expected, result)
		})
	}
}
