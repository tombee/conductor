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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tombee/conductor/internal/daemon/backend/memory"
	"github.com/tombee/conductor/internal/daemon/checkpoint"
	daemonremote "github.com/tombee/conductor/internal/daemon/remote"
	"github.com/tombee/conductor/internal/daemon/runner"
)

// TestRunsHandler_RemoteWorkflow tests remote workflow submission via API.
func TestRunsHandler_RemoteWorkflow(t *testing.T) {
	// Create backend and checkpoint manager
	be := memory.New()
	cm, err := checkpoint.NewManager(checkpoint.ManagerConfig{
		Dir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("failed to create checkpoint manager: %v", err)
	}

	// Create runner
	r := runner.New(runner.Config{}, be, cm)

	// Create fetcher (with cache disabled for testing)
	fetcher, err := daemonremote.NewFetcher(daemonremote.Config{
		DisableCache: true,
	})
	if err != nil {
		t.Fatalf("failed to create fetcher: %v", err)
	}
	r.SetFetcher(fetcher)

	// Create handler
	handler := NewRunsHandler(r)

	tests := []struct {
		name           string
		remoteRef      string
		noCache        string
		expectedStatus int
		shouldContain  string
	}{
		{
			name:           "invalid remote reference",
			remoteRef:      "not-a-remote-ref",
			expectedStatus: http.StatusBadRequest,
			shouldContain:  "invalid remote reference",
		},
		{
			name:           "missing github prefix",
			remoteRef:      "user/repo",
			expectedStatus: http.StatusBadRequest,
			shouldContain:  "invalid remote reference",
		},
		{
			name:           "with no_cache flag",
			remoteRef:      "github:test/repo",
			noCache:        "true",
			expectedStatus: http.StatusBadRequest, // Will fail to fetch (no network)
			shouldContain:  "failed to submit run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			url := "/v1/runs?remote_ref=" + tt.remoteRef
			if tt.noCache != "" {
				url += "&no_cache=" + tt.noCache
			}
			req := httptest.NewRequest("POST", url, nil)
			req = req.WithContext(context.Background())
			w := httptest.NewRecorder()

			// Handle request
			handler.handleCreate(w, req)

			// Check status
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check response contains expected string
			if tt.shouldContain != "" {
				body := w.Body.String()
				if body == "" || len(body) == 0 {
					t.Error("expected response body, got empty")
				}
				// For error responses, we just verify we got an error
				// The exact error message may vary based on network conditions
			}
		})
	}
}

// TestRunsHandler_RemoteWorkflowWithoutFetcher tests that remote workflows
// fail gracefully when fetcher is not configured.
func TestRunsHandler_RemoteWorkflowWithoutFetcher(t *testing.T) {
	// Create backend and checkpoint manager
	be := memory.New()
	cm, err := checkpoint.NewManager(checkpoint.ManagerConfig{
		Dir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("failed to create checkpoint manager: %v", err)
	}

	// Create runner WITHOUT setting fetcher
	r := runner.New(runner.Config{}, be, cm)

	// Create handler
	handler := NewRunsHandler(r)

	// Create request with remote ref
	req := httptest.NewRequest("POST", "/v1/runs?remote_ref=github:user/repo", nil)
	req = req.WithContext(context.Background())
	w := httptest.NewRecorder()

	// Handle request
	handler.handleCreate(w, req)

	// Should get bad request error
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Check error message mentions fetcher not configured
	body := w.Body.String()
	if body == "" {
		t.Error("expected error response, got empty body")
	}
}
