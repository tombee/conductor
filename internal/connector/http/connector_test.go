package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/security"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config uses defaults",
			config:  nil,
			wantErr: false,
		},
		{
			name:    "empty config uses defaults",
			config:  &Config{},
			wantErr: false,
		},
		{
			name: "custom config",
			config: &Config{
				Timeout:         60 * time.Second,
				MaxResponseSize: 5 * 1024 * 1024,
				MaxRedirects:    5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && conn == nil {
				t.Error("New() returned nil connector")
			}
			if !tt.wantErr {
				if conn.config.Timeout == 0 {
					t.Error("connector timeout not set")
				}
				if conn.config.MaxResponseSize == 0 {
					t.Error("connector max response size not set")
				}
			}
		})
	}
}

func TestHTTPConnector_Get(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	conn, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	result, err := conn.Execute(context.Background(), "get", map[string]interface{}{
		"url": server.URL,
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	resp, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatal("Response is not a map")
	}

	if !resp["success"].(bool) {
		t.Error("Expected success=true")
	}

	if resp["status_code"].(int) != 200 {
		t.Errorf("Expected status 200, got %d", resp["status_code"])
	}

	body, ok := resp["body"].(string)
	if !ok {
		t.Fatal("Body is not a string")
	}

	if !strings.Contains(body, "success") {
		t.Errorf("Body does not contain expected content: %s", body)
	}
}

func TestHTTPConnector_Post(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		if !strings.Contains(string(body), "test") {
			t.Errorf("Request body does not contain 'test': %s", body)
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 123}`))
	}))
	defer server.Close()

	conn, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	result, err := conn.Execute(context.Background(), "post", map[string]interface{}{
		"url":  server.URL,
		"body": `{"name": "test"}`,
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	resp, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatal("Response is not a map")
	}

	if resp["status_code"].(int) != 201 {
		t.Errorf("Expected status 201, got %d", resp["status_code"])
	}
}

func TestHTTPConnector_InvalidURL(t *testing.T) {
	conn, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	tests := []struct {
		name string
		url  string
	}{
		{"ftp scheme", "ftp://example.com"},
		{"no scheme", "example.com"},
		{"file scheme", "file:///etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := conn.Execute(context.Background(), "get", map[string]interface{}{
				"url": tt.url,
			})

			if err == nil {
				t.Error("Expected error for invalid URL scheme")
			}

			if _, ok := err.(*InvalidURLError); !ok {
				t.Errorf("Expected InvalidURLError, got %T: %v", err, err)
			}
		})
	}
}

func TestHTTPConnector_Timeout(t *testing.T) {
	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	conn, err := New(&Config{
		Timeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	_, err = conn.Execute(context.Background(), "get", map[string]interface{}{
		"url": server.URL,
	})

	if err == nil {
		t.Error("Expected timeout error")
	}

	// Timeout can be returned as TimeoutError or NetworkError depending on timing
	if _, ok := err.(*TimeoutError); !ok {
		if _, ok := err.(*NetworkError); !ok {
			t.Errorf("Expected TimeoutError or NetworkError, got %T: %v", err, err)
		}
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("Error should mention timeout or deadline: %v", err)
	}
}

func TestHTTPConnector_ResponseSizeLimit(t *testing.T) {
	// Create server that returns large response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write 1MB of data
		data := make([]byte, 1024*1024)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	conn, err := New(&Config{
		MaxResponseSize: 512 * 1024, // 512KB limit
	})
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	_, err = conn.Execute(context.Background(), "get", map[string]interface{}{
		"url": server.URL,
	})

	if err == nil {
		t.Error("Expected error for response size limit")
	}

	if _, ok := err.(*NetworkError); !ok {
		t.Errorf("Expected NetworkError, got %T: %v", err, err)
	}

	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("Error should mention size limit: %v", err)
	}
}

func TestHTTPConnector_SecurityBlocking(t *testing.T) {
	// Test private IP blocking
	secConfig := security.DefaultHTTPSecurityConfig()
	secConfig.DenyPrivateIPs = true

	conn, err := New(&Config{
		SecurityConfig: secConfig,
	})
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	privateIPs := []string{
		"http://127.0.0.1/",
		"http://localhost/",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://172.16.0.1/",
	}

	for _, url := range privateIPs {
		t.Run(url, func(t *testing.T) {
			_, err := conn.Execute(context.Background(), "get", map[string]interface{}{
				"url": url,
			})

			if err == nil {
				t.Errorf("Expected error for private IP: %s", url)
			}

			if _, ok := err.(*SecurityBlockedError); !ok {
				t.Errorf("Expected SecurityBlockedError, got %T: %v", err, err)
			}
		})
	}
}

func TestHTTPConnector_ParseJSON(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "count": 42}`))
	}))
	defer server.Close()

	conn, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	result, err := conn.Execute(context.Background(), "get", map[string]interface{}{
		"url":        server.URL,
		"parse_json": true,
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	resp, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatal("Response is not a map")
	}

	body, ok := resp["body"].(map[string]interface{})
	if !ok {
		t.Fatalf("Body was not parsed as JSON: %T", resp["body"])
	}

	if body["status"] != "ok" {
		t.Errorf("Expected status=ok in parsed JSON, got %v", body["status"])
	}

	// count is decoded as float64 by JSON
	if count, ok := body["count"].(float64); !ok || count != 42 {
		t.Errorf("Expected count=42 in parsed JSON, got %v", body["count"])
	}
}

