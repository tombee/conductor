package permissions

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// CheckPathRead checks if a path is allowed for read access.
// Returns nil if allowed, error if denied.
func CheckPathRead(ctx *PermissionContext, path string) error {
	if ctx == nil || ctx.Paths == nil {
		// No restrictions
		return nil
	}

	// Normalize path for consistent matching
	normalizedPath := normalizePath(path)

	// Check against read patterns
	if len(ctx.Paths.Read) == 0 {
		// No read patterns = no read access
		return &PermissionError{
			Type:      "paths.read",
			Resource:  path,
			Allowed:   ctx.Paths.Read,
			Message:   "no read permissions configured",
		}
	}

	// Check if path matches any allowed pattern
	for _, pattern := range ctx.Paths.Read {
		normalizedPattern := normalizePath(pattern)
		matched, err := doublestar.Match(normalizedPattern, normalizedPath)
		if err != nil {
			// Invalid pattern - skip it
			continue
		}
		if matched {
			return nil // Access allowed
		}
	}

	// Path doesn't match any allowed pattern
	return &PermissionError{
		Type:     "paths.read",
		Resource: path,
		Allowed:  ctx.Paths.Read,
		Message:  "path not in allowed read patterns",
	}
}

// CheckPathWrite checks if a path is allowed for write access.
// Returns nil if allowed, error if denied.
func CheckPathWrite(ctx *PermissionContext, path string) error {
	if ctx == nil || ctx.Paths == nil {
		// No restrictions
		return nil
	}

	// Normalize path for consistent matching
	normalizedPath := normalizePath(path)

	// Check against write patterns
	if len(ctx.Paths.Write) == 0 {
		// No write patterns = no write access
		return &PermissionError{
			Type:     "paths.write",
			Resource: path,
			Allowed:  ctx.Paths.Write,
			Message:  "no write permissions configured",
		}
	}

	// Check if path matches any allowed pattern
	for _, pattern := range ctx.Paths.Write {
		normalizedPattern := normalizePath(pattern)
		matched, err := doublestar.Match(normalizedPattern, normalizedPath)
		if err != nil {
			// Invalid pattern - skip it
			continue
		}
		if matched {
			return nil // Access allowed
		}
	}

	// Path doesn't match any allowed pattern
	return &PermissionError{
		Type:     "paths.write",
		Resource: path,
		Allowed:  ctx.Paths.Write,
		Message:  "path not in allowed write patterns",
	}
}

// normalizePath normalizes a file path for consistent matching.
// Converts to forward slashes and handles special directories.
func normalizePath(path string) string {
	// Convert backslashes to forward slashes for consistent matching
	path = strings.ReplaceAll(path, "\\", "/")

	// Remove leading ./ if present
	path = strings.TrimPrefix(path, "./")

	return path
}

// expandSpecialDirectories expands special directory markers like $out, $temp, $workflow_dir.
// This should be called before permission checks are done.
func expandSpecialDirectories(path string, workflowDir, outDir, tempDir string) string {
	// Replace special directory tokens
	result := path

	if workflowDir != "" {
		result = strings.ReplaceAll(result, "$workflow_dir", workflowDir)
	}

	if outDir != "" {
		result = strings.ReplaceAll(result, "$out", outDir)
	}

	if tempDir != "" {
		result = strings.ReplaceAll(result, "$temp", tempDir)
	}

	return result
}
