package transport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AWSTransportConfig configures the AWS SigV4 transport.
type AWSTransportConfig struct {
	// BaseURL is the AWS service endpoint (required)
	BaseURL string

	// Service is the AWS service name (e.g., "s3", "dynamodb", required)
	Service string

	// Region is the AWS region (e.g., "us-east-1", required)
	Region string

	// Timeout for requests (default: 30s)
	Timeout time.Duration

	// Retry configuration
	Retry *RetryConfig
}

// TransportType returns the transport type identifier.
func (c *AWSTransportConfig) TransportType() string {
	return "aws_sigv4"
}

// Validate checks the configuration is valid.
func (c *AWSTransportConfig) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required for aws_sigv4 transport")
	}
	if !strings.HasPrefix(c.BaseURL, "https://") && !strings.HasPrefix(c.BaseURL, "http://") {
		return fmt.Errorf("base_url must start with http:// or https://")
	}
	if c.Service == "" {
		return fmt.Errorf("service is required for aws_sigv4 transport")
	}
	if c.Region == "" {
		return fmt.Errorf("region is required for aws_sigv4 transport")
	}
	if c.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}
	return nil
}

// AWSTransport implements Transport for AWS services with SigV4 signing.
type AWSTransport struct {
	config      *AWSTransportConfig
	client      *http.Client
	awsConfig   aws.Config
	signer      *v4.Signer
	credentials aws.Credentials
	credExpiry  time.Time
	credMutex   sync.RWMutex
	rateLimiter RateLimiter
}

// NewAWSTransport creates a new AWS SigV4 transport.
func NewAWSTransport(cfg *AWSTransportConfig) (*AWSTransport, error) {
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

	// Load AWS config with credential chain
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, &TransportError{
			Type:      ErrorTypeAuth,
			Message:   fmt.Sprintf("failed to load AWS configuration: %v", err),
			Retryable: false,
			Cause:     err,
		}
	}

	transport := &AWSTransport{
		config: cfg,
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		awsConfig: awsCfg,
		signer:    v4.NewSigner(),
	}

	// Validate credentials by getting caller identity
	if err := transport.validateCredentials(ctx); err != nil {
		return nil, err
	}

	return transport, nil
}

// validateCredentials calls STS GetCallerIdentity to ensure credentials are valid.
func (t *AWSTransport) validateCredentials(ctx context.Context) error {
	// Get credentials
	if err := t.refreshCredentials(ctx); err != nil {
		return err
	}

	// Create STS client
	stsClient := sts.NewFromConfig(t.awsConfig)

	// Call GetCallerIdentity with 5-second timeout
	validationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := stsClient.GetCallerIdentity(validationCtx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return &TransportError{
			Type:      ErrorTypeAuth,
			Message:   fmt.Sprintf("AWS credential validation failed: %v", sanitizeAWSError(err.Error())),
			Retryable: false,
			Cause:     err,
		}
	}

	return nil
}

// refreshCredentials retrieves and caches AWS credentials.
func (t *AWSTransport) refreshCredentials(ctx context.Context) error {
	t.credMutex.Lock()
	defer t.credMutex.Unlock()

	// Check if cached credentials are still valid
	if !t.credExpiry.IsZero() && time.Now().Before(t.credExpiry) {
		return nil
	}

	// Retrieve credentials from provider chain
	creds, err := t.awsConfig.Credentials.Retrieve(ctx)
	if err != nil {
		return &TransportError{
			Type:      ErrorTypeAuth,
			Message:   fmt.Sprintf("unable to resolve AWS credentials: %v", sanitizeAWSError(err.Error())),
			Retryable: false,
			Cause:     err,
		}
	}

	// Cache credentials with max 1 hour TTL
	t.credentials = creds
	expiry := creds.Expires
	if expiry.IsZero() || expiry.Sub(time.Now()) > time.Hour {
		expiry = time.Now().Add(time.Hour)
	}
	t.credExpiry = expiry

	return nil
}

// Execute sends a request with AWS SigV4 signing.
func (t *AWSTransport) Execute(ctx context.Context, req *Request) (*Response, error) {
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

	// Ensure credentials are fresh
	if err := t.refreshCredentials(ctx); err != nil {
		return nil, err
	}

	// Execute with retry logic
	return Execute(ctx, t.config.Retry, func(ctx context.Context) (*Response, error) {
		return t.executeOnce(ctx, req)
	})
}

