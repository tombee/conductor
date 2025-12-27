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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/daemon/runner"
)

func TestStartHandler_HandleStart(t *testing.T) {
	// Create temp directory for test workflows
	tmpDir := t.TempDir()

	// Create test workflow with listen.api
	workflowWithAPI := `
name: test-workflow
listen:
  api:
    secret: "test-secret-token-12345678901234567890"
steps:
  - id: step1
    type: llm
    prompt: "Say hello"
`
	workflowPath := filepath.Join(tmpDir, "test-workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowWithAPI), 0644); err != nil {
		t.Fatalf("Failed to write test workflow: %v", err)
	}

	// Create workflow without listen.api
	workflowWithoutAPI := `
name: no-api-workflow
steps:
  - id: step1
    type: llm
    prompt: "Say hello"
`
	noAPIPath := filepath.Join(tmpDir, "no-api.yaml")
	if err := os.WriteFile(noAPIPath, []byte(workflowWithoutAPI), 0644); err != nil {
		t.Fatalf("Failed to write no-api workflow: %v", err)
	}

	// Create workflow with env var secret
	os.Setenv("TEST_API_SECRET", "env-secret-token-12345678901234567890")
	defer os.Unsetenv("TEST_API_SECRET")

	workflowWithEnvSecret := `
name: env-secret-workflow
listen:
  api:
    secret: "${TEST_API_SECRET}"
steps:
  - id: step1
    type: llm
    prompt: "Say hello"
`
	envSecretPath := filepath.Join(tmpDir, "env-secret.yaml")
	if err := os.WriteFile(envSecretPath, []byte(workflowWithEnvSecret), 0644); err != nil {
		t.Fatalf("Failed to write env-secret workflow: %v", err)
	}

	// Create runner
	r := runner.New(runner.Config{
		MaxParallel:    1,
		DefaultTimeout: 30 * time.Second,
	}, nil, nil)

	handler := NewStartHandler(r, tmpDir)

	tests := []struct {
		name           string
		workflow       string
		authHeader     string
		body           string
		wantStatus     int
		wantErrMessage string
	}{
		{
			name:       "valid request with correct token",
			workflow:   "test-workflow",
			authHeader: "Bearer test-secret-token-12345678901234567890",
			body:       `{"key":"value"}`,
			wantStatus: http.StatusAccepted,
		},
		{
			name:           "missing authorization header",
			workflow:       "test-workflow",
			authHeader:     "",
			body:           `{}`,
			wantStatus:     http.StatusUnauthorized,
			wantErrMessage: "missing Authorization header",
		},
		{
			name:           "invalid authorization header format",
			workflow:       "test-workflow",
			authHeader:     "Basic dGVzdDp0ZXN0",
			body:           `{}`,
			wantStatus:     http.StatusUnauthorized,
			wantErrMessage: "invalid Authorization header format",
		},
		{
			name:           "empty bearer token",
			workflow:       "test-workflow",
			authHeader:     "Bearer ",
			body:           `{}`,
			wantStatus:     http.StatusUnauthorized,
			wantErrMessage: "empty Bearer token",
		},
		{
			name:           "invalid bearer token",
			workflow:       "test-workflow",
			authHeader:     "Bearer wrong-token",
			body:           `{}`,
			wantStatus:     http.StatusUnauthorized,
			wantErrMessage: "invalid Bearer token",
		},
		{
			name:           "workflow without listen.api",
			workflow:       "no-api",
			authHeader:     "Bearer test-secret-token-12345678901234567890",
			body:           `{}`,
			wantStatus:     http.StatusNotFound,
			wantErrMessage: "workflow not found or not available via API",
		},
		{
			name:           "workflow not found",
			workflow:       "nonexistent",
			authHeader:     "Bearer test-secret-token-12345678901234567890",
			body:           `{}`,
			wantStatus:     http.StatusNotFound,
			wantErrMessage: "workflow not found or not available via API",
		},
		{
			name:           "invalid JSON body",
			workflow:       "test-workflow",
			authHeader:     "Bearer test-secret-token-12345678901234567890",
			body:           `{invalid json}`,
			wantStatus:     http.StatusBadRequest,
			wantErrMessage: "invalid JSON",
		},
		{
			name:       "env var secret expansion",
			workflow:   "env-secret",
			authHeader: "Bearer env-secret-token-12345678901234567890",
			body:       `{}`,
			wantStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			handler.RegisterRoutes(mux)

			req := httptest.NewRequest("POST", "/v1/start/"+tt.workflow, bytes.NewBufferString(tt.body))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Status = %d, want %d. Body: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantErrMessage != "" {
				var resp map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if errMsg, ok := resp["error"].(string); !ok || !strings.Contains(errMsg, tt.wantErrMessage) {
					t.Errorf("Error message = %q, want to contain %q", errMsg, tt.wantErrMessage)
				}
			}

			if tt.wantStatus == http.StatusAccepted {
				var resp map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if _, ok := resp["id"]; !ok {
					t.Error("Response missing 'id' field")
				}
				if status, ok := resp["status"].(string); !ok || status == "" {
					t.Error("Response missing or invalid 'status' field")
				}
			}
		})
	}
}

func TestStartHandler_RequestBodySizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	workflow := `
name: test-workflow
listen:
  api:
    secret: "test-secret-token-12345678901234567890"
steps:
  - id: step1
    type: llm
    prompt: "Say hello"
`
	workflowPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflow), 0644); err != nil {
		t.Fatalf("Failed to write test workflow: %v", err)
	}

	r := runner.New(runner.Config{
		MaxParallel:    1,
		DefaultTimeout: 30 * time.Second,
	}, nil, nil)

	handler := NewStartHandler(r, tmpDir)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create a body larger than 1MB
	largeBody := bytes.Repeat([]byte("x"), 2*1024*1024)

	req := httptest.NewRequest("POST", "/v1/start/test", bytes.NewBuffer(largeBody))
	req.Header.Set("Authorization", "Bearer test-secret-token-12345678901234567890")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestStartHandler_Draining(t *testing.T) {
	tmpDir := t.TempDir()

	workflow := `
name: test-workflow
listen:
  api:
    secret: "test-secret-token-12345678901234567890"
steps:
  - id: step1
    type: llm
    prompt: "Say hello"
`
	workflowPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflow), 0644); err != nil {
		t.Fatalf("Failed to write test workflow: %v", err)
	}

	r := runner.New(runner.Config{
		MaxParallel:    1,
		DefaultTimeout: 30 * time.Second,
	}, nil, nil)

	handler := NewStartHandler(r, tmpDir)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Start draining
	r.StartDraining()

	// Give it a moment to enter drain mode
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest("POST", "/v1/start/test", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer test-secret-token-12345678901234567890")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	if w.Header().Get("Retry-After") != "10" {
		t.Errorf("Retry-After = %q, want %q", w.Header().Get("Retry-After"), "10")
	}
}
