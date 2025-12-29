package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/tombee/conductor/pkg/security"
)

// operationWrapper wraps an operation with observability features
func (c *FileConnector) operationWrapper(operation string, path string, fn func() (*Result, error)) (*Result, error) {
	start := time.Now()

	// Execute the operation
	result, err := fn()

	duration := time.Since(start)

	// Prepare audit entry and metrics
	var bytesReadCount int64
	var bytesWrittenCount int64
	var status string
	var errType ErrorType
	var errMsg string

	if err != nil {
		status = "error"
		errMsg = err.Error()

		// Extract error type if it's an OperationError
		if opErr, ok := err.(*OperationError); ok {
			errType = opErr.ErrorType
		} else {
			errType = ErrorTypeInternal
		}
	} else {
		status = "success"

		// Extract bytes from metadata if available
		if result != nil && result.Metadata != nil {
			if bytes, ok := result.Metadata["bytes"].(int); ok {
				// Determine if it's a read or write operation
				if isWriteOperation(operation) {
					bytesWrittenCount = int64(bytes)
				} else {
					bytesReadCount = int64(bytes)
				}
			}
			if size, ok := result.Metadata["size"].(int); ok {
				bytesReadCount = int64(size)
			}
		}
	}

	// Record metrics
	recordMetrics(operation, duration.Seconds(), status, bytesReadCount, bytesWrittenCount, errType)

	// Log audit entry
	c.auditLogger.Log(AuditEntry{
		Timestamp:    start,
		Operation:    operation,
		Path:         path,
		Result:       status,
		Duration:     duration,
		BytesRead:    bytesReadCount,
		BytesWritten: bytesWrittenCount,
		Error:        errMsg,
		// WorkflowID and StepID could be extracted from context if available
	})

	return result, err
}

// isWriteOperation determines if an operation writes data
func isWriteOperation(operation string) bool {
	writeOps := map[string]bool{
		"write":      true,
		"write_text": true,
		"write_json": true,
		"write_yaml": true,
		"append":     true,
		"render":     true,
		"copy":       true, // counts as write to destination
	}
	return writeOps[operation]
}

// wrapError wraps an error in an OperationError, preserving the ErrorType if it's already an OperationError
func wrapError(operation string, message string, err error) error {
	// If it's already an OperationError, preserve its ErrorType
	if opErr, ok := err.(*OperationError); ok {
		return &OperationError{
			Operation: operation,
			Message:   message,
			ErrorType: opErr.ErrorType,
			Cause:     err,
		}
	}

	// Otherwise, default to internal error
	return &OperationError{
		Operation: operation,
		Message:   message,
		ErrorType: ErrorTypeInternal,
		Cause:     err,
	}
}

