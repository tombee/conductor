package permissions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestCheckNetwork(t *testing.T) {
	tests := []struct {
		name      string
		permCtx   *PermissionContext
		host      string
		wantError bool
		errorType string
	}{
		{
			name:      "nil context allows everything",
			permCtx:   nil,
			host:      "example.com",
			wantError: false,
		},
		{
			name: "allowed host matches",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"api.example.com"},
				},
			},
			host:      "api.example.com",
			wantError: false,
		},
		{
			name: "wildcard pattern matches",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"*.example.com"},
				},
			},
			host:      "api.example.com",
			wantError: false,
		},
		{
			name: "wildcard pattern matches subdomain",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"*.example.com"},
				},
			},
			host:      "foo.bar.example.com",
			wantError: false,
		},
		{
			name: "host not in allowed list",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"api.example.com"},
				},
			},
			host:      "evil.com",
			wantError: true,
			errorType: "network.host_denied",
		},
		{
			name: "metadata endpoint blocked by default",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"*"},
				},
			},
			host:      "169.254.169.254",
			wantError: true,
			errorType: "network.blocked",
		},
		{
			name: "private IP blocked by default (10.x)",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"*"},
				},
			},
			host:      "10.0.0.1",
			wantError: true,
			errorType: "network.blocked",
		},
		{
			name: "private IP blocked by default (192.168.x)",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"*"},
				},
			},
			host:      "192.168.1.1",
			wantError: true,
			errorType: "network.blocked",
		},
		{
			name: "localhost blocked by default",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"*"},
				},
			},
			host:      "127.0.0.1",
			wantError: true,
			errorType: "network.blocked",
		},
		{
			name: "custom blocked host",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts:  []string{"*"},
					BlockedHosts: []string{"evil.com"},
				},
			},
			host:      "evil.com",
			wantError: true,
			errorType: "network.blocked",
		},
		{
			name: "blocked takes precedence over allowed",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts:  []string{"*.example.com"},
					BlockedHosts: []string{"bad.example.com"},
				},
			},
			host:      "bad.example.com",
			wantError: true,
			errorType: "network.blocked",
		},
		{
			name: "port is stripped for matching",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"api.example.com"},
				},
			},
			host:      "api.example.com:8080",
			wantError: false,
		},
		{
			name: "IPv6 localhost blocked",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{"*"},
				},
			},
			host:      "::1",
			wantError: true,
			errorType: "network.blocked",
		},
		{
			name: "empty allowed list allows all (except blocked)",
			permCtx: &PermissionContext{
				Network: &workflow.NetworkPermissions{
					AllowedHosts: []string{},
				},
			},
			host:      "example.com",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckNetwork(context.Background(), tt.permCtx, tt.host)
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

func TestMatchesHostPattern(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		pattern  string
		expected bool
	}{
		{
			name:     "exact match",
			hostname: "api.example.com",
			pattern:  "api.example.com",
			expected: true,
		},
		{
			name:     "exact match fails",
			hostname: "api.example.com",
			pattern:  "other.example.com",
			expected: false,
		},
		{
			name:     "wildcard single level",
			hostname: "api.example.com",
			pattern:  "*.example.com",
			expected: true,
		},
		{
			name:     "wildcard multi level",
			hostname: "foo.bar.example.com",
			pattern:  "*.example.com",
			expected: true,
		},
		{
			name:     "wildcard all hosts",
			hostname: "anything.com",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "IP exact match",
			hostname: "192.168.1.1",
			pattern:  "192.168.1.1",
			expected: true,
		},
		{
			name:     "CIDR match",
			hostname: "192.168.1.100",
			pattern:  "192.168.1.0/24",
			expected: true,
		},
		{
			name:     "CIDR no match",
			hostname: "192.168.2.100",
			pattern:  "192.168.1.0/24",
			expected: false,
		},
		{
			name:     "CIDR with hostname (no match)",
			hostname: "example.com",
			pattern:  "192.168.1.0/24",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesHostPattern(tt.hostname, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStripPort(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected string
	}{
		{
			name:     "no port",
			host:     "example.com",
			expected: "example.com",
		},
		{
			name:     "with port",
			host:     "example.com:8080",
			expected: "example.com",
		},
		{
			name:     "IPv4 with port",
			host:     "192.168.1.1:8080",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv6 no port",
			host:     "[::1]",
			expected: "::1",
		},
		{
			name:     "IPv6 with port",
			host:     "[::1]:8080",
			expected: "::1",
		},
		{
			name:     "IPv6 full address with port",
			host:     "[2001:db8::1]:443",
			expected: "2001:db8::1",
		},
		{
			name:     "bare IPv6 localhost",
			host:     "::1",
			expected: "::1",
		},
		{
			name:     "bare IPv6 full address",
			host:     "2001:db8::1",
			expected: "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripPort(tt.host)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesCIDR(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		cidr     string
		expected bool
	}{
		{
			name:     "IP in range",
			hostname: "192.168.1.50",
			cidr:     "192.168.1.0/24",
			expected: true,
		},
		{
			name:     "IP out of range",
			hostname: "192.168.2.50",
			cidr:     "192.168.1.0/24",
			expected: false,
		},
		{
			name:     "IPv6 in range",
			hostname: "2001:db8::1",
			cidr:     "2001:db8::/32",
			expected: true,
		},
		{
			name:     "hostname not IP",
			hostname: "example.com",
			cidr:     "192.168.1.0/24",
			expected: false,
		},
		{
			name:     "invalid CIDR",
			hostname: "192.168.1.1",
			cidr:     "invalid",
			expected: false,
		},
		{
			name:     "localhost in loopback range",
			hostname: "127.0.0.1",
			cidr:     "127.0.0.0/8",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesCIDR(tt.hostname, tt.cidr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
