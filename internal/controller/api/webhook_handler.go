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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tombee/conductor/internal/controller/runner"
	"github.com/tombee/conductor/internal/controller/webhook"
	"github.com/tombee/conductor/pkg/workflow"
)

const maxWebhookBodySize = 10 * 1024 * 1024 // 10MB

// WebhookHandler handles webhook requests for the public API.
// Unlike the control plane webhook handler, this enforces per-workflow
// signature verification based on listen.webhook.secret configuration.
type WebhookHandler struct {
	runner       *runner.Runner
	workflowsDir string
	handlers     map[string]webhook.Handler
}

// NewWebhookHandler creates a new webhook handler for the public API.
func NewWebhookHandler(r *runner.Runner, workflowsDir string) *WebhookHandler {
	return &WebhookHandler{
		runner:       r,
		workflowsDir: workflowsDir,
		handlers: map[string]webhook.Handler{
			"github":  &webhook.GitHubHandler{},
			"slack":   &webhook.SlackHandler{},
			"generic": &webhook.GenericHandler{},
		},
	}
}

// RegisterRoutes registers webhook routes on the public API mux.
func (h *WebhookHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /webhooks/{source}/{workflow}", h.handleWebhook)
}

// handleWebhook handles POST /webhooks/{source}/{workflow}.
// Loads the workflow, verifies it has listen.webhook configured,
// verifies the signature, and triggers the workflow.
func (h *WebhookHandler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Check if runner is draining
	if h.runner.IsDraining() {
		w.Header().Set("Retry-After", "10")
		writeError(w, http.StatusServiceUnavailable, "daemon is shutting down gracefully")
		return
	}

	source := r.PathValue("source")
	workflowName := r.PathValue("workflow")

	if source == "" || workflowName == "" {
		writeError(w, http.StatusBadRequest, "source and workflow required")
		return
	}

	// Clean workflow name to prevent directory traversal
	workflowName = filepath.Clean(workflowName)
	if strings.Contains(workflowName, "..") {
		writeError(w, http.StatusBadRequest, "invalid workflow name")
		return
	}

	// Get handler for source type
	handler, ok := h.handlers[source]
	if !ok {
		handler = h.handlers["generic"]
	}

	// Read body (limit to 10MB for webhooks)
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	// Find and load workflow
	workflowPath, err := h.findWorkflow(workflowName)
	if err != nil {
		// Return 404 to prevent workflow enumeration
		writeError(w, http.StatusNotFound, "webhook not found or not available")
		return
	}

	workflowYAML, err := os.ReadFile(workflowPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "webhook not found or not available")
		return
	}

	// Parse workflow definition
	def, err := workflow.ParseDefinition(workflowYAML)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse workflow")
		return
	}

	// Verify workflow has listen.webhook configured
	if def.Trigger == nil || def.Trigger.Webhook == nil {
		// Return 404 to prevent enumeration of workflows without webhook listeners
		writeError(w, http.StatusNotFound, "webhook not found or not available")
		return
	}

	webhookConfig := def.Trigger.Webhook

	// Expand secret from environment if needed
	secret := webhookConfig.Secret
	if strings.HasPrefix(secret, "${") && strings.HasSuffix(secret, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(secret, "${"), "}")
		secret = os.Getenv(envVar)
	}

	// Verify signature if secret is configured
	if secret != "" {
		if err := handler.Verify(r, body, secret); err != nil {
			writeError(w, http.StatusUnauthorized, "webhook signature verification failed")
			return
		}
	}

	// Parse event type
	event := handler.ParseEvent(r)

	// Check if event is allowed (if events filter is configured)
	if len(webhookConfig.Events) > 0 && !contains(webhookConfig.Events, event) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ignored",
			"message": fmt.Sprintf("event %s not configured for this webhook", event),
		})
		return
	}

	// Extract payload
	payload, err := handler.ExtractPayload(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse payload: %v", err))
		return
	}

	// Create inputs from payload
	inputs := map[string]any{
		"_event":   event,
		"_source":  source,
		"_payload": payload,
	}

	// Apply input mapping if configured
	if webhookConfig.InputMapping != nil {
		for key, mapping := range webhookConfig.InputMapping {
			// Simple mapping support - could be extended with JSONPath
			if val, ok := payload[mapping]; ok {
				inputs[key] = val
			}
		}
	} else {
		// Without mapping, flatten top-level payload fields
		for k, v := range payload {
			inputs[k] = v
		}
	}

	// Submit workflow
	run, err := h.runner.Submit(r.Context(), runner.SubmitRequest{
		WorkflowYAML: workflowYAML,
		Inputs:       inputs,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to trigger workflow: %v", err))
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":   "triggered",
		"run_id":   run.ID,
		"workflow": run.Workflow,
		"event":    event,
	})
}

// findWorkflow looks for a workflow file by name.
func (h *WebhookHandler) findWorkflow(name string) (string, error) {
	// Try various extensions and locations
	extensions := []string{".yaml", ".yml", ""}
	baseDirs := []string{h.workflowsDir, "."}

	for _, baseDir := range baseDirs {
		if baseDir == "" {
			continue
		}
		for _, ext := range extensions {
			path := filepath.Join(baseDir, name+ext)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("workflow not found: %s", name)
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
