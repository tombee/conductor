package httpclient

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/tombee/conductor/internal/tracing"
)

// loggingTransport wraps an http.RoundTripper to add:
// - Request logging with sanitized URLs
// - User-Agent header injection
// - Correlation ID propagation
// - Duration tracking
type loggingTransport struct {
	base      http.RoundTripper
	userAgent string
}

// newLoggingTransport creates a new logging transport that wraps the base transport.
func newLoggingTransport(base http.RoundTripper, userAgent string) *loggingTransport {
	if base == nil {
		base = http.DefaultTransport
	}

	return &loggingTransport{
		base:      base,
		userAgent: userAgent,
	}
}

// RoundTrip implements http.RoundTripper.
// Logs all requests with method, URL (sanitized), status/error, and duration.
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Set User-Agent header if not already set
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", t.userAgent)
	}

	// Inject correlation ID for distributed tracing
	if corrID := tracing.FromContextOrEmpty(req.Context()); corrID.IsValid() {
		req.Header.Set("X-Correlation-ID", corrID.String())
	}

	// Execute request
	resp, err := t.base.RoundTrip(req)
	duration := time.Since(start).Milliseconds()

	// Sanitize URL for logging (remove sensitive query params)
	logURL := sanitizeURL(req.URL)

	// Log based on outcome
	if err != nil {
		slog.Warn("http request failed",
			"method", req.Method,
			"url", logURL,
			"duration_ms", duration,
			"error", err.Error(),
		)
	} else {
		level := slog.LevelDebug
		if resp.StatusCode >= 400 {
			level = slog.LevelWarn
		}
		slog.Log(req.Context(), level, "http request",
			"method", req.Method,
			"url", logURL,
			"status", resp.StatusCode,
			"duration_ms", duration,
		)
	}

	return resp, err
}
