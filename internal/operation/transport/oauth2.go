package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// OAuth2TransportConfig configures the OAuth2 transport.
type OAuth2TransportConfig struct {
	// BaseURL is the API base URL (required)
	BaseURL string

	// Flow is the OAuth2 flow ("client_credentials" or "authorization_code", required)
	Flow string

	// ClientID is the OAuth2 client ID (required, must use ${ENV_VAR} syntax)
	ClientID string

	// ClientSecret is the OAuth2 client secret (required, must use ${ENV_VAR} syntax)
	ClientSecret string

	// TokenURL is the OAuth2 token endpoint (required)
	TokenURL string

	// Scopes are the OAuth2 scopes (optional)
	Scopes []string

	// RefreshToken is the refresh token for authorization_code flow (must use ${ENV_VAR} syntax)
	RefreshToken string

	// Timeout for requests (default: 30s)
	Timeout time.Duration

	// Retry configuration
	Retry *RetryConfig
}

// TransportType returns the transport type identifier.
func (c *OAuth2TransportConfig) TransportType() string {
	return "oauth2"
}

// Validate checks the configuration is valid.
func (c *OAuth2TransportConfig) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required for oauth2 transport")
	}
	if !strings.HasPrefix(c.BaseURL, "https://") && !strings.HasPrefix(c.BaseURL, "http://") {
		return fmt.Errorf("base_url must start with http:// or https://")
	}
	if c.Flow == "" {
		return fmt.Errorf("flow is required for oauth2 transport")
	}
	if c.Flow != "client_credentials" && c.Flow != "authorization_code" {
		return fmt.Errorf("flow must be client_credentials or authorization_code, got %q", c.Flow)
	}
	if c.ClientID == "" {
		return fmt.Errorf("client_id is required for oauth2 transport")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("client_secret is required for oauth2 transport")
	}
	if !hasEnvVarSyntax(c.ClientSecret) {
		return fmt.Errorf("client_secret must use ${VAR_NAME} syntax for security (NFR7)")
	}
	if c.TokenURL == "" {
		return fmt.Errorf("token_url is required for oauth2 transport")
	}
	if c.Flow == "authorization_code" {
		if c.RefreshToken == "" {
			return fmt.Errorf("refresh_token is required for authorization_code flow")
		}
		if !hasEnvVarSyntax(c.RefreshToken) {
			return fmt.Errorf("refresh_token must use ${VAR_NAME} syntax for security (NFR7)")
		}
	}
	if c.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}
	return nil
}

// OAuth2Transport implements Transport for OAuth2-protected APIs.
type OAuth2Transport struct {
	config      *OAuth2TransportConfig
	client      *http.Client
	tokenSource oauth2.TokenSource
	token       *oauth2.Token
	tokenMutex  sync.RWMutex
	refreshing  bool
	refreshCond *sync.Cond
	rateLimiter RateLimiter
}

// NewOAuth2Transport creates a new OAuth2 transport.
func NewOAuth2Transport(cfg *OAuth2TransportConfig) (*OAuth2Transport, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Set defaults
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	retry := cfg.Retry
	if retry == nil {
		retry = DefaultRetryConfig()
	}

	transport := &OAuth2Transport{
		config: cfg,
		client: &http.Client{
			Timeout: timeout,
		},
	}

	// Initialize condition variable for coordinating token refresh
	transport.refreshCond = sync.NewCond(&transport.tokenMutex)

	// Set up token source based on flow
	var tokenSource oauth2.TokenSource
	ctx := context.Background()

	switch cfg.Flow {
	case "client_credentials":
		ccConfig := &clientcredentials.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			TokenURL:     cfg.TokenURL,
			Scopes:       cfg.Scopes,
		}
		tokenSource = ccConfig.TokenSource(ctx)

	case "authorization_code":
		oauthConfig := &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint: oauth2.Endpoint{
				TokenURL: cfg.TokenURL,
			},
			Scopes: cfg.Scopes,
		}

		// Create token from refresh token
		token := &oauth2.Token{
			RefreshToken: cfg.RefreshToken,
		}
		tokenSource = oauthConfig.TokenSource(ctx, token)

	default:
		return nil, fmt.Errorf("unsupported OAuth2 flow: %s", cfg.Flow)
	}

	transport.tokenSource = tokenSource

	// Acquire initial token
	if err := transport.refreshToken(context.Background()); err != nil {
		return nil, &TransportError{
			Type:      ErrorTypeAuth,
			Message:   fmt.Sprintf("failed to acquire OAuth2 token: %v", err),
			Retryable: false,
			Cause:     err,
		}
	}

	return transport, nil
}

