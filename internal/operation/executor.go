package operation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/operation/transport"
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

// parseRetryAfter parses the Retry-After header value.
// Returns the number of seconds to wait, or 0 if invalid.
// Supports both delay-seconds (integer) and HTTP-date formats.
func parseRetryAfter(value string) int {
	// Try parsing as integer (delay-seconds)
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return seconds
	}

	// Try parsing as HTTP-date (RFC1123, RFC850, or ANSI C)
	for _, layout := range []string{
		time.RFC1123,
		time.RFC850,
		time.ANSIC,
	} {
		if t, err := time.Parse(layout, value); err == nil {
			delay := int(time.Until(t).Seconds())
			if delay > 0 {
				return delay
			}
			return 0
		}
	}

	// Invalid format, return 0
	return 0
}

// httpExecutor handles HTTP request execution for connector operations.
type httpExecutor struct {
	connector     *httpConnector
	operation     *workflow.OperationDefinition
	operationName string
	rateLimiter   *RateLimiter
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
	}, nil
}

// buildTransportRequest creates a transport.Request from operation inputs.
func (e *httpExecutor) buildTransportRequest(ctx context.Context, inputs map[string]interface{}) (*transport.Request, error) {
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
	var bodyBytes []byte
	if e.requiresBody() {
		bodyReader, err := e.buildBody(inputs)
		if err != nil {
			return nil, err
		}
		bodyBytes, err = io.ReadAll(bodyReader)
		if err != nil {
			return nil, &Error{
				Type:    ErrorTypeValidation,
				Message: fmt.Sprintf("failed to read request body: %v", err),
			}
		}
	}

	// Build headers map
	headers := make(map[string]string)

	// Apply connector-level headers
	for key, value := range e.connector.def.Headers {
		if isSensitiveHeader(key) {
			return nil, &Error{
				Type:    ErrorTypeValidation,
				Message: fmt.Sprintf("cannot override protected header %q", key),
			}
		}
		expandedValue, err := expandEnvVar(value)
		if err != nil {
			return nil, &Error{
				Type:    ErrorTypeValidation,
				Message: fmt.Sprintf("connector header %q expansion failed: %v", key, err),
			}
		}
		if err := sanitizeHeaderValue(key, expandedValue); err != nil {
			return nil, &Error{
				Type:    ErrorTypeValidation,
				Message: fmt.Sprintf("connector header validation failed: %v", err),
			}
		}
		headers[key] = expandedValue
	}

	// Apply operation-level headers (override connector headers)
	for key, value := range e.operation.Headers {
		if isSensitiveHeader(key) {
			return nil, &Error{
				Type:    ErrorTypeValidation,
				Message: fmt.Sprintf("cannot override protected header %q", key),
			}
		}
		expandedValue, err := expandEnvVar(value)
		if err != nil {
			return nil, &Error{
				Type:    ErrorTypeValidation,
				Message: fmt.Sprintf("operation header %q expansion failed: %v", key, err),
			}
		}
		if err := sanitizeHeaderValue(key, expandedValue); err != nil {
			return nil, &Error{
				Type:    ErrorTypeValidation,
				Message: fmt.Sprintf("operation header validation failed: %v", err),
			}
		}
		headers[key] = expandedValue
	}

	// Apply authentication headers if auth is not handled by transport.
	// This maintains backward compatibility with connectors that use plain auth values.
	// When auth uses ${ENV_VAR} syntax, it's handled by the transport layer.
	if e.connector.def.Auth != nil && !usesEnvVarSyntax(e.connector.def.Auth) {
		// Create a temporary HTTP request to apply auth headers
		tempReq, err := http.NewRequest(e.operation.Method, requestURL, nil)
		if err != nil {
			return nil, &Error{
				Type:    ErrorTypeValidation,
				Message: fmt.Sprintf("failed to create auth request: %v", err),
			}
		}

		if err := ApplyAuth(tempReq, e.connector.def.Auth); err != nil {
			return nil, &Error{
				Type:    ErrorTypeAuth,
				Message: fmt.Sprintf("authentication failed: %v", err),
			}
		}

		// Copy auth headers to our headers map
		for key, values := range tempReq.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}

	// Set Content-Type for JSON if body is present and not already set
	if e.requiresBody() && headers["Content-Type"] == "" {
		headers["Content-Type"] = "application/json"
	}

	return &transport.Request{
		Method:  e.operation.Method,
		URL:     requestURL,
		Headers: headers,
		Body:    bodyBytes,
	}, nil
}

