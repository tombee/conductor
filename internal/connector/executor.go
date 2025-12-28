package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

// sensitiveHeaders are headers that should not be overridden by user input.
var sensitiveHeaders = map[string]bool{
	"content-length":    true,
	"content-encoding":  true,
	"transfer-encoding": true,
	"host":              true,
}

// sanitizeHeaderValue checks for header injection attempts.
// Returns error if the value contains CR, LF, or null bytes.
func sanitizeHeaderValue(name, value string) error {
	// Check for header injection characters
	for i, c := range value {
		if c == '\r' || c == '\n' || c == '\x00' {
			return fmt.Errorf("header %q contains invalid character at position %d", name, i)
		}
	}
	return nil
}

// isSensitiveHeader returns true if the header should not be overridden.
func isSensitiveHeader(name string) bool {
	return sensitiveHeaders[strings.ToLower(name)]
}

// httpExecutor handles HTTP request execution for connector operations.
type httpExecutor struct {
	connector     *httpConnector
	operation     *workflow.OperationDefinition
	operationName string
	rateLimiter   *RateLimiter
	client        *http.Client
}

// newHTTPExecutor creates a new HTTP executor for an operation.
func newHTTPExecutor(connector *httpConnector, operationName string) (*httpExecutor, error) {
	op, exists := connector.def.Operations[operationName]
	if !exists {
		return nil, &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("operation %q not found in connector %q", operationName, connector.name),
		}
	}

	// Create HTTP client with timeout
	timeout := time.Duration(op.Timeout) * time.Second
	if timeout == 0 {
		timeout = time.Duration(connector.config.DefaultTimeout) * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
	}

	// Create rate limiter if configured
	var rateLimiter *RateLimiter
	if connector.def.RateLimit != nil {
		stateFile := ""
		if connector.config.StateFilePath != "" {
			stateFile = fmt.Sprintf("%s/%s.json", connector.config.StateFilePath, connector.name)
		}
		rateLimiter = NewRateLimiter(connector.def.RateLimit, stateFile)
	}

	return &httpExecutor{
		connector:     connector,
		operation:     &op,
		operationName: operationName,
		rateLimiter:   rateLimiter,
		client:        client,
	}, nil
}

// Execute runs the HTTP operation with the given inputs.
func (e *httpExecutor) Execute(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	startTime := time.Now()
	var statusCode int
	var waitDuration time.Duration

	// Inject default fields for observability connectors
	injector := NewDefaultFieldInjector()
	injector.InjectDefaults(inputs, e.connector.name)

	// Wait for rate limiter
	waitStart := time.Now()
	if err := e.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	waitDuration = time.Since(waitStart)

	// Record rate limit wait if it was significant (>1ms)
	if waitDuration > time.Millisecond && e.connector.metricsCollector != nil {
		e.connector.metricsCollector.RecordRateLimitWait(e.connector.name, waitDuration)
	}

	// Defer metrics recording
	defer func() {
		if e.connector.metricsCollector != nil {
			duration := time.Since(startTime)
			e.connector.metricsCollector.RecordRequest(e.connector.name, e.operationName, statusCode, duration)
		}
	}()

	// Build the request URL
	requestURL, err := e.buildURL(inputs)
	if err != nil {
		return nil, err
	}

	// Validate URL for SSRF protection
	if err := ValidateURL(requestURL, e.connector.config.AllowedHosts, e.connector.config.BlockedHosts); err != nil {
		return nil, err
	}

	// Build request body
	var bodyReader io.Reader
	if e.requiresBody() {
		bodyReader, err = e.buildBody(inputs)
		if err != nil {
			return nil, err
		}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, e.operation.Method, requestURL, bodyReader)
	if err != nil {
		return nil, NewConnectionError(err)
	}

	// Apply headers
	if err := e.applyHeaders(req, inputs); err != nil {
		return nil, &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("header processing failed: %v", err),
		}
	}

	// Apply authentication
	if err := ApplyAuth(req, e.connector.def.Auth); err != nil {
		return nil, &Error{
			Type:    ErrorTypeAuth,
			Message: fmt.Sprintf("authentication failed: %v", err),
		}
	}

	// Execute request
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, NewConnectionError(err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{
			Type:    ErrorTypeConnection,
			Message: fmt.Sprintf("failed to read response: %v", err),
		}
	}

	// Capture status code for metrics
	statusCode = resp.StatusCode

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		requestID := resp.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = resp.Header.Get("X-GitHub-Request-Id")
		}
		return nil, ErrorFromHTTPStatus(resp.StatusCode, resp.Status, string(bodyBytes), requestID)
	}

	// Parse response body as JSON
	var responseData interface{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
			// Not JSON, return as string
			responseData = string(bodyBytes)
		}
	}

	// Apply response transform if specified
	transformedData := responseData
	if e.operation.ResponseTransform != "" {
		transformedData, err = TransformResponse(e.operation.ResponseTransform, responseData)
		if err != nil {
			return nil, err
		}
	}

	return &Result{
		Response:    transformedData,
		RawResponse: responseData,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Header,
		Metadata: map[string]interface{}{
			"request_id": resp.Header.Get("X-Request-Id"),
			"method":     e.operation.Method,
			"url":        requestURL,
		},
	}, nil
}

