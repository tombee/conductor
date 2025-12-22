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

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/daemon/backend/memory"
	"github.com/tombee/conductor/internal/daemon/checkpoint"
	"github.com/tombee/conductor/internal/daemon/runner"
)

func setupTestRouter(t *testing.T, routes []Route) (*http.ServeMux, string) {
	t.Helper()

	// Create temp directory for workflows
	tmpDir, err := os.MkdirTemp("", "webhook-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		// Brief delay to allow runner goroutines to release file handles.
		// The runner doesn't have a synchronous shutdown method, so this
		// small sleep prevents intermittent "directory not empty" errors.
		time.Sleep(10 * time.Millisecond)
		os.RemoveAll(tmpDir)
	})

	// Create a test workflow
	testWorkflow := `name: test-webhook-workflow
version: "1.0"
description: Test workflow for webhooks
inputs: []
steps:
  - id: step1
    name: Test Step
    type: llm
    prompt: "test"
outputs: []
`
	err = os.WriteFile(filepath.Join(tmpDir, "test-workflow.yaml"), []byte(testWorkflow), 0644)
	if err != nil {
		t.Fatalf("Failed to create test workflow: %v", err)
	}

	// Create checkpoint directory
	cpDir, err := os.MkdirTemp("", "webhook-cp-*")
	if err != nil {
		t.Fatalf("Failed to create checkpoint dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(cpDir)
	})

	be := memory.New()
	cm, err := checkpoint.NewManager(checkpoint.ManagerConfig{
		Dir: cpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create checkpoint manager: %v", err)
	}

	r := runner.New(runner.Config{
		MaxParallel:    2,
		DefaultTimeout: 30 * time.Second,
	}, be, cm)

	router := NewRouter(Config{
		Routes:       routes,
		WorkflowsDir: tmpDir,
	}, r)

	mux := http.NewServeMux()
	router.RegisterRoutes(mux)

	return mux, tmpDir
}