func TestHTTPConnector_AllOperations(t *testing.T) {
	operations := []string{"get", "post", "put", "patch", "delete"}

	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedMethod := strings.ToUpper(op)
				if r.Method != expectedMethod {
					t.Errorf("Expected %s request, got %s", expectedMethod, r.Method)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}))
			defer server.Close()

			conn, err := New(nil)
			if err != nil {
				t.Fatalf("Failed to create connector: %v", err)
			}

			inputs := map[string]interface{}{
				"url": server.URL,
			}

			// Add body for operations that support it
			if op == "post" || op == "put" || op == "patch" {
				inputs["body"] = `{"test": true}`
			}

			result, err := conn.Execute(context.Background(), op, inputs)
			if err != nil {
				t.Fatalf("Execute(%s) failed: %v", op, err)
			}

			if result == nil {
				t.Fatalf("Execute(%s) returned nil result", op)
			}
		})
	}
}

func TestHTTPConnector_RequestOperation(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != method {
					t.Errorf("Expected %s request, got %s", method, r.Method)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}))
			defer server.Close()

			conn, err := New(nil)
			if err != nil {
				t.Fatalf("Failed to create connector: %v", err)
			}

			result, err := conn.Execute(context.Background(), "request", map[string]interface{}{
				"url":    server.URL,
				"method": method,
			})

			if err != nil {
				t.Fatalf("Execute(request) failed: %v", err)
			}

			if result == nil {
				t.Fatal("Execute(request) returned nil result")
			}
		})
	}
}

func TestHTTPConnector_DNSMonitor(t *testing.T) {
	dnsConfig := security.DefaultDNSSecurityConfig()
	dnsConfig.ExfiltrationLimits.MaxSubdomainDepth = 3

	monitor := security.NewDNSQueryMonitor(*dnsConfig)

	conn, err := New(&Config{
		DNSMonitor: monitor,
	})
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	// This should fail due to subdomain depth (a.b.c.d.example.com = 5 labels > 3 limit)
	_, err = conn.Execute(context.Background(), "get", map[string]interface{}{
		"url": "http://a.b.c.d.example.com/",
	})

	if err == nil {
		t.Error("Expected error for excessive subdomain depth")
	}

	if _, ok := err.(*SecurityBlockedError); !ok {
		t.Errorf("Expected SecurityBlockedError, got %T: %v", err, err)
	}

	if !strings.Contains(err.Error(), "subdomain depth") {
		t.Errorf("Error should mention subdomain depth: %v", err)
	}
}

func TestHTTPConnector_ForbiddenHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	conn, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	forbiddenHeaders := []string{"Host", "Connection", "Transfer-Encoding"}

	for _, header := range forbiddenHeaders {
		t.Run(header, func(t *testing.T) {
			_, err := conn.Execute(context.Background(), "get", map[string]interface{}{
				"url": server.URL,
				"headers": map[string]interface{}{
					header: "malicious-value",
				},
			})

			if err == nil {
				t.Errorf("Expected error for forbidden header: %s", header)
			}

			if _, ok := err.(*SecurityBlockedError); !ok {
				t.Errorf("Expected SecurityBlockedError for %s, got %T: %v", header, err, err)
			}
		})
	}
}

func TestHTTPConnector_CustomHeaders(t *testing.T) {
	receivedHeaders := make(map[string]string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders["X-Custom"] = r.Header.Get("X-Custom")
		receivedHeaders["Authorization"] = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	conn, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	_, err = conn.Execute(context.Background(), "get", map[string]interface{}{
		"url": server.URL,
		"headers": map[string]interface{}{
			"X-Custom":      "test-value",
			"Authorization": "Bearer token123",
		},
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if receivedHeaders["X-Custom"] != "test-value" {
		t.Errorf("Expected X-Custom header 'test-value', got '%s'", receivedHeaders["X-Custom"])
	}

	if receivedHeaders["Authorization"] != "Bearer token123" {
		t.Errorf("Expected Authorization header, got '%s'", receivedHeaders["Authorization"])
	}
}

func TestHTTPConnector_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not found"))
	}))
	defer server.Close()

	conn, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	result, err := conn.Execute(context.Background(), "get", map[string]interface{}{
		"url": server.URL,
	})

	// Should not return error for non-2xx status (error in response, not in execution)
	if err != nil {
		t.Fatalf("Execute() should not fail for HTTP errors: %v", err)
	}

	resp, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatal("Response is not a map")
	}

	if resp["success"].(bool) {
		t.Error("Expected success=false for 404 status")
	}

	if resp["status_code"].(int) != 404 {
		t.Errorf("Expected status 404, got %d", resp["status_code"])
	}

	if _, hasError := resp["error"]; !hasError {
		t.Error("Expected error field in response")
	}
}

// Helper to verify JSON structure
func verifyJSON(t *testing.T, data interface{}, path string, expected interface{}) {
	t.Helper()

	obj, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("Data is not a map at %s", path)
	}

	keys := strings.Split(path, ".")
	var current interface{} = obj

	for i, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			t.Fatalf("Not a map at %s (step %d)", path, i)
		}

		val, exists := m[key]
		if !exists {
			t.Fatalf("Key %s does not exist in %s", key, path)
		}

		if i == len(keys)-1 {
			// Last key - compare value
			if fmt.Sprint(val) != fmt.Sprint(expected) {
				t.Errorf("At %s: expected %v, got %v", path, expected, val)
			}
		} else {
			current = val
		}
	}
}
