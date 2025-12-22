package datadog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/transport"
)

// sendMetric sends metrics to Datadog Metrics API.
func (d *DatadogIntegration) sendMetric(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Build series array
	var series []map[string]interface{}

	// Check if this is a batch submission (series array provided)
	if seriesInput, ok := inputs["series"].([]interface{}); ok && len(seriesInput) > 0 {
		// Batch submission
		series = make([]map[string]interface{}, 0, len(seriesInput))
		for i, item := range seriesInput {
			metricMap, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("series[%d] must be an object", i)
			}

			metric, err := d.buildMetricObject(metricMap, i)
			if err != nil {
				return nil, err
			}
			series = append(series, metric)
		}
	} else {
		// Single metric submission
		metric, err := d.buildMetricObject(inputs, -1)
		if err != nil {
			return nil, err
		}
		series = []map[string]interface{}{metric}
	}

	// Build request payload
	payload := map[string]interface{}{
		"series": series,
	}

	// Marshal request body
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metric payload: %w", err)
	}

	// Build request
	url := d.getAPIBaseURL() + "/api/v2/series"
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

	// Add metrics_accepted field as per spec
	response["metrics_accepted"] = len(series)

	return &operation.Result{
		Response:    response,
		RawResponse: resp.Body,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Headers,
		Metadata:    resp.Metadata,
	}, nil
}

// buildMetricObject builds a single metric object from inputs.
// index is used for error messages (-1 for single metric mode).
func (d *DatadogIntegration) buildMetricObject(inputs map[string]interface{}, index int) (map[string]interface{}, error) {
	prefix := ""
	if index >= 0 {
		prefix = fmt.Sprintf("series[%d].", index)
	}

	// Validate required fields
	metricName, ok := inputs["metric"].(string)
	if !ok || metricName == "" {
		return nil, fmt.Errorf("missing required parameter: %smetric", prefix)
	}

	value, ok := inputs["value"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: %svalue", prefix)
	}

	// Convert value to float64
	var floatValue float64
	switch v := value.(type) {
	case float64:
		floatValue = v
	case float32:
		floatValue = float64(v)
	case int:
		floatValue = float64(v)
	case int64:
		floatValue = float64(v)
	default:
		return nil, fmt.Errorf("%svalue must be a number", prefix)
	}

	// Build metric object
	metric := map[string]interface{}{
		"metric": metricName,
	}

	// Add timestamp (auto-populate if omitted)
	if timestamp, ok := inputs["timestamp"]; ok {
		switch ts := timestamp.(type) {
		case int64:
			metric["points"] = []map[string]interface{}{
				{"timestamp": ts, "value": floatValue},
			}
		case int:
			metric["points"] = []map[string]interface{}{
				{"timestamp": int64(ts), "value": floatValue},
			}
		case float64:
			metric["points"] = []map[string]interface{}{
				{"timestamp": int64(ts), "value": floatValue},
			}
		}
	} else {
		metric["points"] = []map[string]interface{}{
			{"timestamp": getCurrentTimestamp(), "value": floatValue},
		}
	}

	// Add optional fields
	if metricType, ok := inputs["type"].(string); ok && metricType != "" {
		// Validate type enum
		validTypes := map[string]bool{
			"gauge": true, "count": true, "rate": true,
		}
		if !validTypes[metricType] {
			return nil, fmt.Errorf("invalid %stype: %s (must be one of: gauge, count, rate)", prefix, metricType)
		}
		metric["type"] = metricType
	}

	if tags, ok := inputs["tags"].([]interface{}); ok && len(tags) > 0 {
		tagStrings := make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				tagStrings = append(tagStrings, tagStr)
			}
		}
		if len(tagStrings) > 0 {
			metric["tags"] = tagStrings
		}
	}

	if unit, ok := inputs["unit"].(string); ok && unit != "" {
		metric["unit"] = unit
	}

	if interval, ok := inputs["interval"]; ok {
		switch i := interval.(type) {
		case int:
			metric["interval"] = i
		case int64:
			metric["interval"] = i
		case float64:
			metric["interval"] = int64(i)
		}
	}

	return metric, nil
}
