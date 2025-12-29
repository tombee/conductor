package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// get performs an HTTP GET request.
func (c *HTTPAction) get(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	url, ok := inputs["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required")
	}

	req, err := c.validateAndPrepareRequest(ctx, "GET", url, nil, inputs)
	if err != nil {
		return nil, err
	}

	return c.executeRequest(req, inputs)
}

// post performs an HTTP POST request with optional body.
func (c *HTTPAction) post(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	url, ok := inputs["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required")
	}

	// Get body from inputs
	var body io.Reader
	if bodyStr, ok := inputs["body"].(string); ok && bodyStr != "" {
		body = strings.NewReader(bodyStr)
	}

	req, err := c.validateAndPrepareRequest(ctx, "POST", url, body, inputs)
	if err != nil {
		return nil, err
	}

	// Set Content-Type if body is provided and not already set
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.executeRequest(req, inputs)
}

// put performs an HTTP PUT request with optional body.
func (c *HTTPAction) put(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	url, ok := inputs["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required")
	}

	// Get body from inputs
	var body io.Reader
	if bodyStr, ok := inputs["body"].(string); ok && bodyStr != "" {
		body = strings.NewReader(bodyStr)
	}

	req, err := c.validateAndPrepareRequest(ctx, "PUT", url, body, inputs)
	if err != nil {
		return nil, err
	}

	// Set Content-Type if body is provided and not already set
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.executeRequest(req, inputs)
}

// patch performs an HTTP PATCH request with optional body.
func (c *HTTPAction) patch(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	url, ok := inputs["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required")
	}

	// Get body from inputs
	var body io.Reader
	if bodyStr, ok := inputs["body"].(string); ok && bodyStr != "" {
		body = strings.NewReader(bodyStr)
	}

	req, err := c.validateAndPrepareRequest(ctx, "PATCH", url, body, inputs)
	if err != nil {
		return nil, err
	}

	// Set Content-Type if body is provided and not already set
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.executeRequest(req, inputs)
}

// delete performs an HTTP DELETE request.
func (c *HTTPAction) delete(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	url, ok := inputs["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required")
	}

	req, err := c.validateAndPrepareRequest(ctx, "DELETE", url, nil, inputs)
	if err != nil {
		return nil, err
	}

	return c.executeRequest(req, inputs)
}

// request performs an HTTP request with a configurable method.
func (c *HTTPAction) request(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	url, ok := inputs["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required")
	}

	method, ok := inputs["method"].(string)
	if !ok || method == "" {
		return nil, fmt.Errorf("method is required for request operation")
	}

	// Normalize method to uppercase
	method = strings.ToUpper(method)

	// Validate method
	allowedMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	methodAllowed := false
	for _, allowed := range allowedMethods {
		if method == allowed {
			methodAllowed = true
			break
		}
	}
	if !methodAllowed {
		return nil, fmt.Errorf("invalid HTTP method: %s (allowed: %v)", method, allowedMethods)
	}

	// Get body from inputs (if applicable)
	var body io.Reader
	if bodyStr, ok := inputs["body"].(string); ok && bodyStr != "" {
		body = strings.NewReader(bodyStr)
	}

	req, err := c.validateAndPrepareRequest(ctx, method, url, body, inputs)
	if err != nil {
		return nil, err
	}

	// Set Content-Type if body is provided and not already set
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.executeRequest(req, inputs)
}

// parseJSONStringImpl is the actual JSON parsing implementation.
func parseJSONStringImpl(jsonStr string, target *interface{}) error {
	return json.Unmarshal([]byte(jsonStr), target)
}

func init() {
	// Replace the placeholder with the actual implementation
	parseJSONString = parseJSONStringImpl
}
