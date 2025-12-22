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

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/daemon/backend/memory"
	"github.com/tombee/conductor/internal/daemon/checkpoint"
	"github.com/tombee/conductor/internal/daemon/runner"
)

func setupTestServer(t *testing.T) (*http.ServeMux, *runner.Runner) {
	t.Helper()

	// Use a temp directory that we manage manually to avoid cleanup race conditions
	tmpDir, err := os.MkdirTemp("", "runs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		// Give any background goroutines time to finish
		time.Sleep(10 * time.Millisecond)
		os.RemoveAll(tmpDir)
	})

	be := memory.New()
	cm, err := checkpoint.NewManager(checkpoint.ManagerConfig{
		Dir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create checkpoint manager: %v", err)
	}

	r := runner.New(runner.Config{
		MaxParallel:    2,
		DefaultTimeout: 30 * time.Second,
	}, be, cm)

	mux := http.NewServeMux()
	handler := NewRunsHandler(r)
	handler.RegisterRoutes(mux)

	return mux, r
}

func TestRunsHandler_CreateRun(t *testing.T) {
	mux, _ := setupTestServer(t)

	tests := []struct {
		name           string
		contentType    string
		body           string
		wantStatus     int
		wantErrContain string
	}{
		{
			name:        "valid YAML workflow",
			contentType: "application/x-yaml",
			body: `name: test-workflow
version: "1.0"
description: A test workflow
inputs: []
steps:
  - id: step1
    name: Test Step
    type: llm
    prompt: "test"
outputs: []
`,
			wantStatus: http.StatusAccepted,
		},
		{
			name:        "valid text/yaml workflow",
			contentType: "text/yaml",
			body: `name: test-workflow-2
version: "1.0"
description: Another test workflow
inputs: []
steps:
  - id: step1
    name: Test Step
    type: llm
    prompt: "test"
outputs: []
`,
			wantStatus: http.StatusAccepted,
		},
		{
			name:           "invalid YAML",
			contentType:    "application/x-yaml",
			body:           "invalid: yaml: content:",
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "failed to submit run",
		},
		{
			name:           "JSON with workflow reference not supported",
			contentType:    "application/json",
			body:           `{"workflow": "my-workflow"}`,
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "workflow_yaml field required",
		},
		{
			name:           "unsupported content type",
			contentType:    "text/plain",
			body:           "hello",
			wantStatus:     http.StatusUnsupportedMediaType,
			wantErrContain: "content-type must be",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/runs", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d. Body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantErrContain != "" && !strings.Contains(rec.Body.String(), tt.wantErrContain) {
				t.Errorf("expected error containing %q, got %s", tt.wantErrContain, rec.Body.String())
			}
		})
	}
}

