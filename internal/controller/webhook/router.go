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

// Package webhook provides webhook handling for external triggers.
package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tombee/conductor/internal/controller/runner"
)

// Route defines a webhook route mapping.
type Route struct {
	// Path is the URL path to match (e.g., "/webhooks/github")
	Path string `yaml:"path" json:"path"`

	// Source is the webhook source type (github, slack, generic)
	Source string `yaml:"source" json:"source"`

	// Workflow is the workflow to trigger
	Workflow string `yaml:"workflow" json:"workflow"`

	// Events limits which events trigger the workflow (source-specific)
	Events []string `yaml:"events,omitempty" json:"events,omitempty"`

	// Secret is used for signature verification
	Secret string `yaml:"secret,omitempty" json:"secret,omitempty"`

	// InputMapping defines how to map webhook payload to workflow inputs
	InputMapping map[string]string `yaml:"input_mapping,omitempty" json:"input_mapping,omitempty"`
}

// Config contains webhook router configuration.
type Config struct {
	Routes       []Route `yaml:"routes" json:"routes"`
	WorkflowsDir string  `yaml:"workflows_dir" json:"workflows_dir"`
}

// Router routes incoming webhooks to workflows.
type Router struct {
	routes       []Route
	runner       *runner.Runner
	workflowsDir string
	handlers     map[string]Handler
	logger       *slog.Logger
}

// Handler processes webhooks for a specific source type.
type Handler interface {
	// Verify verifies the webhook signature.
	Verify(r *http.Request, body []byte, secret string) error

	// ParseEvent parses the event type from the request.
	ParseEvent(r *http.Request) string

	// ExtractPayload extracts the payload as a map.
	ExtractPayload(body []byte) (map[string]any, error)
}

// NewRouter creates a new webhook router.
func NewRouter(cfg Config, r *runner.Runner) *Router {
	router := &Router{
		routes:       cfg.Routes,
		runner:       r,
		workflowsDir: cfg.WorkflowsDir,
		handlers:     make(map[string]Handler),
		logger:       slog.Default().With(slog.String("component", "webhook")),
	}

	// Register built-in handlers
	router.handlers["github"] = &GitHubHandler{}
	router.handlers["slack"] = &SlackHandler{}
	router.handlers["generic"] = &GenericHandler{}

	return router
}

// RegisterRoutes registers webhook routes on the given mux.
func (router *Router) RegisterRoutes(mux *http.ServeMux) {
	for _, route := range router.routes {
		route := route // capture for closure
		mux.HandleFunc("POST "+route.Path, func(w http.ResponseWriter, r *http.Request) {
			router.handleWebhook(w, r, route)
		})
	}

	// Also register a catch-all for dynamic webhooks
	mux.HandleFunc("POST /webhooks/{source}/{workflow}", router.handleDynamicWebhook)
}

// handleWebhook handles a webhook request for a specific route.
func (router *Router) handleWebhook(w http.ResponseWriter, r *http.Request, route Route) {
	// Check if runner is draining (graceful shutdown in progress)
	if router.runner.IsDraining() {
		w.Header().Set("Retry-After", "10")
		writeError(w, http.StatusServiceUnavailable, "daemon is shutting down gracefully")
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	// Get handler for source
	handler, ok := router.handlers[route.Source]
	if !ok {
		handler = router.handlers["generic"]
	}

	// Verify signature if secret is configured
	if route.Secret != "" {
		if err := handler.Verify(r, body, route.Secret); err != nil {
			router.logger.Warn("Webhook signature verification failed",
				slog.String("path", route.Path),
				slog.String("source", route.Source),
				slog.Any("error", err),
			)
			writeError(w, http.StatusUnauthorized, "signature verification failed")
			return
		}
	}

	// Parse event type
	event := handler.ParseEvent(r)

	// Check if this event should trigger the workflow
	if len(route.Events) > 0 && !contains(route.Events, event) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ignored",
			"message": fmt.Sprintf("Event '%s' not in configured events", event),
		})
		return
	}

	// Extract payload
	payload, err := handler.ExtractPayload(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse payload: %v", err))
		return
	}

	// Map inputs
	inputs := router.mapInputs(payload, route.InputMapping, event)

	// Load and trigger workflow
	workflowPath, err := router.findWorkflow(route.Workflow)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("workflow not found: %s", route.Workflow))
		return
	}

	workflowYAML, err := os.ReadFile(workflowPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read workflow: %v", err))
		return
	}

	run, err := router.runner.Submit(r.Context(), runner.SubmitRequest{
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

// handleDynamicWebhook handles webhooks to /webhooks/{source}/{workflow}
func (router *Router) handleDynamicWebhook(w http.ResponseWriter, r *http.Request) {
	// Check if runner is draining (graceful shutdown in progress)
	if router.runner.IsDraining() {
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

	// Get handler for source
	handler, ok := router.handlers[source]
	if !ok {
		handler = router.handlers["generic"]
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	// Parse event
	event := handler.ParseEvent(r)

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

	// Merge top-level payload fields
	for k, v := range payload {
		inputs[k] = v
	}

	// Load and trigger workflow
	workflowPath, err := router.findWorkflow(workflowName)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("workflow not found: %s", workflowName))
		return
	}

	workflowYAML, err := os.ReadFile(workflowPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read workflow: %v", err))
		return
	}

	run, err := router.runner.Submit(r.Context(), runner.SubmitRequest{
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

// mapInputs maps webhook payload to workflow inputs using the mapping.
func (router *Router) mapInputs(payload map[string]any, mapping map[string]string, event string) map[string]any {
	inputs := map[string]any{
		"_event":   event,
		"_payload": payload,
	}

	if mapping == nil {
		// Without mapping, flatten top-level payload into inputs
		for k, v := range payload {
			inputs[k] = v
		}
		return inputs
	}

	// Apply mapping
	for inputName, expr := range mapping {
		value := evaluateExpression(expr, payload)
		if value != nil {
			inputs[inputName] = value
		}
	}

	return inputs
}

// evaluateExpression evaluates a simple JSONPath-like expression.
// Supports: $.field, $.nested.field, literal values
func evaluateExpression(expr string, payload map[string]any) any {
	if !strings.HasPrefix(expr, "$") {
		// Literal value
		return expr
	}

	// Simple JSONPath: $.field.subfield
	path := strings.TrimPrefix(expr, "$.")
	parts := strings.Split(path, ".")

	var current any = payload
	for _, part := range parts {
		if part == "" {
			continue
		}
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
	}

	return current
}

// findWorkflow finds a workflow file by name.
func (router *Router) findWorkflow(name string) (string, error) {
	// Security: prevent directory traversal
	name = filepath.Clean(name)
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid workflow name")
	}

	extensions := []string{".yaml", ".yml", ""}
	baseDirs := []string{router.workflowsDir, "."}

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

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