// refreshToken acquires a new access token.
func (t *OAuth2Transport) refreshToken(ctx context.Context) error {
	token, err := t.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	t.tokenMutex.Lock()
	t.token = token
	t.tokenMutex.Unlock()

	return nil
}

// needsRefresh checks if the token needs to be refreshed.
// Returns true if token is expired or will expire within 5 minutes.
func (t *OAuth2Transport) needsRefresh() bool {
	t.tokenMutex.RLock()
	defer t.tokenMutex.RUnlock()

	if t.token == nil {
		return true
	}

	// Refresh 5 minutes before expiry
	refreshThreshold := time.Now().Add(5 * time.Minute)
	return t.token.Expiry.Before(refreshThreshold)
}

// ensureToken ensures a valid token is available, refreshing if necessary.
func (t *OAuth2Transport) ensureToken(ctx context.Context) error {
	// Fast path: token is still valid
	if !t.needsRefresh() {
		return nil
	}

	// Slow path: need to refresh token
	t.tokenMutex.Lock()
	defer t.tokenMutex.Unlock()

	// Double-check after acquiring lock (another goroutine may have refreshed)
	refreshThreshold := time.Now().Add(5 * time.Minute)
	if t.token != nil && t.token.Expiry.After(refreshThreshold) {
		return nil
	}

	// If another goroutine is already refreshing, wait for it
	for t.refreshing {
		// Wait with timeout
		done := make(chan struct{})
		go func() {
			t.refreshCond.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Check if refresh was successful
			if t.token != nil && t.token.Expiry.After(time.Now().Add(5*time.Minute)) {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			return fmt.Errorf("timeout waiting for token refresh")
		}
	}

	// We're the refreshing goroutine
	t.refreshing = true
	t.tokenMutex.Unlock()

	// Perform refresh (without holding the lock)
	err := t.refreshToken(ctx)

	t.tokenMutex.Lock()
	t.refreshing = false
	t.refreshCond.Broadcast()

	return err
}

// Execute sends a request with OAuth2 authentication.
func (t *OAuth2Transport) Execute(ctx context.Context, req *Request) (*Response, error) {
	// Validate request
	if err := t.validateRequest(req); err != nil {
		return nil, &TransportError{
			Type:      ErrorTypeInvalidReq,
			Message:   fmt.Sprintf("invalid request: %s", err.Error()),
			Retryable: false,
			Cause:     err,
		}
	}

	// Apply rate limiting if configured
	if t.rateLimiter != nil {
		if err := t.rateLimiter.Wait(ctx); err != nil {
			return nil, &TransportError{
				Type:      ErrorTypeCancelled,
				Message:   "rate limiter cancelled",
				Retryable: false,
				Cause:     err,
			}
		}
	}

	// Ensure we have a valid token
	if err := t.ensureToken(ctx); err != nil {
		return nil, &TransportError{
			Type:      ErrorTypeAuth,
			Message:   fmt.Sprintf("failed to acquire OAuth2 token: %v", err),
			Retryable: false,
			Cause:     err,
		}
	}

	// Execute with retry logic
	return Execute(ctx, t.config.Retry, func(ctx context.Context) (*Response, error) {
		return t.executeOnce(ctx, req)
	})
}

// validateRequest checks if the request is valid.
func (t *OAuth2Transport) validateRequest(req *Request) error {
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

	// URL must be non-empty
	if req.URL == "" {
		return fmt.Errorf("URL is required")
	}

	return nil
}

// executeOnce performs a single request execution with OAuth2 authentication.
func (t *OAuth2Transport) executeOnce(ctx context.Context, req *Request) (*Response, error) {
	// Build URL
	requestURL := req.URL
	if !strings.HasPrefix(requestURL, "http://") && !strings.HasPrefix(requestURL, "https://") {
		requestURL = t.config.BaseURL + requestURL
	}

	// Create HTTP request
	var body io.Reader
	if req.Body != nil {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, requestURL, body)
	if err != nil {
		return nil, &TransportError{
			Type:      ErrorTypeInvalidReq,
			Message:   fmt.Sprintf("failed to create request: %v", err),
			Retryable: false,
			Cause:     err,
		}
	}

	// Apply headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Set Content-Type if body is present and not already set
	if req.Body != nil && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Add OAuth2 authorization header
	t.tokenMutex.RLock()
	if t.token != nil {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.token.AccessToken))
	}
	t.tokenMutex.RUnlock()

	// Execute request
	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, t.classifyHTTPError(err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &TransportError{
			Type:      ErrorTypeConnection,
			Message:   fmt.Sprintf("failed to read response body: %v", err),
			Retryable: true,
			Cause:     err,
		}
	}

	// Extract request ID
	requestID := resp.Header.Get("X-Request-ID")

	// Check for errors
	if resp.StatusCode >= 400 {
		return nil, t.parseOAuth2Error(resp.StatusCode, respBody, requestID)
	}

	// Build response
	response := &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
		Metadata: map[string]interface{}{
			MetadataRequestID: requestID,
		},
	}

	return response, nil
}