// convertTransportResponse converts a transport.Response to a connector Result.
func (e *httpExecutor) convertTransportResponse(resp *transport.Response) (*Result, error) {
	// Parse response body as JSON
	var responseData interface{}
	if len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, &responseData); err != nil {
			// Not JSON, return as string
			responseData = string(resp.Body)
		}
	}

	// Apply response transform if specified
	transformedData := responseData
	if e.operation.ResponseTransform != "" {
		var err error
		transformedData, err = TransformResponse(e.operation.ResponseTransform, responseData)
		if err != nil {
			return nil, err
		}
	}

	// Extract request ID from response metadata or headers
	requestID := ""
	if resp.Metadata != nil {
		if id, ok := resp.Metadata[transport.MetadataRequestID].(string); ok {
			requestID = id
		}
	}
	if requestID == "" && resp.Headers != nil {
		if ids := resp.Headers["X-Request-Id"]; len(ids) > 0 {
			requestID = ids[0]
		} else if ids := resp.Headers["X-Github-Request-Id"]; len(ids) > 0 {
			requestID = ids[0]
		}
	}

	return &Result{
		Response:    transformedData,
		RawResponse: responseData,
		StatusCode:  resp.StatusCode,
		Headers:     resp.Headers,
		Metadata: map[string]interface{}{
			"request_id": requestID,
			"method":     e.operation.Method,
		},
	}, nil
}

// convertTransportError converts a transport error to a connector Error.
func (e *httpExecutor) convertTransportError(err error) error {
	// Check if it's already a transport error
	if transportErr, ok := err.(*transport.TransportError); ok {
		// Convert transport error type to connector error type
		var errType ErrorType
		switch transportErr.Type {
		case transport.ErrorTypeAuth:
			errType = ErrorTypeAuth
		case transport.ErrorTypeRateLimit:
			errType = ErrorTypeRateLimit
		case transport.ErrorTypeServer:
			errType = ErrorTypeServer
		case transport.ErrorTypeTimeout:
			errType = ErrorTypeTimeout
		case transport.ErrorTypeConnection:
			errType = ErrorTypeConnection
		case transport.ErrorTypeClient:
			// Map client errors based on status code
			if transportErr.StatusCode == 404 {
				errType = ErrorTypeNotFound
			} else {
				errType = ErrorTypeValidation
			}
		case transport.ErrorTypeInvalidReq:
			errType = ErrorTypeValidation
		case transport.ErrorTypeCancelled:
			errType = ErrorTypeTimeout
		default:
			errType = ErrorTypeConnection
		}

		return &Error{
			Type:       errType,
			Message:    transportErr.Message,
			StatusCode: transportErr.StatusCode,
			RequestID:  transportErr.RequestID,
			Cause:      transportErr.Cause,
		}
	}

	// Not a transport error, wrap as connection error
	return NewConnectionError(err)
}

// Execute runs the HTTP operation with the given inputs.
func (e *httpExecutor) Execute(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	startTime := time.Now()
	var statusCode int
	var waitDuration time.Duration

	// Inject default fields for observability connectors
	injector := NewDefaultFieldInjector()
	injector.InjectDefaults(inputs, e.connector.name)

	// Validate inputs for observability connectors
	validator := NewValidator()
	if err := validator.ValidateConnectorInputs(e.connector.name, e.operationName, inputs); err != nil {
		return nil, err
	}

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

	// Build transport request from inputs
	transportReq, err := e.buildTransportRequest(ctx, inputs)
	if err != nil {
		return nil, err
	}

	// Execute request through transport
	transportResp, err := e.connector.transport.Execute(ctx, transportReq)
	if err != nil {
		return nil, e.convertTransportError(err)
	}

	// Capture status code for metrics
	statusCode = transportResp.StatusCode

	// Check for HTTP errors
	if transportResp.StatusCode >= 400 {
		requestID := ""
		if transportResp.Headers != nil {
			if ids := transportResp.Headers["X-Request-Id"]; len(ids) > 0 {
				requestID = ids[0]
			} else if ids := transportResp.Headers["X-Github-Request-Id"]; len(ids) > 0 {
				requestID = ids[0]
			}
		}
		err := ErrorFromHTTPStatus(transportResp.StatusCode, http.StatusText(transportResp.StatusCode), string(transportResp.Body), requestID)

		// Extract Retry-After header for rate limit errors
		if err.Type == ErrorTypeRateLimit {
			if retryAfterValues := transportResp.Headers["Retry-After"]; len(retryAfterValues) > 0 {
				err.RetryAfter = parseRetryAfter(retryAfterValues[0])
			}
		}

		return nil, err
	}

	// Convert transport response to connector result
	result, err := e.convertTransportResponse(transportResp)
	if err != nil {
		return nil, err
	}

	// Add URL to metadata
	result.Metadata["url"] = transportReq.URL

	return result, nil
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
