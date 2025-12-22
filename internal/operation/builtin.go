package operation

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/action/file"
	"github.com/tombee/conductor/internal/action/http"
	"github.com/tombee/conductor/internal/action/shell"
	"github.com/tombee/conductor/internal/action/transform"
	"github.com/tombee/conductor/internal/action/utility"
	"github.com/tombee/conductor/pkg/security"
	"github.com/tombee/conductor/pkg/workflow"
)

func init() {
	// Register the action registry factory with the workflow package.
	// This enables WithWorkflowDir() to automatically initialize builtin actions.
	workflow.SetDefaultActionRegistryFactory(func(workflowDir string) (workflow.OperationRegistry, error) {
		config := &BuiltinConfig{
			WorkflowDir: workflowDir,
		}
		registry, err := NewBuiltinRegistry(config)
		if err != nil {
			return nil, err
		}
		return registry.AsWorkflowRegistry(), nil
	})
}

// BuiltinConfig holds configuration for builtin actions.
type BuiltinConfig struct {
	// WorkflowDir is the base directory for ./ paths
	WorkflowDir string

	// OutputDir is the directory for $out/ paths
	OutputDir string

	// TempDir is the directory for $temp/ paths
	TempDir string

	// MaxFileSize is the maximum file size in bytes (default 100MB)
	MaxFileSize int64

	// MaxParseSize is the maximum file size for auto-detection parsing (default 10MB)
	MaxParseSize int64

	// AllowSymlinks controls whether symlinks are followed
	AllowSymlinks bool

	// AllowAbsolute controls whether absolute paths are allowed
	AllowAbsolute bool

	// DNSMonitor provides DNS query monitoring for HTTP action
	DNSMonitor *security.DNSQueryMonitor

	// SecurityConfig provides HTTP security validation
	SecurityConfig *security.HTTPSecurityConfig
}

// builtinNames lists all builtin action names.
var builtinNames = map[string]bool{
	"file":      true,
	"shell":     true,
	"transform": true,
	"utility":   true,
	"http":      true,
}

// IsBuiltin returns true if the action name is a builtin.
func IsBuiltin(name string) bool {
	return builtinNames[name]
}

// BuiltinProvider wraps a builtin action to implement the Provider interface.
type BuiltinProvider struct {
	name            string
	fileAction      *file.FileAction
	shellAction     *shell.ShellAction
	transformAction *transform.TransformAction
	utilityAction   *utility.UtilityAction
	httpAction      *http.HTTPAction
}

// NewBuiltin creates a builtin action by name.
func NewBuiltin(name string, config *BuiltinConfig) (Provider, error) {
	if !IsBuiltin(name) {
		return nil, fmt.Errorf("unknown builtin action: %s", name)
	}

	switch name {
	case "file":
		fileConfig := &file.Config{
			WorkflowDir:   config.WorkflowDir,
			OutputDir:     config.OutputDir,
			TempDir:       config.TempDir,
			MaxFileSize:   config.MaxFileSize,
			MaxParseSize:  config.MaxParseSize,
			AllowSymlinks: config.AllowSymlinks,
			AllowAbsolute: config.AllowAbsolute,
		}
		if fileConfig.MaxFileSize == 0 {
			fileConfig.MaxFileSize = 100 * 1024 * 1024 // 100MB default
		}
		if fileConfig.MaxParseSize == 0 {
			fileConfig.MaxParseSize = 10 * 1024 * 1024 // 10MB default
		}

		fc, err := file.New(fileConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create file action: %w", err)
		}

		return &BuiltinProvider{
			name:       "file",
			fileAction: fc,
		}, nil

	case "shell":
		shellConfig := &shell.Config{
			WorkingDir: config.WorkflowDir,
		}
		sc, err := shell.New(shellConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create shell action: %w", err)
		}

		return &BuiltinProvider{
			name:        "shell",
			shellAction: sc,
		}, nil

	case "transform":
		tc, err := transform.New(nil) // Use default config
		if err != nil {
			return nil, fmt.Errorf("failed to create transform action: %w", err)
		}

		return &BuiltinProvider{
			name:            "transform",
			transformAction: tc,
		}, nil

	case "utility":
		uc, err := utility.New(nil) // Use default config
		if err != nil {
			return nil, fmt.Errorf("failed to create utility action: %w", err)
		}

		return &BuiltinProvider{
			name:          "utility",
			utilityAction: uc,
		}, nil

	case "http":
		httpConfig := &http.Config{
			DNSMonitor:     config.DNSMonitor,
			SecurityConfig: config.SecurityConfig,
		}
		hc, err := http.New(httpConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create http action: %w", err)
		}

		return &BuiltinProvider{
			name:       "http",
			httpAction: hc,
		}, nil

	default:
		return nil, fmt.Errorf("unknown builtin action: %s", name)
	}
}

