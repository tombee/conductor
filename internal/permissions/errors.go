package permissions

import (
	"fmt"
	"strings"
)

// PermissionError represents a permission denial error.
// Per FR3.4, errors do NOT reveal whether the denied resource exists (no information leakage).
type PermissionError struct {
	// Type is the permission type that was denied (e.g., "paths.read", "network.request")
	Type string

	// Resource is the resource that was denied (file path, host, secret name, etc.)
	Resource string

	// Allowed is the list of allowed patterns
	Allowed []string

	// Blocked is the list of blocked patterns (for network, tools, etc.)
	Blocked []string

	// Message provides additional context
	Message string
}

// Error implements the error interface.
// The error message includes:
// 1. The denied resource
// 2. The allowed/blocked patterns
// 3. A "permission denied" indicator
// But does NOT reveal whether the resource exists.
func (e *PermissionError) Error() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("permission denied: %s", e.Type))

	if e.Resource != "" {
		parts = append(parts, fmt.Sprintf("resource: %s", e.Resource))
	}

	if e.Message != "" {
		parts = append(parts, e.Message)
	}

	if len(e.Allowed) > 0 {
		parts = append(parts, fmt.Sprintf("allowed patterns: [%s]", strings.Join(e.Allowed, ", ")))
	} else if len(e.Blocked) == 0 {
		// Only show "none" if there are no blocked patterns either
		parts = append(parts, "allowed patterns: none")
	}

	if len(e.Blocked) > 0 {
		parts = append(parts, fmt.Sprintf("blocked patterns: [%s]", strings.Join(e.Blocked, ", ")))
	}

	return strings.Join(parts, "; ")
}

// IsPermissionError returns true if the error is a PermissionError.
func IsPermissionError(err error) bool {
	_, ok := err.(*PermissionError)
	return ok
}
