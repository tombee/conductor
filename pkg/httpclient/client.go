// Package httpclient provides a unified HTTP client factory with consistent
// timeout, retry, and observability behavior across the conductor codebase.
//
// The client factory composes transport layers to provide:
//   - Automatic retries with exponential backoff and jitter
//   - Request logging with sanitized URLs (sensitive params redacted)
//   - User-Agent header injection
//   - Correlation ID propagation for distributed tracing
//   - TLS 1.2+ with secure defaults
//   - Connection pooling for performance
//
// Example usage:
//
//	cfg := httpclient.DefaultConfig()
//	cfg.UserAgent = "my-service/1.0"
//	client, err := httpclient.New(cfg)
//	if err != nil {
//	    return err
//	}
//
//	resp, err := client.Get("https://api.example.com/resource")
package httpclient

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// New creates a new HTTP client with the given configuration.
// The client includes:
//   - Retry logic with exponential backoff (configurable)
//   - Request logging with sanitized URLs
//   - User-Agent header injection
//   - Correlation ID propagation
//   - TLS 1.2 minimum, TLS 1.3 preferred
//   - Connection pooling with sensible defaults
//
// Returns an error if the configuration is invalid.
func New(cfg Config) (*http.Client, error) {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Create base HTTP transport with TLS and connection pooling
	baseTransport := &http.Transport{
		// TLS configuration: 1.2 minimum, 1.3 preferred
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
		},

		// Connection pooling settings
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,

		// Timeouts
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: cfg.Timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Layer 1: Logging transport (innermost custom layer)
	// Logs requests, sets User-Agent, injects correlation ID
	loggingTrans := newLoggingTransport(baseTransport, cfg.UserAgent)

	// Layer 2: Retry transport (outermost custom layer)
	// Handles retries with exponential backoff
	// Only applied if retries are enabled
	var finalTransport http.RoundTripper = loggingTrans
	if cfg.RetryAttempts > 0 {
		finalTransport = newRetryTransport(loggingTrans, cfg)
	}

	return &http.Client{
		Transport: finalTransport,
		Timeout:   cfg.Timeout,
	}, nil
}
