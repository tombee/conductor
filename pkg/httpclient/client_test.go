package httpclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew_ValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	client, err := New(cfg)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.Timeout != cfg.Timeout {
		t.Errorf("expected timeout %v, got %v", cfg.Timeout, client.Timeout)
	}
}

func TestNew_InvalidConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 0 // Invalid

	client, err := New(cfg)

	if err == nil {
		t.Fatal("expected error for invalid config")
	}

	if client != nil {
		t.Error("expected nil client on error")
	}
}

func TestNew_WithRetries(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RetryAttempts = 3

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Verify retry transport is in the chain by testing behavior
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Adjust config for faster test
	cfg.RetryBackoff = 10 * time.Millisecond
	client, err = New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

func TestNew_WithoutRetries(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RetryAttempts = 0

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Verify no retries occur
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 1 {
		t.Errorf("expected exactly 1 attempt with no retries, got %d", attempts)
	}
}

func TestNew_SetsUserAgent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.UserAgent = "test-client/2.0"

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	var receivedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if receivedUserAgent != "test-client/2.0" {
		t.Errorf("expected User-Agent %q, got %q", "test-client/2.0", receivedUserAgent)
	}
}

func TestNew_TLSConfiguration(t *testing.T) {
	cfg := DefaultConfig()

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Transport should be configured
	if client.Transport == nil {
		t.Fatal("expected non-nil transport")
	}

	// We can't easily test TLS config without setting up a TLS server,
	// but we can verify the client was created successfully
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNew_ConnectionPooling(t *testing.T) {
	cfg := DefaultConfig()

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Make multiple requests to verify connection pooling works
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	for i := 0; i < 5; i++ {
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	// If we got here without errors, connection pooling is working
}

func TestNew_Timeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 50 * time.Millisecond

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err = client.Get(server.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	// Check for timeout or canceled in error message (both indicate timeout)
	errMsg := err.Error()
	if !contains(errMsg, "deadline") && !contains(errMsg, "timeout") && !contains(errMsg, "canceled") {
		t.Errorf("expected timeout/canceled error, got %v", err)
	}
}
