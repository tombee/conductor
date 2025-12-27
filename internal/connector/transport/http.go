package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPTransport implements the Transport interface for HTTP/HTTPS requests.
// Supports bearer, basic, and API key authentication with configurable timeouts,
// TLS settings, and default headers.
type HTTPTransport struct {
	config       *HTTPTransportConfig
	client       *http.Client
	rateLimiter  RateLimiter
}

// HTTPTransportConfig configures the HTTP transport.
type HTTPTransportConfig struct {
	// BaseURL is the base URL for requests (required)
	BaseURL string

	// Timeout is the request timeout (default: 30s)
	Timeout time.Duration

	// Headers are default headers applied to all requests
	Headers map[string]string

	// Auth configures authentication
	Auth *AuthConfig

	// TLSInsecure disables TLS certificate validation (default: false)
	// WARNING: Only use for development/testing
	TLSInsecure bool

	// RetryConfig configures retry behavior (optional, uses defaults if nil)
	RetryConfig *RetryConfig
}

// AuthConfig configures HTTP authentication.
type AuthConfig struct {
	// Type is the authentication type ("bearer", "basic", "api_key")
	Type string

	// Token is the bearer token (for type: bearer)
	// Must use ${ENV_VAR} syntax for security
	Token string

	// Username for basic auth (type: basic)
	Username string

	// Password for basic auth (type: basic)
	// Must use ${ENV_VAR} syntax for security
	Password string

	// HeaderName is the header name for API key auth (type: api_key)
	// Example: "X-API-Key"
	HeaderName string

	// HeaderValue is the API key value (type: api_key)
	// Must use ${ENV_VAR} syntax for security
	HeaderValue string
}

// TransportType returns "http".
func (c *HTTPTransportConfig) TransportType() string {
	return "http"
}

// Validate checks if the configuration is valid.
func (c *HTTPTransportConfig) Validate() error {
	// BaseURL is required
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}

	// Validate BaseURL is a valid URL
	parsedURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}

	// BaseURL must have scheme and host
	if parsedURL.Scheme == "" {
		return fmt.Errorf("base_url must include scheme (http:// or https://)")
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("base_url must include host")
	}

	// Validate scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("base_url scheme must be http or https, got %q", parsedURL.Scheme)
	}

	// Validate timeout if specified
	if c.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %v", c.Timeout)
	}

	// Validate auth if specified
	if c.Auth != nil {
		if err := c.Auth.Validate(); err != nil {
			return fmt.Errorf("invalid auth configuration: %w", err)
		}
	}

	// Validate retry config if specified
	if c.RetryConfig != nil {
		if err := c.RetryConfig.Validate(); err != nil {
			return fmt.Errorf("invalid retry configuration: %w", err)
		}
	}

	return nil
}

// hasEnvVarSyntax checks if a string uses ${VAR_NAME} syntax for environment variable substitution.
func hasEnvVarSyntax(s string) bool {
	return strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}")
}

// Validate checks if the auth configuration is valid.
func (a *AuthConfig) Validate() error {
	switch a.Type {
	case "bearer":
		if a.Token == "" {
			return fmt.Errorf("token is required for bearer auth")
		}
		if !hasEnvVarSyntax(a.Token) {
			return fmt.Errorf("token must use ${VAR_NAME} syntax for security (NFR7)")
		}

	case "basic":
		if a.Username == "" {
			return fmt.Errorf("username is required for basic auth")
		}
		if a.Password == "" {
			return fmt.Errorf("password is required for basic auth")
		}
		if !hasEnvVarSyntax(a.Password) {
			return fmt.Errorf("password must use ${VAR_NAME} syntax for security (NFR7)")
		}

	case "api_key":
		if a.HeaderName == "" {
			return fmt.Errorf("header_name is required for api_key auth")
		}
		if a.HeaderValue == "" {
			return fmt.Errorf("header_value is required for api_key auth")
		}
		if !hasEnvVarSyntax(a.HeaderValue) {
			return fmt.Errorf("header_value must use ${VAR_NAME} syntax for security (NFR7)")
		}

	default:
		return fmt.Errorf("invalid auth type: %q (must be bearer, basic, or api_key)", a.Type)
	}

	return nil
}

