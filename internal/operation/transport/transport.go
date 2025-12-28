// Package transport provides protocol-level abstractions for connector execution.
//
// The transport layer separates protocol concerns (HTTP, AWS SigV4, OAuth2) from
// connector-level concerns (operation definition, input validation, response transformation).
// All transports implement the Transport interface, providing unified authentication,
// request signing, error parsing, retry logic, and rate limiting.
package transport

import (
	"context"
)

// Transport executes requests with protocol-specific handling.
// Each transport implementation (HTTP, AWS SigV4, OAuth2) handles authentication,
// request signing, and error parsing according to its protocol requirements.
type Transport interface {
	// Execute sends a request and returns a response.
	// The context controls cancellation and deadlines.
	// Returns TransportError on failure.
	Execute(ctx context.Context, req *Request) (*Response, error)

	// Name returns the transport identifier (e.g., "http", "aws_sigv4", "oauth2").
	Name() string

	// SetRateLimiter configures rate limiting for this transport.
	// Rate limiting occurs before request execution, respecting configured limits.
	SetRateLimiter(limiter RateLimiter)
}

// Request represents a transport-agnostic request.
// Transports validate requests before execution and return InvalidRequest errors
// for invalid method, URL, or other protocol violations.
type Request struct {
	// Method is the HTTP method (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)
	// Required, must be non-empty
	Method string

	// URL is the full request URL
	// Required, must be valid per RFC 3986
	URL string

	// Headers are request headers (case-insensitive)
	// Optional, may be nil or empty map
	Headers map[string]string

	// Body is the request body
	// Optional, may be nil or empty slice
	Body []byte

	// Metadata contains transport-specific data
	// Used to pass transport-specific configuration (e.g., AWS service name)
	Metadata map[string]interface{}
}

// Response represents a transport-agnostic response.
type Response struct {
	// StatusCode is the HTTP status code
	StatusCode int

	// Headers contains response headers
	Headers map[string][]string

	// Body is the response body
	Body []byte

	// Metadata contains transport-specific data (e.g., AWS RequestID)
	Metadata map[string]interface{}
}

// Standard metadata keys used across transports
const (
	// MetadataRequestID is the service request ID
	MetadataRequestID = "request_id"

	// MetadataAWSRequestID is the AWS x-amzn-RequestId header value
	MetadataAWSRequestID = "aws_request_id"

	// MetadataRetryCount is the number of retries performed for this request
	MetadataRetryCount = "retry_count"
)

// RateLimiter provides rate limiting for transport requests.
// Implementations should block until a request is allowed.
type RateLimiter interface {
	// Wait blocks until a request is allowed under the rate limit.
	// Returns an error if the context is cancelled before the request can proceed.
	Wait(ctx context.Context) error
}
