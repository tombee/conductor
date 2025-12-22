package permissions

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// CheckTool checks if a tool is allowed for use.
// Returns nil if allowed, error if denied.
// Supports wildcards (file.*, !shell.run) with negation patterns taking precedence.
func CheckTool(ctx *PermissionContext, toolName string) error {
	if ctx == nil || ctx.Tools == nil {
		// No restrictions
		return nil
	}

	// Check blocked list first (takes precedence)
	for _, pattern := range ctx.Tools.Blocked {
		// Remove leading ! if present (blocked patterns may be specified with or without it)
		checkPattern := strings.TrimPrefix(pattern, "!")

		if matchesToolPattern(toolName, checkPattern) {
			return &PermissionError{
				Type:     "tools.blocked",
				Resource: toolName,
				Blocked:  ctx.Tools.Blocked,
				Message:  "tool is in blocked list",
			}
		}
	}

	// If allowed list is empty, allow all (except blocked)
	if len(ctx.Tools.Allowed) == 0 {
		return nil
	}

	// Check if tool matches any allowed pattern
	for _, pattern := range ctx.Tools.Allowed {
		if matchesToolPattern(toolName, pattern) {
			return nil // Access allowed
		}
	}

	// Tool doesn't match any allowed pattern
	return &PermissionError{
		Type:     "tools.denied",
		Resource: toolName,
		Allowed:  ctx.Tools.Allowed,
		Message:  "tool not in allowed patterns",
	}
}

// matchesToolPattern checks if a tool name matches a pattern.
// Supports glob patterns like "file.*" or "http.request".
func matchesToolPattern(toolName, pattern string) bool {
	// Exact match
	if toolName == pattern {
		return true
	}

	// Glob pattern match
	matched, err := doublestar.Match(pattern, toolName)
	if err != nil {
		// Invalid pattern - treat as exact match
		return toolName == pattern
	}

	return matched
}

// FilterAllowedTools filters a list of tool names to only include those allowed by permissions.
// This is used to filter the tool registry before sending to LLM providers.
func FilterAllowedTools(ctx *PermissionContext, toolNames []string) []string {
	if ctx == nil || ctx.Tools == nil {
		// No restrictions - return all tools
		return toolNames
	}

	allowed := make([]string, 0, len(toolNames))
	for _, toolName := range toolNames {
		if CheckTool(ctx, toolName) == nil {
			allowed = append(allowed, toolName)
		}
	}

	return allowed
}
