package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tombee/conductor/pkg/tools"
)

// HTTPTool provides HTTP request capabilities.
type HTTPTool struct {
	// timeout sets the maximum request time
	timeout time.Duration

	// allowedHosts restricts which hosts can be accessed
	// If empty, all hosts are allowed
	allowedHosts []string

	// client is the HTTP client
	client *http.Client
}

// NewHTTPTool creates a new HTTP tool with default settings.
func NewHTTPTool() *HTTPTool {
	return &HTTPTool{
		timeout:      30 * time.Second,
		allowedHosts: []string{},
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// WithTimeout sets the HTTP request timeout.
func (t *HTTPTool) WithTimeout(timeout time.Duration) *HTTPTool {
	t.timeout = timeout
	t.client.Timeout = timeout
	return t
}

// WithAllowedHosts restricts which hosts can be accessed.
func (t *HTTPTool) WithAllowedHosts(hosts []string) *HTTPTool {
	t.allowedHosts = hosts
	return t
}

// Name returns the tool identifier.
func (t *HTTPTool) Name() string {
	return "http"
}

// Description returns a human-readable description.
func (t *HTTPTool) Description() string {
	return "Make HTTP requests to external APIs"
}

// Schema returns the tool's input/output schema.
func (t *HTTPTool) Schema() *tools.Schema {
	return &tools.Schema{
		Inputs: &tools.ParameterSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"method": {
					Type:        "string",
					Description: "HTTP method (GET, POST, PUT, DELETE, etc.)",
					Default:     "GET",
				},
				"url": {
					Type:        "string",
					Description: "The URL to request",
					Format:      "uri",
				},
				"headers": {
					Type:        "object",
					Description: "HTTP headers to include (optional)",
				},
				"body": {
					Type:        "string",
					Description: "Request body (optional, for POST/PUT)",
				},
			},
			Required: []string{"url"},
		},
		Outputs: &tools.ParameterSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"success": {
					Type:        "boolean",
					Description: "Whether the request succeeded (2xx status)",
				},
				"status_code": {
					Type:        "number",
					Description: "HTTP status code",
				},
				"headers": {
					Type:        "object",
					Description: "Response headers",
				},
				"body": {
					Type:        "string",
					Description: "Response body",
				},
				"error": {
					Type:        "string",
					Description: "Error message if request failed",
				},
			},
		},
	}
}

// Execute performs an HTTP request.
func (t *HTTPTool) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Extract URL
	url, ok := inputs["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url must be a string")
	}

	// Validate URL
	if err := t.validateURL(url); err != nil {
		return nil, err
	}

	// Extract method (default: GET)
	method := "GET"
	if methodRaw, ok := inputs["method"]; ok {
		method, ok = methodRaw.(string)
		if !ok {
			return nil, fmt.Errorf("method must be a string")
		}
		method = strings.ToUpper(method)
	}

	// Extract body (optional)
	var body io.Reader
	if bodyRaw, ok := inputs["body"]; ok {
		bodyStr, ok := bodyRaw.(string)
		if !ok {
			return nil, fmt.Errorf("body must be a string")
		}
		body = bytes.NewBufferString(bodyStr)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	// Extract and set headers (optional)
	if headersRaw, ok := inputs["headers"]; ok {
		headers, ok := headersRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("headers must be an object")
		}
		for key, value := range headers {
			valueStr, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("header values must be strings")
			}
			req.Header.Set(key, valueStr)
		}
	}

	// Set default Content-Type for POST/PUT if not specified
	if (method == "POST" || method == "PUT") && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute request
	resp, err := t.client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{
			"success":     false,
			"status_code": resp.StatusCode,
			"error":       fmt.Sprintf("failed to read response body: %v", err),
		}, nil
	}

	// Convert response headers to map
	headers := make(map[string]interface{})
	for key, values := range resp.Header {
		if len(values) == 1 {
			headers[key] = values[0]
		} else {
			headers[key] = values
		}
	}

	// Determine success (2xx status codes)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	return map[string]interface{}{
		"success":     success,
		"status_code": resp.StatusCode,
		"headers":     headers,
		"body":        string(respBody),
	}, nil
}

// validateURL checks if a URL is allowed.
func (t *HTTPTool) validateURL(url string) error {
	// Basic URL validation
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid URL scheme: must be http or https")
	}

	// Check if host is in allowed list
	if len(t.allowedHosts) > 0 {
		allowed := false
		for _, allowedHost := range t.allowedHosts {
			if strings.Contains(url, allowedHost) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("host not allowed: %s", url)
		}
	}

	return nil
}

// ParseJSON is a helper to parse JSON response bodies.
func ParseJSON(body string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return result, nil
}
