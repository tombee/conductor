package jenkins

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// listNodes lists all build agents/nodes.
func (c *JenkinsIntegration) listNodes(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Build URL
	url := c.baseURL + "/computer/api/json"

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
	var nodeList NodeListResponse
	if err := c.ParseJSONResponse(resp, &nodeList); err != nil {
		return nil, err
	}

	// Transform to simplified format
	result := make([]map[string]interface{}, len(nodeList.Computer))
	for i, node := range nodeList.Computer {
		nodeInfo := map[string]interface{}{
			"display_name":  node.DisplayName,
			"description":   node.Description,
			"num_executors": node.NumExecutors,
			"offline":       node.Offline,
			"mode":          node.Mode,
		}

		// Add offline cause if present
		if node.OfflineCause != nil {
			nodeInfo["offline_reason"] = node.OfflineCause.Description
		}

		result[i] = nodeInfo
	}

	return c.ToResult(resp, result), nil
}

// getNode gets details about a specific node.
func (c *JenkinsIntegration) getNode(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"node_name"}); err != nil {
		return nil, err
	}

	// Build URL - note: master node is accessed via "(master)"
	nodeName := inputs["node_name"].(string)
	if nodeName == "master" || nodeName == "built-in" {
		nodeName = "(master)"
	}

	url, err := c.BuildURL("/computer/{node_name}/api/json", map[string]interface{}{
		"node_name": nodeName,
	})
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
	var node Node
	if err := c.ParseJSONResponse(resp, &node); err != nil {
		return nil, err
	}

	// Return simplified result
	result := map[string]interface{}{
		"display_name":        node.DisplayName,
		"description":         node.Description,
		"num_executors":       node.NumExecutors,
		"offline":             node.Offline,
		"mode":                node.Mode,
		"temporarily_offline": node.TemporarilyOffline,
	}

	// Add offline cause if present
	if node.OfflineCause != nil {
		result["offline_reason"] = node.OfflineCause.Description
	}

	// Add monitor data if available
	if node.MonitorData.DiskSpace != nil {
		result["disk_space_available"] = node.MonitorData.DiskSpace.Size
	}

	if node.MonitorData.ResponseTime != nil {
		result["response_time_ms"] = node.MonitorData.ResponseTime.Average
	}

	if node.MonitorData.Architecture != nil {
		result["architecture"] = node.MonitorData.Architecture.Value
	}

	return c.ToResult(resp, result), nil
}
