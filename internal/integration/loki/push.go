package loki

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/transport"
)

// pushLogs pushes logs to Loki.
func (l *LokiIntegration) pushLogs(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Build streams array for Loki push API
	var streams []map[string]interface{}

	// Check if this is a batch submission (entries array provided)
	if entriesInput, ok := inputs["entries"].([]interface{}); ok && len(entriesInput) > 0 {
		// Batch submission
		stream, err := l.buildStreamFromEntries(entriesInput, inputs)
		if err != nil {
			return nil, err
		}
		streams = []map[string]interface{}{stream}
	} else if line, ok := inputs["line"].(string); ok && line != "" {
		// Single entry submission
		stream, err := l.buildStreamFromSingleEntry(line, inputs)
		if err != nil {
			return nil, err
		}
		streams = []map[string]interface{}{stream}
	} else {
		return nil, fmt.Errorf("either 'line' or 'entries' must be provided")
	}

	// Build request payload
	payload := map[string]interface{}{
		"streams": streams,
	}

	// Marshal request body
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log payload: %w", err)
	}

	// Build request
	url := l.baseURL + "/loki/api/v1/push"
	req := &transport.Request{
		Method:  "POST",
		URL:     url,
		Headers: l.defaultHeaders(),
		Body:    bodyBytes,
	}

	// Execute request
	resp, err := l.transport.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse response (Loki typically returns empty or minimal response)
	var response map[string]interface{}
	if len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, &response); err != nil {
			// If response is not JSON, create a simple response
			response = map[string]interface{}{
				"status": "ok",
			}
		}
	} else {
		// Success with no body
		response = map[string]interface{}{
			"status": "ok",
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

// buildStreamFromSingleEntry builds a Loki stream from a single log entry.
func (l *LokiIntegration) buildStreamFromSingleEntry(line string, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Get labels (required)
	labels := make(map[string]string)
	if labelsInput, ok := inputs["labels"].(map[string]interface{}); ok {
		for k, v := range labelsInput {
			if strVal, ok := v.(string); ok {
				labels[k] = strVal
			} else {
				labels[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	if len(labels) == 0 {
		return nil, fmt.Errorf("labels are required")
	}

	// Get timestamp (optional - auto-populate if omitted)
	timestamp := getCurrentTimestampNano()
	if ts, ok := inputs["timestamp"]; ok {
		parsedTS, err := parseTimestamp(ts)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %w", err)
		}
		timestamp = parsedTS
	}

	// Build stream
	stream := map[string]interface{}{
		"stream": labels,
		"values": [][]string{
			{strconv.FormatInt(timestamp, 10), line},
		},
	}

	return stream, nil
}

// buildStreamFromEntries builds a Loki stream from multiple log entries.
func (l *LokiIntegration) buildStreamFromEntries(entriesInput []interface{}, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Get labels (required)
	labels := make(map[string]string)
	if labelsInput, ok := inputs["labels"].(map[string]interface{}); ok {
		for k, v := range labelsInput {
			if strVal, ok := v.(string); ok {
				labels[k] = strVal
			} else {
				labels[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	if len(labels) == 0 {
		return nil, fmt.Errorf("labels are required for batch entries")
	}

	// Build values array
	values := make([][]string, 0, len(entriesInput))
	for i, entryInput := range entriesInput {
		entryMap, ok := entryInput.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("entries[%d] must be an object", i)
		}

		// Get line (required)
		line, ok := entryMap["line"].(string)
		if !ok || line == "" {
			return nil, fmt.Errorf("entries[%d].line is required", i)
		}

		// Get timestamp (optional - auto-populate if omitted)
		timestamp := getCurrentTimestampNano()
		if ts, ok := entryMap["timestamp"]; ok {
			parsedTS, err := parseTimestamp(ts)
			if err != nil {
				return nil, fmt.Errorf("entries[%d].timestamp is invalid: %w", i, err)
			}
			timestamp = parsedTS
		}

		values = append(values, []string{strconv.FormatInt(timestamp, 10), line})
	}

	// Build stream
	stream := map[string]interface{}{
		"stream": labels,
		"values": values,
	}

	return stream, nil
}

// parseTimestamp parses a timestamp from either RFC3339Nano string or Unix nanoseconds.
func parseTimestamp(ts interface{}) (int64, error) {
	switch v := ts.(type) {
	case string:
		// Try parsing as RFC3339Nano
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return 0, fmt.Errorf("invalid timestamp format (expected RFC3339Nano): %w", err)
		}
		return t.UnixNano(), nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("timestamp must be RFC3339Nano string or Unix nanoseconds")
	}
}

// getCurrentTimestampNano returns the current Unix timestamp in nanoseconds.
func getCurrentTimestampNano() int64 {
	return time.Now().UnixNano()
}