// Name returns the transport identifier.
func (t *OAuth2Transport) Name() string {
	return "oauth2"
}

// SetRateLimiter configures rate limiting for this transport.
func (t *OAuth2Transport) SetRateLimiter(limiter RateLimiter) {
	t.rateLimiter = limiter
}

// parseOAuth2Error parses OAuth2 error responses.
func (t *OAuth2Transport) parseOAuth2Error(statusCode int, body []byte, requestID string) error {
	// Try to parse as OAuth2 error response
	var oauth2Err struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &oauth2Err); err == nil && oauth2Err.Error != "" {
		return classifyOAuth2Error(statusCode, oauth2Err.Error, oauth2Err.ErrorDescription, requestID)
	}

	// Fallback to generic HTTP error
	errorType := ErrorTypeServer
	retryable := true
	if statusCode < 500 {
		errorType = ErrorTypeClient
		retryable = false
		if statusCode == 429 {
			errorType = ErrorTypeRateLimit
			retryable = true
		} else if statusCode == 401 || statusCode == 403 {
			errorType = ErrorTypeAuth
		}
	}

	return &TransportError{
		Type:       errorType,
		StatusCode: statusCode,
		Message:    fmt.Sprintf("OAuth2 request failed with status %d", statusCode),
		RequestID:  requestID,
		Retryable:  retryable,
		Metadata: map[string]interface{}{
			"response_body": string(body),
		},
	}
}

// classifyOAuth2Error categorizes OAuth2 errors by error code.
func classifyOAuth2Error(statusCode int, errorCode, description, requestID string) error {
	// Determine error type and retryability
	var errorType ErrorType
	var retryable bool

	switch errorCode {
	case "invalid_grant", "unauthorized_client", "access_denied":
		errorType = ErrorTypeAuth
		retryable = false
	case "temporarily_unavailable", "server_error":
		errorType = ErrorTypeServer
		retryable = true
	default:
		if statusCode >= 500 {
			errorType = ErrorTypeServer
			retryable = true
		} else if statusCode == 401 || statusCode == 403 {
			errorType = ErrorTypeAuth
			retryable = false
		} else {
			errorType = ErrorTypeClient
			retryable = false
		}
	}

	message := fmt.Sprintf("OAuth2 error %s", errorCode)
	if description != "" {
		message = fmt.Sprintf("%s: %s", message, description)
	}

	return &TransportError{
		Type:       errorType,
		StatusCode: statusCode,
		Message:    message,
		RequestID:  requestID,
		Retryable:  retryable,
		Metadata: map[string]interface{}{
			"oauth2_error": errorCode,
		},
	}
}

// classifyHTTPError classifies HTTP client errors into TransportError types.
func (t *OAuth2Transport) classifyHTTPError(err error) *TransportError {
	// Check for context cancellation
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
