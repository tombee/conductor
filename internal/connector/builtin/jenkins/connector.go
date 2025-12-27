package jenkins

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/internal/connector/api"
	"github.com/tombee/conductor/internal/connector/transport"
)

// JenkinsConnector implements the Connector interface for Jenkins API.
type JenkinsConnector struct {
	*api.BaseConnector
	baseURL   string
	username  string
	token     string
	transport transport.Transport
}

// NewJenkinsConnector creates a new Jenkins connector.
func NewJenkinsConnector(config *api.ConnectorConfig) (connector.Connector, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("jenkins connector requires base_url configuration")
	}

	base := api.NewBaseConnector("jenkins", config)

	// Jenkins auth can be either:
	// 1. token only (for API token)
	// 2. username + token (for user API token - more common)
	username := ""
	if config.AdditionalAuth != nil {
		if u, ok := config.AdditionalAuth["username"]; ok {
			username = u
		}
	}

	return &JenkinsConnector{
		BaseConnector: base,
		baseURL:       config.BaseURL,
		username:      username,
		token:         config.Token,
		transport:     config.Transport,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *JenkinsConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*connector.Result, error) {
	switch operation {
	// Builds
	case "trigger_build":
		return c.triggerBuild(ctx, inputs)
	case "trigger_build_with_parameters":
		return c.triggerBuildWithParameters(ctx, inputs)
	case "get_build":
		return c.getBuild(ctx, inputs)
	case "get_build_log":
		return c.getBuildLog(ctx, inputs)
	case "cancel_build":
		return c.cancelBuild(ctx, inputs)

	// Jobs
	case "get_job":
		return c.getJob(ctx, inputs)
	case "list_jobs":
		return c.listJobs(ctx, inputs)

	// Queue
	case "get_queue_item":
		return c.getQueueItem(ctx, inputs)
	case "cancel_queue_item":
		return c.cancelQueueItem(ctx, inputs)

	// Test Results
	case "get_test_report":
		return c.getTestReport(ctx, inputs)

	// Nodes
	case "list_nodes":
		return c.listNodes(ctx, inputs)
	case "get_node":
		return c.getNode(ctx, inputs)

	// Build Info
	case "get_last_build":
		return c.getLastBuild(ctx, inputs)
	case "get_last_successful_build":
		return c.getLastSuccessfulBuild(ctx, inputs)
	case "get_last_failed_build":
		return c.getLastFailedBuild(ctx, inputs)

	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (c *JenkinsConnector) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		// Builds
		{Name: "trigger_build", Description: "Trigger a build", Category: "builds", Tags: []string{"write"}},
		{Name: "trigger_build_with_parameters", Description: "Trigger a parameterized build", Category: "builds", Tags: []string{"write"}},
		{Name: "get_build", Description: "Get build details", Category: "builds", Tags: []string{"read"}},
		{Name: "get_build_log", Description: "Get console output", Category: "builds", Tags: []string{"read"}},
		{Name: "cancel_build", Description: "Cancel a running build", Category: "builds", Tags: []string{"write"}},

		// Jobs
		{Name: "get_job", Description: "Get job configuration", Category: "jobs", Tags: []string{"read"}},
		{Name: "list_jobs", Description: "List jobs in folder", Category: "jobs", Tags: []string{"read"}},

		// Queue
		{Name: "get_queue_item", Description: "Get queue item status", Category: "queue", Tags: []string{"read"}},
		{Name: "cancel_queue_item", Description: "Cancel a queued build", Category: "queue", Tags: []string{"write"}},

		// Test Results
		{Name: "get_test_report", Description: "Get test results for a build", Category: "tests", Tags: []string{"read"}},

		// Nodes
		{Name: "list_nodes", Description: "List build agents", Category: "nodes", Tags: []string{"read"}},
		{Name: "get_node", Description: "Get node details", Category: "nodes", Tags: []string{"read"}},

		// Build Info
		{Name: "get_last_build", Description: "Get last build info", Category: "builds", Tags: []string{"read"}},
		{Name: "get_last_successful_build", Description: "Get last successful build", Category: "builds", Tags: []string{"read"}},
		{Name: "get_last_failed_build", Description: "Get last failed build", Category: "builds", Tags: []string{"read"}},
	}
}

// OperationSchema returns the schema for an operation.
func (c *JenkinsConnector) OperationSchema(operation string) *api.OperationSchema {
	// This would return detailed schema information for each operation
	// For now, returning nil (would be implemented based on requirements)
	return nil
}

// ExecuteRequest overrides BaseConnector to use Basic auth instead of Bearer.
func (c *JenkinsConnector) ExecuteRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (*transport.Response, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	// Jenkins uses Basic auth (username:token)
	if c.token != "" {
		var authString string
		if c.username != "" {
			// username:token format
			authString = c.username + ":" + c.token
		} else {
			// If no username, use token as username with empty password
			authString = c.token + ":"
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(authString))
		headers["Authorization"] = "Basic " + encoded
	}

	req := &transport.Request{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
	}

	return c.transport.Execute(ctx, req)
}
