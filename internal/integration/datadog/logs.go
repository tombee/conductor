package datadog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/transport"
)

// sendLog sends a log event to Datadog Logs API.
func (d *DatadogIntegration) sendLog(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameter: message
	message, ok := inputs["message"].(string)
	if !ok || message == "" {
		return nil, fmt.Errorf("missing required parameter: message")
	}

	// Build log entry
	logEntry := map[string]interface{}{
		"message": message,
	}

	// Add optional fields
	if status, ok := inputs["status"].(string); ok && status != "" {
		// Validate status enum
		validStatuses := map[string]bool{
			"debug": true, "info": true, "warn": true, "error": true, "critical": true,
		}
		if !validStatuses[status] {
			return nil, fmt.Errorf("invalid status: %s (must be one of: debug, info, warn, error, critical)", status)
		}
		logEntry["status"] = status
	}

	if service, ok := inputs["service"].(string); ok && service != "" {
		logEntry["service"] = service
	}

	if source, ok := inputs["source"].(string); ok && source != "" {
		logEntry["ddsource"] = source
	}

	if tags, ok := inputs["tags"].([]interface{}); ok && len(tags) > 0 {
		// Convert to string array
		tagStrings := make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				tagStrings = append(tagStrings, tagStr)
			}
		}
		if len(tagStrings) > 0 {
			logEntry["ddtags"] = tagStrings
		}
	}

	if attributes, ok := inputs["attributes"].(map[string]interface{}); ok && len(attributes) > 0 {
		logEntry["attributes"] = attributes
	}

	if hostname, ok := inputs["hostname"].(string); ok && hostname != "" {
		logEntry["hostname"] = hostname
	}

	// Add timestamp (auto-populate if omitted)
	if timestamp, ok := inputs["timestamp"]; ok {
		switch ts := timestamp.(type) {
		case int64:
			logEntry["timestamp"] = ts * 1000 // Convert to milliseconds
		case int:
			logEntry["timestamp"] = int64(ts) * 1000
		case float64:
			logEntry["timestamp"] = int64(ts) * 1000
		}
	} else {
		logEntry["timestamp"] = getCurrentTimestamp() * 1000 // milliseconds
	}

	// Wrap in array for API
	payload := []map[string]interface{}{logEntry}

	// Marshal request body
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log payload: %w", err)
	}

	// Build request
	url := d.getLogsBaseURL() + "/api/v2/logs"
	req := &transport.Request{
		Method:  "POST",
		URL:     url,
		Headers: d.defaultHeaders(),
		Body:    bodyBytes,
	}

	// Execute request
	resp, err := d.transport.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse response
	var response map[string]interface{}
	if len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, &response); err != nil {
			// If response is not JSON, create a simple response
			response = map[string]interface{}{
				"status": resp.StatusCode,
			}
		}
	} else {
		response = map[string]interface{}{
			"status": resp.StatusCode,
		}
	}

	// Add logs_accepted field as per spec
	response["logs_accepted"] = 1

	return &operation.Result{
		Response:    response,
		RawResponse: resp.Body,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Headers,
		Metadata:    resp.Metadata,
	}, nil
}
