package cloudwatch

import (
	"context"
	"fmt"
	"sync"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
)

// CloudWatchIntegration implements the Connector interface for AWS CloudWatch.
// Supports both CloudWatch Logs and CloudWatch Metrics operations.
type CloudWatchIntegration struct {
	name      string
	transport transport.Transport
	region    string

	// Sequence token cache for CloudWatch Logs streams
	// Key format: "logGroup/logStream"
	sequenceTokens map[string]string
	tokenMutex     sync.RWMutex

	// Auto-create stream setting
	autoCreateStream bool
}

// NewCloudWatchIntegration creates a new CloudWatch integration.
func NewCloudWatchIntegration(config *api.ConnectorConfig) (operation.Connector, error) {
	if config.Transport == nil {
		return nil, fmt.Errorf("transport is required for CloudWatch integration")
	}

	// Verify it's an AWS SigV4 transport
	if config.Transport.Name() != "aws_sigv4" {
		return nil, fmt.Errorf("CloudWatch integration requires aws_sigv4 transport, got: %s", config.Transport.Name())
	}

	// Extract region from additional auth if provided
	region := "us-east-1"
	if config.AdditionalAuth != nil {
		if r, ok := config.AdditionalAuth["region"]; ok {
			region = r
		}
	}

	// Extract auto_create_stream setting (default: true)
	autoCreate := true
	if config.AdditionalAuth != nil {
		if ac, ok := config.AdditionalAuth["auto_create_stream"]; ok {
			autoCreate = ac == "true"
		}
	}

	return &CloudWatchIntegration{
		name:             "cloudwatch",
		transport:        config.Transport,
		region:           region,
		sequenceTokens:   make(map[string]string),
		autoCreateStream: autoCreate,
	}, nil
}

// Name returns the connector identifier.
func (c *CloudWatchIntegration) Name() string {
	return c.name
}

// Execute runs a named operation with the given inputs.
func (c *CloudWatchIntegration) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*operation.Result, error) {
	switch operation {
	case "log":
		return c.putLogEvents(ctx, inputs)
	case "metric":
		return c.putMetricData(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (c *CloudWatchIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		{
			Name:        "log",
			Description: "Send log events to CloudWatch Logs",
			Category:    "logs",
			Tags:        []string{"write"},
		},
		{
			Name:        "metric",
			Description: "Send metrics to CloudWatch Metrics",
			Category:    "metrics",
			Tags:        []string{"write"},
		},
	}
}

// OperationSchema returns the schema for an operation.
func (c *CloudWatchIntegration) OperationSchema(operation string) *api.OperationSchema {
	// For now, returning nil (can be expanded based on requirements)
	return nil
}

// getSequenceToken retrieves the cached sequence token for a stream.
func (c *CloudWatchIntegration) getSequenceToken(logGroup, logStream string) string {
	c.tokenMutex.RLock()
	defer c.tokenMutex.RUnlock()

	key := fmt.Sprintf("%s/%s", logGroup, logStream)
	return c.sequenceTokens[key]
}

// setSequenceToken caches a sequence token for a stream.
func (c *CloudWatchIntegration) setSequenceToken(logGroup, logStream, token string) {
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	key := fmt.Sprintf("%s/%s", logGroup, logStream)
	c.sequenceTokens[key] = token
}

// clearSequenceToken removes a cached sequence token.
func (c *CloudWatchIntegration) clearSequenceToken(logGroup, logStream string) {
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	key := fmt.Sprintf("%s/%s", logGroup, logStream)
	delete(c.sequenceTokens, key)
}
