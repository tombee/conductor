package connector

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/connector/file"
	"github.com/tombee/conductor/internal/connector/shell"
	"github.com/tombee/conductor/internal/connector/transform"
	"github.com/tombee/conductor/internal/connector/utility"
	"github.com/tombee/conductor/pkg/workflow"
)

func init() {
	// Register the action registry factory with the workflow package.
	// This enables WithWorkflowDir() to automatically initialize builtin actions.
	workflow.SetDefaultActionRegistryFactory(func(workflowDir string) (workflow.ConnectorRegistry, error) {
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

// BuiltinConfig holds configuration for builtin connectors.
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
}

// builtinNames lists all builtin connector names.
var builtinNames = map[string]bool{
	"file":      true,
	"shell":     true,
	"transform": true,
	"utility":   true,
}

// IsBuiltin returns true if the connector name is a builtin.
func IsBuiltin(name string) bool {
	return builtinNames[name]
}

// BuiltinConnector wraps a builtin connector to implement the Connector interface.
type BuiltinConnector struct {
	name               string
	fileConnector      *file.FileConnector
	shellConnector     *shell.ShellConnector
	transformConnector *transform.TransformConnector
	utilityConnector   *utility.UtilityConnector
}

// NewBuiltin creates a builtin connector by name.
func NewBuiltin(name string, config *BuiltinConfig) (Connector, error) {
	if !IsBuiltin(name) {
		return nil, fmt.Errorf("unknown builtin connector: %s", name)
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
			return nil, fmt.Errorf("failed to create file connector: %w", err)
		}

		return &BuiltinConnector{
			name:          "file",
			fileConnector: fc,
		}, nil

	case "shell":
		shellConfig := &shell.Config{
			WorkingDir: config.WorkflowDir,
		}
		sc, err := shell.New(shellConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create shell connector: %w", err)
		}

		return &BuiltinConnector{
			name:           "shell",
			shellConnector: sc,
		}, nil

	case "transform":
		tc, err := transform.New(nil) // Use default config
		if err != nil {
			return nil, fmt.Errorf("failed to create transform connector: %w", err)
		}

		return &BuiltinConnector{
			name:               "transform",
			transformConnector: tc,
		}, nil

	case "utility":
		uc, err := utility.New(nil) // Use default config
		if err != nil {
			return nil, fmt.Errorf("failed to create utility connector: %w", err)
		}

		return &BuiltinConnector{
			name:             "utility",
			utilityConnector: uc,
		}, nil

	default:
		return nil, fmt.Errorf("unknown builtin connector: %s", name)
	}
}

// Name returns the connector identifier.
func (c *BuiltinConnector) Name() string {
	return c.name
}

// Execute runs a named operation with the given inputs.
func (c *BuiltinConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*Result, error) {
	switch c.name {
	case "file":
		result, err := c.fileConnector.Execute(ctx, operation, inputs)
		if err != nil {
			return nil, err
		}
		return &Result{
			Response: result.Response,
			Metadata: result.Metadata,
		}, nil

	case "shell":
		result, err := c.shellConnector.Execute(ctx, operation, inputs)
		if err != nil {
			return nil, err
		}
		return &Result{
			Response: result.Response,
			Metadata: result.Metadata,
		}, nil

	case "transform":
		result, err := c.transformConnector.Execute(ctx, operation, inputs)
		if err != nil {
			return nil, err
		}
		return &Result{
			Response: result.Response,
			Metadata: result.Metadata,
		}, nil

	case "utility":
		result, err := c.utilityConnector.Execute(ctx, operation, inputs)
		if err != nil {
			return nil, err
		}
		return &Result{
			Response: result.Response,
			Metadata: result.Metadata,
		}, nil

	default:
		return nil, fmt.Errorf("unknown builtin connector: %s", c.name)
	}
}

// GetBuiltinOperations returns the list of operations for a builtin connector.
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
	default:
		return nil
	}
}

// GetBuiltinDescription returns a description for a builtin connector.
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
	default:
		return ""
	}
}
