package permissions

import (
	"context"

	"github.com/tombee/conductor/pkg/workflow"
)

// contextKey is the type for permission context keys.
type contextKey int

const (
	// permissionsKey is the context key for permission context.
	permissionsKey contextKey = iota
)

// PermissionContext holds the effective permissions for a step execution.
// These permissions are the result of merging workflow-level and step-level
// permissions according to the intersection (most restrictive wins) rule.
type PermissionContext struct {
	// Paths controls file system access
	Paths *workflow.PathPermissions

	// Network controls network access
	Network *workflow.NetworkPermissions

	// Secrets controls which secrets can be accessed
	Secrets *workflow.SecretPermissions

	// Tools controls which tools can be used
	Tools *workflow.ToolPermissions

	// Shell controls shell command execution
	Shell *workflow.ShellPermissions

	// Env controls environment variable access
	Env *workflow.EnvPermissions
}

// WithPermissions attaches permission context to a context.Context.
func WithPermissions(ctx context.Context, perms *PermissionContext) context.Context {
	return context.WithValue(ctx, permissionsKey, perms)
}

// FromContext retrieves permission context from a context.Context.
// Returns nil if no permissions are set.
func FromContext(ctx context.Context) *PermissionContext {
	perms, _ := ctx.Value(permissionsKey).(*PermissionContext)
	return perms
}

// NewPermissionContext creates a permission context from a workflow definition.
// If permDef is nil, returns permissive defaults (allow all).
func NewPermissionContext(permDef *workflow.PermissionDefinition) *PermissionContext {
	if permDef == nil {
		// Permissive defaults for backward compatibility (DECISION-141-1)
		return &PermissionContext{
			Paths: &workflow.PathPermissions{
				Read:  []string{"**/*"},
				Write: []string{"**/*"},
			},
			Network: &workflow.NetworkPermissions{
				AllowedHosts: []string{}, // Empty = all allowed
				BlockedHosts: defaultBlockedHosts(),
			},
			Secrets: &workflow.SecretPermissions{
				Allowed: []string{"*"},
			},
			Tools: &workflow.ToolPermissions{
				Allowed: []string{"*"},
				Blocked: []string{},
			},
			Shell: &workflow.ShellPermissions{
				Enabled:         boolPtr(false), // Default: disabled
				AllowedCommands: []string{},
			},
			Env: &workflow.EnvPermissions{
				Inherit: false,
				Allowed: defaultEnvVars(),
			},
		}
	}

	return &PermissionContext{
		Paths:   permDef.Paths,
		Network: permDef.Network,
		Secrets: permDef.Secrets,
		Tools:   permDef.Tools,
		Shell:   permDef.Shell,
		Env:     permDef.Env,
	}
}

// defaultBlockedHosts returns the default list of blocked hosts per FR4.5
func defaultBlockedHosts() []string {
	return []string{
		// Cloud metadata endpoints
		"169.254.169.254",    // AWS, GCP, Azure metadata
		"fd00:ec2::254",      // AWS IMDSv2 IPv6
		"metadata.google.internal", // GCP metadata alternate
		// Private IP ranges (CIDR notation)
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		// Loopback
		"127.0.0.0/8",
		"::1",
		// Link-local
		"169.254.0.0/16",
	}
}

// defaultEnvVars returns the default allowed environment variables per FR8.6
func defaultEnvVars() []string {
	return []string{"PATH", "HOME", "USER", "TERM"}
}

func boolPtr(b bool) *bool {
	return &b
}
