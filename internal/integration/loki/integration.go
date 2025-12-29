package loki

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
	"golang.org/x/time/rate"
)

// LokiIntegration implements the Connector interface for Grafana Loki.
// Supports log aggregation operations including single and batch log entries.
type LokiIntegration struct {
	name        string
	transport   transport.Transport
	baseURL     string
	rateLimiter *rate.Limiter
}

// NewLokiIntegration creates a new Loki integration.
func NewLokiIntegration(config *api.ConnectorConfig) (operation.Connector, error) {
	if config.Transport == nil {
		return nil, fmt.Errorf("transport is required for Loki integration")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required for Loki integration")
	}

	// Create rate limiter: 5 req/s with burst of 5
	// Also enforces 300 req/min through time-based limiting
	limiter := rate.NewLimiter(rate.Limit(5), 5)

	integration := &LokiIntegration{
		name:        "loki",
		transport:   config.Transport,
		baseURL:     config.BaseURL,
		rateLimiter: limiter,
	}

	// Set rate limiter on transport
	config.Transport.SetRateLimiter(&lokiRateLimiter{limiter: limiter})

	return integration, nil
}

// Name returns the connector identifier.
func (l *LokiIntegration) Name() string {
	return l.name
}

// Execute runs a named operation with the given inputs.
func (l *LokiIntegration) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*operation.Result, error) {
	switch operation {
	case "push":
		return l.pushLogs(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (l *LokiIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		{
			Name:        "push",
			Description: "Push logs to Loki",
			Category:    "logs",
			Tags:        []string{"write"},
		},
	}
}

// OperationSchema returns the schema for an operation.
func (l *LokiIntegration) OperationSchema(operation string) *api.OperationSchema {
	return nil
}

// defaultHeaders returns default headers for Loki API requests.
func (l *LokiIntegration) defaultHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}

// lokiRateLimiter wraps rate.Limiter to implement transport.RateLimiter.
type lokiRateLimiter struct {
	limiter *rate.Limiter
}

// Wait blocks until a request is allowed under the rate limit.
func (r *lokiRateLimiter) Wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}
