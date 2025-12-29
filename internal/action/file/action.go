// Package file provides a builtin action for filesystem operations.
//
// The file action does NOT import the operation package to avoid import cycles.
// The builtin registry bridges between file operations and the operation.Connector interface.
package file

import (
	"context"
)

// FileConnector implements the action interface for file operations.
type FileConnector struct {
	config       *Config
	resolver     *PathResolver
	auditLogger  AuditLogger
	quotaTracker *QuotaTracker
}

// Config holds configuration for the file action.
type Config struct {
	// WorkflowDir is the directory containing the workflow file (base for relative paths)
	WorkflowDir string

	// OutputDir is the $out directory for workflow outputs
	OutputDir string

	// TempDir is the $temp directory for temporary files
	TempDir string

	// AllowedRoots restricts file operations to specific directories
	AllowedRoots []string

	// AllowSymlinks permits following symlinks (default: false)
	AllowSymlinks bool

	// AllowAbsolute permits absolute paths (default: false)
	AllowAbsolute bool

	// MaxFileSize is the maximum file size in bytes (default: 100MB)
	MaxFileSize int64

	// MaxParseSize is the maximum file size for auto-detection parsing (default: 10MB)
	MaxParseSize int64

	// AuditLogger is an optional logger for file operations (nil = no logging)
	AuditLogger AuditLogger

	// QuotaConfig is optional quota configuration (nil = no quotas)
	QuotaConfig *QuotaConfig
}

// DefaultConfig returns sensible defaults for file action configuration.
func DefaultConfig() *Config {
	return &Config{
		AllowSymlinks: false,
		AllowAbsolute: false,
		MaxFileSize:   100 * 1024 * 1024, // 100MB
		MaxParseSize:  10 * 1024 * 1024,  // 10MB
	}
}

// New creates a new file action instance.
func New(config *Config) (*FileConnector, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Apply defaults to unset fields
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 100 * 1024 * 1024 // 100MB
	}
	if config.MaxParseSize == 0 {
		config.MaxParseSize = 10 * 1024 * 1024 // 10MB
	}

	// Create path resolver with security validation
	resolver := NewPathResolver(&PathResolverConfig{
		WorkflowDir:   config.WorkflowDir,
		OutputDir:     config.OutputDir,
		TempDir:       config.TempDir,
		AllowedRoots:  config.AllowedRoots,
		AllowSymlinks: config.AllowSymlinks,
		AllowAbsolute: config.AllowAbsolute,
	})

	// Set up audit logger (default to noop if not provided)
	auditLogger := config.AuditLogger
	if auditLogger == nil {
		auditLogger = &NoopAuditLogger{}
	}

	// Set up quota tracker if configured
	var quotaTracker *QuotaTracker
	if config.QuotaConfig != nil {
		quotaTracker = NewQuotaTracker(config.QuotaConfig)

		// Set default quotas for $out and $temp directories if they're configured
		if config.OutputDir != "" && quotaTracker != nil {
			quotaTracker.SetQuota(config.OutputDir, config.QuotaConfig.DefaultQuota)
		}
		if config.TempDir != "" && quotaTracker != nil {
			quotaTracker.SetQuota(config.TempDir, config.QuotaConfig.DefaultQuota)
		}
	}

	return &FileConnector{
		config:       config,
		resolver:     resolver,
		auditLogger:  auditLogger,
		quotaTracker: quotaTracker,
	}, nil
}

// Name returns the action identifier.
func (c *FileConnector) Name() string {
	return "file"
}

// Result represents the output of a file operation.
type Result struct {
	Response interface{}
	Metadata map[string]interface{}
}

// Note: ErrorType and OperationError are defined in errors.go

// Execute runs a named file operation with the given inputs.
func (c *FileConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*Result, error) {
	// Extract path for observability (best effort)
	path := ""
	if p, ok := inputs["path"].(string); ok {
		path = p
	} else if p, ok := inputs["source"].(string); ok {
		path = p
	} else if p, ok := inputs["template"].(string); ok {
		path = p
	}

	// Wrap the operation with observability
	return c.operationWrapper(operation, path, func() (*Result, error) {
		switch operation {
		case "read":
			return c.read(ctx, inputs)
		case "read_text":
			return c.readText(ctx, inputs)
		case "read_json":
			return c.readJSON(ctx, inputs)
		case "read_yaml":
			return c.readYAML(ctx, inputs)
		case "read_csv":
			return c.readCSV(ctx, inputs)
		case "read_lines":
			return c.readLines(ctx, inputs)
		case "write":
			return c.write(ctx, inputs)
		case "write_text":
			return c.writeText(ctx, inputs)
		case "write_json":
			return c.writeJSON(ctx, inputs)
		case "write_yaml":
			return c.writeYAML(ctx, inputs)
		case "append":
			return c.append(ctx, inputs)
		case "render":
			return c.render(ctx, inputs)
		case "list":
			return c.list(ctx, inputs)
		case "exists":
			return c.exists(ctx, inputs)
		case "stat":
			return c.stat(ctx, inputs)
		case "mkdir":
			return c.mkdir(ctx, inputs)
		case "copy":
			return c.copy(ctx, inputs)
		case "move":
			return c.move(ctx, inputs)
		case "delete":
			return c.delete(ctx, inputs)
		default:
			return nil, &OperationError{
				Operation: operation,
				Message:   "unknown operation",
				ErrorType: ErrorTypeValidation,
			}
		}
	})
}