// NewHTTPTransport creates a new HTTP transport with the given configuration.
func NewHTTPTransport(config *HTTPTransportConfig) (*HTTPTransport, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Set default timeout
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create HTTP client with custom transport for TLS settings
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			// Connection pool settings
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,

			// Timeouts
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: timeout,
			ExpectContinueTimeout: 1 * time.Second,

			// TLS configuration
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.TLSInsecure,
			},
		},
	}

	return &HTTPTransport{
		config: config,
		client: client,
	}, nil
}

// Name returns "http".
func (t *HTTPTransport) Name() string {
	return "http"
}

// SetRateLimiter configures rate limiting for this transport.
func (t *HTTPTransport) SetRateLimiter(limiter RateLimiter) {
	t.rateLimiter = limiter
}

// Execute sends an HTTP request and returns the response.
// Implements retry logic with exponential backoff.
func (t *HTTPTransport) Execute(ctx context.Context, req *Request) (*Response, error) {
	// Validate request
	if err := t.validateRequest(req); err != nil {
		return nil, &TransportError{
			Type:      ErrorTypeInvalidReq,
			Message:   fmt.Sprintf("invalid request: %s", err.Error()),
			Retryable: false,
			Cause:     err,
		}
	}

	// Get retry config (use defaults if not specified)
	retryConfig := t.config.RetryConfig
	if retryConfig == nil {
		retryConfig = DefaultRetryConfig()
	}

	// Execute with retry logic
	return Execute(ctx, retryConfig, func(ctx context.Context) (*Response, error) {
		return t.executeOnce(ctx, req)
	})
}

// executeOnce executes a single HTTP request without retry logic.
func (t *HTTPTransport) executeOnce(ctx context.Context, req *Request) (*Response, error) {
	// Apply rate limiting if configured
	if t.rateLimiter != nil {
		if err := t.rateLimiter.Wait(ctx); err != nil {
			return nil, &TransportError{
				Type:      ErrorTypeCancelled,
				Message:   "rate limit wait cancelled",
				Retryable: false,
				Cause:     err,
			}
		}
	}

	// Build HTTP request
	httpReq, err := t.buildHTTPRequest(ctx, req)
	if err != nil {
		return nil, &TransportError{
			Type:      ErrorTypeInvalidReq,
			Message:   fmt.Sprintf("failed to build HTTP request: %s", err.Error()),
			Retryable: false,
			Cause:     err,
		}
	}

	// Execute HTTP request
	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, t.classifyHTTPError(err)
	}
	defer httpResp.Body.Close()

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &TransportError{
			Type:      ErrorTypeConnection,
			Message:   fmt.Sprintf("failed to read response body: %s", err.Error()),
			Retryable: true,
			Cause:     err,
		}
	}

	// Build transport response
	resp := &Response{
		StatusCode: httpResp.StatusCode,
		Headers:    httpResp.Header,
		Body:       body,
		Metadata:   make(map[string]interface{}),
	}

	// Extract request ID if present
	if requestID := httpResp.Header.Get("X-Request-ID"); requestID != "" {
		resp.Metadata[MetadataRequestID] = requestID
	}

	// Check if response indicates an error
	if httpResp.StatusCode >= 400 {
		// Extract Retry-After header for rate limit responses
		if retryAfter := httpResp.Header.Get("Retry-After"); retryAfter != "" {
			resp.Metadata["retry_after"] = retryAfter
		}
		return nil, t.classifyHTTPStatusError(httpResp.StatusCode, body, resp.Metadata)
	}

	return resp, nil
}

// validateRequest checks if the request is valid.
func (t *HTTPTransport) validateRequest(req *Request) error {
	// Method must be non-empty and valid
	if req.Method == "" {
		return fmt.Errorf("method is required")
	}

	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}
	if !validMethods[req.Method] {
		return fmt.Errorf("invalid HTTP method: %q", req.Method)
	}

	// URL must be non-empty and valid
	if req.URL == "" {
		return fmt.Errorf("URL is required")
	}

	_, err := url.Parse(req.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	return nil
}

