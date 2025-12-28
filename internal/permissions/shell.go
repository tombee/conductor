package permissions

import (
	"strings"
)

// DangerousEnvVars are environment variables that should be blocked by default
// to prevent privilege escalation or information leakage.
var DangerousEnvVars = []string{
	"LD_PRELOAD",      // Can inject malicious libraries
	"LD_LIBRARY_PATH", // Can redirect library loading
	"DYLD_INSERT_LIBRARIES", // macOS equivalent of LD_PRELOAD
	"DYLD_LIBRARY_PATH",     // macOS library path
}

// CheckShell checks if shell command execution is allowed.
// Returns nil if allowed, error if denied.
func CheckShell(ctx *PermissionContext, command string) error {
	if ctx == nil || ctx.Shell == nil {
		// No restrictions
		return nil
	}

	// Check if shell is enabled (nil means not set, which defaults to false for safety)
	if ctx.Shell.Enabled == nil || !*ctx.Shell.Enabled {
		return &PermissionError{
			Type:     "shell.disabled",
			Resource: command,
			Message:  "shell execution is disabled",
		}
	}

	// If allowed commands list is empty, allow all
	if len(ctx.Shell.AllowedCommands) == 0 {
		return nil
	}

	// Check if command starts with any allowed prefix
	for _, allowedPrefix := range ctx.Shell.AllowedCommands {
		if strings.HasPrefix(command, allowedPrefix) {
			return nil // Access allowed
		}
	}

	// Command doesn't match any allowed prefix
	return &PermissionError{
		Type:     "shell.command_denied",
		Resource: command,
		Allowed:  ctx.Shell.AllowedCommands,
		Message:  "command not in allowed prefixes",
	}
}

// CheckEnvVar checks if setting an environment variable is allowed.
// Blocks dangerous environment variables by default.
func CheckEnvVar(ctx *PermissionContext, envVar string) error {
	// Always block dangerous environment variables
	for _, dangerous := range DangerousEnvVars {
		if strings.EqualFold(envVar, dangerous) {
			return &PermissionError{
				Type:     "shell.env_blocked",
				Resource: envVar,
				Blocked:  DangerousEnvVars,
				Message:  "environment variable is blocked for security",
			}
		}
	}

	if ctx == nil || ctx.Env == nil {
		// No additional restrictions
		return nil
	}

	// Check inherit setting
	if !ctx.Env.Inherit {
		// If inherit is disabled, only explicitly allowed env vars are permitted
		// This is a strict mode where the workflow must specify what it needs
		return &PermissionError{
			Type:     "shell.env_inherit_disabled",
			Resource: envVar,
			Message:  "environment variable inheritance is disabled",
		}
	}

	return nil
}

// SanitizeCommand performs best-effort detection of dangerous patterns in shell commands.
// This is a defense-in-depth measure and should not be relied upon as the sole security control.
// Returns an error if obvious dangerous patterns are detected.
func SanitizeCommand(command string) error {
	// Check for obvious code injection patterns
	dangerousPatterns := []string{
		"$(curl",   // Command substitution with network access
		"`curl",    // Backtick command substitution
		"eval ",    // Dynamic code evaluation
		"exec ",    // Replace shell process
		"; curl",   // Command chaining with network access
		"&& curl",  // Conditional chaining with network access
		"| curl",   // Piping to network command
	}

	commandLower := strings.ToLower(command)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(commandLower, pattern) {
			return &PermissionError{
				Type:     "shell.dangerous_pattern",
				Resource: command,
				Message:  "command contains potentially dangerous pattern: " + pattern,
			}
		}
	}

	return nil
}
