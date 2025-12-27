// Package httpclient provides a unified HTTP client factory with consistent
// timeout, retry, and observability behavior for the conductor platform.
//
// The package creates HTTP clients with sensible, secure defaults including:
//   - Automatic retry with exponential backoff and jitter
//   - Request logging with sanitized URLs (sensitive parameters redacted)
//   - User-Agent header injection
//   - Correlation ID propagation for distributed tracing
//   - TLS 1.2 minimum (TLS 1.3 preferred)
//   - Connection pooling for performance
//
// # Usage
//
// Create a client with default settings:
//
//	client, err := httpclient.New(httpclient.DefaultConfig())
//	if err != nil {
//	    return err
//	}
//	resp, err := client.Get("https://api.example.com/resource")
//
// Customize configuration:
//
//	cfg := httpclient.DefaultConfig()
//	cfg.UserAgent = "my-service/2.0"
//	cfg.Timeout = 60 * time.Second
//	cfg.RetryAttempts = 5
//	client, err := httpclient.New(cfg)
//
// # Retry Behavior
//
// The client automatically retries transient errors with exponential backoff:
//   - Retries HTTP 5xx server errors
//   - Retries HTTP 429 (rate limit) with Retry-After header support
//   - Retries HTTP 408 (request timeout)
//   - Retries network errors (connection refused, reset, temporary DNS failures)
//   - Does NOT retry 4xx client errors (except 408, 429)
//   - Only retries idempotent methods (GET, HEAD, OPTIONS) by default
//
// For non-idempotent methods (POST, PUT, PATCH, DELETE), enable explicit retry:
//
//	cfg := httpclient.DefaultConfig()
//	cfg.AllowNonIdempotentRetry = true  // Use with Idempotency-Key headers
//	client, err := httpclient.New(cfg)
//
// # Security
//
// The package includes security features:
//   - Sensitive query parameters (api_key, token, password, etc.) are redacted from logs
//   - Authorization headers are never logged
//   - TLS 1.2 minimum with certificate validation enabled
//   - Connection pooling limits prevent resource exhaustion
//
// # Observability
//
// All requests emit structured logs via log/slog:
//   - Debug level: successful requests (2xx status)
//   - Warn level: failed requests (4xx/5xx status, errors)
//   - Fields: method, url (sanitized), status, duration_ms, error
//   - Correlation IDs automatically propagated when present in request context
//
// # Integration
//
// This package is designed to be used throughout the conductor codebase:
//   - LLM provider HTTP clients
//   - Built-in HTTP tool (with additional SSRF protections)
//   - Connector HTTP transport
//   - Webhook clients
//   - MCP HTTP transport
package httpclient
