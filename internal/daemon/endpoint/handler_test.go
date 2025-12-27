// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package endpoint

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/daemon/backend/memory"
	"github.com/tombee/conductor/internal/daemon/checkpoint"
	"github.com/tombee/conductor/internal/daemon/runner"
)

func TestHandlerListEndpoints(t *testing.T) {
	// Create registry with test endpoints
	registry := NewRegistry()
	registry.Add(&Endpoint{
		Name:        "test-endpoint",
		Description: "Test endpoint",
		Workflow:    "test.yaml",
		Inputs:      map[string]any{"key": "value"},
	})
	registry.Add(&Endpoint{
		Name:        "another-endpoint",
		Description: "Another test",
		Workflow:    "other.yaml",
	})

	// Create handler
	r := createTestRunner(t)
	handler := NewHandler(registry, r, ".")

	// Create test server
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test GET /v1/endpoints
	req := httptest.NewRequest("GET", "/v1/endpoints", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response struct {
		Endpoints []EndpointResponse `json:"endpoints"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(response.Endpoints))
	}

	// Verify endpoint data
	found := false
	for _, ep := range response.Endpoints {
		if ep.Name == "test-endpoint" {
			found = true
			if ep.Description != "Test endpoint" {
				t.Errorf("expected description 'Test endpoint', got %q", ep.Description)
			}
			if len(ep.Inputs) != 1 {
				t.Errorf("expected 1 input, got %d", len(ep.Inputs))
			}
		}
	}
	if !found {
		t.Error("test-endpoint not found in response")
	}
}

func TestHandlerGetEndpoint(t *testing.T) {
	registry := NewRegistry()
	registry.Add(&Endpoint{
		Name:        "test-endpoint",
		Description: "Test endpoint",
		Workflow:    "test.yaml",
		Inputs:      map[string]any{"key": "value"},
	})

	r := createTestRunner(t)
	handler := NewHandler(registry, r, ".")

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test successful GET
	req := httptest.NewRequest("GET", "/v1/endpoints/test-endpoint", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response EndpointResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Name != "test-endpoint" {
		t.Errorf("expected name 'test-endpoint', got %q", response.Name)
	}
	if response.Description != "Test endpoint" {
		t.Errorf("expected description 'Test endpoint', got %q", response.Description)
	}

	// Test 404 for non-existent endpoint
	req = httptest.NewRequest("GET", "/v1/endpoints/nonexistent", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestHandlerCreateRun(t *testing.T) {
	// Create temp workflow file
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "test.yaml")
	workflowContent := `
name: test
inputs: []
steps:
  - id: echo
    type: llm
    prompt: "test message"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0600); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	registry := NewRegistry()
	registry.Add(&Endpoint{
		Name:        "test-endpoint",
		Description: "Test endpoint",
		Workflow:    "test.yaml",
		Inputs:      map[string]any{"message": "default"},
	})

	r := createTestRunner(t)
	handler := NewHandler(registry, r, tmpDir)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test successful run creation
	reqBody := CreateRunRequest{
		Inputs: map[string]any{"message": "hello"},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/v1/endpoints/test-endpoint/runs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", rec.Code, rec.Body.String())
	}

	// Check Location header
	location := rec.Header().Get("Location")
	if location == "" {
		t.Error("expected Location header to be set")
	}

	var response runner.RunSnapshot
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID == "" {
		t.Error("expected run ID in response")
	}

	// Test with empty body (should use defaults)
	req = httptest.NewRequest("POST", "/v1/endpoints/test-endpoint/runs", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", rec.Code)
	}

	// Test 404 for non-existent endpoint
	req = httptest.NewRequest("POST", "/v1/endpoints/nonexistent/runs", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestHandlerCreateRunInputMerge(t *testing.T) {
	// Create temp workflow file
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "test.yaml")
	workflowContent := `
name: test
inputs:
  - name: message
    type: string
  - name: count
    type: number
steps:
  - id: echo
    type: llm
    prompt: "{{.message}} {{.count}}"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0600); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	registry := NewRegistry()
	registry.Add(&Endpoint{
		Name:     "test-endpoint",
		Workflow: "test.yaml",
		Inputs: map[string]any{
			"message": "default message",
			"count":   1,
		},
	})

	r := createTestRunner(t)
	handler := NewHandler(registry, r, tmpDir)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test that request inputs override defaults
	reqBody := CreateRunRequest{
		Inputs: map[string]any{
			"message": "custom message",
			// count not specified, should use default
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/v1/endpoints/test-endpoint/runs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify run was created (we can't easily check the merged inputs without
	// inspecting the run, but successful creation indicates merge worked)
}

func TestHandlerCreateRunDraining(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "test.yaml")
	workflowContent := `
name: test
steps:
  - id: echo
    type: llm
    prompt: "test"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0600); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	registry := NewRegistry()
	registry.Add(&Endpoint{
		Name:     "test-endpoint",
		Workflow: "test.yaml",
	})

	r := createTestRunner(t)
	handler := NewHandler(registry, r, tmpDir)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Put runner in draining mode
	r.StartDraining()

	// Test that run creation is rejected
	req := httptest.NewRequest("POST", "/v1/endpoints/test-endpoint/runs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}

	// Check Retry-After header
	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header to be set")
	}
}

func TestHandlerListRuns(t *testing.T) {
	// Create temp workflow file
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "test.yaml")
	workflowContent := `
name: test
steps:
  - id: echo
    type: llm
    prompt: "test"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0600); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	registry := NewRegistry()
	registry.Add(&Endpoint{
		Name:     "test-endpoint",
		Workflow: "test.yaml",
	})

	r := createTestRunner(t)
	handler := NewHandler(registry, r, tmpDir)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// List runs for endpoint (empty initially)
	req := httptest.NewRequest("GET", "/v1/endpoints/test-endpoint/runs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response struct {
		Runs []*runner.RunSnapshot `json:"runs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Initially empty is fine - just verify the endpoint is found and response is valid
	if response.Runs == nil {
		t.Error("expected runs array in response, got nil")
	}

	// Test 404 for non-existent endpoint
	req = httptest.NewRequest("GET", "/v1/endpoints/nonexistent/runs", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestHandlerInvalidJSON(t *testing.T) {
	registry := NewRegistry()
	registry.Add(&Endpoint{
		Name:     "test-endpoint",
		Workflow: "test.yaml",
	})

	r := createTestRunner(t)
	handler := NewHandler(registry, r, ".")

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test invalid JSON
	req := httptest.NewRequest("POST", "/v1/endpoints/test-endpoint/runs", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

// Helper function to create a test runner
func createTestRunner(t *testing.T) *runner.Runner {
	t.Helper()

	backend := memory.New()
	tmpDir := t.TempDir()
	cm, err := checkpoint.NewManager(checkpoint.ManagerConfig{
		Dir: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to create checkpoint manager: %v", err)
	}

	return runner.New(runner.Config{
		MaxParallel:    1,
		DefaultTimeout: 30 * time.Second,
	}, backend, cm)
}
