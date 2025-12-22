package splunk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/transport"
)

// sendLog sends a log event to Splunk HEC.
func (s *SplunkIntegration) sendLog(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameter: event
	event, ok := inputs["event"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: event")
	}

	// Build HEC event payload
	payload := map[string]interface{}{
		"event": event,
	}

	// Add optional fields
	if index, ok := inputs["index"].(string); ok && index != "" {
		payload["index"] = index
	}

	if source, ok := inputs["source"].(string); ok && source != "" {
		payload["source"] = source
	}

	if sourcetype, ok := inputs["sourcetype"].(string); ok && sourcetype != "" {
		payload["sourcetype"] = sourcetype
	}

	if host, ok := inputs["host"].(string); ok && host != "" {
		payload["host"] = host
	}

	// Add time (auto-populate if omitted)
	if timeInput, ok := inputs["time"]; ok {
		switch t := timeInput.(type) {
		case float64:
			payload["time"] = t
		case int:
			payload["time"] = float64(t)
		case int64:
			payload["time"] = float64(t)
		}
	} else {
		// Auto-populate with current Unix timestamp
		payload["time"] = float64(time.Now().Unix())
	}

	// Add fields if provided
	if fields, ok := inputs["fields"].(map[string]interface{}); ok && len(fields) > 0 {
		payload["fields"] = fields
	}

	// Marshal request body
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log payload: %w", err)
	}

	// Build request
	url := s.baseURL + "/services/collector/event"
	req := &transport.Request{
		Method:  "POST",
		URL:     url,
		Headers: s.defaultHeaders(),
		Body:    bodyBytes,
	}

	// Execute request
	resp, err := s.transport.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse response
	var response map[string]interface{}
	if len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
	} else {
		response = map[string]interface{}{
			"status": resp.StatusCode,
		}
	}

	// Extract response fields as per spec: {text, code, ackId}
	transformedResponse := make(map[string]interface{})
	if text, ok := response["text"]; ok {
		transformedResponse["text"] = text
	}
	if code, ok := response["code"]; ok {
		transformedResponse["code"] = code
	}
	if ackId, ok := response["ackId"]; ok {
		transformedResponse["ackId"] = ackId
	}

	return &operation.Result{
		Response:    transformedResponse,
		RawResponse: resp.Body,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Headers,
		Metadata:    resp.Metadata,
	}, nil
}
