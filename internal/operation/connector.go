package operation

import (
	"context"
	"fmt"
	"strings"

	"github.com/tombee/conductor/internal/operation/transport"
	"github.com/tombee/conductor/pkg/workflow"
)

// TransportRegistry is an alias for transport.Registry to avoid import cycles
// and provide a cleaner API at the connector package level.
type TransportRegistry = transport.Registry

// builtinAPIFactory is a function type for creating builtin API connectors.
type builtinAPIFactory func(connectorName string, baseURL string, authType string, authToken string) (Connector, error)

// builtinAPIFactories holds registered builtin API connector factories.
var builtinAPIFactories = make(map[string]builtinAPIFactory)

// RegisterBuiltinAPI registers a builtin API connector factory.
// This is called by the builtin package during init().
func RegisterBuiltinAPI(name string, factory builtinAPIFactory) {
	builtinAPIFactories[name] = factory
}

// isBuiltinAPI returns true if a builtin API connector is registered.
func isBuiltinAPI(name string) bool {
	_, ok := builtinAPIFactories[name]
	return ok
}

// newBuiltinAPIConnector creates a builtin API connector.
func newBuiltinAPIConnector(name string, baseURL string, authType string, authToken string) (Connector, error) {
	factory, ok := builtinAPIFactories[name]
	if !ok {
		return nil, &Error{
			Type:    ErrorTypeNotFound,
			Message: "builtin API connector not found: " + name,
		}
	}
	return factory(name, baseURL, authType, authToken)
}

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

	// TransportRegistry manages transport instances (HTTP, AWS SigV4, OAuth2)
	// If nil, a default registry will be created
	TransportRegistry *TransportRegistry
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
	// Initialize transport registry if not provided
	if config.TransportRegistry == nil {
		config.TransportRegistry = NewDefaultTransportRegistry()
	}

	// For package-based connectors (from: connectors/github)
	if def.From != "" {
		return newPackageConnector(def, config)
	}

	// For inline HTTP connectors (base_url + operations)
	return newHTTPConnector(def, config)
}

// newPackageConnector creates a connector from a package reference.
func newPackageConnector(def *workflow.ConnectorDefinition, config *Config) (Connector, error) {
	// Extract connector name from "connectors/<name>"
	if strings.HasPrefix(def.From, "connectors/") {
		parts := strings.Split(def.From, "/")
		if len(parts) == 2 {
			connectorName := parts[1]

			// Check if it's a registered builtin API connector
			if isBuiltinAPI(connectorName) {
				var authType, authToken string

				// Handle authentication based on AuthDefinition fields
				if def.Auth != nil {
					switch def.Auth.Type {
					case "bearer", "":
						if def.Auth.Token != "" {
							authType = "bearer"
							authToken = def.Auth.Token
						}
					case "basic":
						if def.Auth.Username != "" {
							authType = "basic"
							authToken = def.Auth.Username + ":" + def.Auth.Password
						}
					case "api_key":
						if def.Auth.Value != "" {
							authType = "api_key"
							authToken = def.Auth.Value
						}
					}
				}

				return newBuiltinAPIConnector(connectorName, def.BaseURL, authType, authToken)
			}
		}
	}

	// Fall back to loading package definition (for custom YAML connectors)
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

	// Create transport based on connector definition
	transportType := def.Transport
	if transportType == "" {
		transportType = "http" // Default to HTTP transport
	}

	var transportInstance transport.Transport
	var err error

	switch transportType {
	case "http":
		httpConfig := toHTTPTransportConfig(def)
		transportInstance, err = config.TransportRegistry.Create("http", httpConfig)
		if err != nil {
			return nil, &Error{
				Type:        ErrorTypeValidation,
				Message:     fmt.Sprintf("failed to create HTTP transport for connector %q: %v", def.Name, err),
				SuggestText: "Check that base_url is valid and authentication is properly configured",
			}
		}

	case "aws_sigv4":
		awsConfig, configErr := toAWSTransportConfig(def)
		if configErr != nil {
			return nil, &Error{
				Type:        ErrorTypeValidation,
				Message:     fmt.Sprintf("invalid AWS configuration for connector %q: %v", def.Name, configErr),
				SuggestText: "Ensure 'aws' section includes 'service' and 'region' fields",
			}
		}
		transportInstance, err = config.TransportRegistry.Create("aws_sigv4", awsConfig)
		if err != nil {
			return nil, &Error{
				Type:        ErrorTypeValidation,
				Message:     fmt.Sprintf("failed to create AWS SigV4 transport for connector %q: %v", def.Name, err),
				SuggestText: "Check AWS credentials and region configuration",
			}
		}

	case "oauth2":
		oauth2Config, configErr := toOAuth2TransportConfig(def)
		if configErr != nil {
			return nil, &Error{
				Type:        ErrorTypeValidation,
				Message:     fmt.Sprintf("invalid OAuth2 configuration for connector %q: %v", def.Name, configErr),
				SuggestText: "Ensure 'oauth2' section includes client_id, client_secret, and token_url",
			}
		}
		transportInstance, err = config.TransportRegistry.Create("oauth2", oauth2Config)
		if err != nil {
			return nil, &Error{
				Type:        ErrorTypeValidation,
				Message:     fmt.Sprintf("failed to create OAuth2 transport for connector %q: %v", def.Name, err),
				SuggestText: "Check OAuth2 credentials and token URL configuration",
			}
		}

	default:
		return nil, &Error{
			Type:        ErrorTypeValidation,
			Message:     fmt.Sprintf("unsupported transport type %q for connector %q", transportType, def.Name),
			SuggestText: "Supported transport types: http, aws_sigv4, oauth2",
		}
	}

	return &httpConnector{
		name:             def.Name,
		def:              def,
		config:           config,
		metricsCollector: metricsCollector,
		transport:        transportInstance,
	}, nil
}

// httpConnector implements Connector for inline HTTP definitions.
type httpConnector struct {
	name             string
	def              *workflow.ConnectorDefinition
	config           *Config
	metricsCollector *MetricsCollector
	transport        transport.Transport
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