// buildHTTPRequest constructs an http.Request from a transport Request.
func (t *HTTPTransport) buildHTTPRequest(ctx context.Context, req *Request) (*http.Request, error) {
	// Create HTTP request
	var bodyReader io.Reader
	if req.Body != nil {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		return nil, err
	}

	// Apply default headers from config
	for key, value := range t.config.Headers {
		httpReq.Header.Set(key, value)
	}

	// Apply request headers (override defaults)
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Apply authentication
	if t.config.Auth != nil {
		if err := t.applyAuth(httpReq); err != nil {
			return nil, err
		}
	}

	// Set Content-Type if body is present and not already set
	if req.Body != nil && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	return httpReq, nil
}

// applyAuth applies authentication to the HTTP request.
func (t *HTTPTransport) applyAuth(req *http.Request) error {
	auth := t.config.Auth

	switch auth.Type {
	case "bearer":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", auth.Token))

	case "basic":
		req.SetBasicAuth(auth.Username, auth.Password)

	case "api_key":
		req.Header.Set(auth.HeaderName, auth.HeaderValue)

	default:
		return fmt.Errorf("unsupported auth type: %q", auth.Type)
	}

	return nil
}

// classifyHTTPError classifies HTTP client errors into TransportError types.
func (t *HTTPTransport) classifyHTTPError(err error) *TransportError {
	// Check for context cancellation in the error itself
	if strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "context deadline exceeded") {
		return &TransportError{
			Type:      ErrorTypeCancelled,
			Message:   "request cancelled",
			Retryable: false,
			Cause:     err,
		}
	}

	// Check for timeout errors
	if isTimeoutError(err) {
		return &TransportError{
			Type:      ErrorTypeTimeout,
			Message:   "request timeout",
			Retryable: true,
			Cause:     err,
		}
	}

	// Check for connection errors
	if isConnectionError(err) {
		return &TransportError{
			Type:      ErrorTypeConnection,
			Message:   "connection error",
			Retryable: true,
			Cause:     err,
		}
	}

	// Default to connection error (retryable)
	return &TransportError{
		Type:      ErrorTypeConnection,
		Message:   fmt.Sprintf("HTTP error: %s", err.Error()),
		Retryable: true,
		Cause:     err,
	}
}

// classifyHTTPStatusError classifies HTTP status code errors into TransportError types.
func (t *HTTPTransport) classifyHTTPStatusError(statusCode int, body []byte, metadata map[string]interface{}) *TransportError {
	// Determine error type based on status code
	var errorType ErrorType
	var retryable bool

	switch {
	case statusCode == 401 || statusCode == 403:
		errorType = ErrorTypeAuth
		retryable = false
	case statusCode == 429:
		errorType = ErrorTypeRateLimit
		retryable = true
		// Extract Retry-After header for metadata
		// (This would be extracted from response headers in actual implementation)
	case statusCode >= 500:
		errorType = ErrorTypeServer
		retryable = true
	case statusCode == 408:
		errorType = ErrorTypeTimeout
		retryable = true
	default:
		// Other 4xx errors
		errorType = ErrorTypeClient
		retryable = false
	}

	// Build error message (sanitize body to avoid leaking sensitive data)
	message := fmt.Sprintf("HTTP %d", statusCode)
	if len(body) > 0 && len(body) < 500 {
		// Include small error responses in message
		message = fmt.Sprintf("HTTP %d: %s", statusCode, strings.TrimSpace(string(body)))
	}

	return &TransportError{
		Type:       errorType,
		StatusCode: statusCode,
		Message:    message,
		Retryable:  retryable,
		Metadata:   metadata,
	}
}

// isTimeoutError checks if an error is a timeout error.
func isTimeoutError(err error) bool {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return false
}

// isConnectionError checks if an error is a connection error.
func isConnectionError(err error) bool {
	// Check for common connection error types
	if _, ok := err.(*net.OpError); ok {
		return true
	}
	if _, ok := err.(*url.Error); ok {
		return true
	}

	// Check error message for connection-related keywords
	errMsg := strings.ToLower(err.Error())
	connectionKeywords := []string{
		"connection refused",
		"connection reset",
		"no such host",
		"network unreachable",
		"eof",
	}

	for _, keyword := range connectionKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}
