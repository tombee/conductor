// Package transform provides a builtin action for data transformation operations.
//
// The transform action does NOT import the operation package to avoid import cycles.
// The builtin registry bridges between transform operations and the operation.Connector interface.
package transform

import (
	"context"
	"fmt"
	"strings"
)

// TransformConnector implements the action interface for transform operations.
type TransformConnector struct {
	config *Config
}

// Config holds configuration for the transform action.
type Config struct {
	// MaxInputSize is the maximum input size in bytes (default: 10MB)
	MaxInputSize int64

	// MaxOutputSize is the maximum output size in bytes (default: 10MB)
	MaxOutputSize int64

	// MaxArrayItems is the maximum number of items in an array (default: 10,000)
	MaxArrayItems int

	// MaxRecursionDepth is the maximum depth for nested structures (default: 100)
	MaxRecursionDepth int

	// ExpressionTimeout is the timeout for jq expression evaluation (default: 1s)
	ExpressionTimeout int64 // nanoseconds
}

// DefaultConfig returns sensible defaults for transform action configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxInputSize:      10 * 1024 * 1024, // 10MB
		MaxOutputSize:     10 * 1024 * 1024, // 10MB
		MaxArrayItems:     10000,
		MaxRecursionDepth: 100,
		ExpressionTimeout: 1000000000, // 1 second in nanoseconds
	}
}

// New creates a new transform action instance.
func New(config *Config) (*TransformConnector, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Apply defaults to unset fields
	if config.MaxInputSize == 0 {
		config.MaxInputSize = 10 * 1024 * 1024 // 10MB
	}
	if config.MaxOutputSize == 0 {
		config.MaxOutputSize = 10 * 1024 * 1024 // 10MB
	}
	if config.MaxArrayItems == 0 {
		config.MaxArrayItems = 10000
	}
	if config.MaxRecursionDepth == 0 {
		config.MaxRecursionDepth = 100
	}
	if config.ExpressionTimeout == 0 {
		config.ExpressionTimeout = 1000000000 // 1 second
	}

	return &TransformConnector{
		config: config,
	}, nil
}

// Name returns the action identifier.
func (c *TransformConnector) Name() string {
	return "transform"
}

// Result represents the output of a transform operation.
type Result struct {
	Response interface{}
	Metadata map[string]interface{}
}

// Execute runs a named transform operation with the given inputs.
func (c *TransformConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*Result, error) {
	switch operation {
	case "parse_json":
		return c.parseJSON(ctx, inputs)
	case "parse_xml":
		return c.parseXML(ctx, inputs)
	case "extract":
		return c.extract(ctx, inputs)
	case "split":
		return c.split(ctx, inputs)
	case "map":
		return c.mapArray(ctx, inputs)
	case "filter":
		return c.filter(ctx, inputs)
	case "flatten":
		return c.flatten(ctx, inputs)
	case "sort":
		return c.sort(ctx, inputs)
	case "group":
		return c.group(ctx, inputs)
	case "merge":
		return c.merge(ctx, inputs)
	case "concat":
		return c.concat(ctx, inputs)
	default:
		return nil, &OperationError{
			Operation: operation,
			Message:   "unknown operation",
			ErrorType: ErrorTypeValidation,
			Suggestion: "Valid operations: parse_json, parse_xml, extract, split, map, filter, flatten, sort, group, merge, concat",
		}
	}
}

// Note: parseJSON is implemented in parse.go

// parseXML implements the parse_xml operation.
// Parses XML with XXE prevention and security audit logging.
func (c *TransformConnector) parseXML(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Extract data input
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation:  "parse_xml",
			Message:    "missing required parameter: data",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide data parameter with XML text to parse",
		}
	}

	// Handle null/undefined input
	if data == nil {
		return nil, &OperationError{
			Operation:  "parse_xml",
			Message:    "input is null or undefined",
			ErrorType:  ErrorTypeEmptyInput,
			Suggestion: "Ensure the input contains valid XML data",
		}
	}

	// Must be a string or bytes
	var xmlBytes []byte
	switch v := data.(type) {
	case string:
		xmlBytes = []byte(v)
	case []byte:
		xmlBytes = v
	default:
		return nil, &OperationError{
			Operation:  "parse_xml",
			Message:    fmt.Sprintf("data must be string or bytes, got %T", data),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Convert data to string before parsing XML",
		}
	}

	// Security: check size limit
	if int64(len(xmlBytes)) > c.config.MaxInputSize {
		return nil, &OperationError{
			Operation:  "parse_xml",
			Message:    fmt.Sprintf("input size %d bytes exceeds maximum %d bytes", len(xmlBytes), c.config.MaxInputSize),
			ErrorType:  ErrorTypeLimitExceeded,
			Suggestion: fmt.Sprintf("Reduce input size to under %d bytes", c.config.MaxInputSize),
		}
	}

	// Security audit logging: log XML parsing attempt
	// In production, this would go to a security audit log
	// For now, we'll add metadata to track the attempt
	metadata := map[string]interface{}{
		"document_size": len(xmlBytes),
		"operation":     "parse_xml",
	}

	// Parse XML options
	options := DefaultXMLParseOptions()
	if attrPrefix, ok := inputs["attribute_prefix"].(string); ok {
		options.AttributePrefix = attrPrefix
	}
	if stripNS, ok := inputs["strip_namespaces"].(bool); ok {
		options.StripNamespaces = stripNS
	}

	// Parse XML with XXE prevention
	result, err := parseXMLToMap(xmlBytes, options)
	if err != nil {
		// Check if it's an XXE prevention error
		if strings.Contains(err.Error(), "XXE prevention") {
			// Security audit: XXE attempt detected
			metadata["xxe_attempt_detected"] = true
			return nil, &OperationError{
				Operation:  "parse_xml",
				Message:    err.Error(),
				ErrorType:  ErrorTypeValidation,
				Suggestion: "Remove DOCTYPE, ENTITY, SYSTEM, or PUBLIC declarations from XML",
			}
		}

		return nil, &OperationError{
			Operation:  "parse_xml",
			Message:    "XML parse error",
			ErrorType:  ErrorTypeParseError,
			Cause:      err,
			Suggestion: "Verify XML is well-formed and uses valid syntax",
		}
	}

	return &Result{
		Response: result,
		Metadata: metadata,
	}, nil
}

// Note: extract is implemented in extract.go
// Note: split, filter, and mapArray are implemented in array.go
// Note: flatten, merge, and concat are implemented in combine.go
// Note: sort and group are implemented in sort.go
