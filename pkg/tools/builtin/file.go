// Package builtin provides built-in tools for common operations.
package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tombee/conductor/pkg/errors"
	"github.com/tombee/conductor/pkg/security"
	"github.com/tombee/conductor/pkg/tools"
)

// FileTool provides file read and write operations.
type FileTool struct {
	// maxFileSize limits the maximum file size that can be read (in bytes)
	maxFileSize int64

	// allowedPaths restricts file operations to specific directories
	// If empty, all paths are allowed
	allowedPaths []string

	// securityConfig provides enhanced security controls
	securityConfig *security.FileSecurityConfig
}

// NewFileTool creates a new file tool with default settings.
func NewFileTool() *FileTool {
	return &FileTool{
		maxFileSize:    10 * 1024 * 1024,                // 10 MB default
		allowedPaths:   []string{},                      // No restrictions by default
		securityConfig: security.DefaultFileSecurityConfig(), // Secure defaults
	}
}

// WithMaxFileSize sets the maximum file size limit.
func (t *FileTool) WithMaxFileSize(size int64) *FileTool {
	t.maxFileSize = size
	return t
}

// WithAllowedPaths restricts file operations to specific directories.
// This sets both read and write allowed paths for backward compatibility.
func (t *FileTool) WithAllowedPaths(paths []string) *FileTool {
	t.allowedPaths = paths
	// Also update security config for consistency
	if t.securityConfig == nil {
		t.securityConfig = security.DefaultFileSecurityConfig()
	}
	t.securityConfig.AllowedReadPaths = paths
	t.securityConfig.AllowedWritePaths = paths
	return t
}

// WithSecurityConfig sets the security configuration.
func (t *FileTool) WithSecurityConfig(config *security.FileSecurityConfig) *FileTool {
	t.securityConfig = config
	return t
}

// WithAllowedReadPaths sets paths that can be read.
func (t *FileTool) WithAllowedReadPaths(paths []string) *FileTool {
	if t.securityConfig == nil {
		t.securityConfig = security.DefaultFileSecurityConfig()
	}
	t.securityConfig.AllowedReadPaths = paths
	return t
}

// WithAllowedWritePaths sets paths that can be written.
func (t *FileTool) WithAllowedWritePaths(paths []string) *FileTool {
	if t.securityConfig == nil {
		t.securityConfig = security.DefaultFileSecurityConfig()
	}
	t.securityConfig.AllowedWritePaths = paths
	return t
}

// WithVerboseErrors enables detailed error messages for debugging.
func (t *FileTool) WithVerboseErrors(verbose bool) *FileTool {
	if t.securityConfig == nil {
		t.securityConfig = security.DefaultFileSecurityConfig()
	}
	t.securityConfig.VerboseErrors = verbose
	return t
}

// Name returns the tool identifier.
func (t *FileTool) Name() string {
	return "file"
}

// Description returns a human-readable description.
func (t *FileTool) Description() string {
	return "Read or write files on the local filesystem"
}

// Schema returns the tool's input/output schema.
func (t *FileTool) Schema() *tools.Schema {
	return &tools.Schema{
		Inputs: &tools.ParameterSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"operation": {
					Type:        "string",
					Description: "The file operation to perform",
					Enum:        []interface{}{"read", "write"},
				},
				"path": {
					Type:        "string",
					Description: "The file path (absolute or relative)",
				},
				"content": {
					Type:        "string",
					Description: "The content to write (required for write operation)",
				},
			},
			Required: []string{"operation", "path"},
		},
		Outputs: &tools.ParameterSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"success": {
					Type:        "boolean",
					Description: "Whether the operation succeeded",
				},
				"content": {
					Type:        "string",
					Description: "The file content (for read operation)",
				},
				"error": {
					Type:        "string",
					Description: "Error message if operation failed",
				},
			},
		},
	}
}

// Execute runs the file operation.
func (t *FileTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Extract operation
	operation, ok := inputs["operation"].(string)
	if !ok {
		return nil, &errors.ValidationError{
			Field:      "operation",
			Message:    "operation must be a string",
			Suggestion: "Provide a valid operation ('read' or 'write')",
		}
	}

	// Extract path
	path, ok := inputs["path"].(string)
	if !ok {
		return nil, &errors.ValidationError{
			Field:      "path",
			Message:    "path must be a string",
			Suggestion: "Provide a valid file path",
		}
	}

	// Execute based on operation
	switch operation {
	case "read":
		// Validate path for read access
		if err := t.validatePath(path, security.ActionRead); err != nil {
			return nil, fmt.Errorf("read access validation failed for path %s: %w", path, err)
		}
		return t.read(ctx, path)
	case "write":
		content, ok := inputs["content"].(string)
		if !ok {
			return nil, &errors.ValidationError{
				Field:      "content",
				Message:    "content must be a string for write operation",
				Suggestion: "Provide content as a string value",
			}
		}
		// Validate path for write access
		if err := t.validatePath(path, security.ActionWrite); err != nil {
			return nil, fmt.Errorf("write access validation failed for path %s: %w", path, err)
		}
		return t.write(ctx, path, content)
	default:
		return nil, &errors.ValidationError{
			Field:      "operation",
			Message:    fmt.Sprintf("unsupported operation: %s", operation),
			Suggestion: "Use 'read' or 'write' as the operation",
		}
	}
}

