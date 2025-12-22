package datadog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/transport"
)

// sendEvent sends an event to Datadog Events API.
func (d *DatadogIntegration) sendEvent(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	title, ok := inputs["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("missing required parameter: title")
	}

	text, ok := inputs["text"].(string)
	if !ok || text == "" {
		return nil, fmt.Errorf("missing required parameter: text")
	}

	// Build event object
	event := map[string]interface{}{
		"title": title,
		"text":  text,
	}

	// Add optional fields
	if priority, ok := inputs["priority"].(string); ok && priority != "" {
		// Validate priority enum
		validPriorities := map[string]bool{
			"normal": true, "low": true,
		}
		if !validPriorities[priority] {
			return nil, fmt.Errorf("invalid priority: %s (must be one of: normal, low)", priority)
		}
		event["priority"] = priority
	}

	if alertType, ok := inputs["alert_type"].(string); ok && alertType != "" {
		// Validate alert_type enum
		validAlertTypes := map[string]bool{
			"info": true, "warning": true, "error": true, "success": true,
		}
		if !validAlertTypes[alertType] {
			return nil, fmt.Errorf("invalid alert_type: %s (must be one of: info, warning, error, success)", alertType)
		}
		event["alert_type"] = alertType
	}

	if tags, ok := inputs["tags"].([]interface{}); ok && len(tags) > 0 {
		tagStrings := make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				tagStrings = append(tagStrings, tagStr)
			}
		}
		if len(tagStrings) > 0 {
			event["tags"] = tagStrings
		}
	}

	if aggregationKey, ok := inputs["aggregation_key"].(string); ok && aggregationKey != "" {
		event["aggregation_key"] = aggregationKey
	}

	if sourceTypeName, ok := inputs["source_type_name"].(string); ok && sourceTypeName != "" {
		event["source_type_name"] = sourceTypeName
	}

	if host, ok := inputs["host"].(string); ok && host != "" {
		event["host"] = host
	}

	// Add timestamp (auto-populate if omitted)
	if dateHappened, ok := inputs["date_happened"]; ok {
		switch ts := dateHappened.(type) {
		case int64:
			event["date_happened"] = ts
		case int:
			event["date_happened"] = int64(ts)
		case float64:
			event["date_happened"] = int64(ts)
		}
	} else {
		event["date_happened"] = getCurrentTimestamp()
	}

	// Marshal request body
	bodyBytes, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event payload: %w", err)
	}

	// Build request
	url := d.getAPIBaseURL() + "/api/v1/events"
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
			response = map[string]interface{}{
				"status": resp.StatusCode,
			}
		}
	} else {
		response = map[string]interface{}{
			"status": resp.StatusCode,
		}
	}

	// Extract event_id if present (per spec response_transform)
	if eventData, ok := response["event"].(map[string]interface{}); ok {
		if eventID, ok := eventData["id"]; ok {
			response["event_id"] = eventID
		}
	}

	return &operation.Result{
		Response:    response,
		RawResponse: resp.Body,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Headers,
		Metadata:    resp.Metadata,
	}, nil
}