func TestWebhookRouter_GenericWebhook(t *testing.T) {
	routes := []Route{
		{
			Path:     "/webhooks/test",
			Source:   "generic",
			Workflow: "test-workflow",
		},
	}
	mux, _ := setupTestRouter(t, routes)

	payload := `{"message": "hello", "value": 42}`
	req := httptest.NewRequest("POST", "/webhooks/test", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("got status %d, want %d. Body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "triggered" {
		t.Errorf("got status %q, want 'triggered'", result["status"])
	}
	if result["run_id"] == nil {
		t.Error("expected run_id in response")
	}
}

func TestWebhookRouter_DynamicWebhook(t *testing.T) {
	mux, _ := setupTestRouter(t, nil)

	payload := `{"action": "created", "data": "test"}`
	req := httptest.NewRequest("POST", "/webhooks/generic/test-workflow", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("got status %d, want %d. Body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "triggered" {
		t.Errorf("got status %q, want 'triggered'", result["status"])
	}
}

func TestWebhookRouter_WorkflowNotFound(t *testing.T) {
	routes := []Route{
		{
			Path:     "/webhooks/missing",
			Source:   "generic",
			Workflow: "nonexistent-workflow",
		},
	}
	mux, _ := setupTestRouter(t, routes)

	payload := `{"test": true}`
	req := httptest.NewRequest("POST", "/webhooks/missing", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d. Body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestWebhookRouter_EventFiltering(t *testing.T) {
	routes := []Route{
		{
			Path:     "/webhooks/events",
			Source:   "generic",
			Workflow: "test-workflow",
			Events:   []string{"push", "pull_request"},
		},
	}
	mux, _ := setupTestRouter(t, routes)

	tests := []struct {
		name       string
		event      string
		wantStatus int
		wantResult string
	}{
		{
			name:       "matching event",
			event:      "push",
			wantStatus: http.StatusAccepted,
			wantResult: "triggered",
		},
		{
			name:       "non-matching event",
			event:      "issues",
			wantStatus: http.StatusOK,
			wantResult: "ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := `{"test": true}`
			req := httptest.NewRequest("POST", "/webhooks/events", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Event-Type", tt.event)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d. Body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			var result map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if result["status"] != tt.wantResult {
				t.Errorf("got status %q, want %q", result["status"], tt.wantResult)
			}
		})
	}
}

func TestWebhookRouter_InputMapping(t *testing.T) {
	routes := []Route{
		{
			Path:     "/webhooks/mapping",
			Source:   "generic",
			Workflow: "test-workflow",
			InputMapping: map[string]string{
				"repo_name":   "$.repository.name",
				"sender_name": "$.sender.login",
				"static_val":  "literal_value",
			},
		},
	}
	mux, _ := setupTestRouter(t, routes)

	payload := `{
		"repository": {"name": "test-repo", "id": 123},
		"sender": {"login": "testuser", "id": 456}
	}`
	req := httptest.NewRequest("POST", "/webhooks/mapping", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("got status %d, want %d. Body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "triggered" {
		t.Errorf("got status %q, want 'triggered'", result["status"])
	}
}

func TestWebhookRouter_GitHubSignatureVerification(t *testing.T) {
	secret := "test-secret-key"
	routes := []Route{
		{
			Path:     "/webhooks/github",
			Source:   "github",
			Workflow: "test-workflow",
			Secret:   secret,
		},
	}
	mux, _ := setupTestRouter(t, routes)

	payload := `{"action": "opened", "number": 1}`

	// Calculate HMAC signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name       string
		signature  string
		wantStatus int
	}{
		{
			name:       "valid signature",
			signature:  signature,
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "invalid signature",
			signature:  "sha256=invalid",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing signature",
			signature:  "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			if tt.signature != "" {
				req.Header.Set("X-Hub-Signature-256", tt.signature)
			}

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d. Body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestWebhookRouter_SlackSignatureVerification(t *testing.T) {
	secret := "slack-signing-secret"
	routes := []Route{
		{
			Path:     "/webhooks/slack",
			Source:   "slack",
			Workflow: "test-workflow",
			Secret:   secret,
		},
	}
	mux, _ := setupTestRouter(t, routes)

	payload := `{"type": "event_callback", "event": {"type": "message"}}`
	// Use current timestamp to pass the 5-minute validation window
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Calculate Slack signature
	sigBaseString := "v0:" + timestamp + ":" + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigBaseString))
	validSignature := "v0=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name       string
		timestamp  string
		signature  string
		wantStatus int
	}{
		{
			name:       "valid signature",
			timestamp:  timestamp,
			signature:  validSignature,
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "invalid signature",
			timestamp:  timestamp,
			signature:  "v0=invalid",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/webhooks/slack", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Slack-Request-Timestamp", tt.timestamp)
			req.Header.Set("X-Slack-Signature", tt.signature)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d. Body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestWebhookRouter_DirectoryTraversal(t *testing.T) {
	mux, _ := setupTestRouter(t, nil)

	// Attempt directory traversal through dynamic webhook
	req := httptest.NewRequest("POST", "/webhooks/generic/../../../etc/passwd", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Should get 404 (not found) or the path should be cleaned and fail
	if rec.Code == http.StatusOK || rec.Code == http.StatusAccepted {
		t.Errorf("directory traversal should be blocked, got status %d", rec.Code)
	}
}

func TestEvaluateExpression(t *testing.T) {
	payload := map[string]any{
		"repository": map[string]any{
			"name":  "test-repo",
			"owner": map[string]any{"login": "testuser"},
		},
		"action": "opened",
	}

	tests := []struct {
		expr string
		want any
	}{
		{"$.action", "opened"},
		{"$.repository.name", "test-repo"},
		{"$.repository.owner.login", "testuser"},
		{"$.missing", nil},
		{"$.repository.missing", nil},
		{"literal_value", "literal_value"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := evaluateExpression(tt.expr, payload)
			if got != tt.want {
				t.Errorf("evaluateExpression(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}