// read reads a file and returns its content.
func (t *FileTool) read(ctx context.Context, path string) (map[string]interface{}, error) {
	// Use secure file opening if security config available
	if t.securityConfig != nil && t.securityConfig.UseFileDescriptors {
		return t.readSecure(ctx, path)
	}

	// Check file size
	info, err := os.Stat(path)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to stat file: %v", err),
		}, nil
	}

	if info.Size() > t.maxFileSize {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("file size (%d bytes) exceeds maximum allowed size (%d bytes)", info.Size(), t.maxFileSize),
		}, nil
	}

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to read file: %v", err),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"content": string(content),
	}, nil
}

// readSecure reads a file using secure file descriptor approach.
func (t *FileTool) readSecure(ctx context.Context, path string) (map[string]interface{}, error) {
	// Open file securely
	file, err := t.securityConfig.OpenFileSecure(path, os.O_RDONLY, 0)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to open file: %v", err),
		}, nil
	}
	defer file.Close()

	// Get file info via descriptor
	info, err := file.Stat()
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to stat file: %v", err),
		}, nil
	}

	// Check file size
	maxSize := t.maxFileSize
	if t.securityConfig.MaxFileSize > 0 && t.securityConfig.MaxFileSize < maxSize {
		maxSize = t.securityConfig.MaxFileSize
	}

	if info.Size() > maxSize {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("file size (%d bytes) exceeds maximum allowed size (%d bytes)", info.Size(), maxSize),
		}, nil
	}

	// Read content
	content := make([]byte, info.Size())
	_, err = file.Read(content)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to read file: %v", err),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"content": string(content),
	}, nil
}

// write writes content to a file.
func (t *FileTool) write(ctx context.Context, path string, content string) (map[string]interface{}, error) {
	// Use atomic write if security config available
	if t.securityConfig != nil {
		return t.writeSecure(ctx, path, content)
	}

	// Determine appropriate permissions based on file path
	fileMode, dirMode := security.DeterminePermissions(path)

	// Ensure parent directory exists with appropriate permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to create directory: %v", err),
		}, nil
	}

	// Write file with appropriate permissions
	if err := os.WriteFile(path, []byte(content), fileMode); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to write file: %v", err),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
	}, nil
}

// writeSecure writes content using atomic write pattern.
func (t *FileTool) writeSecure(ctx context.Context, path string, content string) (map[string]interface{}, error) {
	// Determine appropriate permissions based on file path
	fileMode, dirMode := security.DeterminePermissions(path)

	// Ensure parent directory exists with appropriate permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to create directory: %v", err),
		}, nil
	}

	// Use atomic write with appropriate permissions
	if err := t.securityConfig.WriteFileAtomic(path, []byte(content), fileMode); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to write file: %v", err),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
	}, nil
}

// validatePath checks if a path is allowed for the given action.
func (t *FileTool) validatePath(path string, action security.AccessAction) error {
	// If security config is set, use its comprehensive validation
	if t.securityConfig != nil {
		return t.securityConfig.ValidatePath(path, action)
	}

	// Fallback: basic validation with symlink resolution for defense in depth
	// (maintains backward compatibility when no security config)
	if len(t.allowedPaths) == 0 {
		return nil // Empty = allow all (D5)
	}

	// Prevent directory traversal attacks
	cleanPath := filepath.Clean(path)
	if cleanPath != path {
		return fmt.Errorf("invalid path: directory traversal detected")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Resolve symlinks for defense in depth
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}
	// If file doesn't exist, use the absolute path
	if os.IsNotExist(err) {
		resolvedPath = absPath
	}

	for _, allowedPath := range t.allowedPaths {
		absAllowed, err := filepath.Abs(allowedPath)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absAllowed, resolvedPath)
		if err == nil && !filepath.IsAbs(rel) && !startsWithDotDot(rel) {
			return nil
		}
	}

	return fmt.Errorf("path not allowed: access denied")
}

// startsWithDotDot checks if a path starts with ".."
func startsWithDotDot(path string) bool {
	return len(path) >= 2 && path[0:2] == ".."
}
