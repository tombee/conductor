package jenkins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tombee/conductor/internal/connector/api"
	"github.com/tombee/conductor/internal/connector/transport"
)

func TestNewJenkinsConnector(t *testing.T) {
	tests := []struct {
		name      string
		config    *api.ConnectorConfig
		wantError bool
	}{
		{
			name: "valid config",
			config: &api.ConnectorConfig{
				BaseURL:   "https://jenkins.example.com",
				Token:     "test-token",
				Transport: &transport.HTTPTransport{},
			},
			wantError: false,
		},
		{
			name: "missing base URL",
			config: &api.ConnectorConfig{
				Token:     "test-token",
				Transport: &transport.HTTPTransport{},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewJenkinsConnector(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("NewJenkinsConnector() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestJenkinsConnector_Operations(t *testing.T) {
	config := &api.ConnectorConfig{
		BaseURL:   "https://jenkins.example.com",
		Token:     "test-token",
		Transport: &transport.HTTPTransport{},
	}

	conn, err := NewJenkinsConnector(config)
	if err != nil {
		t.Fatalf("NewJenkinsConnector() error = %v", err)
	}

	jc := conn.(*JenkinsConnector)
	ops := jc.Operations()

	// Verify we have all 15 operations
	if len(ops) != 15 {
		t.Errorf("Operations() returned %d operations, want 15", len(ops))
	}

	// Verify operation names
	expectedOps := map[string]bool{
		"trigger_build":                 true,
		"trigger_build_with_parameters": true,
		"get_build":                     true,
		"get_build_log":                 true,
		"cancel_build":                  true,
		"get_job":                       true,
		"list_jobs":                     true,
		"get_queue_item":                true,
		"cancel_queue_item":             true,
		"get_test_report":               true,
		"list_nodes":                    true,
		"get_node":                      true,
		"get_last_build":                true,
		"get_last_successful_build":     true,
		"get_last_failed_build":         true,
	}

	for _, op := range ops {
		if !expectedOps[op.Name] {
			t.Errorf("Unexpected operation: %s", op.Name)
		}
		delete(expectedOps, op.Name)
	}

	if len(expectedOps) > 0 {
		t.Errorf("Missing operations: %v", expectedOps)
	}
}

func TestJenkinsConnector_BasicAuth(t *testing.T) {
	// Create a test server to verify auth header
	receivedAuth := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"number":   1,
			"url":      "http://example.com/job/test/1",
			"result":   "SUCCESS",
			"building": false,
			"duration": 1000,
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
		AdditionalAuth: map[string]string{
			"username": "test-user",
		},
	}

	conn, err := NewJenkinsConnector(config)
	if err != nil {
		t.Fatalf("NewJenkinsConnector() error = %v", err)
	}

	// Execute a simple operation
	_, err = conn.Execute(context.Background(), "get_build", map[string]interface{}{
		"job_name":     "test",
		"build_number": "1",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify Basic auth header was sent
	if receivedAuth == "" {
		t.Error("No Authorization header was sent")
	}

	// Basic auth should start with "Basic "
	if len(receivedAuth) < 6 || receivedAuth[:6] != "Basic " {
		t.Errorf("Authorization header should start with 'Basic ', got: %s", receivedAuth)
	}
}

func TestJenkinsConnector_GetBuild(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		expectedPath := "/job/test-job/42/api/json"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Build{
			Number:   42,
			URL:      "http://example.com/job/test-job/42",
			Result:   "SUCCESS",
			Building: false,
			Duration: 5000,
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewJenkinsConnector(config)
	if err != nil {
		t.Fatalf("NewJenkinsConnector() error = %v", err)
	}

	result, err := conn.Execute(context.Background(), "get_build", map[string]interface{}{
		"job_name":     "test-job",
		"build_number": 42,
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify response
	resp, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Response is not a map")
	}

	if resp["number"] != 42 {
		t.Errorf("Expected build number 42, got %v", resp["number"])
	}

	if resp["result"] != "SUCCESS" {
		t.Errorf("Expected result SUCCESS, got %v", resp["result"])
	}
}

func TestJenkinsConnector_ListJobs(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(JobListResponse{
			Jobs: []Job{
				{Name: "job1", URL: "http://example.com/job/job1", Color: "blue", Buildable: true},
				{Name: "job2", URL: "http://example.com/job/job2", Color: "red", Buildable: true},
			},
		})
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewJenkinsConnector(config)
	if err != nil {
		t.Fatalf("NewJenkinsConnector() error = %v", err)
	}

	result, err := conn.Execute(context.Background(), "list_jobs", map[string]interface{}{})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify response
	jobs, ok := result.Response.([]map[string]interface{})
	if !ok {
		t.Fatalf("Response is not a slice of maps")
	}

	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}

	if jobs[0]["name"] != "job1" {
		t.Errorf("Expected first job name 'job1', got %v", jobs[0]["name"])
	}
}

func TestJenkinsConnector_ErrorHandling(t *testing.T) {
	// Create a test server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Job not found"}`))
	}))
	defer server.Close()

	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   server.URL,
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewJenkinsConnector(config)
	if err != nil {
		t.Fatalf("NewJenkinsConnector() error = %v", err)
	}

	_, err = conn.Execute(context.Background(), "get_job", map[string]interface{}{
		"job_name": "nonexistent",
	})

	if err == nil {
		t.Fatal("Expected error for 404 response, got nil")
	}

	// The error should contain information about the 404
	// (It might be wrapped by the transport layer, so we just verify it's an error)
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestJenkinsConnector_UnknownOperation(t *testing.T) {
	httpTransport, err := transport.NewHTTPTransport(&transport.HTTPTransportConfig{
		BaseURL: "https://jenkins.example.com",
	})
	if err != nil {
		t.Fatalf("NewHTTPTransport() error = %v", err)
	}

	config := &api.ConnectorConfig{
		BaseURL:   "https://jenkins.example.com",
		Token:     "test-token",
		Transport: httpTransport,
	}

	conn, err := NewJenkinsConnector(config)
	if err != nil {
		t.Fatalf("NewJenkinsConnector() error = %v", err)
	}

	_, err = conn.Execute(context.Background(), "unknown_operation", map[string]interface{}{})

	if err == nil {
		t.Fatal("Expected error for unknown operation, got nil")
	}

	if err.Error() != "unknown operation: unknown_operation" {
		t.Errorf("Unexpected error message: %v", err)
	}
}
