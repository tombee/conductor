package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestCheckSecret(t *testing.T) {
	tests := []struct {
		name       string
		permCtx    *PermissionContext
		secretName string
		wantError  bool
		errorType  string
	}{
		{
			name:       "nil context allows everything",
			permCtx:    nil,
			secretName: "API_KEY",
			wantError:  false,
		},
		{
			name: "exact match allowed",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"API_KEY"},
				},
			},
			secretName: "API_KEY",
			wantError:  false,
		},
		{
			name: "wildcard pattern matches",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"API_*"},
				},
			},
			secretName: "API_KEY",
			wantError:  false,
		},
		{
			name: "double star pattern matches",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"**/API_KEY"},
				},
			},
			secretName: "prod/API_KEY",
			wantError:  false,
		},
		{
			name: "secret not in allowed list",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"API_KEY"},
				},
			},
			secretName: "DATABASE_PASSWORD",
			wantError:  true,
			errorType:  "secrets.access",
		},
		{
			name: "empty allowed list denies all",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{},
				},
			},
			secretName: "API_KEY",
			wantError:  true,
			errorType:  "secrets.access",
		},
		{
			name: "multiple patterns - matches first",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"API_*", "DB_*"},
				},
			},
			secretName: "API_KEY",
			wantError:  false,
		},
		{
			name: "multiple patterns - matches second",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"API_*", "DB_*"},
				},
			},
			secretName: "DB_PASSWORD",
			wantError:  false,
		},
		{
			name: "multiple patterns - matches none",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"API_*", "DB_*"},
				},
			},
			secretName: "GITHUB_TOKEN",
			wantError:  true,
			errorType:  "secrets.access",
		},
		{
			name: "case sensitive matching",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"api_key"},
				},
			},
			secretName: "API_KEY",
			wantError:  true,
			errorType:  "secrets.access",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckSecret(tt.permCtx, tt.secretName)
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

func TestFilterAllowedSecrets(t *testing.T) {
	tests := []struct {
		name        string
		permCtx     *PermissionContext
		secretNames []string
		expected    []string
	}{
		{
			name:        "nil context returns all secrets",
			permCtx:     nil,
			secretNames: []string{"API_KEY", "DB_PASSWORD", "GITHUB_TOKEN"},
			expected:    []string{"API_KEY", "DB_PASSWORD", "GITHUB_TOKEN"},
		},
		{
			name: "filter by exact match",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"API_KEY"},
				},
			},
			secretNames: []string{"API_KEY", "DB_PASSWORD", "GITHUB_TOKEN"},
			expected:    []string{"API_KEY"},
		},
		{
			name: "filter by wildcard pattern",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"API_*"},
				},
			},
			secretNames: []string{"API_KEY", "API_SECRET", "DB_PASSWORD", "GITHUB_TOKEN"},
			expected:    []string{"API_KEY", "API_SECRET"},
		},
		{
			name: "filter by multiple patterns",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"API_*", "DB_*"},
				},
			},
			secretNames: []string{"API_KEY", "DB_PASSWORD", "DB_USER", "GITHUB_TOKEN"},
			expected:    []string{"API_KEY", "DB_PASSWORD", "DB_USER"},
		},
		{
			name: "empty allowed list returns empty",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{},
				},
			},
			secretNames: []string{"API_KEY", "DB_PASSWORD"},
			expected:    []string{},
		},
		{
			name: "no matching secrets returns empty",
			permCtx: &PermissionContext{
				Secrets: &workflow.SecretPermissions{
					Allowed: []string{"NONEXISTENT_*"},
				},
			},
			secretNames: []string{"API_KEY", "DB_PASSWORD"},
			expected:    []string{},
		},
		{
			name:        "empty secret list returns empty",
			permCtx:     nil,
			secretNames: []string{},
			expected:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterAllowedSecrets(tt.permCtx, tt.secretNames)
			assert.Equal(t, tt.expected, result)
		})
	}
}
