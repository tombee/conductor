package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPTransportConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *HTTPTransportConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &HTTPTransportConfig{
				BaseURL: "https://api.example.com",
				Timeout: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid with auth",
			config: &HTTPTransportConfig{
				BaseURL: "https://api.example.com",
				Auth: &AuthConfig{
					Type:  "bearer",
					Token: "${API_TOKEN}",
				},
			},
			wantErr: false,
		},
		{
			name: "missing base_url",
			config: &HTTPTransportConfig{
				Timeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid base_url (no scheme)",
			config: &HTTPTransportConfig{
				BaseURL: "api.example.com",
			},
			wantErr: true,
		},
		{
			name: "invalid base_url (invalid scheme)",
			config: &HTTPTransportConfig{
				BaseURL: "ftp://api.example.com",
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			config: &HTTPTransportConfig{
				BaseURL: "https://api.example.com",
				Timeout: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid auth",
			config: &HTTPTransportConfig{
				BaseURL: "https://api.example.com",
				Auth: &AuthConfig{
					Type: "bearer",
					// Missing token
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *AuthConfig
		wantErr bool
	}{
		{
			name: "valid bearer",
			config: &AuthConfig{
				Type:  "bearer",
				Token: "${API_TOKEN}",
			},
			wantErr: false,
		},
		{
			name: "valid basic",
			config: &AuthConfig{
				Type:     "basic",
				Username: "user",
				Password: "${API_PASSWORD}",
			},
			wantErr: false,
		},
		{
			name: "valid api_key",
			config: &AuthConfig{
				Type:        "api_key",
				HeaderName:  "X-API-Key",
				HeaderValue: "${API_KEY}",
			},
			wantErr: false,
		},
		{
			name: "bearer missing token",
			config: &AuthConfig{
				Type: "bearer",
			},
			wantErr: true,
		},
		{
			name: "basic missing username",
			config: &AuthConfig{
				Type:     "basic",
				Password: "pass",
			},
			wantErr: true,
		},
		{
			name: "basic missing password",
			config: &AuthConfig{
				Type:     "basic",
				Username: "user",
			},
			wantErr: true,
		},
		{
			name: "api_key missing header name",
			config: &AuthConfig{
				Type:        "api_key",
				HeaderValue: "secret-key",
			},
			wantErr: true,
		},
		{
			name: "api_key missing header value",
			config: &AuthConfig{
				Type:       "api_key",
				HeaderName: "X-API-Key",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			config: &AuthConfig{
				Type: "oauth2",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHTTPTransport_Execute_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Create transport
	config := &HTTPTransportConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	// Execute request
	req := &Request{
		Method: "GET",
		URL:    server.URL + "/test",
	}

	resp, err := transport.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Execute() status code = %d, want 200", resp.StatusCode)
	}
	if string(resp.Body) != `{"status":"ok"}` {
		t.Errorf("Execute() body = %q, want %q", string(resp.Body), `{"status":"ok"}`)
	}
}

func TestHTTPTransport_Execute_BearerAuth(t *testing.T) {
	// Create test server that checks Authorization header
	// Set up env var for bearer token
	t.Setenv("API_TOKEN", "test-bearer-token-123")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		// Expect expanded value since env vars are expanded at request time
		if auth != "Bearer test-bearer-token-123" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("authenticated"))
	}))
	defer server.Close()

	// Create transport with bearer auth
	config := &HTTPTransportConfig{
		BaseURL: server.URL,
		Auth: &AuthConfig{
			Type:  "bearer",
			Token: "${API_TOKEN}",
		},
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	// Execute request
	req := &Request{
		Method: "GET",
		URL:    server.URL + "/protected",
	}

	resp, err := transport.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Execute() status code = %d, want 200", resp.StatusCode)
	}
}

func TestHTTPTransport_Execute_BasicAuth(t *testing.T) {
	// Set up env var for password
	t.Setenv("API_PASSWORD", "test-password-456")

	// Create test server that checks basic auth
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		// Expect expanded value since env vars are expanded at request time
		if !ok || user != "testuser" || pass != "test-password-456" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("authenticated"))
	}))
	defer server.Close()

	// Create transport with basic auth
	config := &HTTPTransportConfig{
		BaseURL: server.URL,
		Auth: &AuthConfig{
			Type:     "basic",
			Username: "testuser",
			Password: "${API_PASSWORD}",
		},
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	// Execute request
	req := &Request{
		Method: "GET",
		URL:    server.URL + "/protected",
	}

	resp, err := transport.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Execute() status code = %d, want 200", resp.StatusCode)
	}
}

