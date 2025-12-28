package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/internal/connector/transport"
)

// putLogEvents sends log events to CloudWatch Logs using PutLogEvents API.
// Handles sequence token management and auto-creates log streams if configured.
func (c *CloudWatchConnector) putLogEvents(ctx context.Context, inputs map[string]interface{}) (*connector.Result, error) {
	// Extract required fields
	logGroup, ok := inputs["log_group"].(string)
	if !ok || logGroup == "" {
		return nil, fmt.Errorf("missing required parameter: log_group")
	}

	logStream, ok := inputs["log_stream"].(string)
	if !ok || logStream == "" {
		return nil, fmt.Errorf("missing required parameter: log_stream")
	}

	// Extract message (can be string or object for JSON)
	message := inputs["message"]
	if message == nil {
		return nil, fmt.Errorf("missing required parameter: message")
	}

	// Convert message to string
	var messageStr string
	switch v := message.(type) {
	case string:
		messageStr = v
	case map[string]interface{}:
		// Serialize to JSON
		bytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize message to JSON: %w", err)
		}
		messageStr = string(bytes)
	default:
		messageStr = fmt.Sprint(v)
	}

	// Extract timestamp (CloudWatch expects milliseconds since epoch)
	timestamp, ok := inputs["timestamp"].(int64)
	if !ok {
		// Try to convert from int
		if ts, ok := inputs["timestamp"].(int); ok {
			timestamp = int64(ts)
		} else {
			return nil, fmt.Errorf("missing or invalid timestamp parameter")
		}
	}

	// Build log event
	logEvent := map[string]interface{}{
		"message":   messageStr,
		"timestamp": timestamp,
	}

	// Try to send log events with retries for sequence token issues
	return c.putLogEventsWithRetry(ctx, logGroup, logStream, []map[string]interface{}{logEvent})
}

// putLogEventsWithRetry sends log events with automatic retry for sequence token errors.
func (c *CloudWatchConnector) putLogEventsWithRetry(ctx context.Context, logGroup, logStream string, logEvents []map[string]interface{}) (*connector.Result, error) {
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Get current sequence token
		sequenceToken := c.getSequenceToken(logGroup, logStream)

		// Try to send the log events
		result, err := c.sendPutLogEvents(ctx, logGroup, logStream, sequenceToken, logEvents)

		if err == nil {
			// Success - update sequence token if present
			if nextToken, ok := result.Response.(map[string]interface{})["nextSequenceToken"].(string); ok && nextToken != "" {
				c.setSequenceToken(logGroup, logStream, nextToken)
			}
			return result, nil
		}

		// Check if it's a sequence token error
		if transportErr, ok := err.(*transport.TransportError); ok {
			if c.isInvalidSequenceTokenError(transportErr) {
				// Extract correct sequence token from error message
				if token := c.extractSequenceTokenFromError(transportErr); token != "" {
					c.setSequenceToken(logGroup, logStream, token)
					continue // Retry with correct token
				}
				// Clear cached token and retry
				c.clearSequenceToken(logGroup, logStream)
				continue
			}

			// Check if stream doesn't exist
			if c.isResourceNotFoundError(transportErr) && c.autoCreateStream {
				// Try to create the log stream
				if err := c.createLogStream(ctx, logGroup, logStream); err != nil {
					return nil, fmt.Errorf("failed to create log stream: %w", err)
				}
				// Clear sequence token and retry
				c.clearSequenceToken(logGroup, logStream)
				continue
			}
		}

		// Other error - return immediately
		return nil, err
	}

	return nil, fmt.Errorf("failed to put log events after %d retries", maxRetries)
}

// sendPutLogEvents sends a single PutLogEvents request.
func (c *CloudWatchConnector) sendPutLogEvents(ctx context.Context, logGroup, logStream, sequenceToken string, logEvents []map[string]interface{}) (*connector.Result, error) {
	// Build request body
	body := map[string]interface{}{
		"logGroupName":  logGroup,
		"logStreamName": logStream,
		"logEvents":     logEvents,
	}

	if sequenceToken != "" {
		body["sequenceToken"] = sequenceToken
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Send request to CloudWatch Logs
	req := &transport.Request{
		Method: "POST",
		URL:    "/",
		Headers: map[string]string{
			"Content-Type":         "application/x-amz-json-1.1",
			"X-Amz-Target":         "Logs_20140328.PutLogEvents",
			"X-Amz-Content-Sha256": "", // Will be computed by transport
		},
		Body: bodyBytes,
	}

	resp, err := c.transport.Execute(ctx, req)
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
		response = make(map[string]interface{})
	}

	return &connector.Result{
		Response:    response,
		RawResponse: resp.Body,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Headers,
		Metadata:    resp.Metadata,
	}, nil
}

// createLogStream creates a new log stream.
func (c *CloudWatchConnector) createLogStream(ctx context.Context, logGroup, logStream string) error {
	body := map[string]interface{}{
		"logGroupName":  logGroup,
		"logStreamName": logStream,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req := &transport.Request{
		Method: "POST",
		URL:    "/",
		Headers: map[string]string{
			"Content-Type": "application/x-amz-json-1.1",
			"X-Amz-Target": "Logs_20140328.CreateLogStream",
		},
		Body: bodyBytes,
	}

	_, err = c.transport.Execute(ctx, req)
	return err
}

// isInvalidSequenceTokenError checks if the error is an invalid sequence token error.
func (c *CloudWatchConnector) isInvalidSequenceTokenError(err *transport.TransportError) bool {
	if err.Metadata == nil {
		return false
	}

	awsCode, ok := err.Metadata["aws_error_code"].(string)
	return ok && (awsCode == "InvalidSequenceTokenException" || awsCode == "DataAlreadyAcceptedException")
}

// isResourceNotFoundError checks if the error is a resource not found error.
func (c *CloudWatchConnector) isResourceNotFoundError(err *transport.TransportError) bool {
	if err.Metadata == nil {
		return false
	}

	awsCode, ok := err.Metadata["aws_error_code"].(string)
	return ok && awsCode == "ResourceNotFoundException"
}

// extractSequenceTokenFromError extracts the expected sequence token from error message.
// CloudWatch returns the expected token in the error message like:
// "The next expected sequenceToken is: 495123..."
func (c *CloudWatchConnector) extractSequenceTokenFromError(err *transport.TransportError) string {
	msg := err.Message
	if idx := strings.Index(msg, "sequenceToken is: "); idx != -1 {
		token := msg[idx+len("sequenceToken is: "):]
		// Find end of token (usually at newline or end of string)
		if endIdx := strings.IndexAny(token, "\n "); endIdx != -1 {
			token = token[:endIdx]
		}
		return strings.TrimSpace(token)
	}
	return ""
}