// buildURL constructs the request URL from base URL, path template, and inputs.
func (e *httpExecutor) buildURL(inputs map[string]interface{}) (string, error) {
	// Start with base URL
	baseURL := e.connector.def.BaseURL

	// Substitute path parameters
	path := e.operation.Path
	for key, value := range inputs {
		placeholder := fmt.Sprintf("{%s}", key)
		if !strings.Contains(path, placeholder) {
			continue
		}

		// Convert value to string
		strValue := fmt.Sprintf("%v", value)

		// Validate for path traversal
		if err := ValidatePathParameter(key, strValue); err != nil {
			return "", err
		}

		// URL encode the value
		encodedValue := url.PathEscape(strValue)

		// Replace placeholder
		path = strings.ReplaceAll(path, placeholder, encodedValue)
	}

	// Join base URL and path, ensuring exactly one / between them
	baseURL = strings.TrimSuffix(baseURL, "/")
	path = "/" + strings.TrimPrefix(path, "/")

	return baseURL + path, nil
}

// buildBody creates the request body from inputs.
func (e *httpExecutor) buildBody(inputs map[string]interface{}) (io.Reader, error) {
	// Filter out path parameters from body
	bodyData := make(map[string]interface{})
	pathParams := e.getPathParameters()

	for key, value := range inputs {
		if !pathParams[key] {
			bodyData[key] = value
		}
	}

	// Marshal to JSON
	data, err := json.Marshal(bodyData)
	if err != nil {
		return nil, &Error{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("failed to marshal request body: %v", err),
		}
	}

	return bytes.NewReader(data), nil
}

// getPathParameters extracts parameter names from the path template.
func (e *httpExecutor) getPathParameters() map[string]bool {
	params := make(map[string]bool)
	path := e.operation.Path

	for {
		start := strings.Index(path, "{")
		if start == -1 {
			break
		}

		end := strings.Index(path[start:], "}")
		if end == -1 {
			break
		}
		end += start

		paramName := path[start+1 : end]
		params[paramName] = true

		path = path[end+1:]
	}

	return params
}

// applyHeaders applies connector and operation headers to the request.
func (e *httpExecutor) applyHeaders(req *http.Request, inputs map[string]interface{}) error {
	// Apply connector-level headers
	for key, value := range e.connector.def.Headers {
		// Check if this is a sensitive header
		if isSensitiveHeader(key) {
			return fmt.Errorf("cannot override protected header %q", key)
		}

		expandedValue, err := expandEnvVar(value)
		if err != nil {
			return fmt.Errorf("connector header %q expansion failed: %w", key, err)
		}

		// Validate header value for injection
		if err := sanitizeHeaderValue(key, expandedValue); err != nil {
			return fmt.Errorf("connector header validation failed: %w", err)
		}

		req.Header.Set(key, expandedValue)
	}

	// Apply operation-level headers (override connector headers)
	for key, value := range e.operation.Headers {
		// Check if this is a sensitive header
		if isSensitiveHeader(key) {
			return fmt.Errorf("cannot override protected header %q", key)
		}

		expandedValue, err := expandEnvVar(value)
		if err != nil {
			return fmt.Errorf("operation header %q expansion failed: %w", key, err)
		}

		// Validate header value for injection
		if err := sanitizeHeaderValue(key, expandedValue); err != nil {
			return fmt.Errorf("operation header validation failed: %w", err)
		}

		req.Header.Set(key, expandedValue)
	}

	// Set Content-Type for JSON if body is present
	if e.requiresBody() && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return nil
}

// requiresBody returns true if the HTTP method typically includes a body.
func (e *httpExecutor) requiresBody() bool {
	method := strings.ToUpper(e.operation.Method)
	return method == "POST" || method == "PUT" || method == "PATCH"
}