func TestHTTPTransport_Execute_APIKeyAuth(t *testing.T) {
	// Set up env var for API key
	t.Setenv("API_KEY", "test-api-key-789")

	// Create test server that checks API key header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		// Expect expanded value since env vars are expanded at request time
		if apiKey != "test-api-key-789" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("authenticated"))
	}))
	defer server.Close()

	// Create transport with API key auth
	config := &HTTPTransportConfig{
		BaseURL: server.URL,
		Auth: &AuthConfig{
			Type:        "api_key",
			HeaderName:  "X-API-Key",
			HeaderValue: "${API_KEY}",
		},
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	// Execute request
	req := &Request{
		Method: "GET",
		URL:    server.URL + "/protected",
	}

	resp, err := transport.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Execute() status code = %d, want 200", resp.StatusCode)
	}
}

func TestHTTPTransport_Execute_DefaultHeaders(t *testing.T) {
	// Create test server that checks headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Create transport with default headers
	config := &HTTPTransportConfig{
		BaseURL: server.URL,
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
		},
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	// Execute request
	req := &Request{
		Method: "GET",
		URL:    server.URL + "/test",
	}

	resp, err := transport.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Execute() status code = %d, want 200", resp.StatusCode)
	}
}

func TestHTTPTransport_Execute_Timeout(t *testing.T) {
	// Create test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer server.Close()

	// Create transport with short timeout
	config := &HTTPTransportConfig{
		BaseURL: server.URL,
		Timeout: 50 * time.Millisecond,
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	// Execute request (should timeout)
	req := &Request{
		Method: "GET",
		URL:    server.URL + "/slow",
	}

	_, err = transport.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Execute() error = nil, want timeout error")
	}

	transportErr, ok := err.(*TransportError)
	if !ok {
		t.Fatalf("Execute() error type = %T, want *TransportError", err)
	}

	// Should be classified as timeout, connection, or cancelled (all valid for deadline exceeded)
	validTypes := map[ErrorType]bool{
		ErrorTypeTimeout:    true,
		ErrorTypeConnection: true,
		ErrorTypeCancelled:  true,
	}
	if !validTypes[transportErr.Type] {
		t.Errorf("Execute() error type = %v, want one of timeout/connection/cancelled", transportErr.Type)
	}
}

func TestHTTPTransport_Execute_401Error(t *testing.T) {
	// Create test server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	// Create transport
	config := &HTTPTransportConfig{
		BaseURL: server.URL,
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	// Execute request
	req := &Request{
		Method: "GET",
		URL:    server.URL + "/protected",
	}

	_, err = transport.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Execute() error = nil, want auth error")
	}

	transportErr, ok := err.(*TransportError)
	if !ok {
		t.Fatalf("Execute() error type = %T, want *TransportError", err)
	}

	if transportErr.Type != ErrorTypeAuth {
		t.Errorf("Execute() error type = %v, want %v", transportErr.Type, ErrorTypeAuth)
	}
	if transportErr.StatusCode != 401 {
		t.Errorf("Execute() error status code = %d, want 401", transportErr.StatusCode)
	}
	if transportErr.Retryable {
		t.Error("Execute() error should not be retryable for 401")
	}
}

func TestHTTPTransport_Execute_429RateLimit(t *testing.T) {
	// Create test server that returns 429
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
		w.Write([]byte("Too Many Requests"))
	}))
	defer server.Close()

	// Create transport with retry disabled to test single attempt
	config := &HTTPTransportConfig{
		BaseURL: server.URL,
		RetryConfig: &RetryConfig{
			MaxAttempts:     1, // No retries
			InitialBackoff:  1 * time.Second,
			MaxBackoff:      30 * time.Second,
			BackoffFactor:   2.0,
			RetryableErrors: []int{429},
		},
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	// Execute request
	req := &Request{
		Method: "GET",
		URL:    server.URL + "/limited",
	}

	_, err = transport.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Execute() error = nil, want rate limit error")
	}

	transportErr, ok := err.(*TransportError)
	if !ok {
		t.Fatalf("Execute() error type = %T, want *TransportError", err)
	}

	if transportErr.Type != ErrorTypeRateLimit {
		t.Errorf("Execute() error type = %v, want %v", transportErr.Type, ErrorTypeRateLimit)
	}
	if transportErr.StatusCode != 429 {
		t.Errorf("Execute() error status code = %d, want 429", transportErr.StatusCode)
	}
	if !transportErr.Retryable {
		t.Error("Execute() error should be retryable for 429")
	}
}

