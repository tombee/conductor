// Package builtin provides built-in tools for common operations.
package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tombee/conductor/pkg/tools"
)

// FileTool provides file read and write operations.
type FileTool struct {
	// maxFileSize limits the maximum file size that can be read (in bytes)
	maxFileSize int64

	// allowedPaths restricts file operations to specific directories
	// If empty, all paths are allowed
	allowedPaths []string
}

// NewFileTool creates a new file tool with default settings.
func NewFileTool() *FileTool {
	return &FileTool{
		maxFileSize:  10 * 1024 * 1024, // 10 MB default
		allowedPaths: []string{},        // No restrictions by default
	}
}

// WithMaxFileSize sets the maximum file size limit.
func (t *FileTool) WithMaxFileSize(size int64) *FileTool {
	t.maxFileSize = size
	return t
}

// WithAllowedPaths restricts file operations to specific directories.
func (t *FileTool) WithAllowedPaths(paths []string) *FileTool {
	t.allowedPaths = paths
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
		return nil, fmt.Errorf("operation must be a string")
	}

	// Extract path
	path, ok := inputs["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	// Validate path
	if err := t.validatePath(path); err != nil {
		return nil, err
	}

	// Execute based on operation
	switch operation {
	case "read":
		return t.read(ctx, path)
	case "write":
		content, ok := inputs["content"].(string)
		if !ok {
			return nil, fmt.Errorf("content must be a string for write operation")
		}
		return t.write(ctx, path, content)
	default:
		return nil, fmt.Errorf("unsupported operation: %s (must be 'read' or 'write')", operation)
	}
}

// read reads a file and returns its content.
func (t *FileTool) read(ctx context.Context, path string) (map[string]interface{}, error) {
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

// write writes content to a file.
func (t *FileTool) write(ctx context.Context, path string, content string) (map[string]interface{}, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to create directory: %v", err),
		}, nil
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to write file: %v", err),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
	}, nil
}

// validatePath checks if a path is allowed.
func (t *FileTool) validatePath(path string) error {
	// Prevent directory traversal attacks
	cleanPath := filepath.Clean(path)
	if cleanPath != path {
		return fmt.Errorf("invalid path: directory traversal detected")
	}

	// Check if path is within allowed directories
	if len(t.allowedPaths) > 0 {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path: %w", err)
		}

		allowed := false
		for _, allowedPath := range t.allowedPaths {
			absAllowed, err := filepath.Abs(allowedPath)
			if err != nil {
				continue
			}

			// Check if absPath is within absAllowed
			rel, err := filepath.Rel(absAllowed, absPath)
			if err == nil && !filepath.IsAbs(rel) && !startsWithDotDot(rel) {
				allowed = true
				break
			}
		}

		if !allowed {
			return fmt.Errorf("path not allowed: %s (must be within allowed directories)", path)
		}
	}

	return nil
}

// startsWithDotDot checks if a path starts with ".."
func startsWithDotDot(path string) bool {
	return len(path) >= 2 && path[0:2] == ".."
}
