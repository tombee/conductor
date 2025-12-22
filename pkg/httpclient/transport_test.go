package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tombee/conductor/internal/tracing"
)

func TestLoggingTransport_SetsUserAgent(t *testing.T) {
	// Create test server
	var receivedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create logging transport
	transport := newLoggingTransport(http.DefaultTransport, "test-agent/1.0")

	// Create request
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Execute request
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify User-Agent was set
	if receivedUserAgent != "test-agent/1.0" {
		t.Errorf("expected User-Agent %q, got %q", "test-agent/1.0", receivedUserAgent)
	}
}

func TestLoggingTransport_PreservesExistingUserAgent(t *testing.T) {
	// Create test server
	var receivedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create logging transport
	transport := newLoggingTransport(http.DefaultTransport, "test-agent/1.0")

	// Create request with existing User-Agent
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", "custom-agent/2.0")

	// Execute request
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify existing User-Agent was preserved
	if receivedUserAgent != "custom-agent/2.0" {
		t.Errorf("expected User-Agent %q, got %q", "custom-agent/2.0", receivedUserAgent)
	}
}

func TestLoggingTransport_InjectsCorrelationID(t *testing.T) {
	// Create test server
	var receivedCorrelationID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCorrelationID = r.Header.Get("X-Correlation-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create logging transport
	transport := newLoggingTransport(http.DefaultTransport, "test-agent/1.0")

	// Create request with correlation ID in context
	corrID := tracing.NewCorrelationID()
	ctx := tracing.ToContext(context.Background(), corrID)
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Execute request
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify correlation ID was injected
	if receivedCorrelationID != corrID.String() {
		t.Errorf("expected correlation ID %q, got %q", corrID.String(), receivedCorrelationID)
	}
}

func TestLoggingTransport_NoCorrelationIDWhenNotInContext(t *testing.T) {
	// Create test server
	var receivedCorrelationID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCorrelationID = r.Header.Get("X-Correlation-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create logging transport
	transport := newLoggingTransport(http.DefaultTransport, "test-agent/1.0")

	// Create request without correlation ID
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Execute request
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify no correlation ID was set
	if receivedCorrelationID != "" {
		t.Errorf("expected no correlation ID, got %q", receivedCorrelationID)
	}
}

func TestLoggingTransport_Logs(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create logging transport
	transport := newLoggingTransport(http.DefaultTransport, "test-agent/1.0")

	// Create request
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Execute request (logging output would go to slog)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify request succeeded
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}
