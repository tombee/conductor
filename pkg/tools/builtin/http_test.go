package builtin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/security"
)

// testSecurityConfig returns a permissive config for testing with local servers.
func testSecurityConfig() *security.HTTPSecurityConfig {
	config := security.DefaultHTTPSecurityConfig()
	config.AllowedSchemes = []string{"http", "https"} // Allow HTTP for tests
	config.DenyPrivateIPs = false                      // Allow localhost
	config.DenyMetadata = false                        // Not needed for tests
	config.ResolveBeforeValidation = false            // Skip DNS for local tests
	return config
}

func TestHTTPTool_Name(t *testing.T) {
	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	if tool.Name() != "http" {
		t.Errorf("Name() = %s, want http", tool.Name())
	}
}

func TestHTTPTool_Description(t *testing.T) {
	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestHTTPTool_Schema(t *testing.T) {
	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	schema := tool.Schema()

	if schema == nil {
		t.Fatal("Schema() returned nil")
	}

	if schema.Inputs == nil {
		t.Fatal("Schema inputs is nil")
	}

	// Check required fields
	found := false
	for _, field := range schema.Inputs.Required {
		if field == "url" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Required field 'url' not in schema")
	}
}

func TestHTTPTool_GET(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"url": server.URL,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	success, ok := result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if !success {
		t.Errorf("Request should have succeeded: %v", result)
	}

	statusCode, ok := result["status_code"].(int)
	if !ok {
		t.Fatal("status_code field is not an int")
	}

	if statusCode != 200 {
		t.Errorf("status_code = %d, want 200", statusCode)
	}

	body, ok := result["body"].(string)
	if !ok {
		t.Fatal("body field is not a string")
	}

	if body != `{"message": "success"}` {
		t.Errorf("body = %s, want %s", body, `{"message": "success"}`)
	}
}

func TestHTTPTool_POST(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"created": true}`))
	}))
	defer server.Close()

	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"method": "POST",
		"url":    server.URL,
		"body":   `{"name": "test"}`,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	success, ok := result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if !success {
		t.Errorf("Request should have succeeded: %v", result)
	}

	statusCode, ok := result["status_code"].(int)
	if !ok {
		t.Fatal("status_code field is not an int")
	}

	if statusCode != 201 {
		t.Errorf("status_code = %d, want 201", statusCode)
	}
}

func TestHTTPTool_CustomHeaders(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer token123" {
			t.Errorf("Expected Authorization header, got %s", auth)
		}

		customHeader := r.Header.Get("X-Custom-Header")
		if customHeader != "custom-value" {
			t.Errorf("Expected X-Custom-Header, got %s", customHeader)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"url": server.URL,
		"headers": map[string]interface{}{
			"Authorization":   "Bearer token123",
			"X-Custom-Header": "custom-value",
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	success, ok := result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if !success {
		t.Error("Request should have succeeded")
	}
}

func TestHTTPTool_ResponseHeaders(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Response-ID", "12345")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"url": server.URL,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	headers, ok := result["headers"].(map[string]interface{})
	if !ok {
		t.Fatal("headers field is not a map")
	}

	responseID, ok := headers["X-Response-Id"].(string)
	if !ok {
		t.Fatal("X-Response-Id header is not a string")
	}

	if responseID != "12345" {
		t.Errorf("X-Response-Id = %s, want 12345", responseID)
	}
}

func TestHTTPTool_ErrorStatusCode(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"url": server.URL,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	success, ok := result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if success {
		t.Error("Request with 404 should not succeed")
	}

	statusCode, ok := result["status_code"].(int)
	if !ok {
		t.Fatal("status_code field is not an int")
	}

	if statusCode != 404 {
		t.Errorf("status_code = %d, want 404", statusCode)
	}
}

func TestHTTPTool_Timeout(t *testing.T) {
	// Create test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig()).WithTimeout(50 * time.Millisecond)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"url": server.URL,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	success, ok := result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if success {
		t.Error("Request should timeout and not succeed")
	}

	errMsg, ok := result["error"].(string)
	if !ok {
		t.Fatal("error field is not a string")
	}

	if errMsg == "" {
		t.Error("Error message should not be empty for timeout")
	}
}

func TestHTTPTool_AllowedHosts(t *testing.T) {
	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig()).WithAllowedHosts([]string{"example.com", "api.example.com"})
	ctx := context.Background()

	tests := []struct {
		name      string
		url       string
		shouldErr bool
	}{
		{
			name:      "allowed host",
			url:       "https://api.example.com/endpoint",
			shouldErr: false,
		},
		{
			name:      "disallowed host",
			url:       "https://malicious.com/endpoint",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, map[string]interface{}{
				"url": tt.url,
			})

			if tt.shouldErr && err == nil {
				t.Error("Execute() should have failed for disallowed host")
			}
			if !tt.shouldErr && err != nil {
				// Note: This will fail with network error since we're not using a test server
				// but that's okay - we're testing the validation logic
				t.Logf("Execute() error (expected for real URL): %v", err)
			}
		})
	}
}

func TestHTTPTool_InvalidURL(t *testing.T) {
	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	ctx := context.Background()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "invalid scheme",
			url:  "ftp://example.com",
		},
		{
			name: "no scheme",
			url:  "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, map[string]interface{}{
				"url": tt.url,
			})
			if err == nil {
				t.Error("Execute() should fail with invalid URL")
			}
		})
	}
}

func TestHTTPTool_InvalidInputs(t *testing.T) {
	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	ctx := context.Background()

	tests := []struct {
		name   string
		inputs map[string]interface{}
	}{
		{
			name:   "missing url",
			inputs: map[string]interface{}{},
		},
		{
			name: "invalid url type",
			inputs: map[string]interface{}{
				"url": 123,
			},
		},
		{
			name: "invalid method type",
			inputs: map[string]interface{}{
				"url":    "http://example.com",
				"method": 123,
			},
		},
		{
			name: "invalid body type",
			inputs: map[string]interface{}{
				"url":  "http://example.com",
				"body": 123,
			},
		},
		{
			name: "invalid headers type",
			inputs: map[string]interface{}{
				"url":     "http://example.com",
				"headers": "not a map",
			},
		},
		{
			name: "invalid header value type",
			inputs: map[string]interface{}{
				"url": "http://example.com",
				"headers": map[string]interface{}{
					"Authorization": 123,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, tt.inputs)
			if err == nil {
				t.Error("Execute() should fail with invalid inputs")
			}
		})
	}
}

func TestHTTPTool_ParseJSON(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantErr   bool
		wantKey   string
		wantValue interface{}
	}{
		{
			name:      "valid JSON",
			body:      `{"key": "value"}`,
			wantErr:   false,
			wantKey:   "key",
			wantValue: "value",
		},
		{
			name:    "invalid JSON",
			body:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseJSON(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if val, ok := result[tt.wantKey]; !ok || val != tt.wantValue {
					t.Errorf("ParseJSON() = %v, want key %s with value %v", result, tt.wantKey, tt.wantValue)
				}
			}
		})
	}
}

func TestHTTPTool_MethodCaseInsensitive(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewHTTPTool().WithSecurityConfig(testSecurityConfig())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"method": "put", // lowercase
		"url":    server.URL,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	success, ok := result["success"].(bool)
	if !ok {
		t.Fatal("success field is not a boolean")
	}

	if !success {
		t.Error("Request should have succeeded with lowercase method")
	}
}
