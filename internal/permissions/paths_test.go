package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestCheckPathRead(t *testing.T) {
	tests := []struct {
		name      string
		ctx       *PermissionContext
		path      string
		wantError bool
	}{
		{
			name:      "nil context allows everything",
			ctx:       nil,
			path:      "/etc/passwd",
			wantError: false,
		},
		{
			name: "exact match allowed",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Read: []string{"src/main.go"},
				},
			},
			path:      "src/main.go",
			wantError: false,
		},
		{
			name: "glob pattern match allowed",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Read: []string{"src/**/*.go"},
				},
			},
			path:      "src/foo/bar.go",
			wantError: false,
		},
		{
			name: "double star recursive match",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Read: []string{"src/**"},
				},
			},
			path:      "src/deep/nested/file.go",
			wantError: false,
		},
		{
			name: "path not in allowed patterns",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Read: []string{"src/**"},
				},
			},
			path:      "etc/passwd",
			wantError: true,
		},
		{
			name: "empty read patterns denies all",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Read: []string{},
				},
			},
			path:      "src/main.go",
			wantError: true,
		},
		{
			name: "multiple patterns - matches first",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Read: []string{"src/**", "docs/**"},
				},
			},
			path:      "src/main.go",
			wantError: false,
		},
		{
			name: "multiple patterns - matches second",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Read: []string{"src/**", "docs/**"},
				},
			},
			path:      "docs/README.md",
			wantError: false,
		},
		{
			name: "multiple patterns - matches none",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Read: []string{"src/**", "docs/**"},
				},
			},
			path:      "etc/passwd",
			wantError: true,
		},
		{
			name: "wildcard pattern",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Read: []string{"*.txt"},
				},
			},
			path:      "readme.txt",
			wantError: false,
		},
		{
			name: "leading dot-slash normalization",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Read: []string{"src/**"},
				},
			},
			path:      "./src/main.go",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPathRead(tt.ctx, tt.path)
			if tt.wantError {
				assert.Error(t, err)
				assert.True(t, IsPermissionError(err), "error should be a PermissionError")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckPathWrite(t *testing.T) {
	tests := []struct {
		name      string
		ctx       *PermissionContext
		path      string
		wantError bool
	}{
		{
			name:      "nil context allows everything",
			ctx:       nil,
			path:      "/etc/passwd",
			wantError: false,
		},
		{
			name: "exact match allowed",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Write: []string{"output.txt"},
				},
			},
			path:      "output.txt",
			wantError: false,
		},
		{
			name: "glob pattern match allowed",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Write: []string{"$out/**"},
				},
			},
			path:      "$out/report.json",
			wantError: false,
		},
		{
			name: "path not in allowed patterns",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Write: []string{"$out/**"},
				},
			},
			path:      "/etc/passwd",
			wantError: true,
		},
		{
			name: "empty write patterns denies all",
			ctx: &PermissionContext{
				Paths: &workflow.PathPermissions{
					Write: []string{},
				},
			},
			path:      "$out/file.txt",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPathWrite(tt.ctx, tt.path)
			if tt.wantError {
				assert.Error(t, err)
				assert.True(t, IsPermissionError(err), "error should be a PermissionError")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPermissionError(t *testing.T) {
	err := &PermissionError{
		Type:     "paths.read",
		Resource: "etc/passwd",
		Allowed:  []string{"src/**", "docs/**"},
		Message:  "path not in allowed read patterns",
	}

	errMsg := err.Error()
	assert.Contains(t, errMsg, "permission denied")
	assert.Contains(t, errMsg, "paths.read")
	assert.Contains(t, errMsg, "etc/passwd")
	assert.Contains(t, errMsg, "src/**")
	assert.Contains(t, errMsg, "docs/**")
	assert.Contains(t, errMsg, "path not in allowed read patterns")
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "already normalized",
			path:     "src/main.go",
			expected: "src/main.go",
		},
		{
			name:     "leading dot-slash removed",
			path:     "./src/main.go",
			expected: "src/main.go",
		},
		{
			name:     "backslashes converted to forward slashes",
			path:     "src\\foo\\bar.go",
			expected: "src/foo/bar.go",
		},
		{
			name:     "combined normalization",
			path:     ".\\src\\main.go",
			expected: "src/main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPermissionErrorNoInformationLeakage(t *testing.T) {
	// Test that permission errors don't reveal whether a file exists
	err := &PermissionError{
		Type:     "paths.read",
		Resource: "/secret/nonexistent/file.txt",
		Allowed:  []string{"src/**"},
		Message:  "path not in allowed read patterns",
	}

	errMsg := err.Error()

	// Error should NOT contain words that reveal file existence
	assert.NotContains(t, errMsg, "not found")
	assert.NotContains(t, errMsg, "does not exist")
	assert.NotContains(t, errMsg, "exists")

	// Error SHOULD contain permission-related information
	assert.Contains(t, errMsg, "permission denied")
	assert.Contains(t, errMsg, "allowed patterns")
}