// Name returns the action identifier.
func (c *BuiltinProvider) Name() string {
	return c.name
}

// Execute runs a named operation with the given inputs.
func (c *BuiltinProvider) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*Result, error) {
	switch c.name {
	case "file":
		result, err := c.fileAction.Execute(ctx, operation, inputs)
		if err != nil {
			return nil, err
		}
		return &Result{
			Response: result.Response,
			Metadata: result.Metadata,
		}, nil

	case "shell":
		result, err := c.shellAction.Execute(ctx, operation, inputs)
		if err != nil {
			return nil, err
		}
		return &Result{
			Response: result.Response,
			Metadata: result.Metadata,
		}, nil

	case "transform":
		result, err := c.transformAction.Execute(ctx, operation, inputs)
		if err != nil {
			return nil, err
		}
		return &Result{
			Response: result.Response,
			Metadata: result.Metadata,
		}, nil

	case "utility":
		result, err := c.utilityAction.Execute(ctx, operation, inputs)
		if err != nil {
			return nil, err
		}
		return &Result{
			Response: result.Response,
			Metadata: result.Metadata,
		}, nil

	case "http":
		result, err := c.httpAction.Execute(ctx, operation, inputs)
		if err != nil {
			return nil, err
		}
		return &Result{
			Response: result.Response,
			Metadata: result.Metadata,
		}, nil

	default:
		return nil, fmt.Errorf("unknown builtin action: %s", c.name)
	}
}

// GetBuiltinOperations returns the list of operations for a builtin action.
func GetBuiltinOperations(name string) []string {
	switch name {
	case "file":
		return []string{
			"read", "read_text", "read_json", "read_yaml", "read_csv", "read_lines",
			"write", "write_text", "write_json", "write_yaml", "append", "render",
			"list", "exists", "stat", "mkdir", "copy", "move", "delete",
		}
	case "shell":
		return []string{"run"}
	case "transform":
		return []string{
			"parse_json", "parse_xml", "extract", "split", "map", "filter",
			"flatten", "sort", "group", "merge", "concat",
		}
	case "utility":
		return []string{
			"random_int", "random_choose", "random_weighted", "random_sample", "random_shuffle",
			"id_uuid", "id_nanoid", "id_custom",
			"math_clamp", "math_round", "math_min", "math_max",
		}
	case "http":
		return []string{
			"get", "post", "put", "patch", "delete", "request",
		}
	default:
		return nil
	}
}

// GetBuiltinDescription returns a description for a builtin action.
func GetBuiltinDescription(name string) string {
	switch name {
	case "file":
		return "Filesystem operations (read, write, list, copy, etc.)"
	case "shell":
		return "Shell command execution"
	case "transform":
		return "Data transformation operations (parse, extract, split, map, filter, etc.)"
	case "utility":
		return "Utility functions (random, ID generation, math operations)"
	case "http":
		return "HTTP requests (GET, POST, PUT, PATCH, DELETE with security controls)"
	default:
		return ""
	}
}
