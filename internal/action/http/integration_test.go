package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tombee/conductor/pkg/security"
)

// TestHTTPIntegration_Integration tests the HTTP integration in a realistic workflow scenario.
func TestHTTPAction_Integration(t *testing.T) {
	// Create a realistic API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users":
			if r.Method == "GET" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]`))
			} else if r.Method == "POST" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"id": 3, "name": "Charlie", "created": true}`))
			}

		case "/api/health":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok", "uptime": 12345}`))

		case "/api/error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "Internal server error"}`))

		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "Not found"}`))
		}
	}))
	defer server.Close()

	// Create action with realistic config
	conn, err := New(&Config{
		MaxResponseSize: 10 * 1024 * 1024, // 10MB
		MaxRedirects:    10,
		BlockPrivateIPs: false, // Allow localhost for testing
	})
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Test 1: GET request with JSON parsing
	t.Run("GET /api/users with JSON parsing", func(t *testing.T) {
		result, err := conn.Execute(context.Background(), "get", map[string]interface{}{
			"url":        server.URL + "/api/users",
			"parse_json": true,
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		resp := result.Response.(map[string]interface{})
		if !resp["success"].(bool) {
			t.Error("Expected success=true")
		}

		if resp["status_code"].(int) != 200 {
			t.Errorf("Expected status 200, got %d", resp["status_code"])
		}

		// Verify JSON was parsed
		body, ok := resp["body"].([]interface{})
		if !ok {
			t.Fatalf("Body was not parsed as JSON array: %T", resp["body"])
		}

		if len(body) != 2 {
			t.Errorf("Expected 2 users, got %d", len(body))
		}
	})

	// Test 2: POST request
	t.Run("POST /api/users", func(t *testing.T) {
		result, err := conn.Execute(context.Background(), "post", map[string]interface{}{
			"url":        server.URL + "/api/users",
			"body":       `{"name": "Charlie"}`,
			"parse_json": true,
			"headers": map[string]interface{}{
				"Content-Type": "application/json",
			},
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		resp := result.Response.(map[string]interface{})
		if resp["status_code"].(int) != 201 {
			t.Errorf("Expected status 201, got %d", resp["status_code"])
		}

		body := resp["body"].(map[string]interface{})
		if body["created"] != true {
			t.Error("Expected created=true in response")
		}
	})

	// Test 3: Error handling (500 status)
	t.Run("GET /api/error handles 500 status", func(t *testing.T) {
		result, err := conn.Execute(context.Background(), "get", map[string]interface{}{
			"url":        server.URL + "/api/error",
			"parse_json": true,
		})

		// Should not fail on HTTP errors, just report in response
		if err != nil {
			t.Fatalf("Execute should not fail on HTTP errors: %v", err)
		}

		resp := result.Response.(map[string]interface{})
		if resp["success"].(bool) {
			t.Error("Expected success=false for 500 status")
		}

		if resp["status_code"].(int) != 500 {
			t.Errorf("Expected status 500, got %d", resp["status_code"])
		}

		if _, hasError := resp["error"]; !hasError {
			t.Error("Expected error field in response for 500 status")
		}
	})

	// Test 4: Generic request operation
	t.Run("request operation with PUT method", func(t *testing.T) {
		result, err := conn.Execute(context.Background(), "request", map[string]interface{}{
			"url":    server.URL + "/api/users",
			"method": "PUT",
			"body":   `{"id": 1, "name": "Updated"}`,
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		resp := result.Response.(map[string]interface{})
		if resp == nil {
			t.Fatal("Expected non-nil response")
		}

		// Metadata should include timing
		if result.Metadata["duration_ms"] == nil {
			t.Error("Expected duration_ms in metadata")
		}
	})

	// Test 5: Custom headers
	t.Run("custom headers are sent", func(t *testing.T) {
		headerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-API-Key") != "test-key-123" {
				t.Errorf("Expected X-API-Key header, got %s", r.Header.Get("X-API-Key"))
			}
			if r.Header.Get("User-Agent") != "CustomAgent/1.0" {
				t.Errorf("Expected custom User-Agent, got %s", r.Header.Get("User-Agent"))
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer headerServer.Close()

		_, err := conn.Execute(context.Background(), "get", map[string]interface{}{
			"url": headerServer.URL,
			"headers": map[string]interface{}{
				"X-API-Key":  "test-key-123",
				"User-Agent": "CustomAgent/1.0",
			},
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
	})
}

// TestHTTPIntegration_SecurityIntegration tests security features in realistic scenarios.
func TestHTTPAction_SecurityIntegration(t *testing.T) {
	// Test 1: DNS monitor integration
	t.Run("DNS monitor blocks excessive subdomain depth", func(t *testing.T) {
		dnsConfig := security.DefaultDNSSecurityConfig()
		dnsConfig.ExfiltrationLimits.MaxSubdomainDepth = 3
		monitor := security.NewDNSQueryMonitor(*dnsConfig)

		conn, err := New(&Config{
			DNSMonitor: monitor,
		})
		if err != nil {
			t.Fatalf("Failed to create action: %v", err)
		}

		// This should be blocked: a.b.c.d.example.com = 5 parts > 3 limit
		_, err = conn.Execute(context.Background(), "get", map[string]interface{}{
			"url": "http://a.b.c.d.example.com/",
		})

		if err == nil {
			t.Error("Expected error for excessive subdomain depth")
		}

		if _, ok := err.(*SecurityBlockedError); !ok {
			t.Errorf("Expected SecurityBlockedError, got %T", err)
		}
	})

	// Test 2: Dynamic DNS blocking
	t.Run("DNS monitor blocks dynamic DNS providers", func(t *testing.T) {
		dnsConfig := security.DefaultDNSSecurityConfig()
		dnsConfig.BlockDynamicDNS = true
		monitor := security.NewDNSQueryMonitor(*dnsConfig)

		conn, err := New(&Config{
			DNSMonitor: monitor,
		})
		if err != nil {
			t.Fatalf("Failed to create action: %v", err)
		}

		dynamicDNSHosts := []string{
			"http://test.dyndns.org/",
			"http://example.ngrok.io/",
			"http://demo.duckdns.org/",
		}

		for _, url := range dynamicDNSHosts {
			t.Run(url, func(t *testing.T) {
				_, err := conn.Execute(context.Background(), "get", map[string]interface{}{
					"url": url,
				})

				if err == nil {
					t.Errorf("Expected error for dynamic DNS host: %s", url)
				}

				if _, ok := err.(*SecurityBlockedError); !ok {
					t.Errorf("Expected SecurityBlockedError, got %T", err)
				}
			})
		}
	})

	// Test 3: HTTPS requirement
	t.Run("Security config requires HTTPS", func(t *testing.T) {
		conn, err := New(&Config{
			RequireHTTPS: true,
		})
		if err != nil {
			t.Fatalf("Failed to create action: %v", err)
		}

		_, err = conn.Execute(context.Background(), "get", map[string]interface{}{
			"url": "http://example.com/",
		})

		if err == nil {
			t.Error("Expected error for HTTP when HTTPS is required")
		}

		if _, ok := err.(*SecurityBlockedError); !ok {
			t.Errorf("Expected SecurityBlockedError, got %T", err)
		}
	})

	// Test 4: Allowed hosts validation
	t.Run("Security config validates allowed hosts", func(t *testing.T) {
		secConfig := &security.HTTPSecurityConfig{
			AllowedHosts:   []string{"api.example.com", "*.trusted.com"},
			AllowedSchemes: []string{"http", "https"},
		}

		conn, err := New(&Config{
			SecurityConfig: secConfig,
		})
		if err != nil {
			t.Fatalf("Failed to create action: %v", err)
		}

		// Test allowed exact match
		t.Run("allowed exact host", func(t *testing.T) {
			_, err := conn.Execute(context.Background(), "get", map[string]interface{}{
				"url": "http://api.example.com/",
			})
			// This should fail during actual connection, but validation should pass
			// The error will be a network error, not a security blocked error
			if err != nil {
				if _, ok := err.(*SecurityBlockedError); ok {
					t.Error("Should not be blocked by security config for allowed host")
				}
			}
		})

		// Test blocked host
		t.Run("blocked host", func(t *testing.T) {
			_, err := conn.Execute(context.Background(), "get", map[string]interface{}{
				"url": "http://evil.com/",
			})

			if err == nil {
				t.Error("Expected error for non-allowed host")
			}

			if _, ok := err.(*SecurityBlockedError); !ok {
				t.Errorf("Expected SecurityBlockedError for disallowed host, got %T", err)
			}
		})
	})

	// Test 5: Redirect validation
	t.Run("Redirect validation enforces security on redirect targets", func(t *testing.T) {
		redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/redirect" {
				// Redirect to a localhost address (should be blocked if private IPs are denied)
				http.Redirect(w, r, "http://127.0.0.1:8080/", http.StatusFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer redirectServer.Close()

		secConfig := security.DefaultHTTPSecurityConfig()
		secConfig.AllowedSchemes = []string{"http", "https"}
		secConfig.DenyPrivateIPs = true
		secConfig.MaxRedirects = 10

		conn, err := New(&Config{
			SecurityConfig: secConfig,
			MaxRedirects:   10,
		})
		if err != nil {
			t.Fatalf("Failed to create action: %v", err)
		}

		_, err = conn.Execute(context.Background(), "get", map[string]interface{}{
			"url": redirectServer.URL + "/redirect",
		})

		// Should fail due to redirect to private IP
		if err == nil {
			t.Error("Expected error when redirecting to private IP")
		}
	})
}

// TestHTTPIntegration_WorkflowScenario tests a realistic workflow scenario.
func TestHTTPAction_WorkflowScenario(t *testing.T) {
	// Simulate a workflow that:
	// 1. Checks API health
	// 2. Fetches user list
	// 3. Creates a new user
	// 4. Verifies creation

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/health" && r.Method == "GET":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "healthy"}`))

		case r.URL.Path == "/users" && r.Method == "GET":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"id": 1, "name": "Alice"}]`))

		case r.URL.Path == "/users" && r.Method == "POST":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 2, "name": "Bob", "created": true}`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	conn, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	ctx := context.Background()

	// Step 1: Health check
	t.Run("step1_health_check", func(t *testing.T) {
		result, err := conn.Execute(ctx, "get", map[string]interface{}{
			"url":        server.URL + "/health",
			"parse_json": true,
		})

		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}

		resp := result.Response.(map[string]interface{})
		body := resp["body"].(map[string]interface{})

		if body["status"] != "healthy" {
			t.Errorf("API not healthy: %v", body)
		}
	})

	// Step 2: Fetch users
	var userCount int
	t.Run("step2_fetch_users", func(t *testing.T) {
		result, err := conn.Execute(ctx, "get", map[string]interface{}{
			"url":        server.URL + "/users",
			"parse_json": true,
		})

		if err != nil {
			t.Fatalf("Fetch users failed: %v", err)
		}

		resp := result.Response.(map[string]interface{})
		users := resp["body"].([]interface{})
		userCount = len(users)

		t.Logf("Found %d existing users", userCount)
	})

	// Step 3: Create user
	var newUserID float64
	t.Run("step3_create_user", func(t *testing.T) {
		result, err := conn.Execute(ctx, "post", map[string]interface{}{
			"url":        server.URL + "/users",
			"body":       `{"name": "Bob"}`,
			"parse_json": true,
			"headers": map[string]interface{}{
				"Content-Type": "application/json",
			},
		})

		if err != nil {
			t.Fatalf("Create user failed: %v", err)
		}

		resp := result.Response.(map[string]interface{})
		if resp["status_code"].(int) != 201 {
			t.Errorf("Expected 201 status, got %d", resp["status_code"])
		}

		body := resp["body"].(map[string]interface{})
		newUserID = body["id"].(float64)

		t.Logf("Created user with ID: %.0f", newUserID)
	})

	// Verify workflow state
	if userCount == 0 {
		t.Error("No users found in initial fetch")
	}
	if newUserID == 0 {
		t.Error("New user ID not set")
	}
}