// validateRequest checks if the request is valid.
func (t *AWSTransport) validateRequest(req *Request) error {
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

// executeOnce performs a single request execution with SigV4 signing.
func (t *AWSTransport) executeOnce(ctx context.Context, req *Request) (*Response, error) {
	// Build URL
	url := req.URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = t.config.BaseURL + url
	}

	// Create HTTP request
	var body io.Reader
	if req.Body != nil {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, body)
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

	// Calculate payload hash
	payloadHash := calculatePayloadHash(req.Body)
	httpReq.Header.Set("X-Amz-Content-Sha256", payloadHash)

	// Sign request
	t.credMutex.RLock()
	creds := aws.Credentials{
		AccessKeyID:     t.credentials.AccessKeyID,
		SecretAccessKey: t.credentials.SecretAccessKey,
		SessionToken:    t.credentials.SessionToken,
	}
	t.credMutex.RUnlock()

	err = t.signer.SignHTTP(ctx, creds, httpReq, payloadHash, t.config.Service, t.config.Region, time.Now())
	if err != nil {
		return nil, &TransportError{
			Type:      ErrorTypeInvalidReq,
			Message:   fmt.Sprintf("failed to sign request: %v", err),
			Retryable: false,
			Cause:     err,
		}
	}

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

	// Extract AWS request ID
	requestID := resp.Header.Get("x-amzn-RequestId")
	if requestID == "" {
		requestID = resp.Header.Get("x-amz-request-id")
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		return nil, parseAWSError(resp.StatusCode, respBody, requestID)
	}

	// Build response
	response := &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
		Metadata: map[string]interface{}{
			MetadataAWSRequestID: requestID,
		},
	}

	return response, nil
}

// Name returns the transport identifier.
func (t *AWSTransport) Name() string {
	return "aws_sigv4"
}

// SetRateLimiter configures rate limiting for this transport.
func (t *AWSTransport) SetRateLimiter(limiter RateLimiter) {
	t.rateLimiter = limiter
}

// calculatePayloadHash computes the SHA256 hash of the request body.
func calculatePayloadHash(body []byte) string {
	if body == nil {
		body = []byte{}
	}
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

// parseAWSError parses AWS error responses (XML or JSON).
func parseAWSError(statusCode int, body []byte, requestID string) error {
	// Try to parse as XML first (common for S3, etc.)
	var xmlErr struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	if err := xml.Unmarshal(body, &xmlErr); err == nil && xmlErr.Code != "" {
		return classifyAWSError(statusCode, xmlErr.Code, xmlErr.Message, requestID)
	}

	// Try to parse as JSON (common for newer AWS APIs)
	var jsonErr struct {
		Code    string `json:"__type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &jsonErr); err == nil && jsonErr.Code != "" {
		return classifyAWSError(statusCode, jsonErr.Code, jsonErr.Message, requestID)
	}

	// Fallback to generic error
	errorType := ErrorTypeServer
	retryable := true
	if statusCode < 500 {
		errorType = ErrorTypeClient
		retryable = false
		if statusCode == 429 {
			errorType = ErrorTypeRateLimit
			retryable = true
		}
	}

	return &TransportError{
		Type:       errorType,
		StatusCode: statusCode,
		Message:    fmt.Sprintf("AWS request failed with status %d", statusCode),
		RequestID:  requestID,
		Retryable:  retryable,
		Metadata: map[string]interface{}{
			"response_body": string(body),
		},
	}
}

// classifyAWSError categorizes AWS errors by code and status.
func classifyAWSError(statusCode int, code, message, requestID string) error {
	// Sanitize message
	message = sanitizeAWSError(message)

	// Determine error type and retryability
	var errorType ErrorType
	var retryable bool

	switch code {
	case "SignatureDoesNotMatch", "InvalidSignatureException", "InvalidAccessKeyId":
		errorType = ErrorTypeAuth
		retryable = false
	case "RequestLimitExceeded", "Throttling", "ThrottlingException", "TooManyRequestsException":
		errorType = ErrorTypeRateLimit
		retryable = true
	case "RequestTimeout", "RequestTimeoutException":
		errorType = ErrorTypeTimeout
		retryable = true
	default:
		if statusCode >= 500 {
			errorType = ErrorTypeServer
			retryable = true
		} else if statusCode == 429 {
			errorType = ErrorTypeRateLimit
			retryable = true
		} else if statusCode == 401 || statusCode == 403 {
			errorType = ErrorTypeAuth
			retryable = false
		} else {
			errorType = ErrorTypeClient
			retryable = false
		}
	}

	return &TransportError{
		Type:       errorType,
		StatusCode: statusCode,
		Message:    fmt.Sprintf("AWS error %s: %s", code, message),
		RequestID:  requestID,
		Retryable:  retryable,
		Metadata: map[string]interface{}{
			"aws_error_code": code,
		},
	}
}

// sanitizeAWSError removes credentials and sensitive data from error messages.
func sanitizeAWSError(msg string) string {
	// Find and redact AWS access keys (AKIA followed by 16 alphanumeric characters)
	// Replace with AKIA**** to indicate redaction
	searchPos := 0
	for {
		akiaPos := strings.Index(msg[searchPos:], "AKIA")
		if akiaPos == -1 {
			break
		}
		// Adjust position to absolute index
		akiaPos += searchPos

		// Find the end of the access key (next 16 chars after AKIA)
		endPos := akiaPos + 20 // 4 (AKIA) + 16
		if endPos > len(msg) {
			endPos = len(msg)
		}

		// Redact the access key
		msg = msg[:akiaPos] + "AKIA****" + msg[endPos:]

		// Move search position past the replacement to avoid infinite loop
		searchPos = akiaPos + len("AKIA****")
	}
	// ARNs and bucket names are acceptable for debugging
	return msg
}

// classifyHTTPError classifies HTTP client errors into TransportError types.
func (t *AWSTransport) classifyHTTPError(err error) *TransportError {
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
