package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Result represents the output of an HTTP operation.
type Result struct {
	Response interface{}
	Metadata map[string]interface{}
}

// HTTPConnector executes HTTP requests with security controls.
type HTTPConnector struct {
	config     *Config
	httpClient *http.Client
}

// New creates a new HTTP connector.
func New(config *Config) (*HTTPConnector, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Apply defaults
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxResponseSize == 0 {
		config.MaxResponseSize = 10 * 1024 * 1024 // 10MB
	}
	if config.MaxRedirects == 0 {
		config.MaxRedirects = 10
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= config.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", config.MaxRedirects)
			}
			// Validate redirect target if security config is present
			if config.SecurityConfig != nil {
				if err := config.SecurityConfig.ValidateURL(req.URL.String()); err != nil {
					return fmt.Errorf("redirect target blocked: %w", err)
				}
			}
			return nil
		},
	}

	// Configure secure dialer if security config is present
	if config.SecurityConfig != nil {
		transport := &http.Transport{
			DialContext: config.SecurityConfig.SecureDialContext(nil),
		}
		client.Transport = transport
	}

	return &HTTPConnector{
		config:     config,
		httpClient: client,
	}, nil
}

// Execute runs an HTTP operation.
func (c *HTTPConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*Result, error) {
	switch operation {
	case "get":
		return c.get(ctx, inputs)
	case "post":
		return c.post(ctx, inputs)
	case "put":
		return c.put(ctx, inputs)
	case "patch":
		return c.patch(ctx, inputs)
	case "delete":
		return c.delete(ctx, inputs)
	case "request":
		return c.request(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown http operation: %s", operation)
	}
}

// validateAndPrepareRequest performs security validation and creates the request.
func (c *HTTPConnector) validateAndPrepareRequest(ctx context.Context, method, url string, body io.Reader, inputs map[string]interface{}) (*http.Request, error) {
	// Validate URL scheme
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, &InvalidURLError{
			URL:    url,
			Reason: "only http/https schemes allowed",
		}
	}

	// Enforce HTTPS requirement
	if c.config.RequireHTTPS && strings.HasPrefix(url, "http://") {
		return nil, &SecurityBlockedError{
			URL:    url,
			Reason: "HTTPS required",
		}
	}

	// DNS validation via monitor
	if c.config.DNSMonitor != nil {
		// Extract hostname from URL
		hostname := extractHostname(url)
		if hostname != "" {
			if err := c.config.DNSMonitor.ValidateQuery(hostname); err != nil {
				// DNS validation failed - return security blocked error
				return nil, &SecurityBlockedError{
					URL:    url,
					Reason: fmt.Sprintf("DNS validation failed: %v", err),
				}
			}
		}
	}

	// Security config validation
	if c.config.SecurityConfig != nil {
		if err := c.config.SecurityConfig.ValidateURL(url); err != nil {
			// Security validation failed
			return nil, &SecurityBlockedError{
				URL:    url,
				Reason: err.Error(),
			}
		}

		if err := c.config.SecurityConfig.ValidateMethod(method); err != nil {
			return nil, &InvalidURLError{
				URL:    url,
				Reason: err.Error(),
			}
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, &InvalidURLError{
			URL:    url,
			Reason: err.Error(),
		}
	}

	// Add headers from inputs
	if headers, ok := inputs["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				req.Header.Set(key, strValue)
			}
		}

		// Validate headers
		if c.config.SecurityConfig != nil {
			if err := c.config.SecurityConfig.ValidateHeaders(req.Header); err != nil {
				return nil, &SecurityBlockedError{
					URL:    url,
					Reason: err.Error(),
				}
			}
		}
	}

	// Validate forbidden headers
	forbiddenHeaders := []string{"Host", "Connection", "Transfer-Encoding"}
	for _, header := range forbiddenHeaders {
		if req.Header.Get(header) != "" {
			return nil, &SecurityBlockedError{
				URL:    url,
				Reason: fmt.Sprintf("forbidden header: %s", header),
			}
		}
	}

	// Apply per-request timeout if specified
	if timeoutSec, ok := inputs["timeout"].(int); ok && timeoutSec > 0 {
		timeout := time.Duration(timeoutSec) * time.Second
		ctx, cancel := context.WithTimeout(ctx, timeout)
		_ = cancel // Will be called when request completes
		req = req.WithContext(ctx)
	} else if timeoutSec, ok := inputs["timeout"].(float64); ok && timeoutSec > 0 {
		timeout := time.Duration(timeoutSec) * time.Second
		ctx, cancel := context.WithTimeout(ctx, timeout)
		_ = cancel
		req = req.WithContext(ctx)
	}

	return req, nil
}

// executeRequest performs the HTTP request and returns a structured response.
func (c *HTTPConnector) executeRequest(req *http.Request, inputs map[string]interface{}) (*Result, error) {
	startTime := time.Now()

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if err == context.DeadlineExceeded || strings.Contains(err.Error(), "timeout") {
			return nil, &TimeoutError{
				URL:     req.URL.String(),
				Timeout: c.config.Timeout.String(),
			}
		}
		return nil, &NetworkError{
			URL:    req.URL.String(),
			Reason: err.Error(),
		}
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, c.config.MaxResponseSize+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, &NetworkError{
			URL:    req.URL.String(),
			Reason: fmt.Sprintf("failed to read response: %v", err),
		}
	}

	// Check if response exceeded size limit
	if int64(len(bodyBytes)) > c.config.MaxResponseSize {
		return nil, &NetworkError{
			URL:    req.URL.String(),
			Reason: fmt.Sprintf("response size exceeds %d bytes", c.config.MaxResponseSize),
		}
	}

	bodyString := string(bodyBytes)

	// Build response headers map
	headers := make(map[string][]string)
	for key, values := range resp.Header {
		headers[key] = values
	}

	// Build result
	result := &Result{
		Response: map[string]interface{}{
			"success":     resp.StatusCode >= 200 && resp.StatusCode < 300,
			"status_code": resp.StatusCode,
			"headers":     headers,
			"body":        bodyString,
		},
		Metadata: map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
			"size_bytes":  len(bodyBytes),
		},
	}

	// Parse JSON if requested
	if parseJSON, ok := inputs["parse_json"].(bool); ok && parseJSON {
		// Attempt JSON parsing
		var jsonData interface{}
		if err := parseJSONString(bodyString, &jsonData); err != nil {
			// Keep body as string but add parse error to metadata
			result.Metadata["parse_error"] = err.Error()
		} else {
			// Replace body with parsed JSON
			respMap := result.Response.(map[string]interface{})
			respMap["body"] = jsonData
		}
	}

	// Add error field if not successful
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respMap := result.Response.(map[string]interface{})
		respMap["error"] = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return result, nil
}

// extractHostname extracts hostname from a URL string.
func extractHostname(rawURL string) string {
	// Simple extraction - find between :// and next / or :
	if idx := strings.Index(rawURL, "://"); idx != -1 {
		hostStart := idx + 3
		hostEnd := len(rawURL)

		// Find end of hostname (before path or port)
		for i := hostStart; i < len(rawURL); i++ {
			if rawURL[i] == '/' || rawURL[i] == ':' {
				hostEnd = i
				break
			}
		}

		return rawURL[hostStart:hostEnd]
	}
	return ""
}

// parseJSONString is a variable to allow replacement in operations.go
var parseJSONString = func(jsonStr string, target *interface{}) error {
	// This is a placeholder - replaced in operations.go init()
	return fmt.Errorf("JSON parsing not yet implemented")
}
