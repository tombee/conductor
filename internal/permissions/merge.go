package permissions

import (
	"github.com/tombee/conductor/pkg/workflow"
)

// Merge combines workflow-level and step-level permissions using the intersection
// (most restrictive wins) strategy per FR2.
//
// Precedence (most restrictive wins):
// 1. Step-level permissions
// 2. Workflow-level permissions
// 3. Default permissions
func Merge(workflowPerms, stepPerms *workflow.PermissionDefinition) *PermissionContext {
	// Start with workflow permissions (or defaults if nil)
	effective := NewPermissionContext(workflowPerms)

	// If no step permissions, return workflow/default permissions
	if stepPerms == nil {
		return effective
	}

	// Merge each permission type
	effective.Paths = mergePaths(effective.Paths, stepPerms.Paths)
	effective.Network = mergeNetwork(effective.Network, stepPerms.Network)
	effective.Secrets = mergeSecrets(effective.Secrets, stepPerms.Secrets)
	effective.Tools = mergeTools(effective.Tools, stepPerms.Tools)
	effective.Shell = mergeShell(effective.Shell, stepPerms.Shell)
	effective.Env = mergeEnv(effective.Env, stepPerms.Env)

	return effective
}

// mergePaths merges path permissions using intersection for allowed paths.
func mergePaths(wf, step *workflow.PathPermissions) *workflow.PathPermissions {
	if step == nil {
		return wf
	}

	result := &workflow.PathPermissions{}

	// Merge read patterns: intersection (step can only narrow)
	// nil = not set (inherit from workflow)
	// empty slice = explicitly deny all (most restrictive)
	// non-empty = intersect with workflow patterns
	if step.Read == nil {
		// Not set - inherit from workflow
		result.Read = wf.Read
	} else if len(step.Read) == 0 {
		// Explicitly empty - deny all reads
		result.Read = []string{}
	} else {
		// Intersect with workflow
		result.Read = intersectPatterns(wf.Read, step.Read)
	}

	// Merge write patterns: same logic as read
	if step.Write == nil {
		// Not set - inherit from workflow
		result.Write = wf.Write
	} else if len(step.Write) == 0 {
		// Explicitly empty - deny all writes
		result.Write = []string{}
	} else {
		// Intersect with workflow
		result.Write = intersectPatterns(wf.Write, step.Write)
	}

	return result
}

// mergeNetwork merges network permissions.
func mergeNetwork(wf, step *workflow.NetworkPermissions) *workflow.NetworkPermissions {
	if step == nil {
		return wf
	}

	result := &workflow.NetworkPermissions{}

	// Merge allowed_hosts: intersection (step can only narrow)
	// Note: for network, we check length since Go doesn't distinguish nil vs empty slice in YAML
	if len(step.AllowedHosts) > 0 {
		result.AllowedHosts = intersectPatterns(wf.AllowedHosts, step.AllowedHosts)
	} else {
		result.AllowedHosts = wf.AllowedHosts
	}

	// Merge blocked_hosts: union (cumulative blocking)
	result.BlockedHosts = unionPatterns(wf.BlockedHosts, step.BlockedHosts)

	return result
}

// mergeSecrets merges secret permissions using intersection.
func mergeSecrets(wf, step *workflow.SecretPermissions) *workflow.SecretPermissions {
	if step == nil {
		return wf
	}

	result := &workflow.SecretPermissions{}

	// Merge allowed: intersection (step can only narrow)
	// Note: for secrets, we check length since Go doesn't distinguish nil vs empty slice in YAML
	if len(step.Allowed) > 0 {
		result.Allowed = intersectPatterns(wf.Allowed, step.Allowed)
	} else {
		result.Allowed = wf.Allowed
	}

	return result
}

// mergeTools merges tool permissions.
func mergeTools(wf, step *workflow.ToolPermissions) *workflow.ToolPermissions {
	if step == nil {
		return wf
	}

	result := &workflow.ToolPermissions{}

	// Merge allowed: intersection (step can only narrow)
	// Note: for tools, we check length since Go doesn't distinguish nil vs empty slice in YAML
	if len(step.Allowed) > 0 {
		result.Allowed = intersectPatterns(wf.Allowed, step.Allowed)
	} else {
		result.Allowed = wf.Allowed
	}

	// Merge blocked: union (cumulative blocking)
	result.Blocked = unionPatterns(wf.Blocked, step.Blocked)

	return result
}

// mergeShell merges shell permissions.
func mergeShell(wf, step *workflow.ShellPermissions) *workflow.ShellPermissions {
	if step == nil {
		return wf
	}

	result := &workflow.ShellPermissions{}

	// Merge enabled: most restrictive wins (false if either is false)
	wfEnabled := wf != nil && wf.IsShellEnabled()
	stepEnabled := step.IsShellEnabled()
	enabled := wfEnabled && stepEnabled
	result.Enabled = &enabled

	// Merge allowed_commands: intersection (step can only narrow)
	if wf != nil && len(wf.AllowedCommands) > 0 {
		if len(step.AllowedCommands) > 0 {
			result.AllowedCommands = intersectCommands(wf.AllowedCommands, step.AllowedCommands)
		} else {
			result.AllowedCommands = wf.AllowedCommands
		}
	} else {
		result.AllowedCommands = step.AllowedCommands
	}

	return result
}

// mergeEnv merges environment variable permissions.
func mergeEnv(wf, step *workflow.EnvPermissions) *workflow.EnvPermissions {
	if step == nil {
		return wf
	}

	result := &workflow.EnvPermissions{}

	// Merge inherit: most restrictive wins (false if either is false)
	result.Inherit = wf.Inherit && step.Inherit

	// Merge allowed: intersection (step can only narrow)
	// Note: for env, we check length since Go doesn't distinguish nil vs empty slice in YAML
	if len(step.Allowed) > 0 {
		result.Allowed = intersectPatterns(wf.Allowed, step.Allowed)
	} else {
		result.Allowed = wf.Allowed
	}

	return result
}

// intersectPatterns returns patterns that appear in both lists.
// For intersection logic:
// - If both are empty: return empty
// - If one is empty but was explicitly set: return empty (most restrictive)
// - Otherwise: return patterns that exist in both lists
//
// Note: This function doesn't distinguish between "not set" (nil) and "set to empty" ([]).
// The caller (merge functions) handles that distinction.
func intersectPatterns(a, b []string) []string {
	// If either is empty, intersection is empty
	if len(a) == 0 || len(b) == 0 {
		return []string{}
	}

	// Build a set from list a
	setA := make(map[string]bool)
	for _, pattern := range a {
		setA[pattern] = true
	}

	// Find patterns that exist in both
	result := []string{}
	for _, pattern := range b {
		if setA[pattern] {
			result = append(result, pattern)
		}
	}

	return result
}

// unionPatterns returns patterns from both lists (deduplicated).
func unionPatterns(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, pattern := range a {
		if !seen[pattern] {
			result = append(result, pattern)
			seen[pattern] = true
		}
	}

	for _, pattern := range b {
		if !seen[pattern] {
			result = append(result, pattern)
			seen[pattern] = true
		}
	}

	return result
}

// intersectCommands returns commands that appear in both lists (prefix-based).
func intersectCommands(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}

	// For commands, we use exact match since they are prefixes
	setA := make(map[string]bool)
	for _, cmd := range a {
		setA[cmd] = true
	}

	var result []string
	for _, cmd := range b {
		if setA[cmd] {
			result = append(result, cmd)
		}
	}

	return result
}
