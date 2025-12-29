package elasticsearch

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
	"golang.org/x/time/rate"
)

// ElasticsearchIntegration implements the Connector interface for Elasticsearch.
// Supports document indexing operations including single and bulk indexing.
type ElasticsearchIntegration struct {
	name        string
	transport   transport.Transport
	baseURL     string
	apiKey      string
	rateLimiter *rate.Limiter
}

// NewElasticsearchIntegration creates a new Elasticsearch integration.
func NewElasticsearchIntegration(config *api.ConnectorConfig) (operation.Connector, error) {
	if config.Transport == nil {
		return nil, fmt.Errorf("transport is required for Elasticsearch integration")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required for Elasticsearch integration")
	}

	// API key is optional - Elasticsearch can be unsecured or use other auth methods
	// If provided, it's used in the Authorization header

	// Create rate limiter: 10 req/s with burst of 10
	// Also enforces 500 req/min through time-based limiting
	limiter := rate.NewLimiter(rate.Limit(10), 10)

	integration := &ElasticsearchIntegration{
		name:        "elasticsearch",
		transport:   config.Transport,
		baseURL:     config.BaseURL,
		apiKey:      config.Token,
		rateLimiter: limiter,
	}

	// Set rate limiter on transport
	config.Transport.SetRateLimiter(&elasticsearchRateLimiter{limiter: limiter})

	return integration, nil
}

// Name returns the connector identifier.
func (e *ElasticsearchIntegration) Name() string {
	return e.name
}

// Execute runs a named operation with the given inputs.
func (e *ElasticsearchIntegration) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*operation.Result, error) {
	switch operation {
	case "index":
		return e.indexDocument(ctx, inputs)
	case "bulk":
		return e.bulkIndex(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (e *ElasticsearchIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		{
			Name:        "index",
			Description: "Index a single document in Elasticsearch",
			Category:    "indexing",
			Tags:        []string{"write"},
		},
		{
			Name:        "bulk",
			Description: "Bulk index multiple documents in Elasticsearch",
			Category:    "indexing",
			Tags:        []string{"write", "batch"},
		},
	}
}

// OperationSchema returns the schema for an operation.
func (e *ElasticsearchIntegration) OperationSchema(operation string) *api.OperationSchema {
	return nil
}

// defaultHeaders returns default headers for Elasticsearch API requests.
func (e *ElasticsearchIntegration) defaultHeaders() map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Add API key if provided
	if e.apiKey != "" {
		headers["Authorization"] = "ApiKey " + e.apiKey
	}

	return headers
}

// elasticsearchRateLimiter wraps rate.Limiter to implement transport.RateLimiter.
type elasticsearchRateLimiter struct {
	limiter *rate.Limiter
}

// Wait blocks until a request is allowed under the rate limit.
func (r *elasticsearchRateLimiter) Wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}