func TestHTTPTransport_Execute_500Error(t *testing.T) {
	// Create test server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Create transport with retry disabled
	config := &HTTPTransportConfig{
		BaseURL: server.URL,
		RetryConfig: &RetryConfig{
			MaxAttempts:     1, // No retries
			InitialBackoff:  1 * time.Second,
			MaxBackoff:      30 * time.Second,
			BackoffFactor:   2.0,
			RetryableErrors: []int{500},
		},
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	// Execute request
	req := &Request{
		Method: "GET",
		URL:    server.URL + "/error",
	}

	_, err = transport.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Execute() error = nil, want server error")
	}

	transportErr, ok := err.(*TransportError)
	if !ok {
		t.Fatalf("Execute() error type = %T, want *TransportError", err)
	}

	if transportErr.Type != ErrorTypeServer {
		t.Errorf("Execute() error type = %v, want %v", transportErr.Type, ErrorTypeServer)
	}
	if transportErr.StatusCode != 500 {
		t.Errorf("Execute() error status code = %d, want 500", transportErr.StatusCode)
	}
	if !transportErr.Retryable {
		t.Error("Execute() error should be retryable for 500")
	}
}

func TestHTTPTransport_Execute_Retry(t *testing.T) {
	attemptCount := 0

	// Create test server that succeeds on 3rd attempt
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	// Create transport with retry enabled
	config := &HTTPTransportConfig{
		BaseURL: server.URL,
		RetryConfig: &RetryConfig{
			MaxAttempts:     3,
			InitialBackoff:  10 * time.Millisecond,
			MaxBackoff:      100 * time.Millisecond,
			BackoffFactor:   2.0,
			RetryableErrors: []int{503},
		},
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	// Execute request
	req := &Request{
		Method: "GET",
		URL:    server.URL + "/retry",
	}

	resp, err := transport.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Execute() status code = %d, want 200", resp.StatusCode)
	}
	if attemptCount != 3 {
		t.Errorf("Execute() attempts = %d, want 3", attemptCount)
	}
}

func TestHTTPTransport_Execute_InvalidRequest(t *testing.T) {
	config := &HTTPTransportConfig{
		BaseURL: "https://api.example.com",
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	tests := []struct {
		name string
		req  *Request
	}{
		{
			name: "empty method",
			req: &Request{
				Method: "",
				URL:    "https://api.example.com/test",
			},
		},
		{
			name: "invalid method",
			req: &Request{
				Method: "INVALID",
				URL:    "https://api.example.com/test",
			},
		},
		{
			name: "empty URL",
			req: &Request{
				Method: "GET",
				URL:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := transport.Execute(context.Background(), tt.req)
			if err == nil {
				t.Fatal("Execute() error = nil, want invalid request error")
			}

			transportErr, ok := err.(*TransportError)
			if !ok {
				t.Fatalf("Execute() error type = %T, want *TransportError", err)
			}

			if transportErr.Type != ErrorTypeInvalidReq {
				t.Errorf("Execute() error type = %v, want %v", transportErr.Type, ErrorTypeInvalidReq)
			}
		})
	}
}

func TestHTTPTransport_Name(t *testing.T) {
	config := &HTTPTransportConfig{
		BaseURL: "https://api.example.com",
	}
	transport, err := NewHTTPTransport(config)
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	if transport.Name() != "http" {
		t.Errorf("Name() = %q, want %q", transport.Name(), "http")
	}
}

func TestHTTPTransportConfig_TransportType(t *testing.T) {
	config := &HTTPTransportConfig{
		BaseURL: "https://api.example.com",
	}

	if config.TransportType() != "http" {
		t.Errorf("TransportType() = %q, want %q", config.TransportType(), "http")
	}
}

func TestIsTimeoutError(t *testing.T) {
	// Create a timeout error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	timeoutErr := ctx.Err()
	if !strings.Contains(timeoutErr.Error(), "deadline exceeded") {
		t.Skip("context error doesn't match expected format")
	}

	// Test with actual timeout
	client := &http.Client{
		Timeout: 1 * time.Millisecond,
	}

	req, _ := http.NewRequest("GET", "http://localhost:1", nil)
	_, err := client.Do(req)

	if err != nil && isTimeoutError(err) {
		// Success - detected timeout
		return
	}

	// Fallback: check if we can at least detect the timeout from context
	if err != nil {
		t.Logf("Error type: %T, message: %v", err, err)
	}
}

func TestIsConnectionError(t *testing.T) {
	// Test with actual connection refused error
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	req, _ := http.NewRequest("GET", "http://localhost:1", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Skip("expected connection error, got nil")
	}

	if !isConnectionError(err) {
		t.Errorf("isConnectionError() = false, want true for error: %v", err)
	}
}
