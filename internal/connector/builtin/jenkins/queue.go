package jenkins

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
)

// getQueueItem gets the status of a queued build.
func (c *JenkinsConnector) getQueueItem(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"queue_id"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/queue/item/{queue_id}/api/json", inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "GET", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var queueItem QueueItem
	if err := c.ParseJSONResponse(resp, &queueItem); err != nil {
		return nil, err
	}

	// Return simplified result
	result := map[string]interface{}{
		"id":         queueItem.ID,
		"blocked":    queueItem.Blocked,
		"buildable":  queueItem.Buildable,
		"stuck":      queueItem.Stuck,
		"cancelled":  queueItem.Cancelled,
		"why":        queueItem.Why,
		"task_name":  queueItem.Task.Name,
	}

	// Add executable info if build has started
	if queueItem.Executable != nil {
		result["build_started"] = true
		result["build_number"] = queueItem.Executable.Number
		result["build_url"] = queueItem.Executable.URL
	} else {
		result["build_started"] = false
	}

	return c.ToConnectorResult(resp, result), nil
}

// cancelQueueItem cancels a queued build before it starts.
func (c *JenkinsConnector) cancelQueueItem(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"queue_id"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/queue/cancelItem?id={queue_id}", inputs)
	if err != nil {
		return nil, err
	}

	// Get CRUMB token if needed
	headers := c.defaultHeaders()
	if err := c.addCrumb(ctx, headers); err != nil {
		return nil, fmt.Errorf("failed to get CRUMB token: %w", err)
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "POST", url, headers, nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	return c.ToConnectorResult(resp, map[string]interface{}{
		"cancelled": true,
	}), nil
}