func TestRunsHandler_ListRuns(t *testing.T) {
	mux, r := setupTestServer(t)

	// Create some test runs
	workflow := []byte(`name: list-test
version: "1.0"
description: Test
inputs: []
steps:
  - id: step1
    name: Test
    type: llm
    prompt: "test"
outputs: []
`)

	for i := 0; i < 3; i++ {
		_, err := r.Submit(context.Background(), runner.SubmitRequest{WorkflowYAML: workflow})
		if err != nil {
			t.Fatalf("Failed to submit run: %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/v1/runs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	var result struct {
		Runs  []any `json:"runs"`
		Count int   `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Count != 3 {
		t.Errorf("got count %d, want 3", result.Count)
	}
}

func TestRunsHandler_GetRun(t *testing.T) {
	mux, r := setupTestServer(t)

	// Create a test run
	workflow := []byte(`name: get-test
version: "1.0"
description: Test
inputs: []
steps:
  - id: step1
    name: Test
    type: llm
    prompt: "test"
outputs: []
`)

	run, err := r.Submit(context.Background(), runner.SubmitRequest{WorkflowYAML: workflow})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	tests := []struct {
		name       string
		runID      string
		wantStatus int
	}{
		{
			name:       "existing run",
			runID:      run.ID,
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent run",
			runID:      "non-existent-id",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/runs/"+tt.runID, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d. Body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestRunsHandler_CancelRun(t *testing.T) {
	mux, r := setupTestServer(t)

	// Create a test run with a simple workflow
	workflow := []byte(`name: cancel-test
version: "1.0"
description: Test
inputs: []
steps:
  - id: step1
    name: Test
    type: llm
    prompt: "test"
outputs: []
`)

	run, err := r.Submit(context.Background(), runner.SubmitRequest{WorkflowYAML: workflow})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	// Cancel the run (may succeed or return conflict if already completed)
	req := httptest.NewRequest("DELETE", "/v1/runs/"+run.ID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Accept either OK (cancelled) or Conflict (already completed)
	if rec.Code != http.StatusOK && rec.Code != http.StatusConflict {
		t.Errorf("got status %d, want %d or %d. Body: %s", rec.Code, http.StatusOK, http.StatusConflict, rec.Body.String())
	}
}

func TestRunsHandler_CancelNonExistent(t *testing.T) {
	mux, _ := setupTestServer(t)

	req := httptest.NewRequest("DELETE", "/v1/runs/non-existent-id", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRunsHandler_GetOutput(t *testing.T) {
	mux, r := setupTestServer(t)

	// Create a run
	workflow := []byte(`name: output-test
version: "1.0"
description: Test
inputs: []
steps:
  - id: step1
    name: Test
    type: llm
    prompt: "test"
outputs: []
`)

	run, err := r.Submit(context.Background(), runner.SubmitRequest{WorkflowYAML: workflow})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/runs/"+run.ID+"/output", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Should return conflict (not completed) or OK (completed)
	if rec.Code != http.StatusConflict && rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d or %d. Body: %s", rec.Code, http.StatusConflict, http.StatusOK, rec.Body.String())
	}
}

func TestRunsHandler_GetLogs(t *testing.T) {
	mux, r := setupTestServer(t)

	// Create a test run
	workflow := []byte(`name: logs-test
version: "1.0"
description: Test
inputs: []
steps:
  - id: step1
    name: Test
    type: llm
    prompt: "test"
outputs: []
`)

	run, err := r.Submit(context.Background(), runner.SubmitRequest{WorkflowYAML: workflow})
	if err != nil {
		t.Fatalf("Failed to submit run: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/runs/"+run.ID+"/logs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d. Body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result struct {
		Logs  []any `json:"logs"`
		Count int   `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
}

func TestRunsHandler_ListWithFilters(t *testing.T) {
	mux, r := setupTestServer(t)

	// Create test runs
	workflow := []byte(`name: filter-test
version: "1.0"
description: Test
inputs: []
steps:
  - id: step1
    name: Test
    type: llm
    prompt: "test"
outputs: []
`)

	for i := 0; i < 3; i++ {
		_, err := r.Submit(context.Background(), runner.SubmitRequest{WorkflowYAML: workflow})
		if err != nil {
			t.Fatalf("Failed to submit run: %v", err)
		}
	}

	tests := []struct {
		name      string
		query     string
		wantCount int
	}{
		{
			name:      "filter by workflow",
			query:     "?workflow=filter-test",
			wantCount: 3,
		},
		{
			name:      "filter by non-existent workflow",
			query:     "?workflow=non-existent",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/runs"+tt.query, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
			}

			var result struct {
				Runs  []any `json:"runs"`
				Count int   `json:"count"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if result.Count != tt.wantCount {
				t.Errorf("got count %d, want %d", result.Count, tt.wantCount)
			}
		})
	}
}

func TestRunsHandler_YAMLWithInputs(t *testing.T) {
	mux, _ := setupTestServer(t)

	workflow := `name: input-test
version: "1.0"
description: Test with inputs
inputs:
  - name: message
    type: string
    required: true
steps:
  - id: step1
    name: Test
    type: llm
    prompt: "test"
outputs: []
`

	req := httptest.NewRequest("POST", "/v1/runs?message=hello", bytes.NewBufferString(workflow))
	req.Header.Set("Content-Type", "application/x-yaml")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("got status %d, want %d. Body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	inputs, ok := result["inputs"].(map[string]any)
	if !ok {
		t.Fatal("inputs not found in response")
	}

	if inputs["message"] != "hello" {
		t.Errorf("got message %q, want 'hello'", inputs["message"])
	}
}

// TestConcurrentAPIAccess tests concurrent API access to verify no race conditions.
func TestConcurrentAPIAccess(t *testing.T) {
	mux, _ := setupTestServer(t)

	workflow := []byte(`name: concurrent-test
version: "1.0"
description: Test concurrent API access
inputs: []
steps:
  - id: step1
    name: Test
    type: llm
    prompt: "test"
outputs: []
`)

	// Submit multiple runs concurrently
	var wg sync.WaitGroup
	runIDs := make(chan string, 10)

	// Create runs
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/v1/runs", bytes.NewReader(workflow))
			req.Header.Set("Content-Type", "application/x-yaml")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code == http.StatusAccepted {
				var result map[string]any
				if err := json.NewDecoder(rec.Body).Decode(&result); err == nil {
					if id, ok := result["id"].(string); ok {
						runIDs <- id
					}
				}
			}
		}()
	}

	wg.Wait()
	close(runIDs)

	// Collect run IDs
	var ids []string
	for id := range runIDs {
		ids = append(ids, id)
	}

	// Concurrently access the created runs
	for _, id := range ids {
		// Get
		wg.Add(1)
		go func(runID string) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/v1/runs/"+runID, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
		}(id)

		// Get logs
		wg.Add(1)
		go func(runID string) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/v1/runs/"+runID+"/logs", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
		}(id)

		// Cancel
		wg.Add(1)
		go func(runID string) {
			defer wg.Done()
			req := httptest.NewRequest("DELETE", "/v1/runs/"+runID, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
		}(id)
	}

	// List
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/v1/runs", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
		}()
	}

	wg.Wait()
}