// read implements the file.read operation with auto-detection based on extension.
func (c *FileConnector) read(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	// Resolve and validate path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Check file exists
	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &OperationError{
				Operation: "read",
				Message:   fmt.Sprintf("file not found: %s", path),
				ErrorType: ErrorTypeFileNotFound,
				Cause:     err,
			}
		}
		return nil, &OperationError{
			Operation: "read",
			Message:   fmt.Sprintf("failed to stat file: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	// Check file size limits
	if info.Size() > c.config.MaxFileSize {
		return nil, &OperationError{
			Operation: "read",
			Message:   fmt.Sprintf("file exceeds maximum size of %d bytes", c.config.MaxFileSize),
			ErrorType: ErrorTypeFileTooLarge,
		}
	}

	// Determine format based on extension
	ext := strings.ToLower(filepath.Ext(resolvedPath))

	// For large files, skip auto-detection parsing
	if info.Size() > c.config.MaxParseSize {
		// Log warning (would need logger injected)
		// Fall back to text reading
		return c.readText(ctx, inputs)
	}

	// Auto-detect based on extension
	switch ext {
	case ".json":
		return c.readJSON(ctx, inputs)
	case ".yaml", ".yml":
		return c.readYAML(ctx, inputs)
	case ".csv":
		return c.readCSV(ctx, inputs)
	default:
		// Default to text
		return c.readText(ctx, inputs)
	}
}

// readText implements the file.read_text operation.
func (c *FileConnector) readText(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	// Resolve and validate path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Read file contents
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &OperationError{
				Operation: "read_text",
				Message:   fmt.Sprintf("file not found: %s", path),
				ErrorType: ErrorTypeFileNotFound,
				Cause:     err,
			}
		}
		if os.IsPermission(err) {
			return nil, &OperationError{
				Operation: "read_text",
				Message:   fmt.Sprintf("permission denied: %s", path),
				ErrorType: ErrorTypePermissionDenied,
				Cause:     err,
			}
		}
		return nil, &OperationError{
			Operation: "read_text",
			Message:   fmt.Sprintf("failed to read file: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	// Strip BOM if present
	content = stripBOM(content)

	return &Result{
		Response: string(content),
		Metadata: map[string]interface{}{
			"path": resolvedPath,
			"size": len(content),
		},
	}, nil
}

// readJSON implements the file.read_json operation.
func (c *FileConnector) readJSON(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// First read as text
	textResult, err := c.readText(ctx, inputs)
	if err != nil {
		return nil, err
	}

	content := textResult.Response.(string)

	// Parse JSON
	parsed, err := parseJSON([]byte(content))
	if err != nil {
		path, _ := getStringParam(inputs, "path", false)
		return nil, &OperationError{
			Operation: "read_json",
			Message:   fmt.Sprintf("failed to parse JSON in %s: %v", path, err),
			ErrorType: ErrorTypeParseError,
			Cause:     err,
		}
	}

	// Check for inline extraction
	extract, _ := getStringParam(inputs, "extract", false)
	if extract != "" {
		extracted, err := extractJSONPath(parsed, extract)
		if err != nil {
			return nil, &OperationError{
				Operation: "read_json",
				Message:   fmt.Sprintf("JSONPath extraction failed: %v", err),
				ErrorType: ErrorTypeValidation,
				Cause:     err,
			}
		}
		parsed = extracted
	}

	return &Result{
		Response: parsed,
		Metadata: textResult.Metadata,
	}, nil
}

// readYAML implements the file.read_yaml operation.
func (c *FileConnector) readYAML(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// First read as text
	textResult, err := c.readText(ctx, inputs)
	if err != nil {
		return nil, err
	}

	content := textResult.Response.(string)

	// Parse YAML
	parsed, err := parseYAML([]byte(content))
	if err != nil {
		path, _ := getStringParam(inputs, "path", false)
		return nil, &OperationError{
			Operation: "read_yaml",
			Message:   fmt.Sprintf("failed to parse YAML in %s: %v", path, err),
			ErrorType: ErrorTypeParseError,
			Cause:     err,
		}
	}

	return &Result{
		Response: parsed,
		Metadata: textResult.Metadata,
	}, nil
}

// readCSV implements the file.read_csv operation.
func (c *FileConnector) readCSV(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// First read as text
	textResult, err := c.readText(ctx, inputs)
	if err != nil {
		return nil, err
	}

	content := textResult.Response.(string)

	// Get delimiter (default comma)
	delimiter, _ := getStringParam(inputs, "delimiter", false)
	if delimiter == "" {
		delimiter = ","
	}

	// Parse CSV
	parsed, err := parseCSV([]byte(content), delimiter)
	if err != nil {
		path, _ := getStringParam(inputs, "path", false)
		return nil, &OperationError{
			Operation: "read_csv",
			Message:   fmt.Sprintf("failed to parse CSV in %s: %v", path, err),
			ErrorType: ErrorTypeParseError,
			Cause:     err,
		}
	}

	return &Result{
		Response: parsed,
		Metadata: textResult.Metadata,
	}, nil
}

// readLines implements the file.read_lines operation.
func (c *FileConnector) readLines(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// First read as text
	textResult, err := c.readText(ctx, inputs)
	if err != nil {
		return nil, err
	}

	content := textResult.Response.(string)

	// Split into lines
	lines := strings.Split(content, "\n")

	// Remove trailing empty line if present
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return &Result{
		Response: lines,
		Metadata: textResult.Metadata,
	}, nil
}

// write implements the file.write operation with auto-formatting based on extension.
func (c *FileConnector) write(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	content, ok := inputs["content"]
	if !ok {
		return nil, &OperationError{
			Operation: "write",
			Message:   "missing required parameter: content",
			ErrorType: ErrorTypeValidation,
		}
	}

	// Resolve and validate path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Determine format based on extension
	ext := strings.ToLower(filepath.Ext(resolvedPath))

	// Auto-format based on extension
	var data []byte
	switch ext {
	case ".json":
		// Format as JSON
		data, err = formatJSON(content)
		if err != nil {
			return nil, &OperationError{
				Operation: "write",
				Message:   fmt.Sprintf("failed to format content as JSON: %v", err),
				ErrorType: ErrorTypeValidation,
				Cause:     err,
			}
		}
	case ".yaml", ".yml":
		// Format as YAML
		data, err = formatYAML(content)
		if err != nil {
			return nil, &OperationError{
				Operation: "write",
				Message:   fmt.Sprintf("failed to format content as YAML: %v", err),
				ErrorType: ErrorTypeValidation,
				Cause:     err,
			}
		}
	default:
		// Write as text
		if str, ok := content.(string); ok {
			data = []byte(str)
		} else {
			return nil, &OperationError{
				Operation: "write",
				Message:   "content must be a string for text files",
				ErrorType: ErrorTypeValidation,
			}
		}
	}

	// Write atomically
	if err := c.writeAtomic(resolvedPath, data); err != nil {
		return nil, wrapError("write", fmt.Sprintf("failed to write file: %v", err), err)
	}

	return &Result{
		Response: nil,
		Metadata: map[string]interface{}{
			"path":  resolvedPath,
			"bytes": len(data),
		},
	}, nil
}

// writeText implements the file.write_text operation.
func (c *FileConnector) writeText(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	content, ok := inputs["content"]
	if !ok {
		return nil, &OperationError{
			Operation: "write_text",
			Message:   "missing required parameter: content",
			ErrorType: ErrorTypeValidation,
		}
	}

	// Convert content to string
	var data []byte
	if str, ok := content.(string); ok {
		data = []byte(str)
	} else {
		return nil, &OperationError{
			Operation: "write_text",
			Message:   "content must be a string",
			ErrorType: ErrorTypeValidation,
		}
	}

	// Resolve and validate path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Write atomically
	if err := c.writeAtomic(resolvedPath, data); err != nil {
		return nil, wrapError("write_text", fmt.Sprintf("failed to write file: %v", err), err)
	}

	return &Result{
		Response: nil,
		Metadata: map[string]interface{}{
			"path":  resolvedPath,
			"bytes": len(data),
		},
	}, nil
}

// writeJSON implements the file.write_json operation with pretty-printing.
func (c *FileConnector) writeJSON(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	content, ok := inputs["content"]
	if !ok {
		return nil, &OperationError{
			Operation: "write_json",
			Message:   "missing required parameter: content",
			ErrorType: ErrorTypeValidation,
		}
	}

	// Format as JSON
	data, err := formatJSON(content)
	if err != nil {
		return nil, &OperationError{
			Operation: "write_json",
			Message:   fmt.Sprintf("failed to format content as JSON: %v", err),
			ErrorType: ErrorTypeValidation,
			Cause:     err,
		}
	}

	// Resolve and validate path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Write atomically
	if err := c.writeAtomic(resolvedPath, data); err != nil {
		return nil, wrapError("write_json", fmt.Sprintf("failed to write file: %v", err), err)
	}

	return &Result{
		Response: nil,
		Metadata: map[string]interface{}{
			"path":  resolvedPath,
			"bytes": len(data),
		},
	}, nil
}

// writeYAML implements the file.write_yaml operation with proper YAML formatting.
func (c *FileConnector) writeYAML(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	content, ok := inputs["content"]
	if !ok {
		return nil, &OperationError{
			Operation: "write_yaml",
			Message:   "missing required parameter: content",
			ErrorType: ErrorTypeValidation,
		}
	}

	// Format as YAML
	data, err := formatYAML(content)
	if err != nil {
		return nil, &OperationError{
			Operation: "write_yaml",
			Message:   fmt.Sprintf("failed to format content as YAML: %v", err),
			ErrorType: ErrorTypeValidation,
			Cause:     err,
		}
	}

	// Resolve and validate path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Write atomically
	if err := c.writeAtomic(resolvedPath, data); err != nil {
		return nil, wrapError("write_yaml", fmt.Sprintf("failed to write file: %v", err), err)
	}

	return &Result{
		Response: nil,
		Metadata: map[string]interface{}{
			"path":  resolvedPath,
			"bytes": len(data),
		},
	}, nil
}

// append implements the file.append operation for appending to existing files.
func (c *FileConnector) append(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	content, ok := inputs["content"]
	if !ok {
		return nil, &OperationError{
			Operation: "append",
			Message:   "missing required parameter: content",
			ErrorType: ErrorTypeValidation,
		}
	}

	// Convert content to string
	var data []byte
	if str, ok := content.(string); ok {
		data = []byte(str)
	} else {
		return nil, &OperationError{
			Operation: "append",
			Message:   "content must be a string",
			ErrorType: ErrorTypeValidation,
		}
	}

	// Resolve and validate path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Check quota before appending (if tracker is configured)
	if c.quotaTracker != nil {
		if err := c.quotaTracker.TrackWrite(resolvedPath, int64(len(data))); err != nil {
			return nil, err
		}
	}

	// Determine appropriate permissions based on file path
	fileMode, dirMode := security.DeterminePermissions(resolvedPath)

	// Ensure parent directory exists with appropriate permissions
	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return nil, &OperationError{
			Operation: "append",
			Message:   fmt.Sprintf("failed to create parent directory: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	// Open file for appending (create if doesn't exist)
	file, err := os.OpenFile(resolvedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		if os.IsPermission(err) {
			return nil, &OperationError{
				Operation: "append",
				Message:   fmt.Sprintf("permission denied: %s", path),
				ErrorType: ErrorTypePermissionDenied,
				Cause:     err,
			}
		}
		return nil, &OperationError{
			Operation: "append",
			Message:   fmt.Sprintf("failed to open file: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}
	defer file.Close()

	// Append content
	n, err := file.Write(data)
	if err != nil {
		return nil, &OperationError{
			Operation: "append",
			Message:   fmt.Sprintf("failed to append to file: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	return &Result{
		Response: nil,
		Metadata: map[string]interface{}{
			"path":  resolvedPath,
			"bytes": n,
		},
	}, nil
}

// render implements the file.render operation with restricted Go template execution.
func (c *FileConnector) render(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	templatePath, err := getStringParam(inputs, "template", true)
	if err != nil {
		return nil, err
	}

	outputPath, err := getStringParam(inputs, "output", true)
	if err != nil {
		return nil, err
	}

	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation: "render",
			Message:   "missing required parameter: data",
			ErrorType: ErrorTypeValidation,
		}
	}

	// Resolve template path
	resolvedTemplatePath, err := c.resolver.Resolve(templatePath)
	if err != nil {
		return nil, err
	}

	// Read template file
	templateContent, err := os.ReadFile(resolvedTemplatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &OperationError{
				Operation: "render",
				Message:   fmt.Sprintf("template file not found: %s", templatePath),
				ErrorType: ErrorTypeFileNotFound,
				Cause:     err,
			}
		}
		return nil, &OperationError{
			Operation: "render",
			Message:   fmt.Sprintf("failed to read template: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	// Render template with restricted functions
	rendered, err := renderTemplate(string(templateContent), data)
	if err != nil {
		return nil, &OperationError{
			Operation: "render",
			Message:   fmt.Sprintf("failed to render template: %v", err),
			ErrorType: ErrorTypeValidation,
			Cause:     err,
		}
	}

	// Resolve output path
	resolvedOutputPath, err := c.resolver.Resolve(outputPath)
	if err != nil {
		return nil, err
	}

	// Write atomically
	if err := c.writeAtomic(resolvedOutputPath, []byte(rendered)); err != nil {
		return nil, wrapError("render", fmt.Sprintf("failed to write output: %v", err), err)
	}

	return &Result{
		Response: nil,
		Metadata: map[string]interface{}{
			"template": resolvedTemplatePath,
			"output":   resolvedOutputPath,
			"bytes":    len(rendered),
		},
	}, nil
}

// writeAtomic writes content to a file atomically using temp file + rename pattern.
func (c *FileConnector) writeAtomic(path string, content []byte) error {
	// Check file size limit
	if c.config.MaxFileSize > 0 && int64(len(content)) > c.config.MaxFileSize {
		return fmt.Errorf("content size (%d bytes) exceeds maximum allowed (%d bytes)",
			len(content), c.config.MaxFileSize)
	}

	// Check quota before writing (if tracker is configured)
	if c.quotaTracker != nil {
		if err := c.quotaTracker.TrackWrite(path, int64(len(content))); err != nil {
			return err
		}
	}

	// Determine appropriate permissions based on file path
	fileMode, dirMode := security.DeterminePermissions(path)

	// Create parent directory if it doesn't exist with appropriate permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create temp file in same directory for atomic rename
	tmpFile, err := os.CreateTemp(dir, ".atomic.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on error
	defer func() {
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	// Set permissions on temp file BEFORE writing content (security best practice)
	if err := tmpFile.Chmod(0600); err != nil {
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	// Verify permissions were set correctly via file descriptor
	if err := security.VerifyPermissions(tmpFile, 0600); err != nil {
		return fmt.Errorf("failed to verify temp file permissions: %w", err)
	}

	// Write content to temp file
	if _, err := tmpFile.Write(content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	tmpFile = nil // Prevent cleanup

	// Set final permissions
	if err := os.Chmod(tmpPath, fileMode); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// list implements the file.list operation with glob pattern support.
func (c *FileConnector) list(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	// Get optional pattern parameter
	pattern, _ := getStringParam(inputs, "pattern", false)

	// Get optional recursive flag
	recursive, _ := getBoolParam(inputs, "recursive", false)

	// Get optional type filter (files, dirs, all)
	typeFilter, _ := getStringParam(inputs, "type", false)
	if typeFilter == "" {
		typeFilter = "all"
	}

	// Resolve base path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Check if path exists
	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &OperationError{
				Operation: "list",
				Message:   fmt.Sprintf("path not found: %s", path),
				ErrorType: ErrorTypeFileNotFound,
				Cause:     err,
			}
		}
		return nil, &OperationError{
			Operation: "list",
			Message:   fmt.Sprintf("failed to stat path: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	var files []map[string]interface{}

	// If path is a file, return it
	if !info.IsDir() {
		files = append(files, fileInfoToMap(resolvedPath, info))
		return &Result{
			Response: files,
			Metadata: map[string]interface{}{
				"path":  resolvedPath,
				"count": len(files),
			},
		}, nil
	}

	// Build glob pattern
	globPattern := resolvedPath
	if pattern != "" {
		if recursive {
			globPattern = filepath.Join(resolvedPath, "**", pattern)
		} else {
			globPattern = filepath.Join(resolvedPath, pattern)
		}
	} else if recursive {
		globPattern = filepath.Join(resolvedPath, "**", "*")
	} else {
		globPattern = filepath.Join(resolvedPath, "*")
	}

	// Match files using doublestar
	matches, err := doublestar.FilepathGlob(globPattern)
	if err != nil {
		return nil, &OperationError{
			Operation: "list",
			Message:   fmt.Sprintf("glob pattern error: %v", err),
			ErrorType: ErrorTypeValidation,
			Cause:     err,
		}
	}

	// Filter and collect file info
	for _, match := range matches {
		matchInfo, err := os.Stat(match)
		if err != nil {
			// Skip files that cannot be stat'd
			continue
		}

		// Apply type filter
		if typeFilter == "files" && matchInfo.IsDir() {
			continue
		}
		if typeFilter == "dirs" && !matchInfo.IsDir() {
			continue
		}

		files = append(files, fileInfoToMap(match, matchInfo))
	}

	return &Result{
		Response: files,
		Metadata: map[string]interface{}{
			"path":    resolvedPath,
			"pattern": pattern,
			"count":   len(files),
		},
	}, nil
}

// exists implements the file.exists operation for checking file/directory existence.
func (c *FileConnector) exists(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	// Resolve path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Check if file exists
	_, err = os.Stat(resolvedPath)
	exists := err == nil

	return &Result{
		Response: exists,
		Metadata: map[string]interface{}{
			"path": resolvedPath,
		},
	}, nil
}

// stat implements the file.stat operation for retrieving file metadata.
func (c *FileConnector) stat(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	// Resolve path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Get file info
	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &OperationError{
				Operation: "stat",
				Message:   fmt.Sprintf("file not found: %s", path),
				ErrorType: ErrorTypeFileNotFound,
				Cause:     err,
			}
		}
		return nil, &OperationError{
			Operation: "stat",
			Message:   fmt.Sprintf("failed to stat file: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	return &Result{
		Response: fileInfoToMap(resolvedPath, info),
		Metadata: map[string]interface{}{
			"path": resolvedPath,
		},
	}, nil
}

// mkdir implements the file.mkdir operation with parent directory creation.
func (c *FileConnector) mkdir(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	// Get optional parents flag (default true)
	parents, _ := getBoolParam(inputs, "parents", false)
	if _, ok := inputs["parents"]; !ok {
		parents = true // Default to true
	}

	// Resolve path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Check if directory already exists
	if info, err := os.Stat(resolvedPath); err == nil && info.IsDir() {
		return &Result{
			Response: nil,
			Metadata: map[string]interface{}{
				"path":    resolvedPath,
				"created": false,
			},
		}, nil
	}

	// Create directory
	if parents {
		err = os.MkdirAll(resolvedPath, 0755)
	} else {
		err = os.Mkdir(resolvedPath, 0755)
	}

	if err != nil {
		if os.IsExist(err) {
			// Not an error if directory already exists
			return &Result{
				Response: nil,
				Metadata: map[string]interface{}{
					"path":    resolvedPath,
					"created": false,
				},
			}, nil
		}
		if os.IsPermission(err) {
			return nil, &OperationError{
				Operation: "mkdir",
				Message:   fmt.Sprintf("permission denied: %s", path),
				ErrorType: ErrorTypePermissionDenied,
				Cause:     err,
			}
		}
		return nil, &OperationError{
			Operation: "mkdir",
			Message:   fmt.Sprintf("failed to create directory: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	return &Result{
		Response: nil,
		Metadata: map[string]interface{}{
			"path":    resolvedPath,
			"created": true,
		},
	}, nil
}

// copy implements the file.copy operation for files and directories.
func (c *FileConnector) copy(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	source, err := getStringParam(inputs, "source", true)
	if err != nil {
		return nil, err
	}

	dest, err := getStringParam(inputs, "dest", true)
	if err != nil {
		return nil, err
	}

	// Get optional recursive flag
	recursive, _ := getBoolParam(inputs, "recursive", false)

	// Resolve paths
	resolvedSource, err := c.resolver.Resolve(source)
	if err != nil {
		return nil, err
	}

	resolvedDest, err := c.resolver.Resolve(dest)
	if err != nil {
		return nil, err
	}

	// Check source exists
	sourceInfo, err := os.Stat(resolvedSource)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &OperationError{
				Operation: "copy",
				Message:   fmt.Sprintf("source not found: %s", source),
				ErrorType: ErrorTypeFileNotFound,
				Cause:     err,
			}
		}
		return nil, &OperationError{
			Operation: "copy",
			Message:   fmt.Sprintf("failed to stat source: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	// Check quota before copying (if tracker is configured)
	// For simplicity, check based on source size
	if c.quotaTracker != nil {
		size := sourceInfo.Size()
		if sourceInfo.IsDir() {
			// For directories, we'll check quota during the actual copy
			// This is a pre-check approximation
			size = 0 // Skip pre-check for directories
		}
		if size > 0 {
			if err := c.quotaTracker.TrackWrite(resolvedDest, size); err != nil {
				return nil, err
			}
		}
	}

	// Copy based on type
	var bytesCopied int64
	if sourceInfo.IsDir() {
		if !recursive {
			return nil, &OperationError{
				Operation: "copy",
				Message:   "source is a directory, use recursive=true to copy directories",
				ErrorType: ErrorTypeValidation,
			}
		}
		bytesCopied, err = copyDir(resolvedSource, resolvedDest)
	} else {
		bytesCopied, err = copyFile(resolvedSource, resolvedDest)
	}

	if err != nil {
		return nil, &OperationError{
			Operation: "copy",
			Message:   fmt.Sprintf("failed to copy: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	return &Result{
		Response: nil,
		Metadata: map[string]interface{}{
			"source": resolvedSource,
			"dest":   resolvedDest,
			"bytes":  bytesCopied,
		},
	}, nil
}

// move implements the file.move operation for rename/move operations.
func (c *FileConnector) move(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	source, err := getStringParam(inputs, "source", true)
	if err != nil {
		return nil, err
	}

	dest, err := getStringParam(inputs, "dest", true)
	if err != nil {
		return nil, err
	}

	// Resolve paths
	resolvedSource, err := c.resolver.Resolve(source)
	if err != nil {
		return nil, err
	}

	resolvedDest, err := c.resolver.Resolve(dest)
	if err != nil {
		return nil, err
	}

	// Check source exists
	sourceInfo, err := os.Stat(resolvedSource)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &OperationError{
				Operation: "move",
				Message:   fmt.Sprintf("source not found: %s", source),
				ErrorType: ErrorTypeFileNotFound,
				Cause:     err,
			}
		}
		return nil, &OperationError{
			Operation: "move",
			Message:   fmt.Sprintf("failed to stat source: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	// Attempt rename (works if on same filesystem)
	err = os.Rename(resolvedSource, resolvedDest)
	if err != nil {
		// If rename fails due to cross-device, fall back to copy+delete
		if linkErr, ok := err.(*os.LinkError); ok && linkErr.Err.Error() == "cross-device link" {
			// Copy first
			var bytesCopied int64
			if sourceInfo.IsDir() {
				bytesCopied, err = copyDir(resolvedSource, resolvedDest)
			} else {
				bytesCopied, err = copyFile(resolvedSource, resolvedDest)
			}
			if err != nil {
				return nil, &OperationError{
					Operation: "move",
					Message:   fmt.Sprintf("failed to copy during cross-device move: %v", err),
					ErrorType: ErrorTypeInternal,
					Cause:     err,
				}
			}

			// Delete source
			err = os.RemoveAll(resolvedSource)
			if err != nil {
				return nil, &OperationError{
					Operation: "move",
					Message:   fmt.Sprintf("copied but failed to delete source: %v", err),
					ErrorType: ErrorTypeInternal,
					Cause:     err,
				}
			}

			return &Result{
				Response: nil,
				Metadata: map[string]interface{}{
					"source": resolvedSource,
					"dest":   resolvedDest,
					"bytes":  bytesCopied,
					"method": "copy-delete",
				},
			}, nil
		}

		// Other rename errors
		if os.IsPermission(err) {
			return nil, &OperationError{
				Operation: "move",
				Message:   fmt.Sprintf("permission denied: %v", err),
				ErrorType: ErrorTypePermissionDenied,
				Cause:     err,
			}
		}
		return nil, &OperationError{
			Operation: "move",
			Message:   fmt.Sprintf("failed to move: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	return &Result{
		Response: nil,
		Metadata: map[string]interface{}{
			"source": resolvedSource,
			"dest":   resolvedDest,
			"method": "rename",
		},
	}, nil
}

// delete implements the file.delete operation with safety checks.
func (c *FileConnector) delete(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	path, err := getStringParam(inputs, "path", true)
	if err != nil {
		return nil, err
	}

	// Get optional recursive flag
	recursive, _ := getBoolParam(inputs, "recursive", false)

	// Resolve path
	resolvedPath, err := c.resolver.Resolve(path)
	if err != nil {
		return nil, err
	}

	// Check file exists
	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Not an error if file doesn't exist
			return &Result{
				Response: nil,
				Metadata: map[string]interface{}{
					"path":    resolvedPath,
					"deleted": false,
				},
			}, nil
		}
		return nil, &OperationError{
			Operation: "delete",
			Message:   fmt.Sprintf("failed to stat path: %v", err),
			ErrorType: ErrorTypeInternal,
			Cause:     err,
		}
	}

	// Check if directory and recursive flag
	if info.IsDir() && !recursive {
		return nil, &OperationError{
			Operation: "delete",
			Message:   "path is a directory, use recursive=true to delete directories",
			ErrorType: ErrorTypeValidation,
		}
	}

	// Delete
	var deleteErr error
	if recursive || info.IsDir() {
		deleteErr = os.RemoveAll(resolvedPath)
	} else {
		deleteErr = os.Remove(resolvedPath)
	}

	if deleteErr != nil {
		if os.IsPermission(deleteErr) {
			return nil, &OperationError{
				Operation: "delete",
				Message:   fmt.Sprintf("permission denied: %s", path),
				ErrorType: ErrorTypePermissionDenied,
				Cause:     deleteErr,
			}
		}
		return nil, &OperationError{
			Operation: "delete",
			Message:   fmt.Sprintf("failed to delete: %v", deleteErr),
			ErrorType: ErrorTypeInternal,
			Cause:     deleteErr,
		}
	}

	return &Result{
		Response: nil,
		Metadata: map[string]interface{}{
			"path":    resolvedPath,
			"deleted": true,
		},
	}, nil
}

// Helper function to extract string parameters from inputs
func getStringParam(inputs map[string]interface{}, key string, required bool) (string, error) {
	val, ok := inputs[key]
	if !ok {
		if required {
			return "", &OperationError{
				Message:   fmt.Sprintf("missing required parameter: %s", key),
				ErrorType: ErrorTypeValidation,
			}
		}
		return "", nil
	}

	str, ok := val.(string)
	if !ok {
		return "", &OperationError{
			Message:   fmt.Sprintf("parameter %s must be a string", key),
			ErrorType: ErrorTypeValidation,
		}
	}

	return str, nil
}

// Helper function to extract boolean parameters from inputs
func getBoolParam(inputs map[string]interface{}, key string, required bool) (bool, error) {
	val, ok := inputs[key]
	if !ok {
		if required {
			return false, &OperationError{
				Message:   fmt.Sprintf("missing required parameter: %s", key),
				ErrorType: ErrorTypeValidation,
			}
		}
		return false, nil
	}

	b, ok := val.(bool)
	if !ok {
		return false, &OperationError{
			Message:   fmt.Sprintf("parameter %s must be a boolean", key),
			ErrorType: ErrorTypeValidation,
		}
	}

	return b, nil
}

// fileInfoToMap converts os.FileInfo to a map for response
func fileInfoToMap(path string, info os.FileInfo) map[string]interface{} {
	return map[string]interface{}{
		"path":    path,
		"name":    info.Name(),
		"size":    info.Size(),
		"mode":    info.Mode().String(),
		"modTime": info.ModTime().Format(time.RFC3339),
		"isDir":   info.IsDir(),
	}
}

// copyFile copies a single file from source to destination
func copyFile(source, dest string) (int64, error) {
	// Open source file
	sourceFile, err := os.Open(source)
	if err != nil {
		return 0, fmt.Errorf("failed to open source: %w", err)
	}
	defer sourceFile.Close()

	// Get source file info for permissions
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat source: %w", err)
	}

	// Create destination file
	destFile, err := os.Create(dest)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination: %w", err)
	}
	defer destFile.Close()

	// Copy contents
	bytesCopied, err := io.Copy(destFile, sourceFile)
	if err != nil {
		return bytesCopied, fmt.Errorf("failed to copy contents: %w", err)
	}

	// Set permissions to match source
	if err := os.Chmod(dest, sourceInfo.Mode()); err != nil {
		return bytesCopied, fmt.Errorf("failed to set permissions: %w", err)
	}

	return bytesCopied, nil
}

// copyDir recursively copies a directory from source to destination
func copyDir(source, dest string) (int64, error) {
	// Get source directory info
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return 0, fmt.Errorf("failed to stat source: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(dest, sourceInfo.Mode()); err != nil {
		return 0, fmt.Errorf("failed to create destination directory: %w", err)
	}

	var totalBytes int64

	// Read directory contents
	entries, err := os.ReadDir(source)
	if err != nil {
		return 0, fmt.Errorf("failed to read directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			bytes, err := copyDir(sourcePath, destPath)
			if err != nil {
				return totalBytes, err
			}
			totalBytes += bytes
		} else {
			// Copy file
			bytes, err := copyFile(sourcePath, destPath)
			if err != nil {
				return totalBytes, err
			}
			totalBytes += bytes
		}
	}

	return totalBytes, nil
}
