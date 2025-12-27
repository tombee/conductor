// Package connector provides runtime execution for declarative connectors.
//
// Connectors are deterministic, schema-validated operations that execute without
// LLM involvement. They provide HTTP-based integrations with built-in auth,
// rate limiting, response transforms, and retry logic.
package connector

import (
	"context"

	"github.com/tombee/conductor/pkg/workflow"
)

// Connector represents a configured external integration.
// Each connector can execute multiple named operations.
type Connector interface {
	// Name returns the connector identifier
	Name() string

	// Execute runs a named operation with the given inputs
	Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*Result, error)
}

// PaginatedConnector extends Connector to support paginated operations.
// Connectors that support pagination (GitHub, Slack, Jira, Discord) implement
// this interface to stream results through a channel.
type PaginatedConnector interface {
	Connector

	// ExecutePaginated returns a channel of results for paginated operations.
	// The channel is closed when all results have been sent or an error occurs.
	// Errors are included in the Result.Metadata["error"] field.
	//
	// Supported options in inputs:
	// - paginate: bool - Enable pagination (default: false)
	// - max_results: int - Maximum number of results to return (default: unlimited)
	// - page_size: int - Number of results per page (default: API-specific)
	ExecutePaginated(ctx context.Context, operation string, inputs map[string]interface{}) (<-chan *Result, error)
}

// Result represents the output of a connector operation.
type Result struct {
	// Response is the transformed response data
	Response interface{}

	// RawResponse is the original response before transformation (for debugging)
	RawResponse interface{}

	// StatusCode is the HTTP status code (for HTTP connectors)
	StatusCode int

	// Headers contains response headers
	Headers map[string][]string

	// Metadata contains execution metadata (request ID, timing, etc.)
	Metadata map[string]interface{}
}

// GetResponse returns the transformed response data.
func (r *Result) GetResponse() interface{} {
	return r.Response
}

// GetRawResponse returns the original response before transformation.
func (r *Result) GetRawResponse() interface{} {
	return r.RawResponse
}

// GetStatusCode returns the HTTP status code (for HTTP connectors).
func (r *Result) GetStatusCode() int {
	return r.StatusCode
}

// GetMetadata returns execution metadata.
func (r *Result) GetMetadata() map[string]interface{} {
	return r.Metadata
}

// Config holds runtime configuration for connector execution.
type Config struct {
	// MaxConcurrentRequests limits concurrent operations per connector
	MaxConcurrentRequests int

	// DefaultTimeout is the default operation timeout in seconds
	DefaultTimeout int

	// StateFilePath is where rate limit state is persisted
	StateFilePath string

	// EnableMetrics controls whether Prometheus metrics are exported
	EnableMetrics bool

	// MetricsCollector is the metrics collector instance (optional)
	// If nil and EnableMetrics is true, a new collector will be created
	MetricsCollector *MetricsCollector

	// AllowedHosts restricts which hosts can be accessed (SSRF protection)
	// Empty list means all hosts allowed
	AllowedHosts []string

	// BlockedHosts prevents access to specific hosts (SSRF protection)
	// Defaults to private IP ranges and metadata endpoints
	BlockedHosts []string
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxConcurrentRequests: 10,
		DefaultTimeout:        30,
		StateFilePath:         "", // Set by caller
		EnableMetrics:         false,
		AllowedHosts:          []string{},
		BlockedHosts: []string{
			// Private IP ranges
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"127.0.0.0/8",
			"::1/128",
			// Link-local
			"169.254.0.0/16",
			"fe80::/10",
			// Cloud metadata endpoints
			"169.254.169.254",
		},
	}
}

// New creates a connector from a workflow definition.
func New(def *workflow.ConnectorDefinition, config *Config) (Connector, error) {
	// For package-based connectors (from: connectors/github)
	if def.From != "" {
		return newPackageConnector(def, config)
	}

	// For inline HTTP connectors (base_url + operations)
	return newHTTPConnector(def, config)
}

// newPackageConnector creates a connector from a package reference.
func newPackageConnector(def *workflow.ConnectorDefinition, config *Config) (Connector, error) {
	// Load the package definition
	pkg, err := loadPackage(def.From)
	if err != nil {
		return nil, err
	}

	// Merge package with user overrides (auth, base_url, etc.)
	merged := mergePackageWithOverrides(pkg, def)

	// Create HTTP connector from merged definition
	return newHTTPConnector(merged, config)
}

// newHTTPConnector creates an inline HTTP connector.
func newHTTPConnector(def *workflow.ConnectorDefinition, config *Config) (Connector, error) {
	// Initialize metrics collector if enabled
	var metricsCollector *MetricsCollector
	if config.EnableMetrics {
		if config.MetricsCollector != nil {
			metricsCollector = config.MetricsCollector
		} else {
			metricsCollector = NewMetricsCollector()
		}
	}

	return &httpConnector{
		name:            def.Name,
		def:             def,
		config:          config,
		metricsCollector: metricsCollector,
	}, nil
}

// httpConnector implements Connector for inline HTTP definitions.
type httpConnector struct {
	name            string
	def             *workflow.ConnectorDefinition
	config          *Config
	metricsCollector *MetricsCollector
}

// Name returns the connector identifier.
func (c *httpConnector) Name() string {
	return c.name
}

// Execute runs a named operation.
func (c *httpConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*Result, error) {
	// Create executor for this operation
	executor, err := newHTTPExecutor(c, operation)
	if err != nil {
		return nil, err
	}

	// Execute the operation
	return executor.Execute(ctx, inputs)
}

// GetMetricsCollector returns the metrics collector for this connector.
func (c *httpConnector) GetMetricsCollector() *MetricsCollector {
	return c.metricsCollector
}
