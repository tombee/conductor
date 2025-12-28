package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/internal/connector/transport"
)

// Valid CloudWatch metric units
var validUnits = map[string]bool{
	"Seconds": true, "Microseconds": true, "Milliseconds": true,
	"Bytes": true, "Kilobytes": true, "Megabytes": true, "Gigabytes": true, "Terabytes": true,
	"Bits": true, "Kilobits": true, "Megabits": true, "Gigabits": true, "Terabits": true,
	"Percent": true, "Count": true,
	"Bytes/Second": true, "Kilobytes/Second": true, "Megabytes/Second": true,
	"Gigabytes/Second": true, "Terabytes/Second": true,
	"Bits/Second": true, "Kilobits/Second": true, "Megabits/Second": true,
	"Gigabits/Second": true, "Terabits/Second": true,
	"Count/Second": true, "None": true,
}

// putMetricData sends metrics to CloudWatch using PutMetricData API.
func (c *CloudWatchConnector) putMetricData(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Extract namespace (required)
	namespace, ok := inputs["namespace"].(string)
	if !ok || namespace == "" {
		return nil, fmt.Errorf("missing required parameter: namespace")
	}

	// Check if it's a batch submission
	if metricsArr, ok := inputs["metrics"].([]interface{}); ok {
		// Batch submission
		metricData, err := c.buildMetricDataArray(metricsArr)
		if err != nil {
			return nil, err
		}
		return c.sendPutMetricData(ctx, namespace, metricData)
	}

	// Single metric submission
	metricDatum, err := c.buildMetricDatum(inputs)
	if err != nil {
		return nil, err
	}

	return c.sendPutMetricData(ctx, namespace, []map[string]interface{}{metricDatum})
}

// buildMetricDatum constructs a single metric datum from inputs.
func (c *CloudWatchConnector) buildMetricDatum(inputs map[string]interface{}) (map[string]interface{}, error) {
	// Extract metric name (required)
	metricName, ok := inputs["name"].(string)
	if !ok || metricName == "" {
		return nil, fmt.Errorf("missing required parameter: name")
	}

	// Extract value (required)
	value, ok := inputs["value"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: value")
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
		return nil, fmt.Errorf("invalid value type: %T", value)
	}

	datum := map[string]interface{}{
		"MetricName": metricName,
		"Value":      floatValue,
	}

	// Add optional unit
	if unit, ok := inputs["unit"].(string); ok && unit != "" {
		if !validUnits[unit] {
			return nil, fmt.Errorf("invalid unit: %s", unit)
		}
		datum["Unit"] = unit
	} else {
		datum["Unit"] = "None"
	}

	// Add optional timestamp
	if timestamp, ok := inputs["timestamp"].(string); ok && timestamp != "" {
		datum["Timestamp"] = timestamp
	}

	// Add optional storage resolution
	if resolution, ok := inputs["storage_resolution"].(int); ok {
		if resolution != 1 && resolution != 60 {
			return nil, fmt.Errorf("storage_resolution must be 1 or 60, got: %d", resolution)
		}
		datum["StorageResolution"] = resolution
	}

	// Add optional dimensions
	if dimensions, ok := inputs["dimensions"].(map[string]interface{}); ok && len(dimensions) > 0 {
		if len(dimensions) > 30 {
			return nil, fmt.Errorf("too many dimensions: %d (max 30)", len(dimensions))
		}

		dimArray := make([]map[string]string, 0, len(dimensions))
		for name, value := range dimensions {
			dimArray = append(dimArray, map[string]string{
				"Name":  name,
				"Value": fmt.Sprint(value),
			})
		}
		datum["Dimensions"] = dimArray
	}

	return datum, nil
}

// buildMetricDataArray constructs an array of metric data from inputs.
func (c *CloudWatchConnector) buildMetricDataArray(metrics []interface{}) ([]map[string]interface{}, error) {
	if len(metrics) == 0 {
		return nil, fmt.Errorf("metrics array is empty")
	}

	if len(metrics) > 1000 {
		return nil, fmt.Errorf("too many metrics: %d (max 1000)", len(metrics))
	}

	metricData := make([]map[string]interface{}, 0, len(metrics))

	for i, m := range metrics {
		metricMap, ok := m.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("metric at index %d is not an object", i)
		}

		datum, err := c.buildMetricDatum(metricMap)
		if err != nil {
			return nil, fmt.Errorf("metric at index %d: %w", i, err)
		}

		metricData = append(metricData, datum)
	}

	return metricData, nil
}

// sendPutMetricData sends a PutMetricData request to CloudWatch.
func (c *CloudWatchConnector) sendPutMetricData(ctx context.Context, namespace string, metricData []map[string]interface{}) (*connector.Result, error) {
	// Build request body
	body := map[string]interface{}{
		"Namespace":  namespace,
		"MetricData": metricData,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Send request to CloudWatch
	req := &transport.Request{
		Method: "POST",
		URL:    "/",
		Headers: map[string]string{
			"Content-Type": "application/x-amz-json-1.1",
			"X-Amz-Target": "GraniteServiceVersion20100801.PutMetricData",
		},
		Body: bodyBytes,
	}

	resp, err := c.transport.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	// CloudWatch PutMetricData returns empty body on success
	response := map[string]interface{}{
		"status": "ok",
	}

	return &connector.Result{
		Response:    response,
		RawResponse: resp.Body,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Headers,
		Metadata:    resp.Metadata,
	}, nil
}
